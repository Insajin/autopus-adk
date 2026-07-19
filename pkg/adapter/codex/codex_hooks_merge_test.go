package codex

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMergeHooks_NoExistingFile(t *testing.T) {
	t.Parallel()
	nonExistent := filepath.Join(t.TempDir(), "hooks.json")
	rendered := `{"hooks":{"SessionStart":[{"hooks":[{"command":"auto check"}]}]}}`

	result, err := mergeHooks(nonExistent, rendered)
	require.NoError(t, err)

	var doc hooksDoc
	require.NoError(t, json.Unmarshal(result, &doc))
	group := requireHookGroup(t, doc, "SessionStart", 0)
	assert.Equal(t, "auto check", requireHookHandler(t, group, 0).Command)
	assert.NotContains(t, string(result), "__autopus__", "generated hooks must stay within the official Codex schema")
}

func TestMergeHooks_PreservesUserHooksAndMigratesLegacyFlatEntries(t *testing.T) {
	t.Parallel()
	existingPath := filepath.Join(t.TempDir(), "hooks.json")
	existing := `{"hooks":{"SessionStart":[
		{"command":"user-custom.sh","matcher":"startup"},
		{"command":"auto check","__autopus__":true}
	]}}`
	require.NoError(t, os.WriteFile(existingPath, []byte(existing), 0644))

	rendered := `{"hooks":{"SessionStart":[{"hooks":[{"command":"auto check --v2"}]}]}}`
	result, err := mergeHooks(existingPath, rendered)
	require.NoError(t, err)

	var doc hooksDoc
	require.NoError(t, json.Unmarshal(result, &doc))
	groups := doc.Hooks["SessionStart"]
	require.Len(t, groups, 2)

	assert.Equal(t, "startup", groups[0].Matcher)
	assert.False(t, groups[0].Autopus)
	assert.Equal(t, "user-custom.sh", requireHookHandler(t, groups[0], 0).Command)
	assert.Equal(t, "auto check --v2", requireHookHandler(t, groups[1], 0).Command)
	assert.NotContains(t, string(result), "__autopus__")

	var raw struct {
		Hooks map[string][]map[string]json.RawMessage `json:"hooks"`
	}
	require.NoError(t, json.Unmarshal(result, &raw))
	for _, group := range raw.Hooks["SessionStart"] {
		assert.Contains(t, group, "hooks", "legacy flat entries must be normalized to Codex matcher groups")
		assert.NotContains(t, group, "command", "commands belong inside the nested hooks array")
	}
}

func TestMergeHooks_PreservesUserHookSchemaExtensions(t *testing.T) {
	t.Parallel()
	existingPath := filepath.Join(t.TempDir(), "hooks.json")
	existing := `{
		"description":"user hook configuration",
		"futureTopLevel":{"enabled":true},
		"hooks":{"Stop":[{
			"matcher":"user-stop",
			"futureGroupField":"group-value",
			"hooks":[{
				"type":"command",
				"command":"user-stop.sh",
				"command_windows":"user-stop.cmd",
				"async":false,
				"futureHandlerField":{"mode":"safe"}
			}]
		}]}
	}`
	require.NoError(t, os.WriteFile(existingPath, []byte(existing), 0644))

	rendered := `{"hooks":{"Stop":[{"hooks":[{"command":"auto save"}]}]}}`
	result, err := mergeHooks(existingPath, rendered)
	require.NoError(t, err)

	var raw map[string]any
	require.NoError(t, json.Unmarshal(result, &raw))
	assert.Equal(t, "user hook configuration", raw["description"])
	assert.Equal(t, map[string]any{"enabled": true}, raw["futureTopLevel"])

	hooks := raw["hooks"].(map[string]any)
	stopGroups := hooks["Stop"].([]any)
	userGroup := stopGroups[0].(map[string]any)
	assert.Equal(t, "group-value", userGroup["futureGroupField"])
	userHandler := userGroup["hooks"].([]any)[0].(map[string]any)
	assert.Equal(t, "user-stop.cmd", userHandler["command_windows"])
	assert.Equal(t, false, userHandler["async"])
	assert.Equal(t, map[string]any{"mode": "safe"}, userHandler["futureHandlerField"])
}

func TestMergeHooks_InvalidExistingJSONFailsClosed(t *testing.T) {
	t.Parallel()
	existingPath := filepath.Join(t.TempDir(), "hooks.json")
	original := []byte("{broken")
	require.NoError(t, os.WriteFile(existingPath, original, 0644))

	rendered := `{"hooks":{"Stop":[{"hooks":[{"command":"auto save"}]}]}}`
	_, err := mergeHooks(existingPath, rendered)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "existing hooks JSON")

	after, readErr := os.ReadFile(existingPath)
	require.NoError(t, readErr)
	assert.Equal(t, original, after, "malformed user configuration must never be replaced silently")
}

