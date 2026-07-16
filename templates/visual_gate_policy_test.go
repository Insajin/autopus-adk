package templates_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFrontendVerifySurfacesDescribeCompatibleSnapshotPolicy(t *testing.T) {
	t.Parallel()

	paths := []string{
		filepath.Join("..", "content", "skills", "frontend-verify.md"),
		filepath.Join(templateRoot(), "codex", "skills", "frontend-verify.md.tmpl"),
		filepath.Join(templateRoot(), "gemini", "skills", "frontend-verify", "SKILL.md.tmpl"),
	}
	for _, path := range paths {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			raw, err := os.ReadFile(path)
			require.NoError(t, err)
			text := string(raw)
			assert.Contains(t, text, "latest.v2.json")
			assert.Contains(t, text, "--update-snapshots=none")
			assert.Contains(t, text, "advisory by default")
			assert.Contains(t, text, "custom Playwright project names")
		})
	}
}

func TestFrontendSpecialistSurfacesDoNotClaimQAMESHIngestion(t *testing.T) {
	t.Parallel()

	paths := []string{
		filepath.Join("..", "content", "agents", "frontend-specialist.md"),
		filepath.Join(templateRoot(), "codex", "agents", "frontend-specialist.toml.tmpl"),
		filepath.Join(templateRoot(), "gemini", "agents", "frontend-specialist.md.tmpl"),
	}
	for _, path := range paths {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			raw, err := os.ReadFile(path)
			require.NoError(t, err)
			text := string(raw)
			assert.Contains(t, text, "QAMESH handoff candidate")
			assert.Contains(t, text, "ingestion is not proven")
			assert.NotContains(t, text, "QAMESH-compatible metadata evidence")
		})
	}
}
