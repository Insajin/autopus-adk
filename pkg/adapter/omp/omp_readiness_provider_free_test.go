package omp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type providerFreeReadinessRunner struct {
	calls   [][]string
	outputs map[string][]byte
}

func (runner *providerFreeReadinessRunner) Run(_ context.Context, executable string, args ...string) ([]byte, error) {
	call := append([]string{executable}, args...)
	runner.calls = append(runner.calls, call)
	return runner.outputs[strings.Join(args, " ")], nil
}

func TestProbeOMPReadiness_UsesOnlyProviderFreeMetadataAndRPCState(t *testing.T) {
	runner := &providerFreeReadinessRunner{outputs: map[string][]byte{
		"--version":                             []byte("omp/18.0.5\n"),
		"--help":                                []byte("--mode rpc --no-session --cwd --tools --no-extensions --no-rules --no-lsp --no-pty\n"),
		"config get tools.intentTracing --json": []byte("true\n"),
		"--mode rpc --no-session --cwd . --model openai-codex/gpt-5.6-sol --tools task,hub,todo --no-extensions --no-rules --no-lsp --no-pty": providerFreeRPCFixture(t),
	}}

	report := ProbeOMPReadiness(context.Background(), OMPReadinessOptions{
		Executable: "omp", Root: ".", Runner: runner,
	})

	for _, id := range []string{
		"identity.version", "launch.rpc", "launch.no_session", "launch.cwd",
		"config.intent_tracing", "rpc.protocol_v2", "rpc.state",
		"rpc.commands", "rpc.dump_tools_pre_intent", "rpc.subagent_subscription",
	} {
		result := capabilityByID(t, report, id)
		assert.True(t, result.Supported, "%s: %s", id, result.Reason)
	}
	encoded, err := json.Marshal(runner.calls)
	require.NoError(t, err)
	wire := string(encoded)
	for _, forbidden := range []string{"prompt", "models", "provider", "write", "completion"} {
		assert.NotContains(t, wire, forbidden)
	}
	assert.Empty(t, report.SelectorResolutions)
	assert.Equal(t, "not_probed_provider_free", report.CatalogReason)
}

func TestOMPProviderFreeRPCInput_OnlyNegotiatesAndInspectsLocalState(t *testing.T) {
	input := ompProviderFreeRPCInput()
	assert.Less(t, len(input), 2048)
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
	wire := string(input)
	for _, forbidden := range []string{"prompt", "model", "provider", "write"} {
		assert.NotContains(t, wire, forbidden)
	}
}

func TestEvaluateOMPProviderFreeRPC_SeparatesPreIntentDumpFromEffectiveTracing(t *testing.T) {
	capabilities := evaluateOMPProviderFreeRPC(ompProbeResult{output: providerFreeRPCFixture(t)})

	dump := ompCapabilityFromSlice(t, capabilities, "rpc.dump_tools_pre_intent")
	assert.True(t, dump.Supported)
	assert.Equal(t, "pre_intent_schema_observed", dump.Reason)
	assert.NotContains(t, dump.Reason, "effective")
}

func TestEvaluateOMPProviderFreeRPC_FailsEachMissingResponseClosed(t *testing.T) {
	for _, test := range []struct {
		name, removeID, capability string
	}{
		{name: "negotiation", removeID: "readiness-negotiate", capability: "rpc.protocol_v2"},
		{name: "state", removeID: "readiness-state", capability: "rpc.state"},
		{name: "commands", removeID: "readiness-commands", capability: "rpc.commands"},
		{name: "subscription", removeID: "readiness-subscribe", capability: "rpc.subagent_subscription"},
		{name: "unsubscribe", removeID: "readiness-unsubscribe", capability: "rpc.subagent_subscription"},
	} {
		t.Run(test.name, func(t *testing.T) {
			lines := strings.Split(strings.TrimSpace(string(providerFreeRPCFixture(t))), "\n")
			filtered := lines[:0]
			for _, line := range lines {
				if !strings.Contains(line, `"id":"`+test.removeID+`"`) {
					filtered = append(filtered, line)
				}
			}
			capabilities := evaluateOMPProviderFreeRPC(ompProbeResult{output: []byte(strings.Join(filtered, "\n"))})
			assert.False(t, ompCapabilityFromSlice(t, capabilities, test.capability).Supported)
		})
	}
}

func TestEvaluateOMPProviderFreeRPC_RequiresZeroMessages(t *testing.T) {
	fixture := string(providerFreeRPCFixture(t))
	nonEmpty := strings.Replace(fixture, `"messageCount":0`, `"messageCount":1`, 1)
	require.NotEqual(t, fixture, nonEmpty)

	capabilities := evaluateOMPProviderFreeRPC(ompProbeResult{output: []byte(nonEmpty)})
	assert.False(t, ompCapabilityFromSlice(t, capabilities, "rpc.state").Supported)
	assert.False(t, ompCapabilityFromSlice(t, capabilities, "rpc.dump_tools_pre_intent").Supported)
}

