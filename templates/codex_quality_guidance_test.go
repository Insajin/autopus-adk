package templates_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCodexQualityGuidanceDocumentsLoadedAgentBoundary(t *testing.T) {
	t.Parallel()

	root := templateRoot()
	adaptivePaths := []string{
		filepath.Join(root, "..", "content", "skills", "adaptive-quality.md"),
		filepath.Join(root, "codex", "skills", "adaptive-quality.md.tmpl"),
		filepath.Join(root, "gemini", "skills", "adaptive-quality", "SKILL.md.tmpl"),
	}
	for _, path := range adaptivePaths {
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		content := string(data)
		assert.Contains(t, content, "auto quality <mode> --apply")
		assert.Contains(t, content, "auto quality supervisor inherit --apply")
		assert.Contains(t, content, "User Codex runtime default")
		assert.Contains(t, content, "quality-managed supervisor")
		assert.Contains(t, content, "user-owned root model or effort assignments remain preserved and take precedence")
		assert.Contains(t, content, "new Codex session")
		assert.Contains(t, content, "cannot hot-swap agents already loaded")
		assert.NotContains(t, content, "Ultra uses Sol/`ultra` for the supervisor and orchestra")
	}

	pipelinePaths := []string{
		filepath.Join(root, "..", "content", "skills", "agent-pipeline.md"),
		filepath.Join(root, "codex", "skills", "agent-pipeline.md.tmpl"),
		filepath.Join(root, "gemini", "skills", "agent-pipeline", "SKILL.md.tmpl"),
	}
	for _, path := range pipelinePaths {
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		content := string(data)
		assert.Contains(t, content, "quality-managed depth-0 supervisor")
		assert.Contains(t, content, "`inherit` supervisor keeps the user's Codex runtime default")
		assert.Contains(t, content, "User-owned root model or effort assignments remain preserved and take precedence")
		assert.NotContains(t, content, "the depth-0 supervisor and orchestra use Sol/`ultra`")
	}
}
