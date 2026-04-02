package orchestra

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSchemaBuilder_Generate_AllRoles(t *testing.T) {
	t.Parallel()
	sb := &SchemaBuilder{}
	roles := []string{"debater_r1", "debater_r2", "judge", "reviewer"}

	for _, role := range roles {
		t.Run(role, func(t *testing.T) {
			t.Parallel()
			schema, err := sb.Generate(role)
			require.NoError(t, err)

			var parsed map[string]any
			require.NoError(t, json.Unmarshal([]byte(schema), &parsed))

			assert.Equal(t, "http://json-schema.org/draft-07/schema#", parsed["$schema"])
			assert.Equal(t, "object", parsed["type"])
			assert.NotEmpty(t, parsed["properties"])
			assert.NotEmpty(t, parsed["required"])
		})
	}
}

func TestSchemaBuilder_Generate_DebaterR1Properties(t *testing.T) {
	t.Parallel()
	sb := &SchemaBuilder{}
	schema, err := sb.Generate("debater_r1")
	require.NoError(t, err)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(schema), &parsed))

	props := parsed["properties"].(map[string]any)
	assert.Contains(t, props, "current_state")
	assert.Contains(t, props, "ideas")
	assert.Contains(t, props, "assumptions")
	assert.Contains(t, props, "hmw_questions")

	req := toStringSlice(parsed["required"])
	assert.Contains(t, req, "current_state")
	assert.Contains(t, req, "ideas")
}

func TestSchemaBuilder_Generate_JudgeProperties(t *testing.T) {
	t.Parallel()
	sb := &SchemaBuilder{}
	schema, err := sb.Generate("judge")
	require.NoError(t, err)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(schema), &parsed))

	props := parsed["properties"].(map[string]any)
	assert.Contains(t, props, "consensus_areas")
	assert.Contains(t, props, "top_ideas")
	assert.Contains(t, props, "recommendation")
}

func TestSchemaBuilder_Generate_ReviewerProperties(t *testing.T) {
	t.Parallel()
	sb := &SchemaBuilder{}
	schema, err := sb.Generate("reviewer")
	require.NoError(t, err)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(schema), &parsed))

	props := parsed["properties"].(map[string]any)
	assert.Contains(t, props, "findings")
	assert.Contains(t, props, "verdict")
	assert.Contains(t, props, "summary")
}

func TestSchemaBuilder_Generate_UnknownRole(t *testing.T) {
	t.Parallel()
	sb := &SchemaBuilder{}
	_, err := sb.Generate("unknown")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown role")
}

func TestSchemaBuilder_WriteToFile(t *testing.T) {
	t.Parallel()
	sb := &SchemaBuilder{}
	path, cleanup, err := sb.WriteToFile("debater_r1")
	require.NoError(t, err)
	require.NotEmpty(t, path)
	require.NotNil(t, cleanup)

	// File should exist and contain valid JSON.
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(data, &parsed))
	assert.Equal(t, "object", parsed["type"])

	// Cleanup should remove the file.
	cleanup()
	_, err = os.Stat(path)
	assert.True(t, os.IsNotExist(err))
}

func TestSchemaBuilder_WriteToFile_UnknownRole(t *testing.T) {
	t.Parallel()
	sb := &SchemaBuilder{}
	_, _, err := sb.WriteToFile("bad")
	assert.Error(t, err)
}

func TestSchemaBuilder_EmbedInPrompt(t *testing.T) {
	t.Parallel()
	sb := &SchemaBuilder{}
	embedded, err := sb.EmbedInPrompt("judge")
	require.NoError(t, err)

	// Should be valid compact JSON (no indentation).
	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(embedded), &parsed))
	assert.NotContains(t, embedded, "\n")
}

func TestSchemaBuilder_EmbedInPrompt_UnknownRole(t *testing.T) {
	t.Parallel()
	sb := &SchemaBuilder{}
	_, err := sb.EmbedInPrompt("bad")
	assert.Error(t, err)
}

func TestSchemaBuilder_NestedArraySchema(t *testing.T) {
	t.Parallel()
	sb := &SchemaBuilder{}
	schema, err := sb.Generate("debater_r1")
	require.NoError(t, err)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(schema), &parsed))

	props := parsed["properties"].(map[string]any)
	ideas := props["ideas"].(map[string]any)
	assert.Equal(t, "array", ideas["type"])

	items := ideas["items"].(map[string]any)
	assert.Equal(t, "object", items["type"])
	itemProps := items["properties"].(map[string]any)
	assert.Contains(t, itemProps, "title")
	assert.Contains(t, itemProps, "description")
}

// toStringSlice converts a JSON array ([]any) to []string.
func toStringSlice(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, len(arr))
	for i, item := range arr {
		out[i], _ = item.(string)
	}
	return out
}
