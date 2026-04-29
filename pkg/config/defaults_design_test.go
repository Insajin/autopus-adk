package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultFullConfig_HasDesignDefaults(t *testing.T) {
	t.Parallel()

	cfg := DefaultFullConfig("test-project")
	require.NotNil(t, cfg)

	assert.True(t, cfg.Design.Enabled)
	assert.True(t, cfg.Design.InjectOnReview)
	assert.True(t, cfg.Design.InjectOnVerify)
	assert.False(t, cfg.Design.ExternalImports)
	assert.Greater(t, cfg.Design.MaxContextLines, 0)
}

func TestDesignConf_ZeroValueIsNonBlocking(t *testing.T) {
	t.Parallel()

	var conf DesignConf
	assert.False(t, conf.Enabled)
	assert.Empty(t, conf.Paths)
	assert.Zero(t, conf.MaxContextLines)
	assert.False(t, conf.ExternalImports)
}

func TestLoad_DesignYAML(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	content := `
mode: full
project_name: test
platforms:
  - claude-code
design:
  enabled: true
  paths:
    - docs/design-system/baseline.md
  max_context_lines: 42
  inject_on_review: false
  inject_on_verify: true
  external_imports: true
  ui_globs:
    - "*.view"
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "autopus.yaml"), []byte(content), 0o644))

	cfg, err := Load(dir)
	require.NoError(t, err)
	assert.True(t, cfg.Design.Enabled)
	assert.Equal(t, []string{"docs/design-system/baseline.md"}, cfg.Design.Paths)
	assert.Equal(t, 42, cfg.Design.MaxContextLines)
	assert.False(t, cfg.Design.InjectOnReview)
	assert.True(t, cfg.Design.InjectOnVerify)
	assert.True(t, cfg.Design.ExternalImports)
	assert.Equal(t, []string{"*.view"}, cfg.Design.UIFileGlobs)
}
