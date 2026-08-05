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
	runner.verifyEvidence = pipelineOMPActiveEvidenceVerifierFixture(grant.expires)
	runner.policy = func(got pipelineOMPManagedActiveCandidate) (promptlayer.OMPContextPromotionStaticPolicyV3, error) {
		assert.Equal(t, candidate.Snapshot.Prompt, got.Snapshot.Prompt)
		return expected, nil
	}
	runner.current = func(_ context.Context, got pipelineOMPManagedActiveCandidate,
		_ promptlayer.OMPContextPromotionStaticPolicyV3,
	) (promptlayer.OMPContextPromotionCurrentRuntimeV3, error) {
		assert.Equal(t, candidate.AutoSourceCommit, got.AutoSourceCommit)
		return pipelineOMPActiveCurrentRuntimeFixture(expected), nil
	}
	runner.loadGrant = func(root string, checkedAt time.Time,
		got promptlayer.OMPContextPromotionStaticPolicyV3,
		current promptlayer.OMPContextPromotionCurrentRuntimeV3,
	) (pipelineOMPVerifiedGrant, error) {
		assert.Equal(t, candidate.Snapshot.ProjectDir, root)
		assert.Equal(t, now, checkedAt)
		assert.Equal(t, expected, got)
		assert.Equal(t, expected.SourceCommit, current.SourceCommit)
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

func TestPipelineOMPManagedActiveCoordinator_RejectsGrantThatExpiresBeforeExecute(t *testing.T) {
	now := time.Date(2026, 8, 4, 10, 0, 0, 0, time.UTC)
	candidate, expected, grant := pipelineOMPActiveCoordinatorFixture(t, now)
	grant.expires = now.Add(time.Second)
	runner := newPipelineOMPManagedActiveCoordinator()
	runner.now = func() time.Time { return now }
	runner.verifyEvidence = pipelineOMPActiveEvidenceVerifierFixture(grant.expires)
	runner.policy = func(pipelineOMPManagedActiveCandidate) (promptlayer.OMPContextPromotionStaticPolicyV3, error) {
		return expected, nil
	}
	runner.current = func(context.Context, pipelineOMPManagedActiveCandidate,
		promptlayer.OMPContextPromotionStaticPolicyV3,
	) (promptlayer.OMPContextPromotionCurrentRuntimeV3, error) {
		return pipelineOMPActiveCurrentRuntimeFixture(expected), nil
	}
	runner.loadGrant = func(string, time.Time, promptlayer.OMPContextPromotionStaticPolicyV3,
		promptlayer.OMPContextPromotionCurrentRuntimeV3,
	) (pipelineOMPVerifiedGrant, error) {
		return grant, nil
	}
	runner.spawn = func(context.Context, pipelineOMPManagedActiveCandidate,
		pipelineOMPManagedActivePrepared,
	) (string, error) {
		t.Fatal("expired release-key authority reached provider spawn")
		return "", nil
	}

	prepared, err := runner.Prepare(context.Background(), candidate)
	require.NoError(t, err)
	now = now.Add(2 * time.Second)
	_, err = runner.Execute(context.Background(), candidate, prepared)
	assert.ErrorContains(t, err, "lease expired")
}

func TestPipelineOMPManagedActiveCoordinator_RejectsCrossCandidateBeforeLease(t *testing.T) {
	now := time.Date(2026, 8, 4, 10, 0, 0, 0, time.UTC)
	candidate, expected, grant := pipelineOMPActiveCoordinatorFixture(t, now)
	grant.candidate.Revision = strings.Repeat("f", 40)
	runner := newPipelineOMPManagedActiveCoordinator()
	runner.now = func() time.Time { return now }
	runner.policy = func(pipelineOMPManagedActiveCandidate) (promptlayer.OMPContextPromotionStaticPolicyV3, error) {
		return expected, nil
	}
	runner.current = func(context.Context, pipelineOMPManagedActiveCandidate,
		promptlayer.OMPContextPromotionStaticPolicyV3,
	) (promptlayer.OMPContextPromotionCurrentRuntimeV3, error) {
		return pipelineOMPActiveCurrentRuntimeFixture(expected), nil
	}
	runner.loadGrant = func(string, time.Time, promptlayer.OMPContextPromotionStaticPolicyV3,
		promptlayer.OMPContextPromotionCurrentRuntimeV3,
	) (pipelineOMPVerifiedGrant, error) {
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
	providerAuthority, sessionAuthority            string
	expires                                        time.Time
	producer                                       promptlayer.OMPContextPromotionProducerV1
	candidate                                      promptlayer.OMPContextPromotionCandidateV1
	runtime                                        promptlayer.OMPContextPromotionRuntimeV1
	rows                                           []promptlayer.OMPContextCanaryRowV1
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
func (grant pipelineOMPVerifiedGrantStub) ProviderAuthorityDigest() string {
	return grant.providerAuthority
}
func (grant pipelineOMPVerifiedGrantStub) SessionAuthorityDigest() string {
	return grant.sessionAuthority
}
func (grant pipelineOMPVerifiedGrantStub) CanaryRows() []promptlayer.OMPContextCanaryRowV1 {
	return append([]promptlayer.OMPContextCanaryRowV1(nil), grant.rows...)
}

func pipelineOMPActiveCoordinatorFixture(t *testing.T, now time.Time) (
	pipelineOMPManagedActiveCandidate,
	promptlayer.OMPContextPromotionStaticPolicyV3,
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
	expected := promptlayer.OMPContextPromotionStaticPolicyV3{
		SchemaVersion:      promptlayer.OMPContextPromotionRuntimeSchemaV3,
		ProducerRepository: "Insajin/autopus-adk-evals", ProducerWorkflowRef: "workflow.yml@immutable",
		CandidateRepository: "Insajin/autopus-adk", SourceCommit: candidate.AutoSourceCommit,
		SourceTree: candidate.AutoSourceTree, Target: "darwin-arm64",
		PolicyID: "omp-active-v1", PolicyDigest: pipelineOMPContextCohortHash("policy"),
		AutoVersion: "0.50.94",
		OMPVersion:  "omp/17.2.7", OMPExecutableSHA256: pipelineOMPContextCohortHash("omp"),
		PipelineImplementationDigest: pipelineOMPActiveImplementationDigest(),
		Provider:                     candidate.ScopeProvider, ModelScopeDigest: candidate.ModelScopeDigest,
		CohortManifestDigest: pipelineOMPContextCohortHash("cohort"), OrderSeed: pipelineOMPContextCohortHash("order"),
		OraclePolicyDigest:  pipelineOMPContextCohortHash("oracle"),
		ReleaseLineageKeyID: "release-lineage-2026-q3-k1", ReleaseLineageHandoff: "v1",
		MinimumRollbackFloor: 5093,
	}
	upstream := pipelineOMPContextCohortHash("artifact")
	grant := pipelineOMPVerifiedGrantStub{
		digest: pipelineOMPContextCohortHash("report"), evidence: pipelineOMPContextCohortHash("evidence"),
		policy: expected.PolicyDigest, provider: expected.Provider, modelScope: expected.ModelScopeDigest,
		expires: now.Add(time.Hour), candidate: promptlayer.OMPContextPromotionCandidateV1{
			Repository: expected.CandidateRepository, Revision: expected.SourceCommit,
			TreeSHA: expected.SourceTree, ArtifactSHA256: upstream,
		},
		runtime: promptlayer.OMPContextPromotionRuntimeV1{
			AutoVersion: expected.AutoVersion, AutoBinarySHA256: upstream,
			OMPVersion: expected.OMPVersion, OMPExecutableSHA256: expected.OMPExecutableSHA256,
			ExecutionClass: "external-live", ProductionPathEquivalent: true,
			RuntimeKind:                  "omp-pipeline-managed-rpc",
			PipelineImplementationDigest: expected.PipelineImplementationDigest,
		},
	}
	return candidate, expected, grant
}

func pipelineOMPActiveCurrentRuntimeFixture(
	policy promptlayer.OMPContextPromotionStaticPolicyV3,
) promptlayer.OMPContextPromotionCurrentRuntimeV3 {
	return promptlayer.OMPContextPromotionCurrentRuntimeV3{
		ExecutableSHA256: pipelineOMPContextCohortHash("distributed"),
		SourceCommit:     policy.SourceCommit, SourceTree: policy.SourceTree, Target: policy.Target,
		AutoVersion: policy.AutoVersion, OMPVersion: policy.OMPVersion,
		OMPExecutableSHA256:          policy.OMPExecutableSHA256,
		PipelineImplementationDigest: policy.PipelineImplementationDigest,
	}
}

func pipelineOMPActiveEvidenceVerifierFixture(expires time.Time) pipelineOMPActiveEvidenceVerifier {
	return func(
		pipelineOMPManagedActiveCandidate,
		pipelineOMPVerifiedGrant,
		promptlayer.OMPContextPromotionPolicyV1,
		promptlayer.OMPContextBindingReceipt,
		string, string, string,
		time.Time,
	) (time.Time, error) {
		return expires, nil
	}
}
