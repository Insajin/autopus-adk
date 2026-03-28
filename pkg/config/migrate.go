// Package config provides autopus.yaml schema and migration utilities.
package config

// defaultProviderEntries holds the canonical default settings for known orchestra providers.
// @AX:NOTE [AUTO] hardcoded provider defaults — update when adding new providers or changing CLI flags
var defaultProviderEntries = map[string]ProviderEntry{
	"claude":   {Binary: "claude", Args: []string{"--print", "--model", "opus", "--effort", "high"}, PaneArgs: []string{"--print", "--model", "opus", "--effort", "high"}},
	"codex":    {Binary: "codex", Args: []string{"--quiet"}, PaneArgs: []string{"--quiet"}, PromptViaArgs: true},
	"gemini":   {Binary: "gemini", Args: []string{"-m", "gemini-3.1-pro-preview"}, PaneArgs: []string{"-m", "gemini-3.1-pro-preview"}, PromptViaArgs: true},
	"opencode": {Binary: "opencode", Args: []string{"run", "-m", "openai/gpt-5.4"}, PaneArgs: []string{"-m", "openai/gpt-5.4"}, PromptViaArgs: false},
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

	// Migration 1.5: migrate codex to opencode.
	if migrated, _ := MigrateCodexToOpencode(cfg); migrated {
		changed = true
	}

	// Migration 1.6: migrate opencode from args mode to TUI mode.
	if migrated := MigrateOpencodeToTUI(cfg); migrated {
		changed = true
	}

	// Migration 2: add missing provider entries for platforms, or update if args are empty.
	if cfg.Orchestra.Providers == nil {
		cfg.Orchestra.Providers = make(map[string]ProviderEntry)
	}
	for _, platform := range cfg.Platforms {
		providerName := PlatformToProvider(platform)
		if providerName == "" {
			continue
		}
		existing, exists := cfg.Orchestra.Providers[providerName]
		if !exists || len(existing.Args) == 0 {
			if entry, known := defaultProviderEntries[providerName]; known {
				cfg.Orchestra.Providers[providerName] = entry
				changed = true
			} else if !exists {
				cfg.Orchestra.Providers[providerName] = ProviderEntry{Binary: providerName}
				changed = true
			}
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

	// Add provider if missing, or update if args are empty (stale config).
	existing, exists := cfg.Orchestra.Providers[providerName]
	if !exists || len(existing.Args) == 0 {
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
	case "opencode":
		return "opencode"
	default:
		return ""
	}
}

// MigrateCodexToOpencode replaces codex provider entries with opencode.
// Removes codex from providers and commands, adds opencode if not already present.
// Returns (changed bool, err error).
// @AX:NOTE [AUTO] destructive migration — deletes codex entry permanently; no rollback path
func MigrateCodexToOpencode(cfg *HarnessConfig) (bool, error) {
	if !cfg.Orchestra.Enabled {
		return false, nil
	}
	if cfg.Orchestra.Providers == nil {
		return false, nil
	}

	_, hasCodex := cfg.Orchestra.Providers["codex"]
	if !hasCodex {
		return false, nil
	}

	// Remove codex entry.
	delete(cfg.Orchestra.Providers, "codex")

	// Add opencode entry if not already present, or update if args are empty.
	if existing, hasOpencode := cfg.Orchestra.Providers["opencode"]; !hasOpencode || len(existing.Args) == 0 {
		cfg.Orchestra.Providers["opencode"] = defaultProviderEntries["opencode"]
	}

	// Replace codex with opencode in all command provider lists.
	for cmdName, cmd := range cfg.Orchestra.Commands {
		cmd.Providers = replaceInSlice(cmd.Providers, "codex", "opencode")
		cfg.Orchestra.Commands[cmdName] = cmd
	}

	return true, nil
}

// MigrateOpencodeToTUI migrates opencode from args-based input to TUI mode.
// Clears InteractiveInput (was "args") and removes "run" from PaneArgs so that
// opencode launches as a persistent TUI session with prompt delivery via sendkeys.
// @AX:NOTE [AUTO] one-way migration — converts args mode to TUI mode; no rollback path
func MigrateOpencodeToTUI(cfg *HarnessConfig) bool {
	if cfg.Orchestra.Providers == nil {
		return false
	}
	oc, ok := cfg.Orchestra.Providers["opencode"]
	if !ok {
		return false
	}

	migrated := false

	// Clear args-based interactive input mode.
	if oc.InteractiveInput == "args" {
		oc.InteractiveInput = ""
		migrated = true
	}

	// Disable PromptViaArgs to prevent ENAMETOOLONG on long prompts.
	// Prompts are now delivered via stdin pipe instead of CLI args.
	if oc.PromptViaArgs {
		oc.PromptViaArgs = false
		migrated = true
	}

	// Remove "run" subcmd from PaneArgs (TUI mode doesn't use it).
	cleaned := make([]string, 0, len(oc.PaneArgs))
	for _, arg := range oc.PaneArgs {
		if arg == "run" {
			migrated = true
			continue
		}
		cleaned = append(cleaned, arg)
	}
	oc.PaneArgs = cleaned

	if migrated {
		cfg.Orchestra.Providers["opencode"] = oc
	}
	return migrated
}

// replaceInSlice replaces old with new in a string slice, removing duplicates.
func replaceInSlice(slice []string, old, new string) []string {
	hasNew := false
	result := make([]string, 0, len(slice))
	for _, s := range slice {
		if s == old {
			if !hasNew {
				result = append(result, new)
				hasNew = true
			}
			continue
		}
		if s == new {
			hasNew = true
		}
		result = append(result, s)
	}
	return result
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
