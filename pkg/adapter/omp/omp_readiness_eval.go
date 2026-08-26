package omp

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/insajin/autopus-adk/pkg/detect"
)

func evaluateOMPVersion(result ompProbeResult) (OMPCapabilityResult, string) {
	capability := OMPCapabilityResult{ID: "identity.version", Reason: result.reason}
	if result.reason != "" {
		return capability, ""
	}
	version := strings.TrimSpace(string(result.output))
	if !detect.OMPVersionMatchesIdentity(version) {
		capability.Reason = "output_invalid"
		return capability, ""
	}
	capability.Supported = true
	capability.Reason = "version_verified"
	return capability, version
}

func evaluateOMPHelpCapability(id string, result ompProbeResult, needles ...string) OMPCapabilityResult {
	capability := OMPCapabilityResult{ID: id, Reason: result.reason}
	if result.reason != "" {
		if result.reason == "output_oversized" {
			capability.Reason = "output_invalid"
		}
		return capability
	}
	help := string(result.output)
	for _, needle := range needles {
		if !strings.Contains(help, needle) {
			capability.Reason = "flag_missing"
			return capability
		}
	}
	capability.Supported = true
	capability.Reason = "flag_present"
	return capability
}

func evaluateOMPIntentTracing(result ompProbeResult) OMPCapabilityResult {
	capability := OMPCapabilityResult{ID: "config.intent_tracing", Reason: result.reason}
	if result.reason != "" {
		if result.reason == "output_oversized" {
			capability.Reason = "output_invalid"
		}
		return capability
	}
	enabled, ok := parseOMPIntentTracing(result.output)
	if !ok {
		capability.Reason = "output_invalid"
		return capability
	}
	if !enabled {
		capability.Reason = "effective_intent_tracing_disabled"
		return capability
	}
	capability.Supported = true
	capability.Reason = "effective_intent_tracing_enabled"
	return capability
}

func parseOMPIntentTracing(data []byte) (bool, bool) {
	var direct bool
	if decodeOMPExactJSON(data, &direct) {
		return direct, true
	}
	var wrapper struct {
		Key         string `json:"key"`
		Value       *bool  `json:"value"`
		Type        string `json:"type"`
		Description string `json:"description"`
	}
	if !decodeOMPExactJSON(data, &wrapper) || wrapper.Key != "tools.intentTracing" ||
		wrapper.Value == nil || wrapper.Type != "boolean" {
		return false, false
	}
	return *wrapper.Value, true
}

type ompReadinessRPCFrame struct {
	ID                        string          `json:"id"`
	Type                      string          `json:"type"`
	Method                    string          `json:"method"`
	Command                   string          `json:"command"`
	Success                   bool            `json:"success"`
	ProtocolVersion           int             `json:"protocolVersion"`
	SupportedProtocolVersions []int           `json:"supportedProtocolVersions"`
	Data                      json.RawMessage `json:"data"`
}

type ompReadinessRPCObservation struct {
	ready, negotiated, state, commands, dumpTools, subscribed bool
}

func evaluateOMPProviderFreeRPC(result ompProbeResult) []OMPCapabilityResult {
	ids := []string{
		"rpc.protocol_v2", "rpc.state", "rpc.commands",
		"rpc.dump_tools_pre_intent", "rpc.subagent_subscription",
	}
	if result.reason != "" {
		reason := result.reason
		if reason == "output_oversized" {
			reason = "output_invalid"
		}
		return ompUnsupportedCapabilities(ids, reason)
	}
	observed, ok := parseOMPProviderFreeRPC(result.output)
	if !ok {
		return ompUnsupportedCapabilities(ids, "output_invalid")
	}
	values := []struct {
		ok     bool
		reason string
	}{
		{observed.ready && observed.negotiated, "protocol_v2_negotiated"},
		{observed.state, "idle_state_observed"},
		{observed.commands, "commands_observed"},
		{observed.dumpTools, "pre_intent_schema_observed"},
		{observed.subscribed, "subscription_round_trip_observed"},
	}
	capabilities := make([]OMPCapabilityResult, 0, len(ids))
	for index, value := range values {
		reason := "response_missing"
		if value.ok {
			reason = value.reason
		}
		capabilities = append(capabilities, OMPCapabilityResult{
			ID: ids[index], Supported: value.ok, Reason: reason,
		})
	}
	return capabilities
}

