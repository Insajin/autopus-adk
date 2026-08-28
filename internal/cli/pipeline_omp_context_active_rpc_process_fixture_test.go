package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

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
	body, err := os.ReadFile(filepath.Join(os.Getenv("PI_CODING_AGENT_DIR"), "models.yml"))
	if err != nil {
		return 80
	}
	var catalog struct {
		Providers map[string]struct {
			Models []struct {
				ContextWindow int `yaml:"contextWindow"`
			} `yaml:"models"`
		} `yaml:"providers"`
	}
	if err := yaml.Unmarshal(body, &catalog); err != nil {
		return 80
	}
	modelCount := 0
	for _, provider := range catalog.Providers {
		for _, model := range provider.Models {
			if model.ContextWindow <= 0 {
				return 80
			}
			modelCount++
		}
	}
	if modelCount == 0 {
		return 80
	}
	logFile, err := os.OpenFile(os.Getenv("AUTOPUS_TEST_OMP_ACTIVE_LOG"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return 81
	}
	defer logFile.Close()
	logEncoder, output := json.NewEncoder(logFile), json.NewEncoder(os.Stdout)
	_ = logEncoder.Encode(pipelineOMPRPCRecord{Kind: "start", PID: os.Getpid(), Args: os.Args})
	_ = output.Encode(map[string]any{"type": "ready"})
	messageCount, promptCount, compactionCount := 0, 0, 0
	inputTokens, outputTokens, cacheReadTokens := int64(0), int64(0), int64(0)
	transcript := []json.RawMessage{json.RawMessage(`{"role":"system","content":"safe system context"}`)}
	provider, modelID := "", ""
	for index := 0; index+1 < len(os.Args); index++ {
		if os.Args[index] != "--model" {
			continue
		}
		provider, modelID, _ = strings.Cut(os.Args[index+1], "/")
		break
	}
	if provider == "" || modelID == "" {
		return 80
	}
	modelInput := []string{"text", "image"}
	if modelID == "model-text-only" {
		modelInput = []string{"text"}
	}
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		var command pipelineOMPRPCCommand
		if json.Unmarshal(scanner.Bytes(), &command) != nil {
			return 82
		}
		_ = logEncoder.Encode(pipelineOMPRPCRecord{
			Kind: "command", PID: os.Getpid(), ID: command.ID, Type: command.Type, Enabled: command.Enabled,
			Provider: command.Provider, ModelID: command.ModelID, Message: command.Message,
			Protocol: command.ProtocolVersion,
		})
		switch command.Type {
		case "negotiate_protocol":
			writePipelineOMPActiveResponse(output, command, map[string]any{"protocolVersion": 2})
		case "set_model":
			provider, modelID = command.Provider, command.ModelID
			modelInput = []string{"text", "image"}
			if modelID == "model-text-only" {
				modelInput = []string{"text"}
			}
			writePipelineOMPActiveResponse(output, command, nil)
		case "get_state":
			writePipelineOMPActiveResponse(output, command, map[string]any{
				"sessionId": "active-session", "isStreaming": false, "isCompacting": false,
				"messageCount": messageCount, "queuedMessageCount": 0, "autoCompactionEnabled": false,
				"model": map[string]any{"provider": provider, "id": modelID, "input": modelInput},
			})
		case "get_session_stats":
			writePipelineOMPActiveResponse(output, command, map[string]any{
				"sessionId": "active-session", "tokens": map[string]any{
					"input": inputTokens, "output": outputTokens, "cacheRead": cacheReadTokens,
					"cacheWrite": 0, "total": inputTokens + outputTokens + cacheReadTokens,
				},
			})
		case "prompt":
			promptCount++
			if compactionCount > 0 {
				cacheReadTokens += 40
			} else {
				inputTokens += 100
			}
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
			_ = output.Encode(map[string]any{"type": "agent_start"})
			_ = output.Encode(map[string]any{"type": "turn_start"})
			writePipelineOMPActiveResponse(output, command, nil)
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
			compactionCount++
			inputTokens = 0
			outputTokens = 0
			cacheReadTokens = 0
			messageCount = 0
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
	legacyLifecycle := os.Getenv("AUTOPUS_TEST_OMP_ACTIVE_LEGACY_COMPACTION") == "1"
	if legacyLifecycle {
		_ = output.Encode(map[string]any{"type": "auto_compaction_start", "reason": "manual", "action": "snapcompact"})
	}
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
	writePipelineOMPActiveResponse(output, command, map[string]any{"summary": "safe compacted context"})
	if legacyLifecycle {
		_ = output.Encode(map[string]any{
			"type": "auto_compaction_end", "reason": "manual", "action": "snapcompact",
			"result": map[string]any{"summary": "safe compacted context"},
		})
	}
}
