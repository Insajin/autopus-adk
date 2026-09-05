package templates_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/config"
	tmpl "github.com/insajin/autopus-adk/pkg/template"
)

// noCaptureContractTokens are the strings pkg/qa/capture enforces. These
// templates are generated from content/, so a missing token means the generated
// surface drifted from the source instead of being regenerated.
var noCaptureContractTokens = []string{
	"## No-Capture Contract",
	"no-capture",
	"dom_geometry",
	"accessibility_tree",
	"keyboard_navigation",
	"state_transition",
	"oracles_collected",
	"missing_no_capture_oracle:<kind>",
}

func TestNoCaptureContractRendersOnEveryUXTemplate(t *testing.T) {
	t.Parallel()
	e := tmpl.New()
	cfg := config.DefaultFullConfig("no-capture-project")

	paths := []string{
		filepath.Join(templateRoot(), "codex", "agents", "frontend-specialist.toml.tmpl"),
		filepath.Join(templateRoot(), "codex", "agents", "ux-validator.toml.tmpl"),
		filepath.Join(templateRoot(), "codex", "skills", "frontend-verify.md.tmpl"),
		filepath.Join(templateRoot(), "gemini", "agents", "frontend-specialist.md.tmpl"),
		filepath.Join(templateRoot(), "gemini", "agents", "ux-validator.md.tmpl"),
		filepath.Join(templateRoot(), "gemini", "skills", "frontend-verify", "SKILL.md.tmpl"),
	}

	for _, tmplPath := range paths {
		tmplPath := tmplPath
		t.Run(filepath.Base(filepath.Dir(tmplPath))+"-"+filepath.Base(tmplPath), func(t *testing.T) {
			t.Parallel()
			result, err := e.RenderFile(tmplPath, cfg)
			require.NoError(t, err)
			for _, token := range noCaptureContractTokens {
				assert.Contains(t, result, token)
			}
		})
	}
}
