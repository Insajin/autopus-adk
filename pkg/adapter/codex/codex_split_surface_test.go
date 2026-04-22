package codex

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/config"
)

func TestAdapter_Update_SplitCompilerPrunesRepoVisibleLongTailWhenMovingToPluginSurface(t *testing.T) {
	t.Parallel()

	a := NewWithRoot(t.TempDir())
	fullCfg := config.DefaultFullConfig("split-codex")
	fullCfg.Platforms = []string{"codex", "opencode"}

	_, err := a.Generate(context.Background(), fullCfg)
	require.NoError(t, err)
	assert.FileExists(t, filepath.Join(a.root, ".codex", "skills", "metrics.md"), "acceptance S2: full mode must start with the existing repo-visible Codex skill path")

	splitCfg := config.DefaultFullConfig("split-codex")
	splitCfg.Platforms = []string{"codex", "opencode"}
	splitCfg.Skills.SharedSurface = config.SharedSurfaceCore
	splitCfg.Skills.Compiler.Mode = config.SkillCompilerModeSplit
	splitCfg.Skills.Compiler.CodexLongTailTarget = config.SkillLongTailTargetPlugin

	_, err = a.Update(context.Background(), splitCfg)
	require.NoError(t, err)

	assert.FileExists(t, filepath.Join(a.root, ".autopus", "plugins", "auto", "skills", "metrics", "SKILL.md"), "acceptance S7: split compiler must materialize Codex long-tail skills in the plugin-scoped target")
	assert.NoFileExists(t, filepath.Join(a.root, ".codex", "skills", "metrics.md"), "acceptance S4/S7: stale repo-visible Codex long-tail artifact must be pruned when ownership moves to the plugin surface")
}
