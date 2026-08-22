package cli

// Fixtures and assertions for the observe-session evidence tests. They live in a
// sibling file so the test file that drives them stays inside the 300-line source
// limit; the package is unchanged, so the same tests run.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/pipeline"
	"github.com/insajin/autopus-adk/pkg/promptlayer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func workflowContextObserveEvidenceFixture(t *testing.T, challenge string) (
	workflowContextObserveSessionSetup,
	workflowContextObserveSessionOptions,
	string,
) {
	t.Helper()
	backend, _ := pipelineOMPBackendTestConfig(t)
	writeWorkflowContextObserveCanonicalDocuments(t, backend.ProjectDir, backend.SpecID)
	deliveryOptions := promptlayer.ContextDeliveryOptions{
		Root: backend.ProjectDir, Command: "go",
		SpecDir: filepath.ToSlash(filepath.Join(".autopus", "specs", backend.SpecID)),
	}
	delivery, err := promptlayer.BuildContextDelivery(deliveryOptions)
	require.NoError(t, err)
	require.NoError(t, promptlayer.VerifyContextDeliveryForOptions(deliveryOptions, delivery))
	snapshotHash, err := pipeline.SpecSnapshotHash(backend.SpecDir)
	require.NoError(t, err)
	taskRoot, err := os.MkdirTemp("", "autopus-observe-evidence-test-")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(taskRoot) })
	runtimeBase := filepath.Join(taskRoot, "runtime")
	require.NoError(t, os.Mkdir(runtimeBase, 0o700))
	commit, tree := strings.Repeat("b", 40), strings.Repeat("c", 40)
	endpoint, credential := "http://127.0.0.1:43123", "fixture-provider-token-value-123456"
	backend.Executable = os.Args[0]
	backend.RuntimeBase, backend.SnapshotHash, backend.GitCommitHash = runtimeBase, snapshotHash, commit
	backend.PhaseModels = workflowContextObserveSessionPhaseModels("openai/model-a")
	backend.Environment = append(backend.Environment,
		pipelineOMPActiveEndpointKey+"="+endpoint,
		pipelineOMPActiveCredentialKey+"="+credential,
	)
	backend, err = normalizePipelineOMPBackendConfig(backend)
	require.NoError(t, err)
	snapshot := pipeline.OMPExecutionSnapshot{
		ProjectDir: backend.ProjectDir, SpecID: backend.SpecID, SpecDir: backend.SpecDir,
		SnapshotHash: snapshotHash, GitCommitHash: commit, PhaseID: pipeline.PhaseImplement,
		Attempt: 1, Prompt: "observe-session", ActivePrompt: "observe-session",
	}
	candidate, err := newPipelineOMPManagedActiveCandidate(
		snapshot, backend.PhaseModels[pipeline.PhaseImplement], backend.PhaseModels,
	)
	require.NoError(t, err)
	candidate.AutoSourceCommit, candidate.AutoSourceTree = commit, tree
	policy := promptlayer.OMPContextPromotionPolicyV1{
		Profile: "active", HistoryMode: config.OMPContextHistoryActive, MemoryMode: config.OMPContextMemoryOff,
		HistoryTargetTokens: 1000, Fallback: config.OMPContextFallbackCanonicalFull,
		CapabilityPolicy:  config.OMPContextCapabilityProbeRequired,
		RuntimeRootPolicy: config.OMPContextRuntimeIsolatedTaskOwned,
		MutationScope:     config.OMPContextMutationSessionOverlay,
	}
	policyDigest, err := promptlayer.OMPContextPromotionPolicyDigestV1(policy)
	require.NoError(t, err)
	autoSHA := workflowContextRuntimeHash("installed-auto-binary")
	setup := workflowContextObserveSessionSetup{
		taskRoot: taskRoot, projectDir: backend.ProjectDir, endpoint: endpoint, credential: credential,
		backend: backend, candidate: candidate,
		prepared: pipelineOMPManagedActivePrepared{Binding: pipelineOMPActiveLeaseBinding{
			GrantDigest: challenge, PolicyDigest: policyDigest, WorkspaceID: "autopus-adk", SpecID: backend.SpecID,
			GitCommitHash: commit, AutoSourceCommit: commit, AutoSourceTree: tree,
			ModelScopeDigest: candidate.ModelScopeDigest,
		}},
		deliveryOptions: deliveryOptions, delivery: delivery, canonicalPromptHash: workflowContextRuntimeHash(delivery.Prompt),
		ompVersion: "omp/17.2.7", ompExecutableSHA256: fmt.Sprintf("sha256:%x", backend.executableID.digest[:]),
		autoVersion: "0.50.109", autoExecutableSHA256: autoSHA, candidateTree: tree,
		sandboxMode: pipelineOMPActiveSandboxManaged,
	}
	options := workflowContextObserveSessionOptions{
		ProjectDir: backend.ProjectDir, SpecID: backend.SpecID, Provider: "openai", Model: "model-a",
		ModelContextWindow: pipelineOMPActiveDefaultContextWindow,
		Endpoint:           endpoint, CredentialLocator: "AUTOPUS_TEST_PROVIDER_TOKEN", Executable: os.Args[0],
		TargetGitCommit: commit, SandboxMode: pipelineOMPActiveSandboxManaged,
		WorkspaceID: "autopus-adk", ProducerRepository: "insajin/omp-evals",
		ProducerWorkflowRef: "refs/heads/main@" + commit, ProducerRunID: "123456", ProducerRunAttempt: 1,
		CandidateRepository: "insajin/autopus-adk", PolicyID: "omp-context-active-v1",
		OraclePolicyDigest: workflowContextRuntimeHash("quality-security-oracle"), PromotionPolicy: policy,
		EvidenceValidFor: time.Hour,
	}
	return setup, options, filepath.Join(backend.ProjectDir, "active-native-rpc.jsonl")
}

