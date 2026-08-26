package adapter_test

import (
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

	"github.com/insajin/autopus-adk/pkg/adapter/omp"
)

func assertOMPReadinessProviderFree(t *testing.T, root string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("provider-free stdin capture fixture requires a POSIX executable")
	}
	argsPath := filepath.Join(root, "readiness-args.txt")
	inputPath := filepath.Join(root, "readiness-input.jsonl")
	executable := filepath.Join(root, "omp-readiness-fixture")
	rpcOutput := strings.Join([]string{
		`{"type":"ready","protocolVersion":1,"supportedProtocolVersions":[1,2]}`,
		`{"id":"readiness-negotiate","type":"response","command":"negotiate_protocol","success":true,"data":{"protocolVersion":2}}`,
		`{"id":"readiness-state","type":"response","command":"get_state","success":true,"data":{"sessionId":"readiness-contract","isStreaming":false,"isCompacting":false,"messageCount":0,"queuedMessageCount":0,"dumpTools":[{"name":"task","parameters":{"type":"object","properties":{"context":{"type":"string"},"tasks":{"type":"array"}}}},{"name":"hub","parameters":{"type":"object"}},{"name":"todo","parameters":{"type":"object"}}]}}`,
		`{"id":"readiness-commands","type":"response","command":"get_available_commands","success":true,"data":{"commands":[{"name":"auto","source":"project"},{"name":"auto-plan","source":"project"}]}}`,
		`{"id":"readiness-subscribe","type":"response","command":"set_subagent_subscription","success":true}`,
		`{"id":"readiness-unsubscribe","type":"response","command":"set_subagent_subscription","success":true}`,
	}, "\n")
	script := fmt.Sprintf(`#!/bin/sh
printf '%%s\n' "$*" >> %q
case "$*" in
  "--version") printf 'omp/18.0.5\n' ;;
  "--help") printf '%%s\n' '--mode rpc --no-session --cwd --tools --no-extensions --no-rules --no-lsp --no-pty' ;;
  "config get tools.intentTracing --json") printf 'true\n' ;;
  *)
    cat > %q
    cat <<'AUTOPUS_RPC'
%s
AUTOPUS_RPC
    ;;
esac
`, argsPath, inputPath, rpcOutput)
	require.NoError(t, os.WriteFile(executable, []byte(script), 0o755))

	report := omp.ProbeOMPReadiness(context.Background(), omp.OMPReadinessOptions{
		Executable: executable,
		Root:       root,
	})
	require.Equal(t, "omp/18.0.5", report.Version)
	for _, capability := range report.Capabilities {
		assert.True(t, capability.Supported, "%s: %s", capability.ID, capability.Reason)
	}

	args, err := os.ReadFile(argsPath)
	require.NoError(t, err)
	input, err := os.ReadFile(inputPath)
	require.NoError(t, err)
	argsText := strings.ToLower(string(args))
	inputText := strings.ToLower(string(input))
	assert.Contains(t, argsText, "--model openai-codex/gpt-5.6-sol")
	for _, forbidden := range []string{"prompt", "provider", "write"} {
		assert.NotContains(t, argsText+"\n"+inputText, forbidden)
	}
	assert.NotContains(t, inputText, "model")

	var types []string
	for _, line := range strings.Split(strings.TrimSpace(string(input)), "\n") {
		var frame map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &frame))
		types = append(types, frame["type"].(string))
	}
	assert.Equal(t, []string{
		"negotiate_protocol", "get_state", "get_available_commands",
		"set_subagent_subscription", "set_subagent_subscription",
	}, types)
}
