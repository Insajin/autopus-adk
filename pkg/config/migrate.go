// Package config provides autopus.yaml schema and migration utilities.
package config

// defaultProviderEntries holds the canonical default settings for known orchestra providers.
var defaultProviderEntries = map[string]ProviderEntry{
	"claude": {Binary: "claude", Args: []string{"--print"}},
	"codex":  {Binary: "codex", Args: []string{"--quiet"}, PromptViaArgs: true},
	"gemini": {Binary: "gemini", Args: []string{}, PromptViaArgs: true},
}


// MigrateOrchestraConfig performs all orchestra config migrations.
// It returns (changed bool, err error).
//
// Migrations applied:
//  1. If codex provider exists and PromptViaArgs is false, set it to true.
//  2. For each platform that maps to a known orchestra provider,
//     add the provider entry if it is missing.
//  3. For each orchestra command, ensure every provider in orchestra.Providers
//     is listed in the command's Providers slice.
func MigrateOrchestraConfig(cfg *HarnessConfig) (bool, error) {
	if !cfg.Orchestra.Enabled {
		return false, nil
	}

	changed := false

	// Migration 1: ensure codex PromptViaArgs is true.
	if codex, ok := cfg.Orchestra.Providers["codex"]; ok {
		if !codex.PromptViaArgs {
			codex.PromptViaArgs = true
			cfg.Orchestra.Providers["codex"] = codex
			changed = true
		}
	}

	// Migration 2: add missing provider entries for platforms.
	if cfg.Orchestra.Providers == nil {
		cfg.Orchestra.Providers = make(map[string]ProviderEntry)
	}
	for _, platform := range cfg.Platforms {
		providerName := PlatformToProvider(platform)
		if providerName == "" {
			continue
		}
		if _, exists := cfg.Orchestra.Providers[providerName]; !exists {
			entry := defaultProviderEntries[providerName]
			cfg.Orchestra.Providers[providerName] = entry
			changed = true
		}
	}

	// Migration 3: ensure every provider appears in every command's Providers list.
	if cfg.Orchestra.Commands == nil {
		return changed, nil
	}
	for cmdName, cmd := range cfg.Orchestra.Commands {
		for providerName := range cfg.Orchestra.Providers {
			if !containsString(cmd.Providers, providerName) {
				cmd.Providers = append(cmd.Providers, providerName)
				changed = true
			}
		}
		cfg.Orchestra.Commands[cmdName] = cmd
	}

	return changed, nil
}

// EnsureOrchestraProvider ensures a specific provider exists in the orchestra config.
// This is used by the "platform add" command to keep orchestra config consistent.
func EnsureOrchestraProvider(cfg *HarnessConfig, providerName string) error {
	if !cfg.Orchestra.Enabled {
		return nil
	}

	// Initialize providers map if nil.
	if cfg.Orchestra.Providers == nil {
		cfg.Orchestra.Providers = make(map[string]ProviderEntry)
	}

	// Add provider if it does not already exist.
	if _, exists := cfg.Orchestra.Providers[providerName]; !exists {
		entry, known := defaultProviderEntries[providerName]
		if !known {
			// Use a sensible zero-value entry for unknown providers.
			entry = ProviderEntry{Binary: providerName}
		}
		cfg.Orchestra.Providers[providerName] = entry
	}

	// Initialize commands map if nil.
	if cfg.Orchestra.Commands == nil {
		cfg.Orchestra.Commands = make(map[string]CommandEntry)
		return nil
	}

	// Append provider to each command that does not already list it.
	for cmdName, cmd := range cfg.Orchestra.Commands {
		if !containsString(cmd.Providers, providerName) {
			cmd.Providers = append(cmd.Providers, providerName)
			cfg.Orchestra.Commands[cmdName] = cmd
		}
	}

	return nil
}

// PlatformToProvider maps platform names to orchestra provider names.
func PlatformToProvider(platform string) string {
	switch platform {
	case "claude-code":
		return "claude"
	case "codex":
		return "codex"
	case "gemini-cli":
		return "gemini"
	default:
		return ""
	}
}

// containsString reports whether slice contains s.
func containsString(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