func workflowContextObserveEvidenceInput(t *testing.T, challenge string) (*bytes.Buffer, []string) {
	t.Helper()
	commands, _, err := workflowContextObserveSessionCanaryPlan(challenge)
	require.NoError(t, err)
	var input bytes.Buffer
	encoder := json.NewEncoder(&input)
	tasks := make([]string, 0, 20)
	for _, command := range commands {
		require.NoError(t, encoder.Encode(command))
		if command.Type == "call" && command.PairSequence == 1 {
			tasks = append(tasks, command.Prompt)
		}
	}
	return &input, tasks
}

func writeWorkflowContextObserveCanonicalDocuments(t *testing.T, root, specID string) {
	t.Helper()
	files := map[string]string{
		"AGENTS.md":                     "CANONICAL-AGENT-DOCUMENT\n",
		"ARCHITECTURE.md":               "CANONICAL-ARCHITECTURE-DOCUMENT\n",
		".autopus/project/workspace.md": "CANONICAL-WORKSPACE-DOCUMENT\n",
		".autopus/project/product.md":   "CANONICAL-PRODUCT-DOCUMENT\n",
		".autopus/project/structure.md": "CANONICAL-STRUCTURE-DOCUMENT\n",
		".autopus/project/tech.md":      "CANONICAL-TECH-DOCUMENT\n",
		filepath.ToSlash(filepath.Join(".autopus", "specs", specID, "spec.md")):       "# " + specID + "\nCANONICAL-SPEC-DOCUMENT\n",
		filepath.ToSlash(filepath.Join(".autopus", "specs", specID, "plan.md")):       "CANONICAL-PLAN-DOCUMENT\n",
		filepath.ToSlash(filepath.Join(".autopus", "specs", specID, "acceptance.md")): "CANONICAL-ACCEPTANCE-DOCUMENT\n",
	}
	for name, body := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
		require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	}
}

func decodeWorkflowContextObserveResponses(t *testing.T, body []byte) []workflowContextObserveSessionResponse {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(body))
	var responses []workflowContextObserveSessionResponse
	for decoder.More() {
		var response workflowContextObserveSessionResponse
		require.NoError(t, decoder.Decode(&response))
		responses = append(responses, response)
	}
	return responses
}

func countWorkflowContextObserveOrders(tasks []promptlayer.OMPContextPromotionTaskV1, order string) int {
	count := 0
	for _, task := range tasks {
		if task.Order == order {
			count++
		}
	}
	return count
}

func verifyWorkflowContextObserveProviderLog(t *testing.T, path string, taskBodies []string) {
	t.Helper()
	body, err := os.ReadFile(path)
	require.NoError(t, err)
	decoder := json.NewDecoder(bytes.NewReader(body))
	starts := map[int]bool{}
	compactions := map[int]int{}
	pendingCompaction := map[int]bool{}
	var prompts []pipelineOMPRPCRecord
	for decoder.More() {
		var record pipelineOMPRPCRecord
		require.NoError(t, decoder.Decode(&record))
		if record.Kind == "start" {
			starts[record.PID] = true
		}
		if record.Kind != "command" {
			continue
		}
		switch record.Type {
		case "compact":
			compactions[record.PID]++
			pendingCompaction[record.PID] = true
		case "prompt":
			prompts = append(prompts, record)
			assert.Contains(t, record.Message, "CANONICAL-AGENT-DOCUMENT")
			assert.Contains(t, record.Message, "CANONICAL-SPEC-DOCUMENT")
			assert.Contains(t, record.Message, "CANONICAL-PLAN-DOCUMENT")
			assert.Contains(t, record.Message, "CANONICAL-ACCEPTANCE-DOCUMENT")
			assert.Contains(t, record.Message, "CANONICAL-WORKSPACE-DOCUMENT")
			if compactions[record.PID] > 0 {
				assert.True(t, pendingCompaction[record.PID], "optimized prompt was not admitted after compaction")
				pendingCompaction[record.PID] = false
			}
		}
	}
	assert.Len(t, starts, workflowContextObserveSessionSegmentCount*2)
	assert.Len(t, prompts, workflowContextObserveSessionPairCount*2)
	optimizedPIDs := 0
	compactionFrequencies := make(map[int]int)
	for _, count := range compactions {
		if count > 0 {
			optimizedPIDs++
			compactionFrequencies[count]++
		}
	}
	assert.Equal(t, workflowContextObserveSessionSegmentCount, optimizedPIDs)
	assert.Equal(t, map[int]int{9: 2}, compactionFrequencies)
	for pair := range workflowContextObserveSessionPairCount {
		assert.Equal(t, prompts[pair*2].Message, prompts[pair*2+1].Message)
		assert.Contains(t, prompts[pair*2].Message, taskBodies[pair])
	}
}
