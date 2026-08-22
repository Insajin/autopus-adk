package cli

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/promptlayer"
)

func TestWorkflowContextObserveSessionCanaryPlan_ProducesExactBalancedCohort(t *testing.T) {
	t.Parallel()
	challenge := workflowContextRuntimeHash("release-canary")
	commands, tasks, err := workflowContextObserveSessionCanaryPlan(challenge)
	require.NoError(t, err)
	require.Len(t, commands, 42)
	require.Len(t, tasks, 20)
	assert.True(t, validWorkflowContextObserveHandshake(commands[0]))
	for index, command := range commands[1:41] {
		require.NoError(t, validateWorkflowContextObserveSessionCall(command, index+1))
		task := tasks[index/2]
		assert.Equal(t, task.TaskIDDigest, command.TaskIDDigest)
		assert.Equal(t, string(task.Order[index%2]), command.Variant)
	}
	assert.True(t, validWorkflowContextObserveShutdown(commands[41]))
}

func TestCompanionOMPContextCanaryPlan_BindsInputAndStaticPolicyToOnePlan(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	cfg := config.DefaultFullConfig("release-canary")
	cfg.OMPContextPolicy = config.OMPContextPolicyConf{
		Profile: "active",
		Profiles: map[string]config.OMPContextProfileConf{"active": {
			HistoryMode: config.OMPContextHistoryActive, MemoryMode: config.OMPContextMemoryOff,
			HistoryTargetTokens: 1000, Fallback: config.OMPContextFallbackCanonicalFull,
			CapabilityPolicy:  config.OMPContextCapabilityProbeRequired,
			RuntimeRootPolicy: config.OMPContextRuntimeIsolatedTaskOwned,
			MutationScope:     config.OMPContextMutationSessionOverlay,
		}},
	}
	require.NoError(t, config.Save(root, cfg))
	inputPath := filepath.Join(root, "canary-input.jsonl")
	challenge := workflowContextRuntimeHash("release-challenge")
	options := companionOMPContextCanaryPlanOptions{
		projectDir: root, inputOutput: inputPath, challengeDigest: challenge,
		producerRepository: "Insajin/autopus-adk", producerWorkflowRef: "local-release-prep@" + strings.Repeat("a", 40),
		candidateRepository: "Insajin/autopus-adk", sourceCommit: strings.Repeat("a", 40), sourceTree: strings.Repeat("b", 40),
		target: "darwin-arm64", autoVersion: "0.50.109", provider: "openai-codex", model: "gpt-5.6-sol",
		policyID: "omp-context-active-v1", oraclePolicyDigest: workflowContextRuntimeHash("oracle"),
		ompVersion: "omp/17.2.7", ompExecutableSHA256: workflowContextRuntimeHash("omp"),
		releaseLineageKeyID: "release-lineage-2026-q3-k1", releaseLineageHandoff: "v1", minimumRollbackFloor: 5101,
	}
	var output bytes.Buffer
	command := newCompanionOMPContextCanaryPlanCmd()
	command.SetOut(&output)
	require.NoError(t, runCompanionOMPContextCanaryPlan(command, options))

	file, err := os.Open(inputPath)
	require.NoError(t, err)
	defer file.Close()
	scanner := bufio.NewScanner(file)
	var commands []workflowContextObserveSessionCommand
	for scanner.Scan() {
		var value workflowContextObserveSessionCommand
		require.NoError(t, json.Unmarshal(scanner.Bytes(), &value))
		commands = append(commands, value)
	}
	require.NoError(t, scanner.Err())
	require.Len(t, commands, 42)
	assert.Equal(t, challenge, commands[0].ChallengeDigest)

	policyBytes, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(output.String()))
	require.NoError(t, err)
	var policy promptlayer.OMPContextPromotionStaticPolicyV3
	require.NoError(t, json.Unmarshal(policyBytes, &policy))
	_, tasks, err := workflowContextObserveSessionCanaryPlan(challenge)
	require.NoError(t, err)
	taskBytes, err := json.Marshal(tasks)
	require.NoError(t, err)
	assert.Equal(t, workflowContextRuntimeHash(string(taskBytes)), policy.CohortManifestDigest)
	assert.Equal(t, policy.CohortManifestDigest, policy.OrderSeed)
	assert.Equal(t, pipelineOMPActiveImplementationDigest(), policy.PipelineImplementationDigest)
	assert.Equal(t, options.sourceCommit, policy.SourceCommit)
	assert.Equal(t, options.sourceTree, policy.SourceTree)
	assert.Equal(t, options.provider, policy.Provider)
	assert.Equal(t, os.FileMode(0o600), mustFileMode(t, inputPath))
}
