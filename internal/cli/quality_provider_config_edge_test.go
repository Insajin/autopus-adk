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

func TestUpdateQualityProviderHandlesSupportedDocumentShapes(t *testing.T) {
	t.Run("append quality section", func(t *testing.T) {
		raw := []byte("project_name: demo\n")
		updated, err := updateQualityProvider(raw, config.QualityProviderClaude, "ultra", false)
		require.NoError(t, err)
		assert.Equal(t,
			"project_name: demo\nquality:\n  providers:\n    claude: ultra\n",
			string(updated),
		)
	})

	t.Run("insert before first quality field", func(t *testing.T) {
		raw := []byte("quality:\n  supervisor_model_policy: inherit\n")
		updated, err := updateQualityProvider(raw, config.QualityProviderCodex, "balanced", false)
		require.NoError(t, err)
		assert.Equal(t,
			"quality:\n  providers:\n    codex: balanced\n  supervisor_model_policy: inherit\n",
			string(updated),
		)
	})

	t.Run("replace existing provider", func(t *testing.T) {
		raw := []byte("quality:\n  providers:\n    claude: balanced # keep\n")
		updated, err := updateQualityProvider(raw, config.QualityProviderClaude, "ultra", false)
		require.NoError(t, err)
		assert.Equal(t, "quality:\n  providers:\n    claude: ultra # keep\n", string(updated))
	})

	t.Run("remove missing locations is no-op", func(t *testing.T) {
		for _, raw := range [][]byte{
			[]byte("project_name: demo\n"),
			[]byte("quality:\n  default: balanced\n"),
			[]byte("quality:\n  providers:\n    codex: balanced\n"),
		} {
			updated, err := updateQualityProvider(raw, config.QualityProviderClaude, "", true)
			require.NoError(t, err)
			assert.Equal(t, raw, updated)
		}
	})
}

func TestUpdateQualityProviderRejectsUnsupportedDocumentShapes(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "invalid YAML", raw: "quality: [", want: "parse config"},
		{name: "root sequence", raw: "- quality\n", want: "config root"},
		{name: "quality scalar", raw: "quality: balanced\n", want: "quality must be"},
		{name: "quality flow", raw: "quality: {default: balanced}\n", want: "quality must be"},
		{
			name: "providers scalar",
			raw:  "quality:\n  providers: balanced\n",
			want: "quality.providers must be",
		},
		{
			name: "providers flow",
			raw:  "quality:\n  providers: {claude: balanced}\n",
			want: "quality.providers must be",
		},
		{
			name: "multiline provider",
			raw:  "quality:\n  providers:\n    claude: |\n      balanced\n",
			want: "single-line scalar",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := updateQualityProvider(
				[]byte(tt.raw),
				config.QualityProviderClaude,
				"ultra",
				false,
			)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}

	_, err := updateQualityProvider(
		[]byte("quality:\n  default: balanced\n"),
		config.QualityProviderClaude,
		"line one\nline two",
		false,
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "one line")
}

func TestPersistQualityProviderCreatesMissingConfig(t *testing.T) {
	dir := t.TempDir()
	cfg := config.DefaultFullConfig("new-provider-config")
	cfg.Quality.Providers = map[string]string{config.QualityProviderClaude: "ultra"}

	require.NoError(t, persistQualityProvider(
		dir,
		cfg,
		config.QualityProviderClaude,
		"ultra",
		false,
	))

	loaded, err := config.LoadPreview(dir)
	require.NoError(t, err)
	assert.Equal(t, "ultra", loaded.Quality.Providers[config.QualityProviderClaude])
}

func TestPersistQualityProviderReportsReadFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "autopus.yaml")
	require.NoError(t, os.Mkdir(path, 0o755))
	cfg := config.DefaultFullConfig("provider-read-error")
	cfg.Quality.Providers = map[string]string{config.QualityProviderClaude: "ultra"}

	err := persistQualityProvider(
		dir,
		cfg,
		config.QualityProviderClaude,
		"ultra",
		false,
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read config")
}

func TestValidateQualityYAMLRejectsSemanticDrift(t *testing.T) {
	expected := config.DefaultFullConfig("semantic-drift")
	raw, err := yaml.Marshal(expected)
	require.NoError(t, err)

	drifted := []byte(strings.Replace(
		string(raw),
		"default: balanced",
		"default: ultra",
		1,
	))
	err = validateQualityYAML(drifted, expected)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "quality fields changed unexpectedly")

	invalid := []byte(strings.Replace(
		string(raw),
		"default: balanced",
		"default: missing",
		1,
	))
	err = validateQualityYAML(invalid, expected)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "quality.default")
}

func TestRemoveConfigLinesRejectsInvalidBounds(t *testing.T) {
	raw := []byte("one\ntwo\n")
	assert.Equal(t, raw, removeConfigLines(raw, -1, 1))
	assert.Equal(t, raw, removeConfigLines(raw, 1, 1))
	assert.Equal(t, []byte("one\n"), removeConfigLines(raw, 1, 99))
}
