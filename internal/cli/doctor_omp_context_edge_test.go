package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckOMPContextDoctor_ActiveFailureReasonsAndEffectiveDowngrade(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name   string
		reason string
		mutate func(*ompContextDoctorInput)
	}{
		{"identity", "identity_unverified", func(i *ompContextDoctorInput) { i.Current.IdentityVerified = false }},
		{"readback", "config_readback_unproved", func(i *ompContextDoctorInput) { i.Current.Reason = "config_readback_unproved" }},
		{"overlay current", "overlay_readback_unproved", func(i *ompContextDoctorInput) { i.Current.OverlayReadback = false }},
		{"missing", "receipt_missing", func(i *ompContextDoctorInput) {
			i.Receipt = ompContextDoctorReceiptState{Status: "missing", Freshness: "missing"}
		}},
		{"invalid", "receipt_invalid", func(i *ompContextDoctorInput) { i.Receipt.Status = "invalid" }},
		{"stale", "receipt_stale", func(i *ompContextDoctorInput) { i.Receipt.Freshness = "stale" }},
		{"version", "version_stale", func(i *ompContextDoctorInput) { i.Receipt.Receipt.Capabilities.Version = "omp/17.1.7" }},
		{"source", "installed_lifecycle_unproved", func(i *ompContextDoctorInput) { i.Receipt.Receipt.Capabilities.ProbeSource = "fake-runtime" }},
		{"mode", "mode_readback_mismatch", func(i *ompContextDoctorInput) {
			i.Receipt.Receipt.Mode.EffectiveHistoryMode = config.OMPContextHistoryShadow
		}},
		{"phase", "installed_lifecycle_unproved", func(i *ompContextDoctorInput) { i.Receipt.Receipt.PhaseSequence = []string{"checkpointed", "admitted"} }},
		{"event", "installed_lifecycle_unproved", func(i *ompContextDoctorInput) { i.Receipt.Receipt.Capabilities.CanonicalInjection = false }},
		{"persistence", "persistence_control_unproved", func(i *ompContextDoctorInput) { i.Receipt.Receipt.Capabilities.IsolatedTaskRoot = false }},
		{"cleanup", "cleanup_unproved", func(i *ompContextDoctorInput) { i.Receipt.Receipt.ArtifactCounts.AfterCleanup = 1 }},
		{"overlay receipt", "overlay_readback_unproved", func(i *ompContextDoctorInput) { i.Receipt.Receipt.Mode.ReadbackHash = "" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := validOMPContextDoctorInput(now)
			tt.mutate(&input)
			report := checkOMPContextDoctor(input)
			assert.Equal(t, "blocked", report.Status)
			assert.Equal(t, tt.reason, report.Reason)
			assert.Equal(t, config.OMPContextHistoryShadow, report.EffectiveHistoryMode)
		})
	}
}

func TestCheckOMPContextDoctor_ShadowAndOffModeStatusContracts(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	shadow := validOMPContextDoctorInput(now)
	shadow.RequestedHistoryMode = config.OMPContextHistoryShadow
	shadow.Receipt = ompContextDoctorReceiptState{Status: "missing", Freshness: "missing"}
	report := checkOMPContextDoctor(shadow)
	assert.Equal(t, "degraded", report.Status)
	assert.Equal(t, "receipt_missing", report.Reason)
	assert.Equal(t, config.OMPContextHistoryShadow, report.EffectiveHistoryMode)

	off := validOMPContextDoctorInput(now)
	off.RequestedHistoryMode = config.OMPContextHistoryOff
	off.RequestedMemoryMode = config.OMPContextMemoryOff
	off.Receipt = ompContextDoctorReceiptState{Status: "missing", Freshness: "missing"}
	report = checkOMPContextDoctor(off)
	assert.Equal(t, "supported", report.Status)
	assert.Equal(t, config.OMPContextHistoryOff, report.EffectiveHistoryMode)
	assert.Equal(t, config.OMPContextMemoryOff, report.EffectiveMemoryMode)
}

