package config

import "strings"

const (
	TestProfileStandalone = "standalone"
	TestProfileLocal      = "local"
	TestProfileCI         = "ci"
	TestProfileProd       = "prod"
)

// TestProfileConf holds profile-specific E2E capabilities.
type TestProfileConf struct {
	Capabilities map[string][]string `yaml:"capabilities,omitempty"`
}

// IsValidTestProfile reports whether the profile is supported by `auto test run`.
func IsValidTestProfile(profile string) bool {
	switch normalizeTestProfile(profile) {
	case TestProfileStandalone, TestProfileLocal, TestProfileCI, TestProfileProd:
		return true
	default:
		return false
	}
}

// DefaultTestProfileCapabilities returns the built-in capabilities for a test profile.
func DefaultTestProfileCapabilities(profile string) []string {
	switch normalizeTestProfile(profile) {
	case TestProfileStandalone:
		return []string{TestProfileStandalone}
	case TestProfileLocal:
		return []string{
			TestProfileStandalone,
			TestProfileLocal,
			"docker",
			"db",
			"redis",
			"backend-server",
			"frontend-server",
			"network",
		}
	case TestProfileCI:
		return []string{
			TestProfileStandalone,
			TestProfileCI,
			"db",
			"redis",
			"backend-server",
			"frontend-server",
			"network",
		}
	case TestProfileProd:
		return []string{
			TestProfileStandalone,
			TestProfileProd,
			"db",
			"redis",
			"backend-server",
			"frontend-server",
			"network",
		}
	default:
		return nil
	}
}

// AvailableTestCapabilities returns the effective capabilities for a profile,
// merging built-in defaults with any autopus.yaml additions.
func (c *HarnessConfig) AvailableTestCapabilities(profile string) []string {
	capabilities := append([]string{}, DefaultTestProfileCapabilities(profile)...)
	if c == nil {
		return uniqueNormalizedCapabilities(capabilities)
	}
	capabilities = append(capabilities, c.Profiles.Test.Capabilities[normalizeTestProfile(profile)]...)
	return uniqueNormalizedCapabilities(capabilities)
}

func normalizeTestProfile(profile string) string {
	return strings.ToLower(strings.TrimSpace(profile))
}

func uniqueNormalizedCapabilities(values []string) []string {
	unique := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		normalized := strings.ToLower(strings.TrimSpace(value))
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		unique = append(unique, normalized)
	}
	return unique
}
