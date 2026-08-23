package antigravity

import (
	"encoding/json"
	"testing"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSanitizeUnsupportedClaudeTeamMappings_PreservesUserSettingsStrings(t *testing.T) {
	t.Parallel()

	settings := []byte(`{
  "hooks": {"AfterAgent": [{"command": "audit TeamCreate TeamDelete SendMessage"}]},
  "permissions": {
    "allow": ["run_shell_command(TeamCreate)", "run_shell_command(TeamCreate)"],
    "deny": ["SendMessage"]
  }
}`)
	files := sanitizeUnsupportedClaudeTeamMappings([]adapter.FileMapping{
		{TargetPath: ".gemini/settings.json", Content: settings},
		{TargetPath: ".gemini/agents/autopus/example.md", Content: []byte("TeamCreate TeamDelete SendMessage")},
	})

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(files[0].Content, &decoded))
	hooks := decoded["hooks"].(map[string]any)
	afterAgent := hooks["AfterAgent"].([]any)
	command := afterAgent[0].(map[string]any)["command"]
	assert.Equal(t, "audit TeamCreate TeamDelete SendMessage", command)
	permissions := decoded["permissions"].(map[string]any)
	assert.Equal(t, []any{"run_shell_command(TeamCreate)"}, permissions["allow"])
	assert.Equal(t, []any{"SendMessage"}, permissions["deny"])

	assert.NotContains(t, string(files[1].Content), "TeamCreate")
	assert.NotContains(t, string(files[1].Content), "TeamDelete")
	assert.NotContains(t, string(files[1].Content), "SendMessage")
}

func TestFilterUnsupportedAntigravityPermissions_RemovesOnlyManagedExactValues(t *testing.T) {
	t.Parallel()

	input := &adapter.PermissionSet{
		Allow: []string{"TeamCreate", "run_shell_command(TeamCreate)", "read_file"},
		Deny:  []string{"SendMessage", "custom-SendMessage-policy"},
	}

	got := filterUnsupportedAntigravityPermissions(input)

	assert.Equal(t, []string{"run_shell_command(TeamCreate)", "read_file"}, got.Allow)
	assert.Equal(t, []string{"custom-SendMessage-policy"}, got.Deny)
	assert.Equal(t, []string{"TeamCreate", "run_shell_command(TeamCreate)", "read_file"}, input.Allow)
	assert.Equal(t, []string{"SendMessage", "custom-SendMessage-policy"}, input.Deny)
}