func TestMergeHooks_PreservesUserHandlerInManagedMatcherGroup(t *testing.T) {
	t.Parallel()
	existingPath := filepath.Join(t.TempDir(), "hooks.json")
	existing := `{"hooks":{"Stop":[{
		"matcher":"shared-stop",
		"futureGroupField":"preserve-me",
		"hooks":[
			{"command":"old-autopus.sh","statusMessage":"Running Autopus hook"},
			{"command":"user-stop.sh","futureHandlerField":true}
		]
	}]}}`
	require.NoError(t, os.WriteFile(existingPath, []byte(existing), 0644))

	rendered := `{"hooks":{"Stop":[{"hooks":[{"command":"new-autopus.sh"}]}]}}`
	result, err := mergeHooks(existingPath, rendered)
	require.NoError(t, err)

	var doc hooksDoc
	require.NoError(t, json.Unmarshal(result, &doc))
	groups := doc.Hooks["Stop"]
	require.Len(t, groups, 2)
	assert.Equal(t, "shared-stop", groups[0].Matcher)
	assert.JSONEq(t, `"preserve-me"`, string(groups[0].Extra["futureGroupField"]))
	require.Len(t, groups[0].Hooks, 1)
	assert.Equal(t, "user-stop.sh", groups[0].Hooks[0].Command)
	assert.JSONEq(t, `true`, string(groups[0].Hooks[0].Extra["futureHandlerField"]))
	assert.Equal(t, "new-autopus.sh", requireHookHandler(t, groups[1], 0).Command)
}

func TestMergeHooks_MultipleCategories(t *testing.T) {
	t.Parallel()
	existingPath := filepath.Join(t.TempDir(), "hooks.json")
	existing := hooksDoc{Hooks: map[string]hookGroups{
		"SessionStart": {{Hooks: hookHandlers{{Command: "user-start.sh"}}}},
		"CustomHook":   {{Hooks: hookHandlers{{Command: "my-hook.sh"}}}},
	}}
	data, err := json.Marshal(existing)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(existingPath, data, 0644))

	rendered := `{"hooks":{
		"SessionStart":[{"hooks":[{"command":"auto check"}]}],
		"Stop":[{"hooks":[{"command":"auto save"}]}]
	}}`
	result, err := mergeHooks(existingPath, rendered)
	require.NoError(t, err)

	var doc hooksDoc
	require.NoError(t, json.Unmarshal(result, &doc))
	assert.Len(t, doc.Hooks["SessionStart"], 2)
	assert.Len(t, doc.Hooks["CustomHook"], 1)
	assert.Equal(t, "my-hook.sh", requireHookHandler(t, doc.Hooks["CustomHook"][0], 0).Command)
	assert.Equal(t, "auto save", requireHookHandler(t, requireHookGroup(t, doc, "Stop", 0), 0).Command)
}

func TestMergeHooks_NormalizesNullCategoriesToArrays(t *testing.T) {
	t.Parallel()
	existingPath := filepath.Join(t.TempDir(), "hooks.json")
	require.NoError(t, os.WriteFile(
		existingPath,
		[]byte(`{"hooks":{"SessionStart":null,"Stop":null}}`),
		0644,
	))

	rendered := `{"hooks":{"PreToolUse":[{"hooks":[{"command":"auto check"}]}]}}`
	result, err := mergeHooks(existingPath, rendered)
	require.NoError(t, err)

	assert.NotContains(t, string(result), `"SessionStart": null`)
	assert.NotContains(t, string(result), `"Stop": null`)
	var raw struct {
		Hooks map[string]json.RawMessage `json:"hooks"`
	}
	require.NoError(t, json.Unmarshal(result, &raw))
	assert.JSONEq(t, "[]", string(raw.Hooks["SessionStart"]))
	assert.JSONEq(t, "[]", string(raw.Hooks["Stop"]))
}

func TestStampAutopusMarker(t *testing.T) {
	t.Parallel()
	doc := hooksDoc{Hooks: map[string]hookGroups{
		"SessionStart": {
			{Hooks: hookHandlers{{Command: "a"}}},
			{Hooks: hookHandlers{{Command: "b"}}},
		},
	}}
	stampAutopusMarker(&doc)
	for _, group := range doc.Hooks["SessionStart"] {
		assert.True(t, group.Autopus)
	}
}

func TestHookGroups_MarshalNilAsEmptyArray(t *testing.T) {
	t.Parallel()
	data, err := json.Marshal(struct {
		Groups hookGroups `json:"groups"`
	}{})
	require.NoError(t, err)
	assert.JSONEq(t, `{"groups":[]}`, string(data))
}

func requireHookGroup(t *testing.T, doc hooksDoc, event string, index int) hookGroup {
	t.Helper()
	groups := doc.Hooks[event]
	require.Greater(t, len(groups), index)
	return groups[index]
}

func requireHookHandler(t *testing.T, group hookGroup, index int) hookHandler {
	t.Helper()
	require.Greater(t, len(group.Hooks), index)
	return group.Hooks[index]
}
