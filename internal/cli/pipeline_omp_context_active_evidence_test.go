package cli

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/promptlayer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPipelineOMPActiveEvidence_CurrentRunActivatesWithFreshObservedAuthority(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	candidate, expected, grant, policy := pipelineOMPActiveObservedEvidenceFixture(t, now)
	writePipelineOMPActiveObservedStore(t, candidate, grant, policy, now, time.Hour)
	spawned := 0
	runner := pipelineOMPActiveObservedRunner(now, expected, grant, func() { spawned++ })

	prepared, err := runner.Prepare(context.Background(), candidate)
	require.NoError(t, err)
	assert.Equal(t, candidate.Snapshot.SnapshotHash, prepared.Binding.SnapshotHash)
	assert.Equal(t, candidate.Snapshot.GitCommitHash, prepared.Binding.GitCommitHash)
	assert.NotEmpty(t, prepared.Binding.BindingHash)
	output, err := runner.Execute(context.Background(), candidate, prepared)
	require.NoError(t, err)
	assert.Equal(t, "observed active output", output)
	assert.Equal(t, 1, spawned)
}

func TestPipelineOMPActiveEvidence_StalePartialTamperedMissingAndCrossRunFailClosed(t *testing.T) {
	for name, mutate := range map[string]func(
		t *testing.T,
		candidate *pipelineOMPManagedActiveCandidate,
		grant *pipelineOMPVerifiedGrantStub,
		policy promptlayer.OMPContextPromotionPolicyV1,
		now time.Time,
	){
		"missing": func(*testing.T, *pipelineOMPManagedActiveCandidate, *pipelineOMPVerifiedGrantStub, promptlayer.OMPContextPromotionPolicyV1, time.Time) {
		},
		"stale": func(t *testing.T, candidate *pipelineOMPManagedActiveCandidate, grant *pipelineOMPVerifiedGrantStub, policy promptlayer.OMPContextPromotionPolicyV1, now time.Time) {
			writePipelineOMPActiveObservedStore(t, *candidate, *grant, policy, now.Add(-2*time.Hour), time.Hour)
		},
		"partial grant": func(t *testing.T, candidate *pipelineOMPManagedActiveCandidate, grant *pipelineOMPVerifiedGrantStub, policy promptlayer.OMPContextPromotionPolicyV1, now time.Time) {
			writePipelineOMPActiveObservedStore(t, *candidate, *grant, policy, now, time.Hour)
			grant.rows = grant.rows[:39]
		},
		"missing authority": func(t *testing.T, candidate *pipelineOMPManagedActiveCandidate, grant *pipelineOMPVerifiedGrantStub, policy promptlayer.OMPContextPromotionPolicyV1, now time.Time) {
			writePipelineOMPActiveObservedStore(t, *candidate, *grant, policy, now, time.Hour)
			grant.sessionAuthority = ""
		},
		"tampered cohort": func(t *testing.T, candidate *pipelineOMPManagedActiveCandidate, grant *pipelineOMPVerifiedGrantStub, policy promptlayer.OMPContextPromotionPolicyV1, now time.Time) {
			tampered := *grant
			tampered.rows = append([]promptlayer.OMPContextCanaryRowV1(nil), grant.rows...)
			tampered.rows[0].Tokens++
			writePipelineOMPActiveObservedStore(t, *candidate, tampered, policy, now, time.Hour)
		},
		"cross run snapshot": func(t *testing.T, candidate *pipelineOMPManagedActiveCandidate, grant *pipelineOMPVerifiedGrantStub, policy promptlayer.OMPContextPromotionPolicyV1, now time.Time) {
			writePipelineOMPActiveObservedStore(t, *candidate, *grant, policy, now, time.Hour)
			candidate.Snapshot.SnapshotHash = workflowContextRuntimeHash("different-run-snapshot")
		},
	} {
		t.Run(name, func(t *testing.T) {
			now := time.Now().UTC().Truncate(time.Second)
			candidate, expected, grant, policy := pipelineOMPActiveObservedEvidenceFixture(t, now)
			mutate(t, &candidate, &grant, policy, now)
			spawned := 0
			runner := pipelineOMPActiveObservedRunner(now, expected, grant, func() { spawned++ })
			_, err := runner.Prepare(context.Background(), candidate)
			require.Error(t, err)
			assert.Zero(t, spawned)
		})
	}
}

