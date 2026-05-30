package codex

import (
	"encoding/json"
	"testing"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderPluginManifestJSON_DefaultPromptsFitCodexLimits(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultFullConfig("test-project")
	doc, err := New().renderPluginManifestJSON(cfg, "router")
	require.NoError(t, err)

	var manifest pluginManifest
	require.NoError(t, json.Unmarshal([]byte(doc), &manifest))

	require.Len(t, manifest.Interface.DefaultPrompt, 3)
	for _, prompt := range manifest.Interface.DefaultPrompt {
		assert.LessOrEqual(t, len(prompt), 128)
	}
}

func TestRenderPluginManifestJSON_VersionIncludesProjectCacheKey(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultFullConfig("Alpha Project")
	doc, err := New().renderPluginManifestJSON(cfg, "router-v1")
	require.NoError(t, err)

	var manifest pluginManifest
	require.NoError(t, json.Unmarshal([]byte(doc), &manifest))

	assert.Regexp(t, `^[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?\+codex\.alpha-project\.[0-9a-f]{12}$`, manifest.Version)
}

func TestRenderPluginManifestJSON_VersionChangesAcrossProjects(t *testing.T) {
	t.Parallel()

	cfgA := config.DefaultFullConfig("Alpha Project")
	cfgB := config.DefaultFullConfig("Beta Project")
	docA, err := New().renderPluginManifestJSON(cfgA, "router-v1")
	require.NoError(t, err)
	docB, err := New().renderPluginManifestJSON(cfgB, "router-v1")
	require.NoError(t, err)

	var manifestA pluginManifest
	var manifestB pluginManifest
	require.NoError(t, json.Unmarshal([]byte(docA), &manifestA))
	require.NoError(t, json.Unmarshal([]byte(docB), &manifestB))

	assert.NotEqual(t, manifestA.Version, manifestB.Version)
	assert.Contains(t, manifestB.Version, "+codex.beta-project.")
}
