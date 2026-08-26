package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/adapter"
)

// SPEC-STICKYRULE-001 / REQ-STICKYRULE-COMPILE-02 / AC-STICKYRULE-016 (S16).
//
// The requirement has two halves that pull in opposite directions: a previously
// installed sticky entry must be retracted once no rule is sticky, and
// user-defined configuration must be left untouched. UserPromptSubmit is a
// Claude Code event a user may hook for their own reasons, so owning the whole
// event key satisfies the first half by deleting the user's hooks, which
// violates the second. Autopus therefore owns one entry inside the event, and
// these are the oracles for that boundary.

const (
	stickyEvent   = "UserPromptSubmit"
	stickyCommand = "auto rules sticky --event UserPromptSubmit"
	// staleStickyCommand stands for the same autopus entry as an earlier
	// generation installed it: same marker, different flags.
	staleStickyCommand = "auto rules sticky --event UserPromptSubmit --legacy"
	// userScanScript is the user-authored hook the finding is about. A secret
	// scanner on this event is exactly what event-level preemption deleted.
	userScanScript = ".claude/hooks/user-secret-scan.sh"
	userEvent      = "SessionEnd"
)

// stickySeedSettings carries a user-authored UserPromptSubmit hook, a previously
// installed autopus sticky entry beside it, a stale TaskCreated entry, a
// user-defined event autopus does not manage, and user-owned permissions.
const stickySeedSettings = `{
  "hooks": {
    "SessionEnd": [
      {
        "matcher": "*",
        "hooks": [
          {"type": "command", "command": "user-session-end.sh", "timeout": 12}
        ]
      }
    ],
    "UserPromptSubmit": [
      {
        "matcher": "",
        "hooks": [
          {"type": "command", "command": ".claude/hooks/user-secret-scan.sh", "timeout": 12}
        ]
      },
      {
        "matcher": "",
        "hooks": [
          {"type": "command", "command": "auto rules sticky --event UserPromptSubmit --legacy", "timeout": 3}
        ]
      }
    ],
    "TaskCreated": [
      {
        "matcher": "",
        "hooks": [
          {"type": "command", "command": ".claude/hooks/task-created-validate.sh", "timeout": 5}
        ]
      }
    ]
  },
  "permissions": {
    "allow": ["Bash(ls:*)"]
  }
}`

// mapSettings runs prepareSettingsMapping against a throwaway root seeded with
// the given settings file, returning the raw output and the decoded document.
func mapSettings(t *testing.T, seed string, hooks []adapter.HookConfig) (string, map[string]any) {
	t.Helper()

	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".claude"), 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(root, ".claude", "settings.json"), []byte(seed), 0o644))

	mapping, err := NewWithRoot(root).prepareSettingsMapping(hooks, nil)
	require.NoError(t, err)

	var settings map[string]any
	require.NoError(t, json.Unmarshal(mapping.Content, &settings))
	return string(mapping.Content), settings
}

// mapSticky runs prepareSettingsMapping against the standard seed.
func mapSticky(t *testing.T, hooks []adapter.HookConfig) (string, map[string]any) {
	t.Helper()
	return mapSettings(t, stickySeedSettings, hooks)
}

func hooksOf(t *testing.T, settings map[string]any) map[string]any {
	t.Helper()

	hooks, ok := settings["hooks"].(map[string]any)
	require.True(t, ok, "settings must carry a hooks object")
	return hooks
}

func canonical(t *testing.T, v any) string {
	t.Helper()

	encoded, err := json.Marshal(v)
	require.NoError(t, err)
	return string(encoded)
}

// seededEvent returns the canonical encoding of an event key as it was seeded.
func seededEvent(t *testing.T, event string) string {
	t.Helper()

	var seed map[string]any
	require.NoError(t, json.Unmarshal([]byte(stickySeedSettings), &seed))
	return canonical(t, hooksOf(t, seed)[event])
}

// seededUserPromptEntry is the user-authored UserPromptSubmit entry as seeded.
func seededUserPromptEntry(t *testing.T) string {
	t.Helper()

	var seed map[string]any
	require.NoError(t, json.Unmarshal([]byte(stickySeedSettings), &seed))
	entries, ok := hooksOf(t, seed)[stickyEvent].([]any)
	require.True(t, ok)
	require.Len(t, entries, 2, "the seed pairs a user hook with a stale autopus entry")
	return canonical(t, entries[0])
}

// stickyEntries returns the UserPromptSubmit entry list.
func stickyEntries(t *testing.T, settings map[string]any) []any {
	t.Helper()

	entries, ok := hooksOf(t, settings)[stickyEvent].([]any)
	require.True(t, ok, "UserPromptSubmit must be present")
	return entries
}

// stickyCommands flattens every command registered under UserPromptSubmit. It
// reads the produced JSON directly rather than reusing the writer's own
// classifier, so the oracle does not inherit the implementation's idea of
// ownership.
func stickyCommands(t *testing.T, settings map[string]any) []string {
	t.Helper()

	var commands []string
	for _, raw := range stickyEntries(t, settings) {
		entry, ok := raw.(map[string]any)
		require.True(t, ok)
		inner, ok := entry["hooks"].([]any)
		require.True(t, ok)
		for _, item := range inner {
			hook, ok := item.(map[string]any)
			require.True(t, ok)
			command, ok := hook["command"].(string)
			require.True(t, ok)
			commands = append(commands, command)
		}
	}
	return commands
}

