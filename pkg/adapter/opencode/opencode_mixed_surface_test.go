package opencode

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/config"
)

func TestAdapter_Generate_MixedMode_DefaultsToFullSharedSurface(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("demo")
	cfg.Platforms = []string{"codex", "opencode"}

	_, err := a.Generate(context.Background(), cfg)
	require.NoError(t, err)

	assert.FileExists(t, filepath.Join(dir, ".agents", "skills", "auto", "SKILL.md"))
	assert.FileExists(t, filepath.Join(dir, ".agents", "skills", "planning", "SKILL.md"))
	assert.FileExists(t, filepath.Join(dir, ".agents", "skills", "agent-pipeline", "SKILL.md"))
	assert.FileExists(t, filepath.Join(dir, ".agents", "skills", "worktree-isolation", "SKILL.md"))
	assert.FileExists(t, filepath.Join(dir, ".agents", "skills", "product-discovery", "SKILL.md"))
	assert.FileExists(t, filepath.Join(dir, ".agents", "skills", "competitive-analysis", "SKILL.md"))
	assert.FileExists(t, filepath.Join(dir, ".agents", "skills", "metrics", "SKILL.md"))
}

func TestAdapter_Generate_MarksOnlyMixedSharedSkillsAsOpenCodeOnly(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		platforms []string
		wantMark  bool
	}{
		{name: "mixed Codex and OpenCode", platforms: []string{"codex", "opencode"}, wantMark: true},
		{name: "single OpenCode", platforms: []string{"opencode"}, wantMark: false},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			cfg := config.DefaultFullConfig("demo")
			cfg.Platforms = test.platforms

			_, err := NewWithRoot(dir).Generate(context.Background(), cfg)
			require.NoError(t, err)

			for _, name := range []string{"auto", "auto-status", "auto-go", "planning"} {
				path := filepath.Join(dir, ".agents", "skills", name, "SKILL.md")
				content, readErr := os.ReadFile(path)
				require.NoError(t, readErr, path)
				if test.wantMark {
					assert.Contains(t, string(content), `description: "[OpenCode-only] `, path)
				} else {
					assert.NotContains(t, string(content), "[OpenCode-only]", path)
				}
			}

			commandPath := filepath.Join(dir, ".opencode", "commands", "auto.md")
			command, readErr := os.ReadFile(commandPath)
			require.NoError(t, readErr)
			assert.NotContains(t, string(command), "[OpenCode-only]")
			assert.Contains(t, string(command), "/auto")
		})
	}
}

func TestAdapter_Update_AutoSharedSurfacePrunesLegacyExtendedSkills(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	fullCfg := config.DefaultFullConfig("demo")
	fullCfg.Platforms = []string{"opencode"}

	_, err := a.Generate(context.Background(), fullCfg)
	require.NoError(t, err)
	assert.FileExists(t, filepath.Join(dir, ".agents", "skills", "metrics", "SKILL.md"))

	mixedCfg := config.DefaultFullConfig("demo")
	mixedCfg.Platforms = []string{"codex", "opencode"}
	mixedCfg.Skills.SharedSurface = config.SharedSurfaceAuto

	_, err = a.Update(context.Background(), mixedCfg)
	require.NoError(t, err)

	_, statErr := os.Stat(filepath.Join(dir, ".agents", "skills", "metrics"))
	assert.True(t, os.IsNotExist(statErr), "legacy extended shared skill dir should be pruned in mixed mode")
	assert.FileExists(t, filepath.Join(dir, ".agents", "skills", "planning", "SKILL.md"))
}

func TestAdapter_Generate_AutoSharedSurfaceUsesCoreSharedSkillSetInMixedMode(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := NewWithRoot(dir)
	cfg := config.DefaultFullConfig("demo")
	cfg.Platforms = []string{"codex", "opencode"}
	cfg.Skills.SharedSurface = config.SharedSurfaceAuto

	_, err := a.Generate(context.Background(), cfg)
	require.NoError(t, err)

	assert.FileExists(t, filepath.Join(dir, ".agents", "skills", "planning", "SKILL.md"))
	assert.FileExists(t, filepath.Join(dir, ".agents", "skills", "agent-pipeline", "SKILL.md"))
	assert.FileExists(t, filepath.Join(dir, ".agents", "skills", "make-interfaces-feel-better", "SKILL.md"))
	assert.NoFileExists(t, filepath.Join(dir, ".agents", "skills", "metrics", "SKILL.md"))
	assert.NoFileExists(t, filepath.Join(dir, ".agents", "skills", "product-discovery", "SKILL.md"))
}
