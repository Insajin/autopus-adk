package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/pipeline"
)

func TestPipelineOMPActiveRPC_ReusesOneSessionAcrossManualCompaction(t *testing.T) {
	session, config, logPath := pipelineOMPActiveRPCSessionFixture(t, false)
	t.Cleanup(func() { require.NoError(t, session.Close()) })

	first, firstReceipt, err := session.Execute(context.Background(), "safe plan phase")
	require.NoError(t, err)
	second, secondReceipt, err := session.Execute(context.Background(), "safe implementation phase")
	require.NoError(t, err)

	assert.Equal(t, "safe assistant output 1", first)
	assert.Equal(t, "safe assistant output 2", second)
	assert.Equal(t, firstReceipt.SessionID, secondReceipt.SessionID)
	assert.Equal(t, 0, firstReceipt.CompactionCycles)
	assert.Equal(t, 1, secondReceipt.CompactionCycles)
	assert.True(t, secondReceipt.SameProcess)
	assert.True(t, secondReceipt.SameSession)
	records := readPipelineOMPRPCRecords(t, logPath)
	starts, commands := pipelineOMPRPCRecordsByKind(records)
	require.Len(t, starts, 1)
	assert.Equal(t, 2, countPipelineOMPRPCCommand(commands, "prompt"))
	assert.Equal(t, 1, countPipelineOMPRPCCommand(commands, "compact"))
	assert.GreaterOrEqual(t, countPipelineOMPRPCCommand(commands, "get_messages_page"), 4)
	assertPipelineOMPRPCBooleanCommand(t, commands, "set_auto_compaction", false)
	require.NoError(t, session.Close())
	assertPipelineOMPRuntimeEmpty(t, config.RuntimeBase)
}

func TestPipelineOMPActiveRPC_UnsafeFirstOutputClosesBeforeSecondProviderCall(t *testing.T) {
	session, _, logPath := pipelineOMPActiveRPCSessionFixture(t, true)

	_, _, err := session.Execute(context.Background(), "safe plan phase")
	require.ErrorContains(t, err, "unsafe assistant output")
	_, _, err = session.Execute(context.Background(), "safe implementation phase")
	require.Error(t, err)

	_, commands := pipelineOMPRPCRecordsByKind(readPipelineOMPRPCRecords(t, logPath))
	assert.Equal(t, 1, countPipelineOMPRPCCommand(commands, "prompt"))
	assert.Zero(t, countPipelineOMPRPCCommand(commands, "compact"))
}

func TestPipelineOMPActiveRPC_InheritedParentSandboxUsesDirectVerifiedImage(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("inherited parent sandbox is Darwin-only")
	}
	session, _, _ := pipelineOMPActiveRPCSessionFixtureWithSandbox(
		t, false, pipelineOMPActiveSandboxInheritedParent,
	)
	t.Cleanup(func() { require.NoError(t, session.Close()) })

	output, receipt, err := session.Execute(context.Background(), "safe inherited prompt")

	require.NoError(t, err)
	assert.Equal(t, "safe assistant output 1", output)
	assert.True(t, receipt.SameProcess)
}

func pipelineOMPActiveRPCSessionFixture(
	t *testing.T,
	unsafe bool,
) (*pipelineOMPActiveEvaluatorSession, pipelineOMPBackendConfig, string) {
	return pipelineOMPActiveRPCSessionFixtureWithSandbox(t, unsafe, pipelineOMPActiveSandboxManaged)
}

