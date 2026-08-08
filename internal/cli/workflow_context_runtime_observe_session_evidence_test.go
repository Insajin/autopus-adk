package cli

import (
	"bytes"
	"context"
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

func TestWorkflowContextObserveSession_WritesObservedBodyFreePromotionEvidence(t *testing.T) {
	requireDarwinManagedOMPSandboxForTest(t)
	challenge := workflowContextRuntimeHash("observe-session-challenge")
	setup, options, logPath := workflowContextObserveEvidenceFixture(t, challenge)
	input, taskBodies := workflowContextObserveEvidenceInput(t, challenge)
	var output bytes.Buffer
	prepared := &setup
	ctx := context.WithValue(context.Background(), workflowContextObserveSessionPrepareKey{},
		workflowContextObserveSessionPrepare(func(_ context.Context, got workflowContextObserveSessionOptions, gotChallenge string) (workflowContextObserveSessionSetup, error) {
			require.Equal(t, options, got)
			require.Equal(t, challenge, gotChallenge)
			return *prepared, nil
		}))

	require.NoError(t, RunWorkflowContextObserveSession(ctx, input, &output, options))
	responses := decodeWorkflowContextObserveResponses(t, output.Bytes())
	require.Len(t, responses, 42)
	shutdown := responses[len(responses)-1]
	assert.Equal(t, "shutdown", shutdown.Type)
	assert.Equal(t, 40, shutdown.CallsCompleted)
	assert.Equal(t, 18, shutdown.CompactionCycles)
	assert.True(t, shutdown.CleanupVerified)
	assert.NotEmpty(t, shutdown.EvidenceID)
	assert.NotEmpty(t, shutdown.ReportDigest)
	assert.Zero(t, shutdown.OwnedRootsRemaining)
	assert.Zero(t, shutdown.ProcessesRemaining)

	require.Zero(t, prepared.full.PID())
	require.Zero(t, prepared.optimized.PID())
	_, err := os.Lstat(prepared.taskRoot)
	require.ErrorIs(t, err, os.ErrNotExist)

	reportPath := promptlayer.OMPContextPromotionReportPathV1(options.ProjectDir)
	reportBody, err := os.ReadFile(reportPath)
	require.NoError(t, err)
	for _, forbidden := range append([]string{
		prepared.credential, prepared.endpoint, prepared.projectDir, prepared.taskRoot,
		"credential_locator", "assistant_text", "safe assistant output", "/Users/",
	}, taskBodies...) {
		assert.NotContains(t, string(reportBody), forbidden)
	}
	var report promptlayer.OMPContextPromotionReportV1
	require.NoError(t, json.Unmarshal(reportBody, &report))
	assert.Equal(t, shutdown.EvidenceID, report.EvidenceID)
	assert.Equal(t, challenge, report.ChallengeDigest)
	assert.Len(t, report.Tasks, 20)
	assert.Len(t, report.Observations, 40)
	assert.Equal(t, workflowContextObserveSessionSegmentCount, report.SessionFacts.FullProcessStarts)
	assert.Equal(t, workflowContextObserveSessionSegmentCount, report.SessionFacts.OptimizedProcessStarts)
	assert.Equal(t, workflowContextObserveSessionSegmentCount, report.SessionFacts.FullSessionCount)
	assert.Equal(t, workflowContextObserveSessionSegmentCount, report.SessionFacts.OptimizedSessionCount)
	assert.Equal(t, 10, countWorkflowContextObserveOrders(report.Tasks, "AB"))
	assert.Equal(t, 10, countWorkflowContextObserveOrders(report.Tasks, "BA"))
	rows, err := promptlayer.OMPContextPromotionReportCanaryRowsV1(report)
	require.NoError(t, err)
	require.Len(t, rows, 40)
	for index := 0; index < len(rows); index += 2 {
		full, optimized := rows[index], rows[index+1]
		if full.Variant == promptlayer.OMPContextCanaryVariantOptimizedV1 {
			full, optimized = optimized, full
		}
		assert.Equal(t, int64(100), full.Tokens)
		expectedOptimizedTokens := int64(40)
		if (index/2)%workflowContextObserveSessionSegmentPairs == 0 {
			expectedOptimizedTokens = 100
		}
		assert.Equal(t, expectedOptimizedTokens, optimized.Tokens)
		assert.True(t, optimized.FallbackVerified)
		assert.True(t, optimized.RollbackVerified)
	}

	storeBody, err := os.ReadFile(promptlayer.OMPContextEvidenceStorePath(options.ProjectDir))
	require.NoError(t, err)
	assert.NotContains(t, string(storeBody), prepared.credential)
	assert.NotContains(t, string(storeBody), prepared.projectDir)
	var store promptlayer.OMPContextEvidenceStoreV1
	require.NoError(t, json.Unmarshal(storeBody, &store))
	assert.Equal(t, options.WorkspaceID, store.Binding.WorkspaceID)
	assert.Equal(t, prepared.candidate.Snapshot.SnapshotHash, store.Binding.SnapshotHash)
	assert.Len(t, store.CanaryRows, 40)
	assert.Empty(t, store.HistoryRefs)

	verifyWorkflowContextObserveProviderLog(t, logPath, taskBodies)
}

func TestWorkflowContextObserveSession_WritesBodyFreeErrorFrame(t *testing.T) {
	var output bytes.Buffer
	err := RunWorkflowContextObserveSession(
		context.Background(), strings.NewReader("{}\n"), &output, workflowContextObserveSessionOptions{},
	)
	require.ErrorContains(t, err, "handshake is invalid")
	responses := decodeWorkflowContextObserveResponses(t, output.Bytes())
	require.Len(t, responses, 1)
	assert.Equal(t, workflowContextObserveSessionResponseSchema, responses[0].SchemaVersion)
	assert.Equal(t, "error", responses[0].Type)
	assert.Equal(t, "input_invalid", responses[0].ErrorCode)
	assert.Equal(t, "handshake", responses[0].ErrorStage)
	assert.Zero(t, responses[0].FailedSequence)
	assert.NotContains(t, output.String(), err.Error())
	assert.Equal(t, "network_transport",
		workflowContextObserveSessionErrorCode(fmt.Errorf("provider timed out")))
	assert.Equal(t, "runtime_failed", workflowContextObserveSessionErrorCode(
		fmt.Errorf("observe-session call 6 failed closed: managed active OMP transcript image is invalid"),
	))
}

func TestWorkflowContextObserveSessionHelp_DocumentsExplicitLiveLoopbackCanary(t *testing.T) {
	cmd := newWorkflowContextObserveSessionCmd()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs([]string{"--help"})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, output.String(), "http://127.0.0.1:43123")
	assert.Contains(t, output.String(), "--explicit-live")
	assert.Contains(t, output.String(), "promotion-report-v1.json")
}

func TestWorkflowContextObserveSessionCommand_RequiresExplicitLiveAcknowledgement(t *testing.T) {
	cmd := newWorkflowContextObserveSessionCmd()
	cmd.SetArgs(nil)
	err := cmd.Execute()
	require.ErrorContains(t, err, "requires --explicit-live")
}

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
		autoVersion: "0.50.103", autoExecutableSHA256: autoSHA, candidateTree: tree,
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
