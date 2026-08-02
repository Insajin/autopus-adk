package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppendOMPContextDoctorChecks_NoOptInPreservesExactCheckSet(t *testing.T) {
	t.Parallel()

	existing := []jsonCheck{{ID: "doctor.platform.omp.catalog", Status: "pass", Detail: "catalog reason=catalog_ready"}}
	before, err := json.Marshal(existing)
	require.NoError(t, err)
	var beforeText bytes.Buffer
	renderOMPDoctorChecksText(&beforeText, existing)
	got := appendOMPContextDoctorChecks(existing, ompContextDoctorReport{})
	after, err := json.Marshal(got)
	require.NoError(t, err)
	var afterText bytes.Buffer
	renderOMPDoctorChecksText(&afterText, got)
	assert.Equal(t, before, after)
	assert.Equal(t, beforeText.Bytes(), afterText.Bytes())
}

func TestCheckOMPContextDoctor_FreshInstalledLifecycleIsSupportedAndBodyFree(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	report := checkOMPContextDoctor(validOMPContextDoctorInput(now))
	assert.Equal(t, "supported", report.Status)
	assert.Equal(t, "fresh", report.Reason)
	assert.Equal(t, config.OMPContextHistoryActive, report.RequestedHistoryMode)
	assert.Equal(t, config.OMPContextHistoryActive, report.EffectiveHistoryMode)
	assert.Equal(t, config.OMPContextMemoryOff, report.EffectiveMemoryMode)
	assert.Equal(t, "fresh", report.ReceiptFreshness)
	assert.True(t, report.Checkpoint)
	assert.True(t, report.Rehydrated)
	assert.True(t, report.ExactMatch)
	assert.Equal(t, 0, report.ArtifactCleanupCount)

	checks := projectOMPContextDoctorChecks(report)
	encoded, err := json.Marshal(checks)
	require.NoError(t, err)
	text := string(encoded)
	for _, expected := range []string{
		"requested_history=active effective_history=active",
		"checkpoint=true rehydrated=true exact_match=true",
		"cleanup_count=0 root_class=isolated_task_owned",
		"receipt=valid freshness=fresh identity_current=true",
		"active_injection=false",
	} {
		assert.Contains(t, text, expected)
	}
	for _, forbidden := range []string{"TASK-7", "session-1", "docs/", "/Users/", "sk-secret", "provider/model"} {
		assert.NotContains(t, text, forbidden)
	}
}

func TestOMPContextDoctorProjection_TextAndJSONExposeExactSameBodyFreeRows(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	checks := projectOMPContextDoctorChecks(checkOMPContextDoctor(validOMPContextDoctorInput(now)))
	jsonBytes, err := json.Marshal(sanitizeJSONChecks(checks))
	require.NoError(t, err)
	var terminal bytes.Buffer
	renderOMPDoctorChecksText(&terminal, checks)
	for _, check := range checks {
		assert.Contains(t, terminal.String(), check.ID+": "+check.Detail)
		assert.Contains(t, string(jsonBytes), check.ID)
		assert.Contains(t, string(jsonBytes), check.Detail)
	}
	assertOMPContextDoctorBodyFree(t, checks)
}

func TestOMPContextDoctorProjection_RedactsUnsafeReportFields(t *testing.T) {
	t.Parallel()

	unsafe := "sk-secret-/Users/private/raw-body"
	report := ompContextDoctorReport{
		Enabled: true, Profile: unsafe, Status: "blocked", Reason: unsafe,
		RequestedHistoryMode: unsafe, EffectiveHistoryMode: unsafe,
		RequestedMemoryMode: unsafe, EffectiveMemoryMode: unsafe,
		FallbackMode: unsafe, FallbackReason: unsafe, ReceiptStatus: unsafe,
		ReceiptFreshness: unsafe, CurrentVersion: unsafe, RootClass: unsafe,
		Capabilities: []ompContextDoctorCapability{{ID: unsafe, Required: true, Reason: unsafe}},
	}
	assertOMPContextDoctorBodyFree(t, projectOMPContextDoctorChecks(report))
}

