package orchestra

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSchemaBuilderReviewJudgeContract(t *testing.T) {
	t.Parallel()

	sb := &SchemaBuilder{}
	schema, err := sb.Generate("review_judge")
	require.NoError(t, err)

	var root map[string]any
	require.NoError(t, json.Unmarshal([]byte(schema), &root))
	properties, ok := root["properties"].(map[string]any)
	require.True(t, ok)
	assert.ElementsMatch(t, []string{"verdict", "findings", "rationale"}, mapKeys(properties))

	verdict, ok := properties["verdict"].(map[string]any)
	require.True(t, ok)
	assert.ElementsMatch(t, []string{"PASS", "REVISE", "REJECT"}, toStringSlice(verdict["enum"]))

	findings, ok := properties["findings"].(map[string]any)
	require.True(t, ok)
	item, ok := findings["items"].(map[string]any)
	require.True(t, ok)
	findingProperties, ok := item["properties"].(map[string]any)
	require.True(t, ok)
	decision := findingProperties["decision"].(map[string]any)
	assert.ElementsMatch(t, []string{"accept", "reject", "merge"}, toStringSlice(decision["enum"]))
	sources := findingProperties["sources"].(map[string]any)
	assert.Equal(t, "array", sources["type"])
	assert.Equal(t, "string", sources["items"].(map[string]any)["type"])

	required := toStringSlice(item["required"])
	assert.NotContains(t, required, "id")
	assert.NotContains(t, required, "category")
	assert.NotContains(t, required, "scope_ref")
	assert.NotContains(t, required, "reason")
	assert.Contains(t, required, "decision")
	assert.Contains(t, required, "sources")
}

func TestSchemaBuilderReviewJudgeSupportsFileAndPrompt(t *testing.T) {
	t.Parallel()

	sb := &SchemaBuilder{}
	embedded, err := sb.EmbedInPrompt("review_judge")
	require.NoError(t, err)
	assert.NotContains(t, embedded, "\n")
	assert.True(t, json.Valid([]byte(embedded)))

	path, cleanup, err := sb.WriteToFile("review_judge")
	require.NoError(t, err)
	t.Cleanup(cleanup)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.True(t, json.Valid(data))
}

func mapKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}
