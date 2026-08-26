package content_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/content"
)

func TestOMPStaticTaskRenderer_UsesIntentBatchCoreAcrossDynamicVariants(t *testing.T) {
	for _, test := range []struct {
		name, source string
	}{
		{name: "batch off", source: "`Agent(subagent_type=\"executor\", prompt=\"Implement\")`"},
		{name: "batch on", source: "```text\nAgent(subagent_type=\"executor\", prompt=\"Implement\")\nAgent(subagent_type=\"reviewer\", prompt=\"Review\")\n```"},
		{name: "isolation none", source: "Agent(subagent_type=\"executor\", prompt=\"Implement\")"},
		{name: "isolation on", source: "Agent(subagent_type=\"executor\", prompt=\"Implement\", isolated=true)"},
		{name: "effort off", source: "Agent(subagent_type=\"executor\", prompt=\"Implement\")"},
		{name: "effort on", source: "Agent(subagent_type=\"executor\", prompt=\"Implement\", effort=\"hi\")"},
	} {
		t.Run(test.name, func(t *testing.T) {
			rendered := content.ReplacePlatformReferences(test.source, "omp")
			payload := firstOMPJSONExample(t, rendered)
			assert.Equal(t, []string{"context", "i", "tasks"}, sortedOMPJSONKeys(payload))
			assert.NotEmpty(t, payload["i"])
			encoded, err := json.Marshal(payload)
			require.NoError(t, err)
			assert.NotContains(t, string(encoded), `"isolated"`)
			assert.NotContains(t, string(encoded), `"effort"`)
			assert.Contains(t, rendered, "current dynamic `task` schema")
			assert.Contains(t, rendered, "Add `isolated` or `effort` only after")
		})
	}
}

func TestOMPCoordinationExamples_IncludeIntentForTaskHubAndTodo(t *testing.T) {
	rendered := content.ReplacePlatformReferences(
		`TeamCreate(name="delivery") TaskCreate(subject="Implement") SendMessage(recipient="executor", content="Review")`,
		"omp",
	)
	assert.Contains(t, rendered, `"i": "Dispatching bounded OMP work"`)
	assert.Contains(t, rendered, `{"i":"Updating parent-owned progress","op":"append"`)
	assert.Contains(t, rendered, `{"i":"Following up with an existing worker","op":"send"`)
}

func firstOMPJSONExample(t *testing.T, body string) map[string]any {
	t.Helper()
	const fence = "```json\n"
	start := strings.Index(body, fence)
	require.NotEqual(t, -1, start)
	remaining := body[start+len(fence):]
	end := strings.Index(remaining, "\n```")
	require.NotEqual(t, -1, end)
	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(remaining[:end]), &payload))
	return payload
}

func sortedOMPJSONKeys(value map[string]any) []string {
	keys := make([]string, 0, len(value))
	for key := range value {
		keys = append(keys, key)
	}
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}
