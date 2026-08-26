package claude

// SPEC-STICKYRULE-001 / REQ-STICKYRULE-COMPILE-02 ownership boundary oracles.
//
// settings.json is shared with the user and carries no per-entry provenance, so
// autopus decides what it may delete by reading the command back out. Two ways
// to get that wrong destroy user configuration: claiming an entry that only
// mentions the sticky command, and overwriting a UserPromptSubmit value whose
// shape this writer does not understand. The oracles below are the boundary on
// both sides — what must be retracted, and what must survive untouched.
//
// The fixtures live in claude_sticky_settings_test.go.

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/adapter"
)

// seedUserPromptValue builds a settings document whose UserPromptSubmit key
// holds the given raw JSON value.
func seedUserPromptValue(raw string) string {
	return `{"hooks":{"UserPromptSubmit":` + raw + `}}`
}

// seedUserPromptCommand builds a settings document holding one hook entry that
// runs the given command.
func seedUserPromptCommand(t *testing.T, command string) string {
	t.Helper()

	encoded, err := json.Marshal(command)
	require.NoError(t, err)
	return seedUserPromptValue(`[{"matcher":"","hooks":[{"type":"command","command":` +
		string(encoded) + `,"timeout":9}]}]`)
}

// mentioningCommands name the sticky command without being it. A DLP or
// secret-scanning hook that excludes the sticky invocation from its own scan is
// the realistic case, and a bare substring test cannot tell it from the autopus
// entry — so regeneration silently deletes it.
var mentioningCommands = []string{
	`dlp-scan --exclude "auto rules sticky"`,
	`.claude/hooks/audit.sh --note 'installed by auto rules sticky'`,
	"auto rules stickyfoo --event UserPromptSubmit",
}

// TestPrepareSettingsMapping_UserHookMentioningTheCommandSurvives is the
// false-positive side of ownership: a hook that only names the command is not
// autopus's to delete, on either the retraction or the installation path.
func TestPrepareSettingsMapping_UserHookMentioningTheCommandSurvives(t *testing.T) {
	t.Parallel()

	for _, command := range mentioningCommands {
		t.Run(command, func(t *testing.T) {
			t.Parallel()
			seed := seedUserPromptCommand(t, command)

			_, retracted := mapSettings(t, seed, nil)
			assert.Equal(t, []string{command}, stickyCommands(t, retracted),
				"retraction must not delete a user hook that only names the command")

			_, installed := mapSettings(t, seed, []adapter.HookConfig{stickyHookConfig()})
			assert.Equal(t, []string{command, stickyCommand}, stickyCommands(t, installed),
				"installation must add the autopus entry beside the user hook, not replace it")
		})
	}
}

// TestPrepareSettingsMapping_OwnedCommandsAreStillRetracted is the other side:
// narrowing ownership to the exact emitted command would orphan every entry an
// earlier generation installed with different flags, so an anchored invocation
// stays owned whatever follows it.
func TestPrepareSettingsMapping_OwnedCommandsAreStillRetracted(t *testing.T) {
	t.Parallel()

	owned := []string{stickyCommand, staleStickyCommand, "auto rules sticky"}
	for _, command := range owned {
		t.Run(command, func(t *testing.T) {
			t.Parallel()

			_, settings := mapSettings(t, seedUserPromptCommand(t, command), nil)
			_, hooksExist := settings["hooks"]
			assert.False(t, hooksExist,
				"an Autopus-only hooks object must be retracted whatever flags it carries")
		})
	}
}

// malformedUserPromptValues are shapes a hand-edited settings.json can hold that
// this writer does not understand. Claude Code expects an array, but the file is
// user-owned, so a single entry object or a bare scalar is reachable — and a
// []any type assertion turns each of them into nil, which the install path then
// assigns over.
var malformedUserPromptValues = []string{
	`{"matcher":"","hooks":[{"type":"command","command":"user-scan.sh"}]}`,
	`"user-scan.sh"`,
	`42`,
}

// TestPrepareSettingsMapping_MalformedUserPromptValueIsPreserved requires an
// unrecognized value to survive both paths byte-identically: retraction leaves
// it alone, and installation carries it alongside the new entry instead of
// replacing it.
func TestPrepareSettingsMapping_MalformedUserPromptValueIsPreserved(t *testing.T) {
	t.Parallel()

	for _, raw := range malformedUserPromptValues {
		t.Run(raw, func(t *testing.T) {
			t.Parallel()

			seed := seedUserPromptValue(raw)
			var original any
			require.NoError(t, json.Unmarshal([]byte(raw), &original))

			_, retracted := mapSettings(t, seed, nil)
			assert.Equal(t, canonical(t, original),
				canonical(t, hooksOf(t, retracted)[stickyEvent]),
				"retraction must leave an unrecognized value byte-identical")

			_, installed := mapSettings(t, seed, []adapter.HookConfig{stickyHookConfig()})
			entries := stickyEntries(t, installed)
			require.Len(t, entries, 2,
				"the unrecognized value is kept beside the autopus entry")
			assert.Equal(t, canonical(t, original), canonical(t, entries[0]),
				"the user's value must survive byte-identically")
			assert.Equal(t, stickyCommand, installedCommand(t, entries[1]),
				"exactly one autopus entry is appended after it")
		})
	}
}

// installedCommand reads the single command out of one hook entry.
func installedCommand(t *testing.T, entry any) string {
	t.Helper()

	obj, ok := entry.(map[string]any)
	require.True(t, ok, "the appended entry must be a hook object")
	inner, ok := obj["hooks"].([]any)
	require.True(t, ok)
	require.Len(t, inner, 1)
	hook, ok := inner[0].(map[string]any)
	require.True(t, ok)
	command, ok := hook["command"].(string)
	require.True(t, ok)
	return command
}
