package claude_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/adapter/claude"
	"github.com/insajin/autopus-adk/pkg/config"
)

// hookEntries returns the settings.json hook entries registered for an event.
func hookEntries(t *testing.T, root, event string) []map[string]any {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, ".claude", "settings.json"))
	require.NoError(t, err)

	var settings map[string]any
	require.NoError(t, json.Unmarshal(raw, &settings))

	hooks, ok := settings["hooks"].(map[string]any)
	require.True(t, ok, "settings.json must hold a hooks object")

	list, _ := hooks[event].([]any)
	entries := make([]map[string]any, 0, len(list))
	for _, item := range list {
		entry, ok := item.(map[string]any)
		require.True(t, ok)
		entries = append(entries, entry)
	}
	return entries
}

func entryCommands(t *testing.T, entry map[string]any) []string {
	t.Helper()
	nested, ok := entry["hooks"].([]any)
	require.True(t, ok, "hook entry must hold a hooks array")
	commands := make([]string, 0, len(nested))
	for _, item := range nested {
		hook, ok := item.(map[string]any)
		require.True(t, ok)
		command, _ := hook["command"].(string)
		commands = append(commands, command)
	}
	return commands
}

// TestClaudeGenerate_DispatcherEntryIsDeduplicatedPerMatcher is the S10 oracle
// for REQ-CONDRULE-COMPILE-03.
func TestClaudeGenerate_DispatcherEntryIsDeduplicatedPerMatcher(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".claude"), 0o755))
	seeded := `{"hooks":{"Notification":[{"matcher":"","hooks":[{"type":"command","command":"user-notify"}]}]}}`
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, ".claude", "settings.json"), []byte(seeded), 0o644))

	_, err := claude.NewWithRoot(dir).Generate(context.Background(), config.DefaultFullConfig("condrule"))
	require.NoError(t, err)

	var dispatchers []map[string]any
	for _, entry := range hookEntries(t, dir, "PreToolUse") {
		for _, command := range entryCommands(t, entry) {
			if strings.Contains(command, "auto rules fire") {
				dispatchers = append(dispatchers, entry)
				break
			}
		}
	}

	require.Len(t, dispatchers, 1,
		"three Bash rules must collapse to one dispatcher entry")
	assert.Equal(t, "Bash", dispatchers[0]["matcher"])
	assert.Len(t, entryCommands(t, dispatchers[0]), 1,
		"the dispatcher entry holds exactly one hook object")

	assert.NotEmpty(t, hookEntries(t, dir, "Notification"),
		"user-defined unmanaged event keys must survive regeneration")
}

// TestClaudeGenerate_ConditionalManifestIsDeterministic is the S9 manifest
// oracle for REQ-CONDRULE-COMPILE-04 and REQ-CONDRULE-COMPILE-07.
func TestClaudeGenerate_ConditionalManifestIsDeterministic(t *testing.T) {
	t.Parallel()

	first := readGeneratedManifest(t, generateClaudeSurface(t))
	second := readGeneratedManifest(t, generateClaudeSurface(t))
	assert.Equal(t, string(first), string(second),
		"regeneration must produce a byte-identical manifest")

	var manifest struct {
		Rules []struct {
			Name       string   `json:"name"`
			Event      string   `json:"event"`
			Matcher    string   `json:"matcher"`
			Conditions []string `json:"conditions"`
			Body       string   `json:"body"`
		} `json:"rules"`
	}
	require.NoError(t, json.Unmarshal(first, &manifest))

	names := make([]string, 0, len(manifest.Rules))
	for _, rule := range manifest.Rules {
		names = append(names, rule.Name)
		assert.Equal(t, rule.Name+".md", rule.Body, "body paths stay root-relative")
		assert.Equal(t, "PreToolUse", rule.Event)
		assert.Equal(t, "Bash", rule.Matcher)
		assert.NotEmpty(t, rule.Conditions)
	}
	assert.Equal(t, []string{"lore-commit", "shell-portability", "worktree-safety"}, names)
}

func readGeneratedManifest(t *testing.T, root string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, conditionalManifest))
	require.NoError(t, err, "the conditional manifest must be generated")
	return raw
}
