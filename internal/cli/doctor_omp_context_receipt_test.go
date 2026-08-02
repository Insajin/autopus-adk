package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/promptlayer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadOMPContextDoctorReceipt_ValidFreshStrictReceipt(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	receipt := newValidOMPContextDoctorReceiptFixture(now.Add(-5 * time.Minute))
	writeOMPContextDoctorReceiptFixture(t, root, receipt)
	state := readOMPContextDoctorReceipt(root, now)
	assert.Equal(t, "valid", state.Status)
	assert.Equal(t, "fresh", state.Freshness)
	assert.Equal(t, receipt.Capabilities.Version, state.Receipt.Capabilities.Version)
}

func TestReadOMPContextDoctorReceipt_RejectsTamperModeSymlinkAndAbsolutePath(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name   string
		mutate func(*WorkflowContextRuntimeReceipt)
	}{
		{name: "tampered hash", mutate: func(r *WorkflowContextRuntimeReceipt) { r.BindingHash = "sha256:tampered" }},
		{name: "overlay mismatch", mutate: func(r *WorkflowContextRuntimeReceipt) { r.Mode.ReadbackHash = runtimeHash("other") }},
		{name: "unsafe root", mutate: func(r *WorkflowContextRuntimeReceipt) { r.RootClass = "user_root" }},
		{name: "absolute path", mutate: func(r *WorkflowContextRuntimeReceipt) { r.Fallback.Reason = "/Users/private/raw-body" }},
		{name: "active memory injection", mutate: func(r *WorkflowContextRuntimeReceipt) { r.MemoryInjections = []string{"entry"} }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			receipt := newValidOMPContextDoctorReceiptFixture(now.Add(-time.Minute))
			tt.mutate(&receipt)
			writeOMPContextDoctorReceiptFixture(t, root, receipt)
			assert.Equal(t, "invalid", readOMPContextDoctorReceipt(root, now).Status)
		})
	}

	root := t.TempDir()
	outside := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".autopus"), 0o700))
	require.NoError(t, os.Symlink(outside, filepath.Join(root, ".autopus", "runtime")))
	assert.Equal(t, "invalid", readOMPContextDoctorReceipt(root, now).Status)
}

func TestReadOMPContextDoctorReceipt_MissingStaleFutureAndUnsafeModes(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	assert.Equal(t, "missing", readOMPContextDoctorReceipt(t.TempDir(), now).Status)

	staleRoot := t.TempDir()
	writeOMPContextDoctorReceiptFixture(t, staleRoot, newValidOMPContextDoctorReceiptFixture(now.Add(-25*time.Hour)))
	assert.Equal(t, "stale", readOMPContextDoctorReceipt(staleRoot, now).Freshness)

	futureRoot := t.TempDir()
	writeOMPContextDoctorReceiptFixture(t, futureRoot, newValidOMPContextDoctorReceiptFixture(now.Add(10*time.Minute)))
	assert.Equal(t, "invalid", readOMPContextDoctorReceipt(futureRoot, now).Status)

	modeRoot := t.TempDir()
	path := writeOMPContextDoctorReceiptFixture(t, modeRoot, newValidOMPContextDoctorReceiptFixture(now))
	require.NoError(t, os.Chmod(path, 0o644))
	assert.Equal(t, "invalid", readOMPContextDoctorReceipt(modeRoot, now).Status)
}

func newValidOMPContextDoctorReceiptFixture(checkedAt time.Time) WorkflowContextRuntimeReceipt {
	hash := runtimeHash("doctor-receipt")
	return WorkflowContextRuntimeReceipt{
		SchemaVersion: WorkflowContextRuntimeReceiptSchemaVersion, Event: "terminal",
		WorkspaceID: "workspace", SpecID: "SPEC-OMP-004", TaskID: "TASK-7", Phase: "go", SessionID: "session-1",
		BindingHash: hash, OptionsHash: hash, SnapshotHash: hash, PromptManifestHash: hash,
		FullDocumentRefs:      []promptlayer.OMPContextDocumentReference{{SourceRef: "docs/policy.md", SourceHash: hash, PromptHash: hash, Complete: true}},
		RequiredEphemeralRefs: []promptlayer.OMPContextHashedReference{{ID: "original_task", Hash: hash}},
		FrozenFindingIDs:      []string{}, WorkerResultFields: promptlayer.OMPWorkerResultSchema(),
		HistoryCreditRows: []WorkflowContextHistoryCredit{}, ShadowCandidateRefs: []promptlayer.OMPContextPlanReference{},
		DocumentOmissions: []string{}, MemoryInjections: []string{},
		Capabilities: WorkflowContextCapabilities{
			Version: "omp/17.1.8", ExecutableIdentity: true, SettingsSchema: true, OverlayReadback: true,
			PreCompactionEvent: true, PostCompactionEvent: true, CanonicalInjection: true,
			AdmissionBlocking: true, IsolatedTaskRoot: true, CleanupReadback: true,
			MemoryInterception: true, AuthNoneLoopback: true, ProbeSource: "installed-canary", CheckedAt: checkedAt,
		},
		RootClass:      config.OMPContextRuntimeIsolatedTaskOwned,
		ArtifactCounts: WorkflowContextArtifactCounts{Before: 2, AfterCleanup: 0},
		Cleanup:        WorkflowContextCleanupReceipt{Attempted: true, Verified: true, Reason: "verified"},
		Mode: WorkflowContextModeReceipt{
			RequestedHistoryMode: config.OMPContextHistoryActive, EffectiveHistoryMode: config.OMPContextHistoryActive,
			EffectiveMemoryMode: config.OMPContextMemoryOff, PreviousHistoryMode: config.OMPContextHistoryShadow,
			OverlayHash: hash, ReadbackHash: hash,
		},
		Fallback: WorkflowContextFallbackReceipt{Mode: WorkflowContextFallbackNone}, ExactMatch: true,
		Outcome:       WorkflowContextOutcomeAdmitted,
		PhaseSequence: []string{"checkpointed", "compacted", "rehydrated", "admitted"},
	}
}

func writeOMPContextDoctorReceiptFixture(t *testing.T, root string, receipt WorkflowContextRuntimeReceipt) string {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(WorkflowContextReceiptRelativePath(receipt.TaskID, receipt.SessionID)))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	data, err := json.Marshal(receipt)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, append(data, '\n'), 0o600))
	return path
}
