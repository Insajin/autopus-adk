package claude_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/adapter/claude"
	"github.com/insajin/autopus-adk/pkg/config"
)

func TestClaudeAdapter_Generate_PropagatesWorkspacePolicyContext(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := claude.NewWithRoot(dir)

	_, err := a.Generate(context.Background(), config.DefaultFullConfig("demo"))
	require.NoError(t, err)

	autoSkill, err := os.ReadFile(filepath.Join(dir, ".claude", "skills", "auto", "SKILL.md"))
	require.NoError(t, err)
	assert.Contains(t, string(autoSkill), ".autopus/project/workspace.md")
	assert.Contains(t, string(autoSkill), "nested git repo")
	assert.Contains(t, string(autoSkill), "generated/runtime")
	assert.Contains(t, string(autoSkill), "tracking/commit policy")
}
