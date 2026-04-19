package e2e

import "strings"

// MissingScenarioRequirements returns the normalized requirements not present
// in the available capability set.
func MissingScenarioRequirements(s Scenario, available []string) []string {
	required := splitScenarioCapabilities(s.Requires)
	if len(required) == 0 {
		return nil
	}

	availableSet := make(map[string]struct{}, len(available))
	for _, capability := range available {
		normalized := normalizeScenarioCapability(capability)
		if normalized == "" {
			continue
		}
		availableSet[normalized] = struct{}{}
	}

	missing := make([]string, 0, len(required))
	for _, capability := range required {
		if _, ok := availableSet[capability]; ok {
			continue
		}
		missing = append(missing, capability)
	}
	return missing
}

func splitScenarioCapabilities(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.EqualFold(raw, "N/A") {
		return nil
	}

	parts := strings.Split(raw, ",")
	capabilities := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		normalized := normalizeScenarioCapability(part)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		capabilities = append(capabilities, normalized)
	}
	return capabilities
}

func normalizeScenarioCapability(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || strings.EqualFold(value, "N/A") {
		return ""
	}
	return strings.ToLower(value)
}
