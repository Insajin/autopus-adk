// Package cli tests orchestra provider config argv correctness.
// SPEC-ORCH-021 REQ-014/015/016: pane argv stays interactive (gemini no --print,
// codex no leading exec), codex carries the structured --output-schema flag, and
// codex participates in the default structured review provider set.
package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/config"
)

// containsArg reports whether args contains target.
func containsArg(args []string, target string) bool {
	for _, a := range args {
		if a == target {
			return true
		}
	}
	return false
}

// TestProviderConfig_GeminiPaneNoPrint covers S17: the gemini pane argv must not
// contain --print (which is the non-interactive headless print mode).
func TestProviderConfig_GeminiPaneNoPrint(t *testing.T) {
	cfg := buildProviderConfigs([]string{"gemini"})[0]
	assert.False(t, containsArg(cfg.PaneArgs, "--print"),
		"gemini pane argv must not contain --print (interactive session)")
}

// TestProviderConfig_CodexPaneNotExec covers S19: the codex pane argv is
// non-empty and does not begin with the non-interactive `exec` subcommand.
func TestProviderConfig_CodexPaneNotExec(t *testing.T) {
	cfg := buildProviderConfigs([]string{"codex"})[0]
	require.NotEmpty(t, cfg.PaneArgs, "codex pane argv must be non-empty")
	assert.NotEqual(t, "exec", cfg.PaneArgs[0],
		"codex pane argv must not begin with exec (interactive TUI)")
}

// TestProviderConfig_CodexStructuredSchema covers S18: codex's structured argv
// carries the --output-schema flag, and codex is present in the default
// structured review provider set.
func TestProviderConfig_CodexStructuredSchema(t *testing.T) {
	cfg := buildProviderConfigs([]string{"codex"})[0]
	assert.Equal(t, "--output-schema", cfg.SchemaFlag,
		"codex structured argv must carry --output-schema")

	// Codex must be in the DEFAULT review provider set (cross-check with S20).
	names := resolveSpecReviewProviderNames(config.DefaultFullConfig("argv-test"), false)
	assert.Contains(t, names, "codex",
		"codex must be present in the default structured review provider set")
}

// TestResolveSpecReviewProviders_DefaultIncludesCodex covers S20: the default
// structured review provider set contains codex and resolves to the full
// [claude, codex, gemini] set with no silent codex drop.
func TestResolveSpecReviewProviders_DefaultIncludesCodex(t *testing.T) {
	cfg := config.DefaultFullConfig("argv-test")
	names := resolveSpecReviewProviderNames(cfg, false)

	assert.Contains(t, names, "claude")
	assert.Contains(t, names, "codex", "codex must not be silently dropped from review")
	assert.Contains(t, names, "gemini")
	assert.ElementsMatch(t, []string{"claude", "codex", "gemini"}, names,
		"default review provider set must be exactly [claude, codex, gemini]")
}