func pipelineOMPActiveRPCSessionFixtureWithSandbox(
	t *testing.T,
	unsafe bool,
	sandboxMode pipelineOMPActiveSandboxMode,
) (*pipelineOMPActiveEvaluatorSession, pipelineOMPBackendConfig, string) {
	t.Helper()
	requireDarwinManagedOMPSandboxForTest(t)
	config, _ := pipelineOMPBackendTestConfig(t)
	config.Executable = os.Args[0]
	model := "model-a"
	if unsafe {
		model = "model-unsafe"
	}
	logPath := filepath.Join(config.ProjectDir, "active-native-rpc.jsonl")
	config.PhaseModels = map[pipeline.PhaseID]string{pipeline.PhasePlan: "provider-a/" + model}
	config.Environment = append(config.Environment,
		pipelineOMPActiveEndpointKey+"=http://127.0.0.1:43123",
		pipelineOMPActiveCredentialKey+"=fixture-token-value",
	)
	config, err := normalizePipelineOMPBackendConfig(config)
	require.NoError(t, err)
	snapshot := pipeline.OMPExecutionSnapshot{
		ProjectDir: config.ProjectDir, SpecID: config.SpecID, SpecDir: config.SpecDir,
		SnapshotHash: config.SnapshotHash, GitCommitHash: config.GitCommitHash,
		PhaseID: pipeline.PhasePlan, Attempt: 1, Prompt: "canonical", ActivePrompt: "active",
	}
	candidate, err := newPipelineOMPManagedActiveCandidate(
		snapshot, config.PhaseModels[pipeline.PhasePlan], config.PhaseModels,
	)
	require.NoError(t, err)
	prepared := pipelineOMPManagedActivePrepared{Binding: pipelineOMPActiveLeaseBinding{
		GrantDigest: workflowContextRuntimeHash("grant"), WorkspaceID: "autopus-adk",
		SpecID: config.SpecID, GitCommitHash: config.GitCommitHash,
		ModelScopeDigest: candidate.ModelScopeDigest,
	}}
	session, err := startPipelineOMPActiveEvaluatorSession(
		context.Background(), config, candidate, prepared, true, sandboxMode,
	)
	require.NoError(t, err)
	return session, config, logPath
}

func pipelineOMPActiveNativeFixture(args []string) (string, string, bool) {
	values := make(map[string]string)
	for index := 0; index+1 < len(args); index++ {
		switch args[index] {
		case "--mode", "--cwd", "--model":
			values[args[index]] = args[index+1]
			index++
		}
	}
	if values["--mode"] != "rpc" || values["--cwd"] == "" || !strings.Contains(values["--model"], "/model-") {
		return "", "", false
	}
	unsafe := "0"
	if strings.HasSuffix(values["--model"], "/model-unsafe") {
		unsafe = "1"
	}
	return filepath.Join(values["--cwd"], "active-native-rpc.jsonl"), unsafe, true
}

