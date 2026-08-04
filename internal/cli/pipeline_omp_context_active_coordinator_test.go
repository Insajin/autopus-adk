package cli

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/pipeline"
	"github.com/insajin/autopus-adk/pkg/promptlayer"
)

func TestPipelineOMPManagedActiveCoordinator_NarrowsOpaqueGrantToOneUseLease(t *testing.T) {
	now := time.Date(2026, 8, 4, 10, 0, 0, 0, time.UTC)
	candidate, expected, grant := pipelineOMPActiveCoordinatorFixture(t, now)
	spawnCalls := 0
	runner := newPipelineOMPManagedActiveCoordinator()
	runner.now = func() time.Time { return now }
	runner.expectation = func(got pipelineOMPManagedActiveCandidate) (promptlayer.OMPContextPromotionExpectationV2, error) {
		assert.Equal(t, candidate.Snapshot.Prompt, got.Snapshot.Prompt)
		return expected, nil
	}
	runner.loadGrant = func(root string, checkedAt time.Time,
		got promptlayer.OMPContextPromotionExpectationV2,
	) (pipelineOMPVerifiedGrant, error) {
		assert.Equal(t, candidate.Snapshot.ProjectDir, root)
		assert.Equal(t, now, checkedAt)
		assert.Equal(t, expected, got)
		return grant, nil
	}
	runner.spawn = func(_ context.Context, _ pipelineOMPManagedActiveCandidate,
		prepared pipelineOMPManagedActivePrepared,
	) (string, error) {
		spawnCalls++
		assert.True(t, prepared.grant.Valid())
		return "active provider output", nil
	}

	prepared, err := runner.Prepare(context.Background(), candidate)
	require.NoError(t, err)
	output, err := runner.Execute(context.Background(), candidate, prepared)
	require.NoError(t, err)
	assert.Equal(t, "active provider output", output)
	assert.Equal(t, 1, spawnCalls)

	_, err = runner.Execute(context.Background(), candidate, prepared)
	assert.ErrorContains(t, err, "already consumed")
	assert.Equal(t, 1, spawnCalls)
}

func TestPipelineOMPManagedActiveCoordinator_RejectsCrossCandidateBeforeLease(t *testing.T) {
	now := time.Date(2026, 8, 4, 10, 0, 0, 0, time.UTC)
	candidate, expected, grant := pipelineOMPActiveCoordinatorFixture(t, now)
	grant.candidate.Revision = strings.Repeat("f", 40)
	runner := newPipelineOMPManagedActiveCoordinator()
	runner.now = func() time.Time { return now }
	runner.expectation = func(pipelineOMPManagedActiveCandidate) (promptlayer.OMPContextPromotionExpectationV2, error) {
		return expected, nil
	}
	runner.loadGrant = func(string, time.Time, promptlayer.OMPContextPromotionExpectationV2) (pipelineOMPVerifiedGrant, error) {
		return grant, nil
	}
	runner.spawn = func(context.Context, pipelineOMPManagedActiveCandidate, pipelineOMPManagedActivePrepared) (string, error) {
		t.Fatal("cross-candidate grant reached spawn")
		return "", nil
	}

	_, err := runner.Prepare(context.Background(), candidate)
	assert.ErrorContains(t, err, "coordinates mismatch")
}

type pipelineOMPVerifiedGrantStub struct {
	digest, evidence, policy, provider, modelScope string
	expires                                        time.Time
	producer                                       promptlayer.OMPContextPromotionProducerV1
	candidate                                      promptlayer.OMPContextPromotionCandidateV1
	runtime                                        promptlayer.OMPContextPromotionRuntimeV1
}

func (grant pipelineOMPVerifiedGrantStub) Valid() bool {
	return grant.digest != "" && !grant.expires.IsZero()
}
func (grant pipelineOMPVerifiedGrantStub) ReportDigest() string { return grant.digest }
func (grant pipelineOMPVerifiedGrantStub) EvidenceID() string   { return grant.evidence }
func (grant pipelineOMPVerifiedGrantStub) ExpiresAt() time.Time { return grant.expires }
func (grant pipelineOMPVerifiedGrantStub) ProducerCoordinates() promptlayer.OMPContextPromotionProducerV1 {
	return grant.producer
}
func (grant pipelineOMPVerifiedGrantStub) CandidateCoordinates() promptlayer.OMPContextPromotionCandidateV1 {
	return grant.candidate
}
func (grant pipelineOMPVerifiedGrantStub) PolicyDigest() string { return grant.policy }
func (grant pipelineOMPVerifiedGrantStub) RuntimeCoordinates() promptlayer.OMPContextPromotionRuntimeV1 {
	return grant.runtime
}
func (grant pipelineOMPVerifiedGrantStub) ProviderScope() (string, string) {
	return grant.provider, grant.modelScope
}