func TestEvaluateOMPProviderFreeRPC_RejectsWrongRequiredResponseCommand(t *testing.T) {
	fixture := string(providerFreeRPCFixture(t))
	wrongCommand := strings.Replace(fixture, `"command":"get_state"`, `"command":"prompt"`, 1)
	require.NotEqual(t, fixture, wrongCommand)

	capabilities := evaluateOMPProviderFreeRPC(ompProbeResult{output: []byte(wrongCommand)})
	for _, capability := range capabilities {
		assert.False(t, capability.Supported, "%s accepted prompt response", capability.ID)
		assert.Equal(t, "output_invalid", capability.Reason)
	}
}

func TestEvaluateOMPProviderFreeRPC_FailsClosedOnLifecycleFrames(t *testing.T) {
	for _, test := range []struct {
		name, frame string
	}{
		{name: "provider lifecycle", frame: `{"type":"agent_start"}`},
		{name: "turn lifecycle", frame: `{"type":"turn_start"}`},
		{name: "message lifecycle", frame: `{"type":"message_end"}`},
		{name: "tool lifecycle", frame: `{"type":"tool_execution_start"}`},
		{name: "unrelated response", frame: `{"id":"other","type":"response","command":"get_state","success":true}`},
		{name: "provider response", frame: `{"id":"prompt","type":"response","command":"prompt","success":true}`},
		{name: "duplicate probe response", frame: `{"id":"readiness-state","type":"response","command":"get_state","success":true}`},
		{name: "interactive extension UI", frame: `{"id":"auth","type":"extension_ui_request","method":"input","title":"credential"}`},
		{name: "confirm extension UI", frame: `{"id":"auth","type":"extension_ui_request","method":"confirm","title":"login"}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			output := append(providerFreeRPCFixture(t), []byte(test.frame+"\n")...)
			capabilities := evaluateOMPProviderFreeRPC(ompProbeResult{output: output})
			for _, capability := range capabilities {
				assert.False(t, capability.Supported, "%s accepted %s", capability.ID, test.name)
				assert.Equal(t, "output_invalid", capability.Reason)
			}
		})
	}
}

func TestEvaluateOMPProviderFreeRPC_AllowsCurrentFixtureAndStartupMetadata(t *testing.T) {
	output := append(providerFreeRPCFixture(t),
		[]byte("{\"type\":\"available_commands_update\",\"commands\":[]}\n"+
			"{\"type\":\"extension_ui_request\",\"id\":\"startup\",\"method\":\"setTitle\",\"title\":\"omp\"}\n")...)
	capabilities := evaluateOMPProviderFreeRPC(ompProbeResult{output: output})
	for _, capability := range capabilities {
		assert.True(t, capability.Supported, "%s: %s", capability.ID, capability.Reason)
	}
}

func providerFreeRPCFixture(t *testing.T) []byte {
	t.Helper()
	frames := []map[string]any{
		{"type": "ready", "protocolVersion": 1, "supportedProtocolVersions": []int{1, 2}},
		{"id": "readiness-negotiate", "type": "response", "command": "negotiate_protocol", "success": true, "data": map[string]any{"protocolVersion": 2}},
		{"id": "readiness-state", "type": "response", "command": "get_state", "success": true, "data": map[string]any{
			"sessionId": "provider-free-readiness", "isStreaming": false, "isCompacting": false,
			"messageCount": 0, "queuedMessageCount": 0,
			"dumpTools": []any{
				map[string]any{"name": "task", "description": "spawn", "parameters": map[string]any{"type": "object", "properties": map[string]any{"context": map[string]any{"type": "string"}, "tasks": map[string]any{"type": "array"}}}},
				map[string]any{"name": "hub", "description": "coordinate", "parameters": map[string]any{"type": "object"}},
				map[string]any{"name": "todo", "description": "progress", "parameters": map[string]any{"type": "object"}},
			},
		}},
		{"id": "readiness-commands", "type": "response", "command": "get_available_commands", "success": true, "data": map[string]any{"commands": []any{map[string]any{"name": "auto", "source": "project"}, map[string]any{"name": "auto-plan", "source": "project"}}}},
		{"id": "readiness-subscribe", "type": "response", "command": "set_subagent_subscription", "success": true},
		{"id": "readiness-unsubscribe", "type": "response", "command": "set_subagent_subscription", "success": true},
	}
	var body strings.Builder
	for _, frame := range frames {
		encoded, err := json.Marshal(frame)
		require.NoError(t, err)
		body.Write(encoded)
		body.WriteByte('\n')
	}
	return []byte(body.String())
}

func ompCapabilityFromSlice(t *testing.T, capabilities []OMPCapabilityResult, id string) OMPCapabilityResult {
	t.Helper()
	for _, capability := range capabilities {
		if capability.ID == id {
			return capability
		}
	}
	t.Fatalf("missing capability %s", id)
	return OMPCapabilityResult{}
}

func capabilityByID(t *testing.T, report OMPReadinessReport, id string) OMPCapabilityResult {
	t.Helper()
	return ompCapabilityFromSlice(t, report.Capabilities, id)
}