func runPipelineOMPActiveRPCFixture() int {
	logFile, err := os.OpenFile(os.Getenv("AUTOPUS_TEST_OMP_ACTIVE_LOG"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return 81
	}
	defer logFile.Close()
	logEncoder, output := json.NewEncoder(logFile), json.NewEncoder(os.Stdout)
	_ = logEncoder.Encode(pipelineOMPRPCRecord{Kind: "start", PID: os.Getpid(), Args: os.Args})
	_ = output.Encode(map[string]any{"type": "ready"})
	messageCount, promptCount := 0, 0
	inputTokens, outputTokens := int64(0), int64(0)
	transcript := []json.RawMessage{json.RawMessage(`{"role":"system","content":"safe system context"}`)}
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		var command pipelineOMPRPCCommand
		if json.Unmarshal(scanner.Bytes(), &command) != nil {
			return 82
		}
		_ = logEncoder.Encode(pipelineOMPRPCRecord{
			Kind: "command", ID: command.ID, Type: command.Type, Enabled: command.Enabled,
			Provider: command.Provider, ModelID: command.ModelID, Message: command.Message,
			Protocol: command.ProtocolVersion,
		})
		switch command.Type {
		case "negotiate_protocol":
			writePipelineOMPActiveResponse(output, command, map[string]any{"protocolVersion": 2})
		case "get_state":
			writePipelineOMPActiveResponse(output, command, map[string]any{
				"sessionId": "active-session", "isStreaming": false, "isCompacting": false,
				"messageCount": messageCount, "queuedMessageCount": 0, "autoCompactionEnabled": false,
			})
		case "get_session_stats":
			writePipelineOMPActiveResponse(output, command, map[string]any{
				"sessionId": "active-session", "tokens": map[string]any{
					"input": inputTokens, "output": outputTokens, "total": inputTokens + outputTokens,
				},
			})
		case "prompt":
			promptCount++
			inputTokens += int64(100 - promptCount*10)
			outputTokens += 10
			messageCount += 2
			assistant := fmt.Sprintf("safe assistant output %d", promptCount)
			if os.Getenv("AUTOPUS_TEST_OMP_ACTIVE_UNSAFE") == "1" {
				assistant = "Authorization: Bearer abcdefghijklmnop"
			}
			transcript = append(transcript,
				json.RawMessage(fmt.Sprintf(`{"role":"user","content":%q}`, command.Message)),
				json.RawMessage(fmt.Sprintf(`{"role":"assistant","content":%q}`, assistant)),
			)
			writePipelineOMPActiveResponse(output, command, nil)
			_ = output.Encode(map[string]any{"type": "agent_start"})
			_ = output.Encode(map[string]any{"type": "turn_start"})
			_ = output.Encode(map[string]any{"type": "turn_end"})
			_ = output.Encode(map[string]any{"type": "agent_end", "isTerminal": true})
		case "get_last_assistant_text":
			text := fmt.Sprintf("safe assistant output %d", promptCount)
			if os.Getenv("AUTOPUS_TEST_OMP_ACTIVE_UNSAFE") == "1" {
				text = "Authorization: Bearer abcdefghijklmnop"
			}
			writePipelineOMPActiveResponse(output, command, map[string]any{"text": text})
		case "get_messages_page":
			writePipelineOMPActiveResponse(output, command, pipelineOMPActiveMessagesPage{
				Messages: transcript, TotalMessages: len(transcript), NextCursor: nil,
			})
		case "compact":
			inputTokens += 20
			outputTokens += 5
			writePipelineOMPActiveCompaction(output, command)
		default:
			writePipelineOMPActiveResponse(output, command, nil)
		}
	}
	if scanner.Err() != nil {
		return 83
	}
	return 0
}

func writePipelineOMPActiveResponse(output *json.Encoder, command pipelineOMPRPCCommand, data any) {
	_ = output.Encode(map[string]any{
		"id": command.ID, "type": "response", "command": command.Type, "success": true, "data": data,
	})
}

func writePipelineOMPActiveCompaction(output *json.Encoder, command pipelineOMPRPCCommand) {
	binding := WorkflowContextBridgeBinding{
		SchemaVersion: workflowContextBridgeSchemaVersion,
		BindingHash:   os.Getenv("AUTOPUS_OMP_CONTEXT_BINDING_HASH"),
		OptionsHash:   os.Getenv("AUTOPUS_OMP_CONTEXT_OPTIONS_HASH"),
		SessionHash:   os.Getenv("AUTOPUS_OMP_CONTEXT_SESSION_HASH"),
		NonceHash:     os.Getenv("AUTOPUS_OMP_CONTEXT_NONCE_HASH"),
	}
	_ = output.Encode(map[string]any{"type": "auto_compaction_start", "reason": "manual", "action": "snapcompact"})
	for index, event := range []string{WorkflowContextEventPreCompaction, WorkflowContextEventPostCompaction} {
		envelope, _ := json.Marshal(workflowContextManagedBridgeEnvelope{
			SchemaVersion: binding.SchemaVersion, Event: event, BindingHash: binding.BindingHash,
			OptionsHash: binding.OptionsHash, SessionHash: binding.SessionHash, NonceHash: binding.NonceHash,
		})
		message, _ := json.Marshal(string(envelope))
		_ = output.Encode(map[string]any{
			"id": fmt.Sprintf("active-ack-%d", index+1), "type": "extension_ui_request", "method": "confirm",
			"title": "Autopus context " + event, "message": json.RawMessage(message),
		})
	}
	writePipelineOMPActiveResponse(output, command, map[string]any{})
	_ = output.Encode(map[string]any{
		"type": "auto_compaction_end", "reason": "manual", "action": "snapcompact",
		"result": map[string]any{"summary": "safe compacted context"},
	})
}