func parseOMPProviderFreeRPC(data []byte) (ompReadinessRPCObservation, bool) {
	observation := ompReadinessRPCObservation{}
	responses := make(map[string]ompReadinessRPCFrame)
	for _, line := range bytes.Split(data, []byte{'\n'}) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var frame ompReadinessRPCFrame
		if json.Unmarshal(line, &frame) != nil || frame.Type == "" {
			return observation, false
		}
		switch frame.Type {
		case "ready":
			if observation.ready || frame.ProtocolVersion != 1 ||
				!containsOMPProtocol(frame.SupportedProtocolVersions, 2) {
				return observation, false
			}
			observation.ready = true
		case "available_commands_update":
		case "extension_ui_request":
			if frame.ID == "" || frame.Method != "setStatus" && frame.Method != "setWidget" && frame.Method != "setTitle" {
				return observation, false
			}
		case "response":
			var command string
			switch frame.ID {
			case ompReadinessNegotiateID:
				command = "negotiate_protocol"
			case ompReadinessStateID:
				command = "get_state"
			case ompReadinessCommandsID:
				command = "get_available_commands"
			case ompReadinessSubscribeID, ompReadinessUnsubscribeID:
				command = "set_subagent_subscription"
			default:
				return observation, false
			}
			if frame.Command != command || responses[frame.ID].Type != "" {
				return observation, false
			}
			responses[frame.ID] = frame
		default:
			return observation, false
		}
	}
	negotiation, ok := successfulOMPResponse(responses, ompReadinessNegotiateID, "negotiate_protocol")
	if ok {
		var payload struct {
			ProtocolVersion int `json:"protocolVersion"`
		}
		observation.negotiated = json.Unmarshal(negotiation.Data, &payload) == nil && payload.ProtocolVersion == 2
	}
	state, ok := successfulOMPResponse(responses, ompReadinessStateID, "get_state")
	if ok {
		observation.state, observation.dumpTools = parseOMPReadinessState(state.Data)
	}
	commands, ok := successfulOMPResponse(responses, ompReadinessCommandsID, "get_available_commands")
	observation.commands = ok && parseOMPReadinessCommands(commands.Data)
	_, subscribed := successfulOMPResponse(responses, ompReadinessSubscribeID, "set_subagent_subscription")
	_, unsubscribed := successfulOMPResponse(responses, ompReadinessUnsubscribeID, "set_subagent_subscription")
	observation.subscribed = subscribed && unsubscribed
	return observation, true
}

func successfulOMPResponse(
	responses map[string]ompReadinessRPCFrame,
	id, command string,
) (ompReadinessRPCFrame, bool) {
	frame, ok := responses[id]
	return frame, ok && frame.Success && frame.Command == command
}

func parseOMPReadinessState(data []byte) (bool, bool) {
	var state struct {
		SessionID          string            `json:"sessionId"`
		IsStreaming        *bool             `json:"isStreaming"`
		IsCompacting       *bool             `json:"isCompacting"`
		MessageCount       *int              `json:"messageCount"`
		QueuedMessageCount *int              `json:"queuedMessageCount"`
		DumpTools          []json.RawMessage `json:"dumpTools"`
	}
	if json.Unmarshal(data, &state) != nil || strings.TrimSpace(state.SessionID) == "" ||
		state.IsStreaming == nil || state.IsCompacting == nil || state.MessageCount == nil ||
		state.QueuedMessageCount == nil || *state.IsStreaming || *state.IsCompacting ||
		*state.MessageCount != 0 || *state.QueuedMessageCount != 0 {
		return false, false
	}
	return true, parseOMPPreIntentTools(state.DumpTools)
}

func parseOMPPreIntentTools(rawTools []json.RawMessage) bool {
	found := map[string]bool{}
	for _, raw := range rawTools {
		var tool struct {
			Name       string         `json:"name"`
			Parameters map[string]any `json:"parameters"`
		}
		if json.Unmarshal(raw, &tool) != nil || tool.Name == "" || tool.Parameters == nil {
			return false
		}
		found[tool.Name] = true
		if tool.Name == "task" {
			properties, ok := tool.Parameters["properties"].(map[string]any)
			if !ok || properties["i"] != nil {
				return false
			}
			_, batchContext := properties["context"]
			_, batchTasks := properties["tasks"]
			_, flatTask := properties["task"]
			if !(batchContext && batchTasks) && !flatTask {
				return false
			}
		}
	}
	return found["task"] && found["hub"] && found["todo"]
}

func parseOMPReadinessCommands(data []byte) bool {
	var payload struct {
		Commands []struct {
			Name   string `json:"name"`
			Source string `json:"source"`
		} `json:"commands"`
	}
	if json.Unmarshal(data, &payload) != nil || len(payload.Commands) == 0 {
		return false
	}
	for _, command := range payload.Commands {
		if strings.TrimSpace(command.Name) == "" || strings.TrimSpace(command.Source) == "" {
			return false
		}
	}
	return true
}

func ompUnsupportedCapabilities(ids []string, reason string) []OMPCapabilityResult {
	capabilities := make([]OMPCapabilityResult, 0, len(ids))
	for _, id := range ids {
		capabilities = append(capabilities, OMPCapabilityResult{ID: id, Reason: reason})
	}
	return capabilities
}

func containsOMPProtocol(values []int, want int) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