func pipelineOMPActiveObservedEvidenceFixture(t *testing.T, now time.Time) (
	pipelineOMPManagedActiveCandidate,
	promptlayer.OMPContextPromotionStaticPolicyV3,
	pipelineOMPVerifiedGrantStub,
	promptlayer.OMPContextPromotionPolicyV1,
) {
	t.Helper()
	candidate, expected, grant := pipelineOMPActiveCoordinatorFixture(t, now)
	policy := promptlayer.OMPContextPromotionPolicyV1{
		Profile: "active", HistoryMode: config.OMPContextHistoryActive, MemoryMode: config.OMPContextMemoryOff,
		HistoryTargetTokens: 1000, Fallback: config.OMPContextFallbackCanonicalFull,
		CapabilityPolicy:  config.OMPContextCapabilityProbeRequired,
		RuntimeRootPolicy: config.OMPContextRuntimeIsolatedTaskOwned,
		MutationScope:     config.OMPContextMutationSessionOverlay,
	}
	policyDigest, err := promptlayer.OMPContextPromotionPolicyDigestV1(policy)
	require.NoError(t, err)
	expected.PolicyDigest, grant.policy = policyDigest, policyDigest
	grant.providerAuthority = workflowContextRuntimeHash("endpoint-credential-authority")
	grant.sessionAuthority = workflowContextRuntimeHash("stable-two-session-authority")
	grant.rows = pipelineOMPActiveObservedRows()
	return candidate, expected, grant, policy
}

func pipelineOMPActiveObservedRows() []promptlayer.OMPContextCanaryRowV1 {
	rows := make([]promptlayer.OMPContextCanaryRowV1, 0, 40)
	for index := range 20 {
		fullOrder, optimizedOrder := 1, 2
		if index%2 == 1 {
			fullOrder, optimizedOrder = 2, 1
		}
		taskID := workflowContextRuntimeHash("active-observed-task-" + strings.Repeat("x", index+1))
		rows = append(rows,
			promptlayer.OMPContextCanaryRowV1{
				TaskID: taskID, Variant: promptlayer.OMPContextCanaryVariantFullV1, Order: fullOrder,
				Tokens: 10000, IntegrityPassed: true, SecurityPassed: true, QualityScore: 100,
			},
			promptlayer.OMPContextCanaryRowV1{
				TaskID: taskID, Variant: promptlayer.OMPContextCanaryVariantOptimizedV1, Order: optimizedOrder,
				Tokens: 7500, IntegrityPassed: true, SecurityPassed: true, QualityScore: 100,
				FallbackVerified: true, RollbackVerified: true,
			},
		)
	}
	return rows
}

func writePipelineOMPActiveObservedStore(
	t *testing.T,
	candidate pipelineOMPManagedActiveCandidate,
	grant pipelineOMPVerifiedGrantStub,
	policy promptlayer.OMPContextPromotionPolicyV1,
	checkedAt time.Time,
	validFor time.Duration,
) {
	t.Helper()
	require.NoError(t, promptlayer.WriteOMPContextEvidenceStoreV1(
		candidate.Snapshot.ProjectDir,
		promptlayer.OMPContextEvidenceStoreV1{
			Binding: promptlayer.OMPContextEvidenceStoreBindingV1{
				WorkspaceID: "autopus-adk", SpecID: candidate.Snapshot.SpecID,
				SnapshotHash:   candidate.Snapshot.SnapshotHash,
				GitCommitHash:  candidate.Snapshot.GitCommitHash,
				RuntimeVersion: grant.runtime.OMPVersion, CheckedAt: checkedAt, ValidFor: validFor,
			},
			Policy: policy, CanaryRows: grant.rows,
		},
	))
}

func pipelineOMPActiveObservedRunner(
	now time.Time,
	expected promptlayer.OMPContextPromotionStaticPolicyV3,
	grant pipelineOMPVerifiedGrantStub,
	onSpawn func(),
) *pipelineOMPManagedActiveCoordinator {
	runner := newPipelineOMPManagedActiveCoordinator()
	runner.now = func() time.Time { return now }
	runner.policy = func(pipelineOMPManagedActiveCandidate) (promptlayer.OMPContextPromotionStaticPolicyV3, error) {
		return expected, nil
	}
	runner.current = func(context.Context, pipelineOMPManagedActiveCandidate, promptlayer.OMPContextPromotionStaticPolicyV3) (promptlayer.OMPContextPromotionCurrentRuntimeV3, error) {
		return pipelineOMPActiveCurrentRuntimeFixture(expected), nil
	}
	runner.loadGrant = func(string, time.Time, promptlayer.OMPContextPromotionStaticPolicyV3, promptlayer.OMPContextPromotionCurrentRuntimeV3) (pipelineOMPVerifiedGrant, error) {
		return grant, nil
	}
	runner.spawn = func(context.Context, pipelineOMPManagedActiveCandidate, pipelineOMPManagedActivePrepared) (string, error) {
		onSpawn()
		return "observed active output", nil
	}
	return runner
}
