package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/adapter/claude"
	"github.com/insajin/autopus-adk/pkg/config"
)

func normalizeStatusLineMode(raw string) config.StatusLineMode {
	return config.NormalizeStatusLineMode(raw)
}

func validateStatusLineMode(raw string) error {
	mode := normalizeStatusLineMode(raw)
	if mode == "" || mode.IsValid() {
		return nil
	}
	return fmt.Errorf("invalid --statusline-mode %q: must be keep, merge, or replace", raw)
}

func applyStatusLineMode(cmd *cobra.Command, dir string, cfg *config.HarnessConfig, rawMode string, allowPrompt bool) (string, error) {
	if cfg == nil || !containsPlatform(cfg.Platforms, "claude-code") {
		return "", nil
	}

	existing := claude.InspectStatusLine(dir)
	mode := normalizeStatusLineMode(rawMode)
	if mode == "" {
		switch {
		case existing.IsAutopusMerge():
			mode = config.StatusLineModeMerge
		case existing.IsAutopusReplace() || !existing.HasCommand():
			mode = config.StatusLineModeReplace
		case existing.IsUserManaged() && allowPrompt:
			mode = promptStatusLineMode(cmd.OutOrStdout(), existing.Command)
		default:
			mode = config.StatusLineModeKeep
		}
	}

	cfg.Runtime.StatusLine.Mode = mode
	return describeStatusLineDecision(existing, mode), nil
}

func promptStatusLineMode(out io.Writer, existingCommand string) config.StatusLineMode {
	options := []string{
		"Keep existing statusLine (recommended)",
		"Merge existing + Autopus statusLine",
		"Replace with Autopus statusLine",
	}
	question := fmt.Sprintf("Existing Claude statusLine detected:\n    %s\n  How should Autopus handle it?", existingCommand)
	switch promptChoice(out, question, options, 0) {
	case 1:
		return config.StatusLineModeMerge
	case 2:
		return config.StatusLineModeReplace
	default:
		return config.StatusLineModeKeep
	}
}

func describeStatusLineDecision(existing claude.StatusLineState, mode config.StatusLineMode) string {
	switch mode {
	case config.StatusLineModeKeep:
		if existing.IsUserManaged() {
			return fmt.Sprintf("Claude statusLine: existing command preserved (%s)", existing.Command)
		}
		if existing.IsAutopusMerge() {
			return "Claude statusLine: existing combined mode preserved"
		}
		if existing.IsAutopusReplace() {
			return "Claude statusLine: existing Autopus statusLine preserved"
		}
		return "Claude statusLine: no existing command to preserve"
	case config.StatusLineModeMerge:
		if existing.IsUserManaged() {
			return fmt.Sprintf("Claude statusLine: merged existing command + Autopus (%s)", existing.Command)
		}
		return "Claude statusLine: combined mode enabled"
	case config.StatusLineModeReplace:
		if existing.IsUserManaged() {
			return fmt.Sprintf("Claude statusLine: replaced existing command with Autopus (%s)", existing.Command)
		}
		return "Claude statusLine: Autopus statusLine enabled"
	default:
		return ""
	}
}

func buildStatusLinePreviewItems(dir string, cfg *config.HarnessConfig) []previewItem {
	if cfg == nil || !containsPlatform(cfg.Platforms, "claude-code") {
		return nil
	}

	existing := claude.InspectStatusLine(dir)
	mode := cfg.Runtime.StatusLine.Mode
	if !mode.IsValid() {
		mode = config.StatusLineModeReplace
	}

	item := previewItem{
		Path:     ".claude/settings.json",
		Category: "generated_surface",
		Scope:    "claude-code",
	}

	switch mode {
	case config.StatusLineModeKeep:
		if !existing.IsUserManaged() {
			return nil
		}
		item.Kind = "retain"
		item.Reason = "existing user-managed statusLine would be preserved; rerun with --statusline-mode merge or replace to change it"
	case config.StatusLineModeMerge:
		item.Kind = "emit"
		if existing.IsUserManaged() {
			item.Reason = "Claude statusLine would switch to combined mode (existing command + Autopus)"
		} else {
			item.Reason = "Claude statusLine would stay in combined mode and refresh managed wrapper assets"
		}
	case config.StatusLineModeReplace:
		item.Kind = "emit"
		if existing.IsUserManaged() {
			item.Reason = "Claude statusLine would be replaced with Autopus via --statusline-mode replace"
		} else {
			item.Reason = "Claude statusLine would point to the managed Autopus command"
		}
	default:
		return nil
	}

	return []previewItem{item}
}
