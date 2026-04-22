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
	dir := t.TempDir()
	nonExistent := filepath.Join(dir, "hooks.json")

	rendered := `{"hooks":{"SessionStart":[{"command":"auto check"}]}}`
	result, err := mergeHooks(nonExistent, rendered)
	require.NoError(t, err)

	var doc hooksDoc
	require.NoError(t, json.Unmarshal(result, &doc))
	require.Len(t, doc.Hooks["SessionStart"], 1)
	// Should have __autopus__ marker stamped
	assert.True(t, doc.Hooks["SessionStart"][0].Autopus)
}

func TestMergeHooks_PreservesUserHooks(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	existingPath := filepath.Join(dir, "hooks.json")

	// Existing file with a user hook (no __autopus__ marker) and an old autopus hook
	existing := hooksDoc{
		Hooks: map[string]hookEntries{
			"SessionStart": {
				{Command: "user-custom.sh"},
				{Command: "auto check", Autopus: true},
			},
		},
	}
	data, _ := json.Marshal(existing)
	require.NoError(t, os.WriteFile(existingPath, data, 0644))

	// New rendered autopus hooks (template output, no marker yet)
	rendered := `{"hooks":{"SessionStart":[{"command":"auto check --v2"}]}}`
	result, err := mergeHooks(existingPath, rendered)
	require.NoError(t, err)

	var doc hooksDoc
	require.NoError(t, json.Unmarshal(result, &doc))

	entries := doc.Hooks["SessionStart"]
	require.Len(t, entries, 2, "user hook + new autopus hook")

	// First entry should be preserved user hook
	assert.Equal(t, "user-custom.sh", entries[0].Command)
	assert.False(t, entries[0].Autopus)

	// Second entry should be the new autopus hook
	assert.Equal(t, "auto check --v2", entries[1].Command)
	assert.True(t, entries[1].Autopus)
}

func TestMergeHooks_InvalidExistingJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	existingPath := filepath.Join(dir, "hooks.json")
	require.NoError(t, os.WriteFile(existingPath, []byte("{broken"), 0644))

	rendered := `{"hooks":{"Stop":[{"command":"auto save"}]}}`
	result, err := mergeHooks(existingPath, rendered)
	require.NoError(t, err)

	var doc hooksDoc
	require.NoError(t, json.Unmarshal(result, &doc))
	require.Len(t, doc.Hooks["Stop"], 1)
	assert.True(t, doc.Hooks["Stop"][0].Autopus)
}

func TestMergeHooks_MultipleCategories(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	existingPath := filepath.Join(dir, "hooks.json")

	existing := hooksDoc{
		Hooks: map[string]hookEntries{
			"SessionStart": {{Command: "user-start.sh"}},
			"CustomHook":   {{Command: "my-hook.sh"}},
		},
	}
	data, _ := json.Marshal(existing)
	require.NoError(t, os.WriteFile(existingPath, data, 0644))

	rendered := `{"hooks":{"SessionStart":[{"command":"auto check"}],"Stop":[{"command":"auto save"}]}}`
	result, err := mergeHooks(existingPath, rendered)
	require.NoError(t, err)

	var doc hooksDoc
	require.NoError(t, json.Unmarshal(result, &doc))

	// SessionStart: user hook preserved + autopus hook added
	assert.Len(t, doc.Hooks["SessionStart"], 2)
	// CustomHook: user-only category preserved
	assert.Len(t, doc.Hooks["CustomHook"], 1)
	assert.Equal(t, "my-hook.sh", doc.Hooks["CustomHook"][0].Command)
	// Stop: autopus-only new category
	assert.Len(t, doc.Hooks["Stop"], 1)
	assert.True(t, doc.Hooks["Stop"][0].Autopus)
}

func TestMergeHooks_NormalizesNullCategoriesToArrays(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	existingPath := filepath.Join(dir, "hooks.json")

	require.NoError(t, os.WriteFile(
		existingPath,
		[]byte(`{"hooks":{"SessionStart":null,"Stop":null}}`),
		0644,
	))

	rendered := `{"hooks":{"PreToolUse":[{"command":"auto check"}]}}`
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
	doc := hooksDoc{
		Hooks: map[string]hookEntries{
			"SessionStart": {{Command: "a"}, {Command: "b"}},
		},
	}
	stampAutopusMarker(&doc)
	for _, e := range doc.Hooks["SessionStart"] {
		assert.True(t, e.Autopus)
	}
}

func TestHookEntries_MarshalNilAsEmptyArray(t *testing.T) {
	t.Parallel()

	data, err := json.Marshal(struct {
		Entries hookEntries `json:"entries"`
	}{})
	require.NoError(t, err)
	assert.JSONEq(t, `{"entries":[]}`, string(data))
}
