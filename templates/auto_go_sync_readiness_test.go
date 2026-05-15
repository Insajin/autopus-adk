package templates_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/config"
	tmpl "github.com/insajin/autopus-adk/pkg/template"
)

func TestAutoGoSyncReadinessContracts(t *testing.T) {
	t.Parallel()

	e := tmpl.New()
	cfg := config.DefaultFullConfig("sync-ready-project")
	root := templateRoot()
	workflowPaths := []string{
		filepath.Join(root, "..", "content", "skills", "agent-pipeline.md"),
		filepath.Join(root, "claude", "commands", "auto-router.md.tmpl"),
		filepath.Join(root, "codex", "prompts", "auto-go.md.tmpl"),
		filepath.Join(root, "codex", "skills", "auto-go.md.tmpl"),
		filepath.Join(root, "codex", "skills", "agent-pipeline.md.tmpl"),
		filepath.Join(root, "gemini", "skills", "auto-go", "SKILL.md.tmpl"),
		filepath.Join(root, "gemini", "skills", "agent-pipeline", "SKILL.md.tmpl"),
	}
	for _, path := range workflowPaths {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			result, err := renderOrReadTemplate(e, path, cfg)
			require.NoError(t, err)
			assert.Contains(t, result, "Sync Readiness Gate")
			assert.Contains(t, result, "completion_verdict_preview")
			assert.Contains(t, result, "spec_status_after_go")
			assert.Contains(t, result, "implemented")
			assert.NotContains(t, result, "Update the SPEC file status to `\"done\"`")
			assert.NotContains(t, result, "SPEC status = \"done\"")
		})
	}

	reviewerPaths := []string{
		filepath.Join(root, "..", "content", "agents", "reviewer.md"),
		filepath.Join(root, "codex", "agents", "reviewer.toml.tmpl"),
		filepath.Join(root, "gemini", "agents", "reviewer.md.tmpl"),
	}
	for _, path := range reviewerPaths {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			result, err := renderOrReadTemplate(e, path, cfg)
			require.NoError(t, err)
			assert.Contains(t, result, "implemented")
			assert.Contains(t, result, "Sync Readiness Gate")
			assert.NotContains(t, result, "SPEC status를 \"done\"")
		})
	}
}
