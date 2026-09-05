package cli

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const ompReviewRPCFixtureEnv = "AUTOPUS_TEST_OMP_REVIEW_RPC"

type ompReviewRPCRecord struct {
	Kind     string   `json:"kind"`
	PID      int      `json:"pid,omitempty"`
	Args     []string `json:"args,omitempty"`
	CWD      string   `json:"cwd,omitempty"`
	Path     string   `json:"path,omitempty"`
	Content  string   `json:"content,omitempty"`
	Mode     uint32   `json:"mode,omitempty"`
	ID       string   `json:"id,omitempty"`
	Type     string   `json:"type,omitempty"`
	Enabled  *bool    `json:"enabled,omitempty"`
	Provider string   `json:"provider,omitempty"`
	ModelID  string   `json:"modelId,omitempty"`
	Level    string   `json:"level,omitempty"`
	Message  string   `json:"message,omitempty"`
	Protocol int      `json:"protocolVersion,omitempty"`
}

func configureOMPReviewRPCFixture(t *testing.T, mode string) (string, string) {
	t.Helper()
	fixtureDir := t.TempDir()
	logPath := filepath.Join(fixtureDir, "review-rpc.jsonl")
	executable := filepath.Join(fixtureDir, "omp")
	script := "#!/bin/sh\nexec " + shellQuotePipelineOMP(os.Args[0]) + " -test.run=^$ -- \"$@\"\n"
	require.NoError(t, os.WriteFile(executable, []byte(script), 0o700))
	t.Setenv("PATH", fixtureDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv(ompReviewRPCFixtureEnv, "1")
	t.Setenv("AUTOPUS_TEST_OMP_REVIEW_LOG", logPath)
	t.Setenv("AUTOPUS_TEST_OMP_REVIEW_MODE", mode)
	return t.TempDir(), logPath
}

func ompReviewFixtureFlagValue(args []string, flag string) string {
	for index, arg := range args {
		if arg == flag && index+1 < len(args) {
			return args[index+1]
		}
	}
	return ""
}

func runOMPReviewRPCFixture() int {
	logFile, err := os.OpenFile(os.Getenv("AUTOPUS_TEST_OMP_REVIEW_LOG"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return 81
	}
	defer logFile.Close()
	cwd, err := os.Getwd()
	if err != nil {
		return 82
	}
	logEncoder, output := json.NewEncoder(logFile), json.NewEncoder(os.Stdout)
	_ = logEncoder.Encode(ompReviewRPCRecord{Kind: "start", PID: os.Getpid(), Args: os.Args, CWD: cwd})
	overlayPath := ompReviewFixtureFlagValue(os.Args, "--config")
	overlay, err := os.ReadFile(overlayPath)
	if err != nil {
		return 85
	}
	overlayInfo, err := os.Lstat(overlayPath)
	if err != nil {
		return 86
	}
	_ = logEncoder.Encode(ompReviewRPCRecord{
		Kind: "overlay", Path: overlayPath, Content: string(overlay), Mode: uint32(overlayInfo.Mode().Perm()),
	})
	_ = output.Encode(map[string]any{"type": "ready"})

	mode, messageCount := os.Getenv("AUTOPUS_TEST_OMP_REVIEW_MODE"), 0
	var pinnedProvider, pinnedModel string
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		var command ompReviewRPCRecord
		if json.Unmarshal(scanner.Bytes(), &command) != nil {
			return 83
		}
		command.Kind = "command"
		_ = logEncoder.Encode(command)
		switch command.Type {
		case "negotiate_protocol":
			writeOMPReviewRPCResponse(output, command, true, map[string]any{"protocolVersion": 2}, "")
		case "set_model":
			if mode == "set-model-error" {
				writeOMPReviewRPCResponse(output, command, false, nil, "model unavailable")
				continue
			}
			pinnedProvider, pinnedModel = command.Provider, command.ModelID
			writeOMPReviewRPCResponse(output, command, true, nil, "")
		case "prompt":
			if mode == "hang-prompt" {
				continue
			}
			messageCount = 2
			if mode == "bare-ack" {
				// omp 18.1.x: bare success, no prompt_result, agent_end without isTerminal.
				writeOMPReviewRPCResponse(output, command, true, nil, "")
				_ = output.Encode(map[string]any{"type": "agent_start"})
				_ = output.Encode(map[string]any{"type": "agent_end", "messages": []any{}})
				continue
			}
			writeOMPReviewRPCResponse(output, command, true, map[string]any{"agentInvoked": true}, "")
			_ = output.Encode(map[string]any{"id": command.ID, "type": "prompt_result", "agentInvoked": true})
			_ = output.Encode(map[string]any{"type": "agent_start"})
			_ = output.Encode(map[string]any{
				"type": "agent_end", "isTerminal": true, "messages": ompReviewFixtureTurnMessages(mode, pinnedProvider, pinnedModel),
			})
		case "get_state":
			// Mirror the built-in tools the launcher asked for; a leaked MCP tool
			// models the discovery paths --tools does not cover.
			var dumpTools []map[string]any
			for i, arg := range os.Args {
				if arg == "--tools" && i+1 < len(os.Args) {
					for _, name := range strings.Split(os.Args[i+1], ",") {
						dumpTools = append(dumpTools, map[string]any{"name": name})
					}
				}
			}
			if mode == "leaked-tool" {
				dumpTools = append(dumpTools, map[string]any{"name": "mcp__filesystem_delete"})
			}
			writeOMPReviewRPCResponse(output, command, true, map[string]any{
				"sessionId": "review-session", "isStreaming": false, "isCompacting": false,
				"messageCount": messageCount, "queuedMessageCount": 0, "dumpTools": dumpTools,
			}, "")
		case "get_last_assistant_text":
			text := "review output"
			if mode == "empty" {
				text = ""
			}
			writeOMPReviewRPCResponse(output, command, true, map[string]any{"text": text}, "")
		default:
			writeOMPReviewRPCResponse(output, command, true, nil, "")
		}
	}
	if scanner.Err() != nil {
		return 84
	}
	return 0
}

// ompReviewFixtureTurnMessages mirrors omp 18.1.x agent_end.messages: the
// assistant entry carries the executed provider/model and the stop reason.
// "provider-error-once" fails the first process only, using a marker file so
// the retry (a fresh process) can observe the earlier attempt.
func ompReviewFixtureTurnMessages(mode, provider, model string) []map[string]any {
	assistant := map[string]any{
		"role": "assistant", "provider": provider, "model": model, "stopReason": "stop",
		"content": []map[string]any{{"type": "text", "text": "review output"}},
	}
	switch mode {
	case "provider-error":
		assistant["stopReason"], assistant["errorStatus"], assistant["errorMessage"] = "error", 404, "404 model: gone"
	case "provider-error-once":
		marker := filepath.Join(filepath.Dir(os.Getenv("AUTOPUS_TEST_OMP_REVIEW_LOG")), "first-attempt-failed")
		if _, err := os.Stat(marker); err != nil {
			_ = os.WriteFile(marker, nil, 0o600)
			assistant["stopReason"], assistant["errorStatus"], assistant["errorMessage"] = "error", 529, "overloaded_error"
		}
	case "model-drift":
		assistant["model"] = "claude-sonnet-5"
	}
	return []map[string]any{
		{"role": "user", "content": []map[string]any{{"type": "text", "text": "prompt"}}},
		assistant,
	}
}

func writeOMPReviewRPCResponse(output *json.Encoder, command ompReviewRPCRecord, success bool, data any, message string) {
	_ = output.Encode(map[string]any{
		"id": command.ID, "type": "response", "command": command.Type,
		"success": success, "data": data, "error": message,
	})
}

func readOMPReviewRPCRecords(t *testing.T, path string) []ompReviewRPCRecord {
	t.Helper()
	file, err := os.Open(path)
	require.NoError(t, err)
	defer file.Close()
	var records []ompReviewRPCRecord
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var record ompReviewRPCRecord
		require.NoError(t, json.Unmarshal(scanner.Bytes(), &record))
		records = append(records, record)
	}
	require.NoError(t, scanner.Err())
	return records
}
