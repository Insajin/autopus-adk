package codex

import (
	"strconv"
	"strings"
)

func codexStringEquals(raw, want string) bool {
	value, ok := parseCodexStringValue(raw)
	return ok && value == want
}

func parseCodexStringValue(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}
	switch raw[0] {
	case '"':
		end := closingDoubleQuote(raw)
		if end < 0 {
			return "", false
		}
		value, err := strconv.Unquote(raw[:end+1])
		return value, err == nil
	case '\'':
		end := strings.IndexByte(raw[1:], '\'')
		if end < 0 {
			return "", false
		}
		return raw[1 : end+1], true
	default:
		return "", false
	}
}

func closingDoubleQuote(value string) int {
	escaped := false
	for i := 1; i < len(value); i++ {
		switch {
		case escaped:
			escaped = false
		case value[i] == '\\':
			escaped = true
		case value[i] == '"':
			return i
		}
	}
	return -1
}

func collectCodexConfigOverrides(content string) map[string]string {
	overrides := make(map[string]string)
	var section string
	var scan codexTOMLScanState
	for _, line := range strings.Split(content, "\n") {
		if scan.skipSyntaxLine(line) {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if parsedSection, ok := parseCodexConfigSection(trimmed); ok {
			section = parsedSection
			continue
		}
		key, value, ok := parseCodexConfigAssignment(trimmed)
		if !ok {
			continue
		}
		scan.observeValue(value)
		if isUserOwnedCodexConfigKey(section, key) {
			overrides[section+"."+key] = value
		}
	}
	return overrides
}

func hasStandaloneCodexComment(content, comment string) bool {
	var scan codexTOMLScanState
	for _, line := range strings.Split(content, "\n") {
		if scan.skipSyntaxLine(line) {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == comment {
			return true
		}
		if _, value, ok := parseCodexConfigAssignment(trimmed); ok {
			scan.observeValue(value)
		}
	}
	return false
}

func markedCodexModelOverrides(content string, overrides map[string]string) (map[string]string, bool) {
	var scan codexTOMLScanState
	for _, line := range strings.Split(content, "\n") {
		if scan.skipSyntaxLine(line) {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == codexUserModelMarker {
			return copyCodexOverrides(overrides), true
		}
		if strings.HasPrefix(trimmed, codexUserModelMarker+":") {
			marked := make(map[string]string)
			keyList := strings.TrimSpace(strings.TrimPrefix(trimmed, codexUserModelMarker+":"))
			for _, key := range strings.Split(keyList, ",") {
				key = strings.TrimSpace(key)
				if !isUserOwnedCodexConfigKey("", key) {
					continue
				}
				if value, ok := overrides["."+key]; ok {
					marked["."+key] = value
				}
			}
			return marked, true
		}
		if _, value, ok := parseCodexConfigAssignment(trimmed); ok {
			scan.observeValue(value)
		}
	}
	return nil, false
}

func copyCodexOverrides(overrides map[string]string) map[string]string {
	copied := make(map[string]string, len(overrides))
	for key, value := range overrides {
		copied[key] = value
	}
	return copied
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
	equals := strings.IndexByte(trimmed, '=')
	if equals < 0 {
		return "", "", false
	}
	key, ok := parseCodexConfigKey(strings.TrimSpace(trimmed[:equals]))
	if !ok {
		return "", "", false
	}
	value := strings.TrimSpace(trimmed[equals+1:])
	if value == "" {
		return "", "", false
	}
	return key, value, true
}

func parseCodexConfigKey(raw string) (string, bool) {
	if raw == "" {
		return "", false
	}
	if raw[0] == '"' {
		value, err := strconv.Unquote(raw)
		return value, err == nil && value != "" && !strings.Contains(value, ".")
	}
	if raw[0] == '\'' {
		if len(raw) < 2 || raw[len(raw)-1] != '\'' {
			return "", false
		}
		value := raw[1 : len(raw)-1]
		return value, value != "" && !strings.Contains(value, ".")
	}
	if strings.ContainsAny(raw, " \t.\"'") {
		return "", false
	}
	return raw, true
}

func replaceCodexConfigAssignmentValue(line, value string) string {
	prefix, _, ok := strings.Cut(line, "=")
	if !ok {
		return line
	}
	return prefix + "= " + value
}
