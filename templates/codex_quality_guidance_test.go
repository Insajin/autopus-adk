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
	paths := []string{
		filepath.Join(root, "..", "content", "skills", "adaptive-quality.md"),
		filepath.Join(root, "codex", "skills", "adaptive-quality.md.tmpl"),
		filepath.Join(root, "gemini", "skills", "adaptive-quality", "SKILL.md.tmpl"),
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		content := string(data)
		assert.Contains(t, content, "auto quality set <mode>")
		assert.Contains(t, content, "auto update")
		assert.Contains(t, content, "new Codex session")
		assert.Contains(t, content, "cannot hot-swap agents already loaded")
	}
}