func TestCheckOMPContextDoctor_VersionOnlyNeverPassesActiveAndDowngrades(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	input := validOMPContextDoctorInput(now)
	input.Current.ConfigListSchema = false
	input.Current.OverlayReadback = false
	input.Receipt.Receipt.Capabilities.PreCompactionEvent = false
	report := checkOMPContextDoctor(input)
	assert.Equal(t, "blocked", report.Status)
	assert.Equal(t, "config_schema_unproved", report.Reason)
	assert.Equal(t, config.OMPContextHistoryShadow, report.EffectiveHistoryMode)
	assert.NotEqual(t, "fresh", report.Status)

	capability := ompContextDoctorCapabilityByID(t, report, "lifecycle.pre_compaction")
	assert.False(t, capability.Supported)
	assert.Equal(t, "installed_lifecycle_unproved", capability.Reason)
}

func TestCheckOMPContextDoctor_MemoryShadowIsIndependentAndNeverInjected(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	input := validOMPContextDoctorInput(now)
	input.RequestedHistoryMode = config.OMPContextHistoryOff
	input.RequestedMemoryMode = config.OMPContextMemoryShadow
	input.Receipt.Receipt.Mode.RequestedHistoryMode = config.OMPContextHistoryOff
	input.Receipt.Receipt.Mode.EffectiveHistoryMode = config.OMPContextHistoryOff
	input.Receipt.Receipt.Mode.EffectiveMemoryMode = config.OMPContextMemoryShadow

	report := checkOMPContextDoctor(input)
	assert.Equal(t, config.OMPContextHistoryOff, report.EffectiveHistoryMode)
	assert.Equal(t, config.OMPContextMemoryShadow, report.EffectiveMemoryMode)
	assert.True(t, report.MemoryInterception)
	assert.True(t, report.MemoryProvenance)
	assert.False(t, report.MemoryActiveInjection)

	input.Receipt.Receipt.Capabilities.MemoryInterception = false
	report = checkOMPContextDoctor(input)
	assert.Equal(t, "degraded", report.Status)
	assert.Equal(t, "memory_interception_unproved", report.Reason)
	assert.Equal(t, config.OMPContextMemoryOff, report.EffectiveMemoryMode)
	assert.False(t, report.MemoryActiveInjection)
}

func ompContextDoctorCapabilityByID(t *testing.T, report ompContextDoctorReport, id string) ompContextDoctorCapability {
	t.Helper()
	for _, capability := range report.Capabilities {
		if capability.ID == id {
			return capability
		}
	}
	t.Fatalf("missing capability %s", id)
	return ompContextDoctorCapability{}
}

func validOMPContextDoctorInput(now time.Time) ompContextDoctorInput {
	receipt := newValidOMPContextDoctorReceiptFixture(now.Add(-5 * time.Minute))
	return ompContextDoctorInput{
		Enabled: true, Profile: "safe", RequestedHistoryMode: config.OMPContextHistoryActive,
		RequestedMemoryMode: config.OMPContextMemoryOff, FallbackMode: config.OMPContextFallbackCanonicalFull,
		RuntimeRootPolicy: config.OMPContextRuntimeIsolatedTaskOwned,
		Current: ompContextCurrentProbe{
			Version: "omp/17.1.8", IdentityVerified: true, ConfigListSchema: true,
			CompactionSchema: true, MemorySchema: true, OverlayReadback: true,
		},
		Receipt: ompContextDoctorReceiptState{Status: "valid", Freshness: "fresh", Receipt: receipt},
	}
}

func assertOMPContextDoctorBodyFree(t *testing.T, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	require.NoError(t, err)
	lower := strings.ToLower(string(data))
	for _, forbidden := range []string{"raw-body", "sk-secret", "/users/", "authorization:", "bearer "} {
		assert.NotContains(t, lower, forbidden)
	}
}
