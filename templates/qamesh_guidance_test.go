package templates_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/config"
	tmpl "github.com/insajin/autopus-adk/pkg/template"
)

func TestQAMESHGuidanceSourceContracts(t *testing.T) {
	t.Parallel()

	root := templateRoot()
	staticPaths := []string{
		filepath.Join(root, "..", "content", "skills", "testing-strategy.md"),
		filepath.Join(root, "codex", "prompts", "auto-qa.md.tmpl"),
		filepath.Join(root, "codex", "skills", "auto-qa.md.tmpl"),
		filepath.Join(root, "gemini", "commands", "auto", "qa.toml.tmpl"),
		filepath.Join(root, "gemini", "skills", "auto-qa", "SKILL.md.tmpl"),
	}
	for _, path := range staticPaths {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			body, err := os.ReadFile(path)
			require.NoError(t, err)
			assertQAMESHGuidance(t, string(body))
		})
	}
}

func TestQAMESHRouterTemplateGuidance(t *testing.T) {
	t.Parallel()

	e := tmpl.New()
	cfg := config.DefaultFullConfig("qa-project")
	paths := []string{
		filepath.Join(templateRoot(), "claude", "commands", "auto-router.md.tmpl"),
		filepath.Join(templateRoot(), "codex", "prompts", "auto.md.tmpl"),
		filepath.Join(templateRoot(), "gemini", "commands", "auto-router.md.tmpl"),
	}
	for _, tmplPath := range paths {
		tmplPath := tmplPath
		t.Run(filepath.Base(filepath.Dir(tmplPath))+"-"+filepath.Base(tmplPath), func(t *testing.T) {
			t.Parallel()
			result, err := e.RenderFile(tmplPath, cfg)
			require.NoError(t, err)
			assertQAMESHGuidance(t, result)
		})
	}
}

func assertQAMESHGuidance(t *testing.T, body string) {
	t.Helper()
	assert.Contains(t, body, "QAMESH")
	assert.Contains(t, body, "auto qa plan")
	assert.Contains(t, body, "auto qa run")
	assert.Contains(t, body, "auto qa evidence")
	assert.Contains(t, body, "auto qa feedback")
}