func pipelineOMPActiveCoordinatorFixture(t *testing.T, now time.Time) (
	pipelineOMPManagedActiveCandidate,
	promptlayer.OMPContextPromotionExpectationV2,
	pipelineOMPVerifiedGrantStub,
) {
	t.Helper()
	request := newWorkflowContextRuntimeFixture(t)
	root := request.Binding.DeliveryOptions.Root
	cfg := config.DefaultFullConfig("autopus-adk")
	cfg.OMPContextPolicy = config.OMPContextPolicyConf{Profile: "active", Profiles: map[string]config.OMPContextProfileConf{
		"active": {
			HistoryMode: config.OMPContextHistoryActive, MemoryMode: config.OMPContextMemoryOff,
			HistoryTargetTokens: 1000, Fallback: config.OMPContextFallbackCanonicalFull,
			CapabilityPolicy:  config.OMPContextCapabilityProbeRequired,
			RuntimeRootPolicy: config.OMPContextRuntimeIsolatedTaskOwned,
			MutationScope:     config.OMPContextMutationSessionOverlay,
		},
	}}
	require.NoError(t, config.Save(root, cfg))
	snapshot := pipeline.OMPExecutionSnapshot{
		ProjectDir: root, SpecID: "SPEC-OMP-004", SpecDir: runtimeSpecDir,
		SnapshotHash: "sha256:" + strings.Repeat("a", 64), GitCommitHash: strings.Repeat("b", 40),
		PhaseID: pipeline.PhaseImplement, Attempt: 1, Prompt: "implement exact phase",
		ActivePrompt:     "implement active exact phase",
		CompletedHistory: []string{"prior output"},
	}
	phaseModels := map[pipeline.PhaseID]string{
		pipeline.PhasePlan: "openai/gpt-5.4", pipeline.PhaseTestScaffold: "openai/gpt-5.4",
		pipeline.PhaseImplement: "openai/gpt-5.4", pipeline.PhaseValidate: "openai/gpt-5.4",
		pipeline.PhaseReview: "openai/gpt-5.4",
	}
	scopeProvider, modelScopeDigest, err := pipelineOMPActiveModelScope(phaseModels)
	require.NoError(t, err)
	candidate := pipelineOMPManagedActiveCandidate{
		Snapshot: snapshot, OriginalTask: "/auto go SPEC-OMP-004", DeliveryCommand: "go",
		Provider: "openai", Model: "gpt-5.4", ScopeProvider: scopeProvider, ModelScopeDigest: modelScopeDigest,
		AutoSourceCommit: strings.Repeat("d", 40),
		AutoSourceTree:   strings.Repeat("e", 40),
	}
	expected := promptlayer.OMPContextPromotionExpectationV2{
		ProducerRepository: "Insajin/autopus-adk-evals", ProducerWorkflowRef: "workflow.yml@immutable",
		Candidate: promptlayer.OMPContextPromotionCandidateV1{
			Repository: "Insajin/autopus-adk", Revision: candidate.AutoSourceCommit,
			TreeSHA: candidate.AutoSourceTree, ArtifactSHA256: pipelineOMPContextCohortHash("artifact"),
		},
		PolicyID: "omp-active-v1", PolicyDigest: pipelineOMPContextCohortHash("policy"),
		AutoVersion: "0.50.93", AutoBinarySHA256: pipelineOMPContextCohortHash("auto"),
		OMPVersion: "omp/17.2.7", OMPExecutableSHA256: pipelineOMPContextCohortHash("omp"),
		PipelineImplementationDigest: pipelineOMPActiveImplementationDigest(),
		Provider:                     candidate.ScopeProvider, ModelScopeDigest: candidate.ModelScopeDigest,
		CohortManifestDigest: pipelineOMPContextCohortHash("cohort"), OrderSeed: pipelineOMPContextCohortHash("order"),
		OraclePolicyDigest: pipelineOMPContextCohortHash("oracle"),
	}
	grant := pipelineOMPVerifiedGrantStub{
		digest: pipelineOMPContextCohortHash("report"), evidence: pipelineOMPContextCohortHash("evidence"),
		policy: expected.PolicyDigest, provider: expected.Provider, modelScope: expected.ModelScopeDigest,
		expires: now.Add(time.Hour), candidate: expected.Candidate,
		runtime: promptlayer.OMPContextPromotionRuntimeV1{
			AutoVersion: expected.AutoVersion, AutoBinarySHA256: expected.AutoBinarySHA256,
			OMPVersion: expected.OMPVersion, OMPExecutableSHA256: expected.OMPExecutableSHA256,
			ExecutionClass: "external-live", ProductionPathEquivalent: true,
			RuntimeKind:                  "omp-pipeline-managed-rpc",
			PipelineImplementationDigest: expected.PipelineImplementationDigest,
		},
	}
	return candidate, expected, grant
}
