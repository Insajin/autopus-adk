package pipeline_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/memindex"
	"github.com/insajin/autopus-adk/pkg/pipeline"
	"github.com/insajin/autopus-adk/pkg/promptlayer"
)

func TestPhasePromptBuilder_BoundedReceiptAugmentsRequiredContextForEveryPhase(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), phaseDeliverySpecID)
	require.NoError(t, os.Mkdir(dir, 0o700))
	legacyBodies := []string{
		"# " + phaseDeliverySpecID + ": Receipt fixture\nHUGE_SPEC_" + strings.Repeat("spec ", 4000),
		"HUGE_ACCEPTANCE_" + strings.Repeat("acceptance ", 4000),
		"HUGE_PLAN_FILE_" + strings.Repeat("plan ", 4000),
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(legacyBodies[0]), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "acceptance.md"), []byte(legacyBodies[1]), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "plan.md"), []byte(legacyBodies[2]), 0o644))
	receipt := memindex.ContextResult{
		BudgetTokens: 1200, RecallBudgetTokens: 420, EstimatedTokens: 700,
		SnapshotHash: "snapshot-a", PromptManifestHash: "manifest-a",
		Prompt: "## Context Receipt\noutcome_lock: BOUNDED_RECEIPT_MARKER\nsnapshot_hash: snapshot-a\nAuthorization: Bearer receipt-secret-token",
	}
	previous := map[pipeline.PhaseID]string{
		pipeline.PhasePlan:         "HUGE_PLAN_OUTPUT_" + strings.Repeat("prior plan ", 4000),
		pipeline.PhaseTestScaffold: "HUGE_TEST_OUTPUT_" + strings.Repeat("prior test ", 4000),
		pipeline.PhaseImplement:    "HUGE_IMPLEMENT_OUTPUT_" + strings.Repeat("prior implementation ", 4000),
		pipeline.PhaseValidate:     "HUGE_VALIDATE_OUTPUT_" + strings.Repeat("prior validation ", 4000),
	}
	phases := []pipeline.PhaseID{
		pipeline.PhasePlan,
		pipeline.PhaseTestScaffold,
		pipeline.PhaseImplement,
		pipeline.PhaseValidate,
		pipeline.PhaseReview,
	}
	expectedMarkers := map[pipeline.PhaseID][]string{
		pipeline.PhasePlan:         {"HUGE_SPEC_", "HUGE_PLAN_FILE_", "HUGE_ACCEPTANCE_"},
		pipeline.PhaseTestScaffold: {"HUGE_SPEC_", "HUGE_PLAN_FILE_", "HUGE_ACCEPTANCE_", "HUGE_PLAN_OUTPUT_"},
		pipeline.PhaseImplement:    {"HUGE_SPEC_", "HUGE_PLAN_FILE_", "HUGE_ACCEPTANCE_", "HUGE_PLAN_OUTPUT_", "HUGE_TEST_OUTPUT_"},
		pipeline.PhaseValidate:     {"HUGE_SPEC_", "HUGE_PLAN_FILE_", "HUGE_ACCEPTANCE_", "HUGE_IMPLEMENT_OUTPUT_"},
		pipeline.PhaseReview:       {"HUGE_SPEC_", "HUGE_PLAN_FILE_", "HUGE_ACCEPTANCE_", "HUGE_VALIDATE_OUTPUT_"},
	}

	for _, phaseID := range phases {
		phaseID := phaseID
		t.Run(string(phaseID), func(t *testing.T) {
			t.Parallel()

			prompt, manifest, err := pipeline.NewPhasePromptBuilder(dir).BuildPromptWithManifest(phaseID, pipeline.PhaseContext{
				ContextResult:   &receipt,
				PreviousResults: previous,
			})
			require.NoError(t, err)

			assert.Contains(t, prompt, "BOUNDED_RECEIPT_MARKER")
			assert.NotContains(t, prompt, "receipt-secret-token")
			for _, marker := range expectedMarkers[phaseID] {
				assert.Contains(t, prompt, marker)
			}
			entry := pipelineManifestEntry(manifest, "phase:context-receipt")
			assert.Equal(t, promptlayer.KindSnapshot, entry.Kind)
			assert.Equal(t, "context-receipt", entry.SourceRef)
			assert.Positive(t, entry.TokenEstimate)
			assert.Equal(t, promptlayer.RedactionRedacted, entry.RedactionStatus)
		})
	}
}

func TestPhasePromptBuilder_EmptyReceiptUsesLegacyPromptForEveryPhase(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("legacy spec"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "acceptance.md"), []byte("legacy acceptance"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "plan.md"), []byte("legacy plan"), 0o644))
	previous := map[pipeline.PhaseID]string{
		pipeline.PhasePlan:         "legacy plan output",
		pipeline.PhaseTestScaffold: "legacy test output",
		pipeline.PhaseImplement:    "legacy implementation output",
		pipeline.PhaseValidate:     "legacy validation output",
	}

	for _, phaseID := range []pipeline.PhaseID{
		pipeline.PhasePlan,
		pipeline.PhaseTestScaffold,
		pipeline.PhaseImplement,
		pipeline.PhaseValidate,
		pipeline.PhaseReview,
	} {
		phaseID := phaseID
		t.Run(string(phaseID), func(t *testing.T) {
			t.Parallel()

			builder := pipeline.NewPhasePromptBuilder(dir)
			wantPrompt, wantManifest, err := builder.BuildPromptWithManifest(phaseID, pipeline.PhaseContext{PreviousResults: previous})
			require.NoError(t, err)
			empty := memindex.ContextResult{Prompt: " \n\t "}
			gotPrompt, gotManifest, err := builder.BuildPromptWithManifest(phaseID, pipeline.PhaseContext{
				ContextResult:   &empty,
				PreviousResults: previous,
			})
			require.NoError(t, err)

			assert.Equal(t, wantPrompt, gotPrompt)
			assert.Equal(t, wantManifest, gotManifest)
		})
	}
}

func TestPhasePromptBuilder_DynamicReceiptDoesNotInvalidateStableInstruction(t *testing.T) {
	t.Parallel()

	stable := promptlayer.Layer{ID: "command", Kind: promptlayer.KindStable, Group: promptlayer.GroupMethodologyTools, SourceRef: "auto-go", Content: "implement safely"}
	receiptA := promptlayer.SnapshotLayer("receipt", "context-receipt", "snapshot A")
	receiptB := promptlayer.SnapshotLayer("receipt", "context-receipt", "snapshot B")
	taskA := promptlayer.Layer{ID: "task", Kind: promptlayer.KindEphemeral, Group: promptlayer.GroupUserRequest, SourceRef: "user", Content: "task A"}
	taskB := promptlayer.Layer{ID: "task", Kind: promptlayer.KindEphemeral, Group: promptlayer.GroupUserRequest, SourceRef: "user", Content: "task B"}

	a, err := promptlayer.Render([]promptlayer.Layer{stable, receiptA, taskA})
	require.NoError(t, err)
	b, err := promptlayer.Render([]promptlayer.Layer{stable, receiptB, taskB})
	require.NoError(t, err)

	assert.Equal(t, pipelineManifestEntry(a.Manifest, "command").Hash, pipelineManifestEntry(b.Manifest, "command").Hash)
	assert.NotEqual(t, pipelineManifestEntry(a.Manifest, "receipt").Hash, pipelineManifestEntry(b.Manifest, "receipt").Hash)
	changes := promptlayer.CompareManifests(a.Manifest, b.Manifest)
	for _, change := range changes {
		assert.NotEqual(t, promptlayer.InvalidationStableSourceChanged, change.Reason)
	}
}
