package claude_test

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	contentfs "github.com/insajin/autopus-adk/content"
	"github.com/insajin/autopus-adk/pkg/adapter/claude"
	"github.com/insajin/autopus-adk/pkg/config"
)

// generateClaudeAgentsForMode renders the claude surface under one quality
// preset and returns the installed agent directory.
func generateClaudeAgentsForMode(t *testing.T, mode string) string {
	t.Helper()
	dir := t.TempDir()
	cfg := config.DefaultFullConfig("agent-model")
	cfg.Quality.Providers = map[string]string{config.QualityProviderClaude: mode}
	_, err := claude.NewWithRoot(dir).Generate(context.Background(), cfg)
	require.NoError(t, err)
	return filepath.Join(dir, ".claude", "agents", "autopus")
}

// agentFrontmatterLines returns the frontmatter lines of one installed agent.
func agentFrontmatterLines(t *testing.T, agentDir, name string) []string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(agentDir, name))
	require.NoError(t, err)
	frontmatter, _ := splitFrontmatterBlock(string(raw))
	require.NotEmpty(t, frontmatter, "%s must ship frontmatter", name)
	return strings.Split(frontmatter, "\n")
}

// sourceFrontmatterLines returns the frontmatter lines of the embedded source.
func sourceFrontmatterLines(t *testing.T, name string) []string {
	t.Helper()
	raw, err := fs.ReadFile(contentfs.FS, "agents/"+name)
	require.NoError(t, err)
	frontmatter, _ := splitFrontmatterBlock(string(raw))
	require.NotEmpty(t, frontmatter)
	return strings.Split(frontmatter, "\n")
}

// TestClaudeAgentModelFollowsBalancedPreset pins the balanced preset as the
// tier source: executor is promoted to opus even though the source file still
// says sonnet, and an agent the preset leaves at sonnet stays there.
func TestClaudeAgentModelFollowsBalancedPreset(t *testing.T) {
	t.Parallel()
	agentDir := generateClaudeAgentsForMode(t, "balanced")

	assert.Contains(t, agentFrontmatterLines(t, agentDir, "executor.md"), "model: opus")
	assert.Contains(t, agentFrontmatterLines(t, agentDir, "tester.md"), "model: sonnet")
}

// TestClaudeAgentModelFollowsUltraPreset proves the preset — not the source
// frontmatter — decides the tier: the reasoning core uses fable while other
// ultra agents use opus.
func TestClaudeAgentModelFollowsUltraPreset(t *testing.T) {
	t.Parallel()
	agentDir := generateClaudeAgentsForMode(t, "ultra")

	assert.Contains(t, agentFrontmatterLines(t, agentDir, "planner.md"), "model: fable")
	assert.Contains(t, agentFrontmatterLines(t, agentDir, "tester.md"), "model: opus")
}

// TestClaudeAgentProfileRewriteKeepsFrontmatter verifies model and effort move
// together while every unrelated sibling key keeps its authored order.
func TestClaudeAgentProfileRewriteKeepsFrontmatter(t *testing.T) {
	t.Parallel()
	agentDir := generateClaudeAgentsForMode(t, "balanced")

	want := sourceFrontmatterLines(t, "executor.md")
	for i, line := range want {
		switch {
		case strings.HasPrefix(line, "model:"):
			want[i] = "model: opus"
		case strings.HasPrefix(line, "effort:"):
			want[i] = "effort: high"
		}
	}

	got := agentFrontmatterLines(t, agentDir, "executor.md")
	assert.Equal(t, want, got)
	assert.Contains(t, got, "effort: high")
}

func TestClaudeAgentProfileUltraProjectsMaximumEffort(t *testing.T) {
	t.Parallel()
	agentDir := generateClaudeAgentsForMode(t, "ultra")
	assert.Contains(t, agentFrontmatterLines(t, agentDir, "tester.md"), "effort: max")
	assert.Contains(t, agentFrontmatterLines(t, agentDir, "planner.md"), "effort: max")
}
