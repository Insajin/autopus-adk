package promptlayer_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/promptlayer"
)

func TestOMPContextEvidenceStore_RoundTripsBodyFreeVerifiedEvidence(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)
	binding := evidenceStoreBinding(now)
	policy := evidenceStorePolicy()
	document := promptlayer.OMPContextEvidenceStoreV1{
		Binding:    binding,
		Policy:     policy,
		CanaryRows: promotionRows(),
		HistoryRefs: []promptlayer.OMPContextHistoryReference{{
			ID: "history-1", SourceRef: "tool/read-old",
			BodyHash:      "sha256:" + strings.Repeat("d", 64),
			TokenEstimate: 1000, Reason: "completed-superseded",
		}},
	}
	require.NoError(t, promptlayer.WriteOMPContextEvidenceStoreV1(root, document))

	path := promptlayer.OMPContextEvidenceStorePath(root)
	info, err := os.Lstat(path)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	body, err := os.ReadFile(path)
	require.NoError(t, err)
	require.NotContains(t, string(body), "raw prompt body")
	require.NotContains(t, string(body), root)

	verified, err := promptlayer.LoadOMPContextEvidenceStoreV1(
		root, binding,
		promptlayer.OMPContextPromotionSubjectV1{
			WorkspaceID: "workspace-1", SpecID: "SPEC-OMP-004", TaskID: "T5",
			Phase: "implementation", SessionID: "session-1",
			BindingHash: "sha256:" + strings.Repeat("e", 64),
		},
		policy, now.Add(time.Minute),
	)
	require.NoError(t, err)
	require.Len(t, verified.Promotion.Rows, 40)
	require.Len(t, verified.HistoryRefs, 1)
	require.False(t, verified.Promotion.Attestation.IsZero())
}

func TestOMPContextEvidenceStore_ProductionExpectationOwnsFreshnessReadback(t *testing.T) {
	root := t.TempDir()
	checkedAt := time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)
	binding := evidenceStoreBinding(checkedAt)
	policy := evidenceStorePolicy()
	require.NoError(t, promptlayer.WriteOMPContextEvidenceStoreV1(root, promptlayer.OMPContextEvidenceStoreV1{
		Binding: binding, Policy: policy, CanaryRows: promotionRows(),
	}))

	expectation := promptlayer.OMPContextEvidenceExpectationV1{
		WorkspaceID: binding.WorkspaceID, SpecID: binding.SpecID,
		SnapshotHash: binding.SnapshotHash, GitCommitHash: binding.GitCommitHash,
		RuntimeVersion: binding.RuntimeVersion,
	}
	subject := promptlayer.OMPContextPromotionSubjectV1{
		WorkspaceID: binding.WorkspaceID, SpecID: binding.SpecID, TaskID: "pipeline-plan-1",
		Phase: "plan", SessionID: "pipeline-session-1",
		BindingHash: "sha256:" + strings.Repeat("e", 64),
	}
	verified, err := promptlayer.LoadOMPContextEvidenceForExpectationV1(
		root, expectation, subject, policy, checkedAt.Add(time.Minute),
	)
	require.NoError(t, err)
	require.Len(t, verified.Promotion.Rows, 40)
	require.False(t, verified.Promotion.Attestation.IsZero())

	mismatch := expectation
	mismatch.GitCommitHash = strings.Repeat("f", 40)
	_, err = promptlayer.LoadOMPContextEvidenceForExpectationV1(
		root, mismatch, subject, policy, checkedAt.Add(time.Minute),
	)
	require.ErrorContains(t, err, "binding mismatch")
	_, err = promptlayer.LoadOMPContextEvidenceForExpectationV1(
		root, expectation, subject, policy, checkedAt.Add(24*time.Hour),
	)
	require.ErrorContains(t, err, "stale")
}

func TestOMPContextEvidenceStore_RejectsTamperUnknownFieldsAndSymlink(t *testing.T) {
	now := time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)
	binding := evidenceStoreBinding(now)
	document := promptlayer.OMPContextEvidenceStoreV1{
		Binding: binding, Policy: evidenceStorePolicy(), CanaryRows: promotionRows(),
	}

	root := t.TempDir()
	require.NoError(t, promptlayer.WriteOMPContextEvidenceStoreV1(root, document))
	path := promptlayer.OMPContextEvidenceStorePath(root)
	body, err := os.ReadFile(path)
	require.NoError(t, err)
	tampered := append(body[:len(body)-2], []byte(",\"unknown_authority\":true}\n")...)
	require.NoError(t, os.WriteFile(path, tampered, 0o600))
	_, err = promptlayer.LoadOMPContextEvidenceStoreV1(
		root, binding, promptlayer.OMPContextPromotionSubjectV1{}, evidenceStorePolicy(), now,
	)
	require.Error(t, err)

	symlinkRoot := t.TempDir()
	target := filepath.Join(t.TempDir(), "outside.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(promptlayer.OMPContextEvidenceStorePath(symlinkRoot)), 0o700))
	require.NoError(t, os.Symlink(target, promptlayer.OMPContextEvidenceStorePath(symlinkRoot)))
	require.Error(t, promptlayer.WriteOMPContextEvidenceStoreV1(symlinkRoot, document))
}

func evidenceStoreBinding(now time.Time) promptlayer.OMPContextEvidenceStoreBindingV1 {
	return promptlayer.OMPContextEvidenceStoreBindingV1{
		WorkspaceID: "workspace-1", SpecID: "SPEC-OMP-004",
		SnapshotHash:  "sha256:" + strings.Repeat("a", 64),
		GitCommitHash: strings.Repeat("b", 40), PolicyDigest: "sha256:" + strings.Repeat("c", 64),
		RuntimeVersion: "omp/17.1.8", CheckedAt: now, ValidFor: 24 * time.Hour,
	}
}

func evidenceStorePolicy() promptlayer.OMPContextPromotionPolicyV1 {
	return promptlayer.OMPContextPromotionPolicyV1{
		Profile: "history-v1", HistoryMode: "active", MemoryMode: "off",
		HistoryTargetTokens: 1000, Fallback: "canonical-full",
		CapabilityPolicy: "installed-probed", RuntimeRootPolicy: "task-owned",
		MutationScope: "session-overlay",
	}
}

func promotionRows() []promptlayer.OMPContextCanaryRowV1 {
	rows := make([]promptlayer.OMPContextCanaryRowV1, 0, 40)
	for i := range 20 {
		orderA, orderB := 1, 2
		if i%2 == 1 {
			orderA, orderB = 2, 1
		}
		task := "task-" + strings.Repeat("x", i%3) + string(rune('a'+i))
		rows = append(rows,
			promptlayer.OMPContextCanaryRowV1{TaskID: task, Variant: promptlayer.OMPContextCanaryVariantFullV1, Order: orderA, Tokens: 10000, IntegrityPassed: true, SecurityPassed: true, QualityScore: 100},
			promptlayer.OMPContextCanaryRowV1{TaskID: task, Variant: promptlayer.OMPContextCanaryVariantOptimizedV1, Order: orderB, Tokens: 7500, IntegrityPassed: true, SecurityPassed: true, QualityScore: 100, FallbackVerified: true, RollbackVerified: true},
		)
	}
	return rows
}
