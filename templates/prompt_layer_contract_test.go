package templates_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/config"
	tmpl "github.com/insajin/autopus-adk/pkg/template"
)

func TestPromptLayerManifestSourceContracts(t *testing.T) {
	t.Parallel()

	e := tmpl.New()
	cfg := config.DefaultFullConfig("prompt-layer-project")
	root := templateRoot()
	files := map[string][]string{
		filepath.Join(root, "..", "content", "rules", "spec-quality.md"):      {"prompt layer manifest", "stable", "snapshot", "ephemeral", "cache invalidation"},
		filepath.Join(root, "..", "content", "skills", "agent-pipeline.md"):   {"prompt layer manifest", "stable", "snapshot", "ephemeral", "cache invalidation"},
		filepath.Join(root, "shared", "orchestra-context.md.tmpl"):            {"prompt layer manifest", "stable", "snapshot", "ephemeral", "cache invalidation"},
		filepath.Join(root, "shared", "orchestra-reviewer.md.tmpl"):           {"prompt layer manifest", "stable", "snapshot", "ephemeral", "cache invalidation"},
		filepath.Join(root, "codex", "prompts", "auto-plan.md.tmpl"):          {"prompt layer manifest", "stable", "snapshot", "ephemeral", "cache invalidation"},
		filepath.Join(root, "codex", "skills", "auto-plan.md.tmpl"):           {"prompt layer manifest", "stable", "snapshot", "ephemeral", "cache invalidation"},
		filepath.Join(root, "gemini", "skills", "auto-plan", "SKILL.md.tmpl"): {"prompt layer manifest", "stable", "snapshot", "ephemeral", "cache invalidation"},
		filepath.Join(root, "claude", "commands", "auto-router.md.tmpl"):      {"prompt layer manifest", "stable", "snapshot", "ephemeral", "cache invalidation"},
	}

	for path, expected := range files {
		path, expected := path, expected
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			body, err := os.ReadFile(path)
			require.NoError(t, err)
			text := string(body)
			if strings.HasSuffix(path, ".tmpl") && !strings.Contains(path, filepath.Join("shared", "orchestra-")) {
				rendered, renderErr := e.RenderFile(path, cfg)
				require.NoError(t, renderErr)
				text = rendered
			}
			for _, phrase := range expected {
				assert.Contains(t, text, phrase)
			}
		})
	}
}
