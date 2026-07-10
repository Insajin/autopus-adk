package codex

import "strings"

var userOwnedCodexConfigKeys = map[string]map[string]bool{
	"": {
		"model":                   true,
		"model_reasoning_effort":  true,
		"model_reasoning_summary": true,
		"model_verbosity":         true,
	},
}

// preserveUserCodexModelSettings keeps user-selected model controls while
// allowing Autopus-managed harness keys, MCP servers, hooks, and feature flags
// to refresh from the current template.
func preserveUserCodexModelSettings(rendered, existing string) string {
	overrides := collectCodexConfigOverrides(existing)
	if len(overrides) == 0 {
		return rendered
	}

	var section string
	lines := strings.Split(rendered, "\n")
	applied := make(map[string]bool, len(overrides))
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if parsedSection, ok := parseCodexConfigSection(trimmed); ok {
			section = parsedSection
			continue
		}
		key, _, ok := parseCodexConfigAssignment(trimmed)
		if !ok || !isUserOwnedCodexConfigKey(section, key) {
			continue
		}
		overrideKey := section + "." + key
		if replacement, ok := overrides[overrideKey]; ok {
			lines[i] = replaceCodexConfigAssignmentValue(line, replacement)
			applied[overrideKey] = true
		}
	}

	var missingRoot []string
	for _, key := range []string{"model", "model_reasoning_effort", "model_reasoning_summary", "model_verbosity"} {
		overrideKey := "." + key
		if value, ok := overrides[overrideKey]; ok && !applied[overrideKey] {
			missingRoot = append(missingRoot, key+" = "+value)
		}
	}
	if len(missingRoot) > 0 {
		insertAt := len(lines)
		for i, line := range lines {
			if _, ok := parseCodexConfigSection(strings.TrimSpace(line)); ok {
				insertAt = i
				break
			}
		}
		missingRoot = append(missingRoot, "")
		lines = append(lines[:insertAt], append(missingRoot, lines[insertAt:]...)...)
	}
	return strings.Join(lines, "\n")
}

func collectCodexConfigOverrides(content string) map[string]string {
	overrides := make(map[string]string)
	var section string
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if parsedSection, ok := parseCodexConfigSection(trimmed); ok {
			section = parsedSection
			continue
		}
		key, value, ok := parseCodexConfigAssignment(trimmed)
		if !ok || !isUserOwnedCodexConfigKey(section, key) {
			continue
		}
		overrides[section+"."+key] = value
	}
	return overrides
}

func isUserOwnedCodexConfigKey(section, key string) bool {
	keys, ok := userOwnedCodexConfigKeys[section]
	return ok && keys[key]
}

func parseCodexConfigSection(trimmed string) (string, bool) {
	if !strings.HasPrefix(trimmed, "[") || !strings.HasSuffix(trimmed, "]") {
		return "", false
	}
	section := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(trimmed, "["), "]"))
	if section == "" || strings.Contains(section, "[") || strings.Contains(section, "]") {
		return "", false
	}
	return section, true
}

func parseCodexConfigAssignment(trimmed string) (string, string, bool) {
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return "", "", false
	}
	parts := strings.SplitN(trimmed, "=", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	key := strings.TrimSpace(parts[0])
	if key == "" || strings.ContainsAny(key, " \t") {
		return "", "", false
	}
	value := strings.TrimSpace(parts[1])
	if value == "" {
		return "", "", false
	}
	return key, value, true
}

func replaceCodexConfigAssignmentValue(line, value string) string {
	prefix, _, ok := strings.Cut(line, "=")
	if !ok {
		return line
	}
	return prefix + "= " + value
}
