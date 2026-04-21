package codex

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderPluginManifestJSON_DefaultPromptsFitCodexLimits(t *testing.T) {
	t.Parallel()

	doc, err := New().renderPluginManifestJSON()
	require.NoError(t, err)

	var manifest pluginManifest
	require.NoError(t, json.Unmarshal([]byte(doc), &manifest))

	require.Len(t, manifest.Interface.DefaultPrompt, 3)
	for _, prompt := range manifest.Interface.DefaultPrompt {
		assert.LessOrEqual(t, len(prompt), 128)
	}
}
