package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_FileNotExists(t *testing.T) {
	t.Parallel()
	cfg, err := Load(t.TempDir())
	require.NoError(t, err)
	assert.Equal(t, ModeFull, cfg.Mode)
}

func TestLoad_ValidYAML(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	content := `
mode: full
project_name: test
platforms:
  - claude-code
  - codex
architecture:
  auto_generate: true
  enforce: false
lore:
  enabled: true
  stale_threshold_days: 30
spec:
  id_format: "SPEC-{DOMAIN}-{NUMBER}"
`
	err := os.WriteFile(filepath.Join(dir, "autopus.yaml"), []byte(content), 0644)
	require.NoError(t, err)

	cfg, err := Load(dir)
	require.NoError(t, err)
	assert.Equal(t, ModeFull, cfg.Mode)
	assert.Equal(t, "test", cfg.ProjectName)
	assert.Equal(t, []string{"claude-code", "codex"}, cfg.Platforms)
	assert.Equal(t, 30, cfg.Lore.StaleThresholdDays)
	assert.True(t, cfg.Design.Enabled, "missing design section should use defaults for existing configs")
}

func TestLoad_ExplicitDesignDisabledIsPreserved(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	content := `
mode: full
project_name: test
platforms:
  - claude-code
design:
  enabled: false
`
	err := os.WriteFile(filepath.Join(dir, "autopus.yaml"), []byte(content), 0644)
	require.NoError(t, err)

	cfg, err := Load(dir)
	require.NoError(t, err)
	assert.False(t, cfg.Design.Enabled)
}

func TestLoad_WorkflowTeamDefaultFalse(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	content := `
mode: full
project_name: test
platforms:
  - claude-code
workflow:
  team_default: false
`
	err := os.WriteFile(filepath.Join(dir, "autopus.yaml"), []byte(content), 0644)
	require.NoError(t, err)

	cfg, err := Load(dir)
	require.NoError(t, err)
	assert.False(t, cfg.Workflow.TeamDefault)
}

func TestLoad_WorkflowTeamDefaultTrue(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	content := `
mode: full
project_name: test
platforms:
  - claude-code
workflow:
  team_default: true
`
	err := os.WriteFile(filepath.Join(dir, "autopus.yaml"), []byte(content), 0644)
	require.NoError(t, err)

	cfg, err := Load(dir)
	require.NoError(t, err)
	assert.True(t, cfg.Workflow.TeamDefault)
}

// TestLoad_WorkflowSectionAbsentDefaults locks S14: a valid config with NO
// top-level workflow: section is backfilled with the default WorkflowConf
// (team_default=true, coverage_threshold=85), not the zero value.
func TestLoad_WorkflowSectionAbsentDefaults(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	content := `
mode: full
project_name: test
platforms:
  - claude-code
`
	err := os.WriteFile(filepath.Join(dir, "autopus.yaml"), []byte(content), 0644)
	require.NoError(t, err)

	cfg, err := Load(dir)
	require.NoError(t, err)
	assert.True(t, cfg.Workflow.TeamDefault)
	assert.Equal(t, 85, cfg.Workflow.CoverageThreshold)
}

// TestLoad_WorkflowSectionPresentTeamDefaultOmitted locks S16: when the workflow
// section IS present but team_default is omitted (e.g. the user only tunes
// coverage_threshold), team_default backfills to true rather than the zero-value
// false — so a partial section never silently disables the substrate. The
// explicitly-set sibling field (coverage_threshold: 90, a non-default value) is
// preserved, proving the backfill is field-scoped, not a whole-section reset.
func TestLoad_WorkflowSectionPresentTeamDefaultOmitted(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	content := `
mode: full
project_name: test
platforms:
  - claude-code
workflow:
  coverage_threshold: 90
`
	err := os.WriteFile(filepath.Join(dir, "autopus.yaml"), []byte(content), 0644)
	require.NoError(t, err)

	cfg, err := Load(dir)
	require.NoError(t, err)
	assert.True(t, cfg.Workflow.TeamDefault, "omitted team_default in a present workflow section must default true, not silently disable the substrate")
	assert.Equal(t, 90, cfg.Workflow.CoverageThreshold, "explicitly-set coverage_threshold must be preserved (not reset by the team_default backfill)")
}

func TestLoad_EnvVarExpansion(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TEST_PROJECT_NAME", "from-env")
	content := `
mode: full
project_name: "${TEST_PROJECT_NAME}"
platforms:
  - claude-code
`
	err := os.WriteFile(filepath.Join(dir, "autopus.yaml"), []byte(content), 0644)
	require.NoError(t, err)

	cfg, err := Load(dir)
	require.NoError(t, err)
	assert.Equal(t, "from-env", cfg.ProjectName)
}

func TestLoad_InvalidYAML(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "autopus.yaml"), []byte(":::invalid"), 0644)
	require.NoError(t, err)

	_, err = Load(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse config")
}

func TestLoad_InvalidConfig(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	content := `
mode: invalid
project_name: test
platforms:
  - claude-code
`
	err := os.WriteFile(filepath.Join(dir, "autopus.yaml"), []byte(content), 0644)
	require.NoError(t, err)

	_, err = Load(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validate config")
}

func TestSave_RejectsInvalidConfig(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfg := DefaultFullConfig("test")
	cfg.Platforms = []string{"invalid-platform"}
	err := Save(dir, cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validate config")
}

func TestSave(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfg := DefaultFullConfig("save-test")
	err := Save(dir, cfg)
	require.NoError(t, err)

	loaded, err := Load(dir)
	require.NoError(t, err)
	assert.Equal(t, cfg.Mode, loaded.Mode)
	assert.Equal(t, cfg.ProjectName, loaded.ProjectName)
}
