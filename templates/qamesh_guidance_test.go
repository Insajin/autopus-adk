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
			result, err := semanticContractSurface(e, tmplPath, cfg)
			require.NoError(t, err)
			assertQAMESHGuidance(t, result)
		})
	}
}

func TestAutoGoQAMESHScopeBudgetGuidance(t *testing.T) {
	t.Parallel()

	e := tmpl.New()
	cfg := config.DefaultFullConfig("qa-project")
	root := templateRoot()
	paths := []string{
		filepath.Join(root, "..", "content", "skills", "agent-pipeline.md"),
		filepath.Join(root, "claude", "commands", "auto-router.md.tmpl"),
		filepath.Join(root, "codex", "prompts", "auto-go.md.tmpl"),
		filepath.Join(root, "codex", "skills", "auto-go.md.tmpl"),
		filepath.Join(root, "codex", "skills", "agent-pipeline.md.tmpl"),
		filepath.Join(root, "gemini", "commands", "auto-router.md.tmpl"),
		filepath.Join(root, "gemini", "skills", "auto-go", "SKILL.md.tmpl"),
		filepath.Join(root, "gemini", "skills", "agent-pipeline", "SKILL.md.tmpl"),
	}
	for _, path := range paths {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			body, err := renderOrReadTemplate(e, path, cfg)
			require.NoError(t, err)
			assert.Contains(t, body, "affected/fast/smoke QAMESH")
			assert.Contains(t, body, "auto qa plan --lane fast --format json")
			assert.Contains(t, body, "full GUI/native/release matrix")
			assert.Contains(t, body, "auto canary")
			assert.Contains(t, body, "post-deploy smoke/status")
		})
	}
}

func assertQAMESHGuidance(t *testing.T, body string) {
	t.Helper()
	assert.Contains(t, body, "QAMESH")
	assert.Contains(t, body, "auto qa init")
	assert.Contains(t, body, "auto qa plan")
	assert.Contains(t, body, "auto qa run")
	assert.Contains(t, body, "auto qa explore")
	assert.Contains(t, body, "auto qa release")
	assert.Contains(t, body, "auto qa evidence")
	assert.Contains(t, body, "auto qa feedback")
	assert.Contains(t, body, "ADK is a harness")
	assert.Contains(t, body, "project-local Journey Pack")
	assert.Contains(t, body, "QAMESH is the default project QA orchestration layer")
	assert.Contains(t, body, "Playwright")
	assert.Contains(t, body, "not a competing")
	assert.Contains(t, body, "choose between QAMESH and Playwright")
	assert.Contains(t, body, "canary-explicit")
	assert.Contains(t, body, "post-deploy smoke")
}

func renderOrReadTemplate(e *tmpl.Engine, path string, cfg *config.HarnessConfig) (string, error) {
	return semanticContractSurface(e, path, cfg)
}
