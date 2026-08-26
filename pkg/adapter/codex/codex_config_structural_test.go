package codex

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateAndUpdate_PreserveTrailingCommentTableHeaders(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, codexConfigRelPath)
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
	existing := `[features] # keep feature note
goals = false
custom_feature = true

[plugins."user#local"] # keep quoted hash note
enabled = false
`
	require.NoError(t, os.WriteFile(configPath, []byte(existing), 0o644))

	adapter := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("table-comment-project")
	operations := []struct {
		name string
		run  func(context.Context, *config.HarnessConfig) error
	}{
		{
			name: "Generate",
			run: func(ctx context.Context, cfg *config.HarnessConfig) error {
				_, err := adapter.Generate(ctx, cfg)
				return err
			},
		},
		{
			name: "Update",
			run: func(ctx context.Context, cfg *config.HarnessConfig) error {
				_, err := adapter.Update(ctx, cfg)
				return err
			},
		},
	}

	for _, operation := range operations {
		t.Run(operation.name, func(t *testing.T) {
			require.NoError(t, operation.run(context.Background(), cfg))
			body, err := os.ReadFile(configPath)
			require.NoError(t, err)
			content := string(body)
			assert.Equal(t, 1, strings.Count(content, "[features] # keep feature note"))
			assert.Equal(t, 1, strings.Count(content, `[plugins."user#local"] # keep quoted hash note`))
			assert.Contains(t, content, "goals = true")
			assert.Contains(t, content, "custom_feature = true")
			assert.Contains(t, content, "enabled = false")
		})
	}
}

func TestValidateCodexTOMLStructure_TableHeaderCommentsFailClosed(t *testing.T) {
	t.Parallel()

	valid := []string{
		"[features] # note\ngoals = true\n",
		"[plugins.\"user#local\"] # note\nenabled = true\n",
		"[[agents]] # note\nname = \"worker\"\n",
	}
	for _, content := range valid {
		require.NoError(t, validateCodexTOMLStructure(content))
	}

	invalid := []string{
		"[features] trailing # note\ngoals = true\n",
		"[features # note]\ngoals = true\n",
		"[features]] # note\ngoals = true\n",
		"[features] \"quoted # suffix\"\ngoals = true\n",
	}
	for _, content := range invalid {
		assert.Error(t, validateCodexTOMLStructure(content))
	}
}
