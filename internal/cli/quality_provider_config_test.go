package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/insajin/autopus-adk/pkg/config"
)

func TestSaveQualityProviderPreservesRawConfig(t *testing.T) {
	dir := writeQualityTestConfig(t, "balanced")
	path := filepath.Join(dir, "autopus.yaml")
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	raw = []byte(strings.Replace(string(raw),
		"project_name: test-project",
		`project_name: "${AUTOPUS_PROVIDER_TEST}" # keep project comment`,
		1,
	))
	raw = append(raw, []byte("future_extension: keep-me # keep future field\n")...)
	require.NoError(t, os.WriteFile(path, raw, 0o640))
	require.NoError(t, os.Chmod(path, 0o640))
	t.Setenv("AUTOPUS_PROVIDER_TEST", "expanded-name")

	cfg, err := config.LoadPreview(dir)
	require.NoError(t, err)
	require.NoError(t, saveQualityProvider(dir, cfg, config.QualityProviderClaude, "ultra"))

	updated, err := os.ReadFile(path)
	require.NoError(t, err)
	content := string(updated)
	assert.Contains(t, content, `project_name: "${AUTOPUS_PROVIDER_TEST}" # keep project comment`)
	assert.Contains(t, content, "future_extension: keep-me # keep future field")
	assert.Contains(t, content, "providers:\n")
	assert.Contains(t, content, "claude: ultra\n")
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o640), info.Mode().Perm())

	require.NoError(t, removeQualityProvider(dir, cfg, config.QualityProviderClaude))
	finalData, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, raw, finalData)
	assert.NotContains(t, string(finalData), "materialized-secret")
	info, err = os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o640), info.Mode().Perm())
}

func TestRemoveQualityProviderPreservesOtherOverride(t *testing.T) {
	dir := writeQualityTestConfig(t, "balanced")
	cfg, err := config.LoadPreview(dir)
	require.NoError(t, err)
	cfg.Quality.Providers = map[string]string{
		config.QualityProviderClaude: "ultra",
		config.QualityProviderCodex:  "balanced",
	}
	require.NoError(t, config.Save(dir, cfg))

	require.NoError(t, removeQualityProvider(dir, cfg, config.QualityProviderClaude))

	updated, err := config.LoadPreview(dir)
	require.NoError(t, err)
	assert.NotContains(t, updated.Quality.Providers, config.QualityProviderClaude)
	assert.Equal(t, "balanced", updated.Quality.Providers[config.QualityProviderCodex])
	data, err := os.ReadFile(filepath.Join(dir, "autopus.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "providers:")
	assert.NotContains(t, string(data), "claude: ultra")
}

func TestRemoveLastQualityProviderRemovesEmptyMapping(t *testing.T) {
	dir := writeQualityTestConfig(t, "balanced")
	cfg, err := config.LoadPreview(dir)
	require.NoError(t, err)
	cfg.Quality.Providers = map[string]string{config.QualityProviderClaude: "ultra"}
	require.NoError(t, config.Save(dir, cfg))

	require.NoError(t, removeQualityProvider(dir, cfg, config.QualityProviderClaude))

	data, err := os.ReadFile(filepath.Join(dir, "autopus.yaml"))
	require.NoError(t, err)
	var doc yaml.Node
	require.NoError(t, yaml.Unmarshal(data, &doc))
	_, quality := yamlMappingPair(doc.Content[0], "quality")
	_, providers := yamlMappingPair(quality, "providers")
	assert.Nil(t, providers)
	updated, err := config.LoadPreview(dir)
	require.NoError(t, err)
	assert.Empty(t, updated.Quality.Providers)
}
