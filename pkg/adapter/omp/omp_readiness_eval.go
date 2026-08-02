package omp

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"github.com/insajin/autopus-adk/pkg/detect"
	"github.com/insajin/autopus-adk/pkg/processprobe"
)

type ompCatalogModel struct {
	provider  string
	id        string
	available *bool
}

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

func evaluateOMPConfigCapability(result ompProbeResult) OMPCapabilityResult {
	capability := OMPCapabilityResult{ID: "config.overlay_readback", Reason: result.reason}
	if result.reason != "" {
		if result.reason == "output_oversized" {
			capability.Reason = "output_invalid"
		}
		return capability
	}
	directories, ok := parseOMPConfigDirectories(result.output)
	if !ok || len(directories) != 1 || directories[0] != ".agents/skills" {
		capability.Reason = "output_invalid"
		return capability
	}
	capability.Supported = true
	capability.Reason = "output_valid"
	return capability
}

func parseOMPConfigDirectories(data []byte) ([]string, bool) {
	var direct []string
	if decodeOMPExactJSON(data, &direct) {
		return direct, true
	}

	var wrapper map[string]json.RawMessage
	if !decodeOMPExactJSON(data, &wrapper) || len(wrapper) != 4 {
		return nil, false
	}
	for _, key := range []string{"key", "value", "type", "description"} {
		if _, exists := wrapper[key]; !exists {
			return nil, false
		}
	}
	var key, valueType, description string
	var directories []string
	if !decodeOMPExactJSON(wrapper["key"], &key) || key != "skills.customDirectories" ||
		!decodeOMPExactJSON(wrapper["type"], &valueType) || valueType != "array" ||
		!decodeOMPExactJSON(wrapper["description"], &description) || description != "" ||
		!decodeOMPExactJSON(wrapper["value"], &directories) {
		return nil, false
	}
	return directories, true
}

func evaluateOMPCatalog(result ompProbeResult) ([]ompCatalogModel, string, OMPCapabilityResult) {
	capability := OMPCapabilityResult{ID: "catalog.models_json", Reason: result.reason}
	if result.reason != "" {
		switch result.reason {
		case "timeout":
			return nil, "catalog_timeout", capability
		case "exit_nonzero":
			return nil, "catalog_exit_nonzero", capability
		default:
			capability.Reason = "output_invalid"
			return nil, "catalog_oversized", capability
		}
	}
	models, ok := parseOMPModelCatalog(result.output)
	if !ok {
		capability.Reason = "output_invalid"
		return nil, "catalog_invalid", capability
	}
	capability.Supported = true
	capability.Reason = "output_valid"
	if len(models) == 0 {
		return nil, "catalog_empty", capability
	}
	return models, "catalog_ready", capability
}

func parseOMPModelCatalog(data []byte) ([]ompCatalogModel, bool) {
	var top map[string]json.RawMessage
	if !decodeOMPExactJSON(data, &top) || len(top) != 1 {
		return nil, false
	}
	rawModels, ok := top["models"]
	if !ok {
		return nil, false
	}
	var entries []map[string]json.RawMessage
	if !decodeOMPExactJSON(rawModels, &entries) {
		return nil, false
	}
	models := make([]ompCatalogModel, 0, len(entries))
	for _, entry := range entries {
		var provider, id string
		if raw, found := entry["provider"]; !found || json.Unmarshal(raw, &provider) != nil || provider == "" {
			return nil, false
		}
		if raw, found := entry["id"]; !found || json.Unmarshal(raw, &id) != nil || id == "" {
			return nil, false
		}
		model := ompCatalogModel{provider: provider, id: id}
		if raw, found := entry["available"]; found {
			var available bool
			if json.Unmarshal(raw, &available) != nil {
				return nil, false
			}
			model.available = &available
		}
		models = append(models, model)
	}
	return models, true
}

func resolveOMPSelectors(selectors []string, models []ompCatalogModel, catalogReason string) []OMPSelectorResolution {
	results := make([]OMPSelectorResolution, 0, len(selectors))
	for _, selector := range selectors {
		result := OMPSelectorResolution{Selector: selector, Status: "unresolved", Reason: "selector_unresolved"}
		if strings.HasPrefix(selector, "/") || strings.HasSuffix(selector, "/") || strings.Count(selector, "/") > 1 {
			result.Status, result.Reason = "invalid", "selector_malformed"
			results = append(results, result)
			continue
		}
		for _, model := range models {
			canonical := model.provider + "/" + model.id
			if selector != canonical && selector != model.id {
				continue
			}
			result.ResolvedModel, result.Status, result.Reason = canonical, "resolved", "available"
			if model.available != nil && !*model.available {
				result.Reason = "credential_unavailable"
			}
			break
		}
		if result.Status == "unresolved" && catalogReason != "catalog_ready" {
			result.Reason = catalogReason
		}
		results = append(results, result)
	}
	return results
}

func evaluateOMPRPCCapabilities(result ompProbeResult) []OMPCapabilityResult {
	ids := []string{"rpc.command_discovery", "rpc.tool_events", "rpc.terminal"}
	capabilities := make([]OMPCapabilityResult, 0, len(ids))
	if result.reason != "" && result.reason != "timeout" && result.reason != "exit_nonzero" {
		reason := result.reason
		if reason == "output_oversized" {
			reason = "output_invalid"
		}
		for _, id := range ids {
			capabilities = append(capabilities, OMPCapabilityResult{ID: id, Reason: reason})
		}
		return capabilities
	}
	discovery, pairedTools, terminal, valid := parseOMPRPCEvents(result.output)
	if !valid {
		for _, id := range ids {
			capabilities = append(capabilities, OMPCapabilityResult{ID: id, Reason: "output_invalid"})
		}
		return capabilities
	}
	for index, observed := range []bool{discovery, pairedTools, terminal} {
		reason := "event_missing"
		if observed {
			reason = "event_observed"
			if result.reason != "" {
				reason += "_partial_" + result.reason
			}
		} else if result.reason != "" {
			reason = result.reason
		}
		capabilities = append(capabilities, OMPCapabilityResult{
			ID: ids[index], Supported: observed && result.reason == "", Reason: reason,
		})
	}
	return capabilities
}

func parseOMPRPCEvents(data []byte) (discovery, pairedTools, terminal, valid bool) {
	valid = true
	starts := make(map[string]bool)
	for _, line := range bytes.Split(data, []byte{'\n'}) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var event struct {
			Type       string `json:"type"`
			ToolCallID string `json:"toolCallId"`
		}
		if json.Unmarshal(line, &event) != nil || event.Type == "" {
			valid = false
			continue
		}
		switch event.Type {
		case "available_commands_update":
			discovery = true
		case "tool_execution_start":
			if event.ToolCallID != "" {
				starts[event.ToolCallID] = true
			}
		case "tool_execution_end":
			pairedTools = pairedTools || starts[event.ToolCallID]
		case "message_end":
			terminal = terminal || pairedTools
		}
	}
	return discovery, pairedTools, terminal, valid
}

func decodeOMPExactJSON(data []byte, target any) bool {
	decoder := json.NewDecoder(bytes.NewReader(data))
	if decoder.Decode(target) != nil {
		return false
	}
	var extra any
	return errors.Is(decoder.Decode(&extra), io.EOF)
}

func isOMPOutputLimitError(err error) bool {
	return errors.Is(err, processprobe.ErrOutputLimit)
}