// assertUnmanagedSurfacesSurvive pins the untouched halves of the contract: the
// user-defined event key and the user-owned permissions block.
func assertUnmanagedSurfacesSurvive(t *testing.T, settings map[string]any) {
	t.Helper()

	assert.Equal(t, seededEvent(t, userEvent), canonical(t, hooksOf(t, settings)[userEvent]),
		"user-defined event key must survive byte-identically")

	var seed map[string]any
	require.NoError(t, json.Unmarshal([]byte(stickySeedSettings), &seed))
	assert.Equal(t, canonical(t, seed["permissions"]), canonical(t, settings["permissions"]),
		"user permissions must survive byte-identically when no autopus permissions are supplied")
}

func stickyHookConfig() adapter.HookConfig {
	return adapter.HookConfig{
		Event:   stickyEvent,
		Matcher: "",
		Type:    "command",
		Command: stickyCommand,
		Timeout: 5,
	}
}

// S16 creation row: installing sticky replaces the autopus entry in place and
// leaves the user's own hook on the same event untouched.
func TestPrepareSettingsMapping_StickyHookReplacesOnlyTheAutopusEntry(t *testing.T) {
	t.Parallel()

	raw, settings := mapSticky(t, []adapter.HookConfig{stickyHookConfig()})
	entries := stickyEntries(t, settings)
	require.Len(t, entries, 2, "the user hook survives beside exactly one autopus entry")

	assert.Equal(t, seededUserPromptEntry(t), canonical(t, entries[0]),
		"the user-authored UserPromptSubmit hook must survive byte-identically")
	assert.Equal(t, []string{userScanScript, stickyCommand}, stickyCommands(t, settings),
		"exactly one autopus command is registered, and the user command is not displaced")

	autopus, ok := entries[1].(map[string]any)
	require.True(t, ok)
	assert.NotContains(t, autopus, "matcher")
	inner, ok := autopus["hooks"].([]any)
	require.True(t, ok)
	require.Len(t, inner, 1)
	hookEntry, ok := inner[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "command", hookEntry["type"])
	assert.Equal(t, float64(5), hookEntry["timeout"])

	assert.False(t, strings.Contains(raw, staleStickyCommand),
		"the previously installed autopus entry must be replaced, not appended")
	assertUnmanagedSurfacesSurvive(t, settings)
}

// S16 removal row 1: sticky retracted while other hooks remain.
func TestPrepareSettingsMapping_StickyRetractionKeepsTheUserEntry(t *testing.T) {
	t.Parallel()

	_, settings := mapSticky(t, []adapter.HookConfig{
		{Event: "PreToolUse", Matcher: "Write", Type: "command", Command: "guard.sh", Timeout: 30},
		{Event: "TaskCreated", Matcher: "", Type: "command", Command: "task-created.sh", Timeout: 5},
	})
	hooks := hooksOf(t, settings)

	require.Len(t, stickyEntries(t, settings), 1)
	assert.Equal(t, []string{userScanScript}, stickyCommands(t, settings),
		"the stale autopus entry is retracted and the user hook is not")
	assert.Equal(t, seededUserPromptEntry(t), canonical(t, stickyEntries(t, settings)[0]))

	assert.Contains(t, hooks, "PreToolUse")
	taskCreated, ok := hooks["TaskCreated"].([]any)
	require.True(t, ok, "TaskCreated behaviour must not change")
	require.Len(t, taskCreated, 1)

	assertUnmanagedSurfacesSurvive(t, settings)
}

// S16 removal row 2: sticky retracted with no hooks at all.
func TestPrepareSettingsMapping_StickyRetractionWithNoHooksKeepsTheUserEntry(t *testing.T) {
	t.Parallel()

	_, settings := mapSticky(t, nil)

	require.Len(t, stickyEntries(t, settings), 1)
	assert.Equal(t, []string{userScanScript}, stickyCommands(t, settings))
	assert.Equal(t, seededUserPromptEntry(t), canonical(t, stickyEntries(t, settings)[0]))

	_, taskCreatedExists := hooksOf(t, settings)["TaskCreated"]
	assert.False(t, taskCreatedExists, "TaskCreated removal behaviour must not change")

	assertUnmanagedSurfacesSurvive(t, settings)
}

// S16 removal row 3: with nothing but the autopus entry under the event, the key
// itself goes away, which is what keeps retraction observable.
func TestPrepareSettingsMapping_StickyRetractionRemovesAnEmptiedEventKey(t *testing.T) {
	t.Parallel()

	seed := `{"hooks":{"UserPromptSubmit":[{"matcher":"","hooks":` +
		`[{"type":"command","command":"` + stickyCommand + `","timeout":5}]}]}}`

	_, settings := mapSettings(t, seed, nil)

	_, hooksExist := settings["hooks"]
	assert.False(t, hooksExist,
		"the hooks object is deleted once the managed entry was the only hook")
}

// Regeneration is a fixed point: entry-level ownership must not accumulate a
// second autopus entry each time the settings file is rewritten.
func TestPrepareSettingsMapping_StickyRegenerationIsIdempotent(t *testing.T) {
	t.Parallel()

	first, _ := mapSticky(t, []adapter.HookConfig{stickyHookConfig()})
	second, settings := mapSettings(t, first, []adapter.HookConfig{stickyHookConfig()})

	assert.Equal(t, first, second, "regenerating over produced settings must change nothing")
	assert.Equal(t, []string{userScanScript, stickyCommand}, stickyCommands(t, settings))
}
