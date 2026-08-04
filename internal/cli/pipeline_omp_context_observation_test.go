package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkflowContextObservationCommand_EmitsExactBodyFreeFortyCallCohort(t *testing.T) {
	manifest := writePipelineOMPContextObservationFixture(t, pipelineOMPContextObservationFixture())
	cmd := newWorkflowContextObservationCmd()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"--explicit-live", "--manifest", manifest, "--output", "-", "--format", "json"})

	require.NoError(t, cmd.Execute())
	var observed pipelineOMPContextObservationV1
	require.NoError(t, json.Unmarshal(output.Bytes(), &observed))
	assert.Equal(t, pipelineOMPContextObservationSchema, observed.SchemaVersion)
	assert.Len(t, observed.Tasks, 20)
	assert.Len(t, observed.Calls, 40)
	assert.Equal(t, "A", observed.Calls[0].Variant)
	assert.Equal(t, "B", observed.Calls[1].Variant)
	assert.NotContains(t, output.String(), "provider-output-body")
	assert.NotContains(t, strings.ToLower(output.String()), "verdict")
}

func TestWorkflowContextObservationCommand_RejectsUnknownVerdictAndInvalidExecution(t *testing.T) {
	fixture := pipelineOMPContextObservationFixture()
	fixture.Calls[0].RetryCount = 1
	path := writePipelineOMPContextObservationFixture(t, fixture)
	cmd := newWorkflowContextObservationCmd()
	cmd.SetArgs([]string{"--explicit-live", "--manifest", path, "--output", "-", "--format", "json"})
	require.ErrorContains(t, cmd.Execute(), "call 1")

	body, err := json.Marshal(pipelineOMPContextObservationFixture())
	require.NoError(t, err)
	body = bytes.Replace(body, []byte(`"schema_version"`), []byte(`"verdict":true,"schema_version"`), 1)
	unknown := filepath.Join(t.TempDir(), "unknown.json")
	require.NoError(t, os.WriteFile(unknown, body, 0o600))
	cmd = newWorkflowContextObservationCmd()
	cmd.SetArgs([]string{"--explicit-live", "--manifest", unknown, "--output", "-", "--format", "json"})
	require.ErrorContains(t, cmd.Execute(), "unknown field")
}

func TestWorkflowContextObservationCommand_RequiresExplicitLiveAndExactTaskCount(t *testing.T) {
	fixture := pipelineOMPContextObservationFixture()
	fixture.Tasks = fixture.Tasks[:19]
	path := writePipelineOMPContextObservationFixture(t, fixture)
	cmd := newWorkflowContextObservationCmd()
	cmd.SetArgs([]string{"--explicit-live", "--manifest", path, "--output", "-"})
	require.ErrorContains(t, cmd.Execute(), "identity")

	cmd = newWorkflowContextObservationCmd()
	cmd.SetArgs([]string{"--manifest", path, "--output", "-"})
	require.ErrorContains(t, cmd.Execute(), "explicit-live")
}

