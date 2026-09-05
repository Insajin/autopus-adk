package templates_test

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/config"
	tmpl "github.com/insajin/autopus-adk/pkg/template"
)

// codexPipelineNativeSurface concatenates every Go source file that renders the
// Codex agent-pipeline skill. The skill body is assembled from siblings
// (phase flow, gate applicability, probe gate, telemetry, completion), and the
// builder is unexported, so the concatenated sources are the only way this
// package can assert the emitted contract without duplicating the renderer.
func codexPipelineNativeSurface(root string) (string, error) {
	pattern := filepath.Join(root, "..", "pkg", "adapter", "codex",
		"codex_extended_skill_rewrites_pipeline*.go")
	paths, err := filepath.Glob(pattern)
	if err != nil {
		return "", err
	}
	sort.Strings(paths)
	var surface strings.Builder
	for _, path := range paths {
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			return "", readErr
		}
		surface.Write(body)
		surface.WriteString("\n")
	}
	return surface.String(), nil
}

// A go/pipeline surface that names a runtime on one platform and omits it on
// another is a silent behavioral fork: the supervisor on the quiet platform
// re-runs verified work, self-assigns applicability, screenshots in no-capture
// mode, re-discovers the same findings, and reports no lead time. Every surface
// that drives `go` must name all four runtimes the pipeline now consumes.
func TestPipelineRuntimeSurfaceParity(t *testing.T) {
	t.Parallel()

	root := templateRoot()
	e := tmpl.New()
	cfg := config.DefaultFullConfig("pipeline-runtime-project")
	runtimeTokens := []string{
		// gate applicability + exact-input evidence reuse
		"auto spec gates",
		"gate-applicability.json",
		"reusable",
		// no-capture UX verification
		"no-capture",
		// repeat-discovery re-review
		"discovery_repeat_detected",
		// lead-time telemetry
		"first_vertical_slice",
		"auto telemetry leadtime",
	}

	cases := []struct {
		name string
		path string
	}{
		{
			name: "agent-pipeline-content",
			path: filepath.Join(root, "..", "content", "skills", "agent-pipeline.md"),
		},
		{
			name: "gemini-agent-pipeline-template",
			path: filepath.Join(root, "gemini", "skills", "agent-pipeline", "SKILL.md.tmpl"),
		},
		{
			name: "omp-agent-pipeline-template",
			path: filepath.Join(root, "shared", "omp-agent-pipeline.md.tmpl"),
		},
		{
			name: "claude-workflows",
			path: filepath.Join(root, "claude", "commands", "auto-workflows.md.tmpl"),
		},
		{
			name: "codex-go-skill",
			path: filepath.Join(root, "codex", "skills", "auto-go.md.tmpl"),
		},
		{
			name: "codex-go-prompt",
			path: filepath.Join(root, "codex", "prompts", "auto-go.md.tmpl"),
		},
		{
			name: "gemini-go-skill",
			path: filepath.Join(root, "gemini", "skills", "auto-go", "SKILL.md.tmpl"),
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			text, err := semanticContractSurface(e, tc.path, cfg)
			require.NoError(t, err)
			for _, token := range runtimeTokens {
				assert.Contains(t, text, token, "%s should contain %q", tc.path, token)
			}
		})
	}

	t.Run("codex-agent-pipeline-native-source", func(t *testing.T) {
		t.Parallel()
		text, err := codexPipelineNativeSurface(root)
		require.NoError(t, err)
		for _, token := range runtimeTokens {
			assert.Contains(t, text, token,
				"the Codex agent-pipeline rewrite should contain %q", token)
		}
	})
}

// The applicability vocabulary is a closed set shared by the classifier and
// every prompt surface. A surface that keeps the pre-receipt wording tells the
// supervisor that reuse is impossible, so it re-runs evidence the classifier
// already accepted.
func TestPipelineRuntimeSurfaceDropsReusableDisclaimer(t *testing.T) {
	t.Parallel()

	root := templateRoot()
	e := tmpl.New()
	cfg := config.DefaultFullConfig("pipeline-runtime-project")

	paths := []string{
		filepath.Join(root, "..", "content", "skills", "agent-pipeline.md"),
		filepath.Join(root, "..", "content", "agents", "spec-writer.md"),
		filepath.Join(root, "gemini", "skills", "agent-pipeline", "SKILL.md.tmpl"),
		filepath.Join(root, "shared", "omp-agent-pipeline.md.tmpl"),
		filepath.Join(root, "claude", "commands", "auto-workflows.md.tmpl"),
		filepath.Join(root, "codex", "skills", "auto-go.md.tmpl"),
		filepath.Join(root, "codex", "prompts", "auto-go.md.tmpl"),
		filepath.Join(root, "gemini", "skills", "auto-go", "SKILL.md.tmpl"),
		filepath.Join(root, "codex", "skills", "auto-plan.md.tmpl"),
		filepath.Join(root, "codex", "prompts", "auto-plan.md.tmpl"),
		filepath.Join(root, "gemini", "skills", "auto-plan", "SKILL.md.tmpl"),
	}

	for _, path := range paths {
		path := path
		t.Run(filepath.Base(filepath.Dir(path))+"/"+filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			text, err := semanticContractSurface(e, path, cfg)
			require.NoError(t, err)
			for _, stale := range []string{
				"not a valid value in this release",
				"유효한 값이 아닙니다",
				"no exact-input evidence engine exists",
			} {
				assert.NotContains(t, text, stale,
					"%s still disclaims `reusable`; the evidence engine now grants it", path)
			}
		})
	}
}
