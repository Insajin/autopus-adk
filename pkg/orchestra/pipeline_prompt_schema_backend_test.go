package orchestra

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithPromptSchemaOMPProviderEmbedsSchemaDespiteCLIFlag(t *testing.T) {
	t.Parallel()

	data, err := withPromptSchema(
		PromptData{},
		&SchemaBuilder{},
		"reviewer",
		[]ProviderConfig{{Backend: "omp", SchemaFlag: "--output-schema"}},
	)

	require.NoError(t, err)
	assert.Equal(t, "prompt", data.SchemaMethod)
	assert.Contains(t, data.SchemaJSON, `"verdict"`)
	assert.Contains(t, data.SchemaJSON, `"findings"`)
}

func TestWithPromptSchemaLegacyCLIProviderStillUsesSchemaFlag(t *testing.T) {
	t.Parallel()

	data, err := withPromptSchema(
		PromptData{SchemaMethod: "prompt", SchemaJSON: `{"stale":true}`},
		&SchemaBuilder{},
		"reviewer",
		[]ProviderConfig{{SchemaFlag: "--output-schema"}},
	)

	require.NoError(t, err)
	assert.Empty(t, data.SchemaMethod)
	assert.Empty(t, data.SchemaJSON)
}