func pipelineOMPContextObservationFixture() pipelineOMPContextObservationV1 {
	hash := func(value string) string { return pipelineOMPContextCohortHash(value) }
	value := pipelineOMPContextObservationV1{
		SchemaVersion: pipelineOMPContextObservationSchema, EvidenceID: hash("evidence"),
		ChallengeDigest: hash("challenge"), RunID: "12345", Attempt: 1,
		TrustLane: "autopus-main-omp-context-promotion", ProducerRepository: "Insajin/autopus-adk",
		ProducerWorkflowRef: "release/omp-context-observer.yml@main",
		Candidate: pipelineOMPContextObservationCandidate{
			Repository: "Insajin/autopus-adk", Revision: strings.Repeat("b", 40),
			TreeSHA: strings.Repeat("c", 40), ArtifactSHA256: hash("artifact"),
		},
		Policy: pipelineOMPContextObservationPolicy{
			PolicyID: "omp-context-active-v1", PolicyDigest: hash("policy"), HistoryMode: "active", MemoryMode: "off",
			MinPairCount: 20, MinReductionBasisPoints: 2000,
		},
		Runtime: pipelineOMPContextObservationRuntime{
			OMPExecutableSHA256: hash("omp"), OMPVersion: "omp/17.2.7",
			AutoBinarySHA256: hash("auto"), AutoVersion: "0.50.93",
			ExecutionClass: "external-live", RuntimeKind: "omp-pipeline-managed-rpc",
			ProductionPathEquivalent: true, PipelineImplementationDigest: pipelineOMPActiveImplementationDigest(),
		},
		Provider: "openai", Model: "gpt-5.4", ModelScopeDigest: hash("model-scope"),
		CredentialLocator: "AUTOPUS_OMP_CONTEXT_PROVIDER_TOKEN", CohortDigest: hash("cohort"),
		OrderSeed: hash("order-seed"), OraclePolicyDigest: hash("oracle-policy"),
	}
	start := time.Date(2026, 8, 4, 0, 0, 0, 0, time.UTC)
	for taskIndex := 0; taskIndex < 20; taskIndex++ {
		order := "AB"
		if taskIndex%2 == 1 {
			order = "BA"
		}
		taskDigest := hash("task-" + string(rune('a'+taskIndex)))
		value.Tasks = append(value.Tasks, pipelineOMPContextObservationTask{TaskIDDigest: taskDigest, Order: order})
		for pairIndex := 0; pairIndex < 2; pairIndex++ {
			sequence := taskIndex*2 + pairIndex + 1
			inputTokens := int64(10_000)
			if string(order[pairIndex]) == "B" {
				inputTokens = 7_500
			}
			callStart := start.Add(time.Duration(sequence-1) * 2 * time.Second)
			variant := string(order[pairIndex])
			setupCalls, compactionCalls := 0, 0
			maintenanceInput, maintenanceOutput := int64(0), int64(0)
			if variant == "B" {
				maintenanceInput, maintenanceOutput = 500, 50
				setupCalls = 2
				compactionCalls = 1
			}
			value.Calls = append(value.Calls, pipelineOMPContextObservationCall{
				Sequence: sequence, PairSequence: pairIndex + 1, TaskIDDigest: taskDigest,
				Variant: variant, Provider: value.Provider, Model: value.Model, ModelScopeDigest: value.ModelScopeDigest,
				EndpointClass: "external-provider", Transport: "provider-api", CredentialMode: "locator-only",
				ExecutionMode: "external-live", StartedAt: callStart.Format(time.RFC3339Nano),
				CompletedAt: callStart.Add(time.Second).Format(time.RFC3339Nano),
				TokenUsage: pipelineOMPContextTokenUsage{
					PrimaryInputTokens: inputTokens, PrimaryOutputTokens: 100,
					MaintenanceInputTokens: maintenanceInput, MaintenanceOutputTokens: maintenanceOutput,
					TotalTokens: inputTokens + 100 + maintenanceInput + maintenanceOutput,
				},
				IntegrityFacts: pipelineOMPContextIntegrityFacts{
					RequiredDocumentRefs: 4, MatchedDocumentRefs: 4,
					RequiredEphemeralFields: 3, MatchedEphemeralFields: 3,
					SetupProviderRequests: setupCalls, PrimaryProviderRequests: 1,
					CompactionProviderCalls: compactionCalls,
					TotalProviderRequests:   1 + setupCalls + compactionCalls,
				},
				SecurityFacts: pipelineOMPContextSecurityFacts{AuthorizedGatewayRequests: 1 + setupCalls + compactionCalls},
				QualityFacts:  pipelineOMPContextQualityFacts{Assertions: 5, PassedAssertions: 5},
				FallbackFacts: pipelineOMPContextFallbackFacts{Trials: 1, CanonicalFullRestarts: 1, ExactMatches: 1},
				RollbackFacts: pipelineOMPContextRollbackFacts{Trials: 1, EffectiveShadowReadbacks: 1},
				CleanupFacts:  pipelineOMPContextCleanupFacts{OwnedRootsCreated: 1, OwnedRootsRemoved: 1},
				RetryCount:    0, MaxConcurrency: 1, TaskDigest: hash("task-body-" + string(rune(sequence))),
				OutputDigest: hash("provider-output-body-" + string(rune(sequence))), OracleDigest: hash("oracle-" + string(rune(sequence))),
			})
		}
	}
	return value
}

func writePipelineOMPContextObservationFixture(t *testing.T, value pipelineOMPContextObservationV1) string {
	t.Helper()
	body, err := json.Marshal(value)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "observation.json")
	require.NoError(t, os.WriteFile(path, body, 0o600))
	return path
}