func TestCheckOMPContextDoctor_MemoryProvenanceIsSeparateFromInterception(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	input := validOMPContextDoctorInput(now)
	input.RequestedHistoryMode = config.OMPContextHistoryOff
	input.RequestedMemoryMode = config.OMPContextMemoryShadow
	input.Receipt.Receipt.Capabilities.ProbeSource = "fake-runtime"
	report := checkOMPContextDoctor(input)
	assert.True(t, report.MemoryInterception)
	assert.False(t, report.MemoryProvenance)
	assert.False(t, report.MemoryActiveInjection)
	assert.Equal(t, config.OMPContextMemoryOff, report.EffectiveMemoryMode)
	assert.Equal(t, "memory_provenance_unproved", report.Reason)
}

func TestCheckOMPContextDoctor_NoSessionPersistenceIsProved(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	input := validOMPContextDoctorInput(now)
	input.RuntimeRootPolicy = config.OMPContextRuntimeNoSession
	input.Receipt.Receipt.RootClass = config.OMPContextRuntimeNoSession
	input.Receipt.Receipt.Capabilities.IsolatedTaskRoot = false
	input.Receipt.Receipt.Capabilities.NoSession = true
	report := checkOMPContextDoctor(input)
	assert.Equal(t, "supported", report.Status)
	row := ompContextDoctorCapabilityByID(t, report, "persistence.no_session")
	assert.True(t, row.Supported)
	assert.Equal(t, "no_session_proved", row.Reason)
}

func TestReadOMPContextDoctorReceipt_RejectsUnknownTrailingAndDirectoryMode(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	for _, suffix := range []string{`,{"extra":true}`, ` {"extra":true}`} {
		root := t.TempDir()
		receipt := newValidOMPContextDoctorReceiptFixture(now)
		path := writeOMPContextDoctorReceiptFixture(t, root, receipt)
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		data = append(data[:len(data)-1], []byte(suffix)...)
		require.NoError(t, os.WriteFile(path, data, 0o600))
		assert.Equal(t, "invalid", readOMPContextDoctorReceipt(root, now).Status)
	}

	root := t.TempDir()
	path := writeOMPContextDoctorReceiptFixture(t, root, newValidOMPContextDoctorReceiptFixture(now))
	require.NoError(t, os.Chmod(filepath.Dir(path), 0o755))
	assert.Equal(t, "invalid", readOMPContextDoctorReceipt(root, now).Status)
}

func TestReadOMPContextDoctorReceipt_RejectsUnknownJSONField(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	root := t.TempDir()
	receipt := newValidOMPContextDoctorReceiptFixture(now)
	data, err := json.Marshal(receipt)
	require.NoError(t, err)
	data = append(data[:len(data)-1], []byte(`,"raw_body":"raw-body"}`)...)
	path := filepath.Join(root, filepath.FromSlash(WorkflowContextReceiptRelativePath(receipt.TaskID, receipt.SessionID)))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, data, 0o600))
	assert.Equal(t, "invalid", readOMPContextDoctorReceipt(root, now).Status)
}

func TestReadOMPContextDoctorReceipt_EqualTimestampSelectionIsDeterministic(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	root := t.TempDir()
	second := newValidOMPContextDoctorReceiptFixture(now)
	second.TaskID, second.SessionID = "TASK-B", "session-b"
	second.RootClass = config.OMPContextRuntimeIsolatedTaskOwned
	writeOMPContextDoctorReceiptFixture(t, root, second)
	first := newValidOMPContextDoctorReceiptFixture(now)
	first.TaskID, first.SessionID = "TASK-A", "session-a"
	first.RootClass = config.OMPContextRuntimeNoSession
	writeOMPContextDoctorReceiptFixture(t, root, first)

	state := readOMPContextDoctorReceipt(root, now)
	assert.Equal(t, "valid", state.Status)
	assert.Equal(t, config.OMPContextRuntimeNoSession, state.Receipt.RootClass)
}

func TestInspectOMPContextConfigList_StrictShapes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		json string
		ok   bool
	}{
		{`{"compaction":true,"memory":false}`, true},
		{`{"items":[{"key":"compaction","value":true},{"key":"memory","value":false}]}`, true},
		{`{"compaction":"raw-body","memory":false}`, false},
		{`{"compaction":true,"memory":null}`, false},
		{`[]`, false},
		{`{} {}`, false},
	}
	for _, tt := range tests {
		valid, _, _ := inspectOMPContextConfigList([]byte(tt.json))
		assert.Equal(t, tt.ok, valid, tt.json)
	}
}
