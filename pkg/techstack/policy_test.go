package techstack

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInferModeGreenfieldKeywordWinsOverManifest(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"dependencies":{"react":"^18.0.0"}}`), 0o644))

	mode := InferMode(dir, "신규 프로젝트를 최신 기술스택으로 스캐폴드")

	assert.Equal(t, ModeGreenfield, mode)
}

func TestInferModeBrownfieldWhenManifestExists(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/app\n\ngo 1.22\n"), 0o644))

	mode := InferMode(dir, "add a CLI command")

	assert.Equal(t, ModeBrownfield, mode)
}

func TestValidateDecisionGreenfieldRequiresSourceRefs(t *testing.T) {
	t.Parallel()

	err := ValidateDecision(Decision{
		Mode: ModeGreenfield,
		Selected: []Candidate{{
			Name:    "React",
			Version: "19.1.0",
		}},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "source ref")
}

func TestValidateDecisionGreenfieldRejectsPrereleaseByDefault(t *testing.T) {
	t.Parallel()

	err := ValidateDecision(Decision{
		Mode: ModeGreenfield,
		Selected: []Candidate{{
			Name:    "Next.js",
			Version: "16.0.0-rc.1",
			SourceRefs: []SourceRef{{
				Name:      "official release notes",
				Version:   "16.0.0-rc.1",
				CheckedAt: time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC),
			}},
		}},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "allow_prerelease")
}

func TestValidateDecisionGreenfieldPassesWithFreshSources(t *testing.T) {
	t.Parallel()

	checked := time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC)
	err := ValidateDecision(Decision{
		Mode: ModeGreenfield,
		Selected: []Candidate{{
			Name:      "Tailwind CSS",
			Version:   "4.1.0",
			Stability: "stable",
			SourceRefs: []SourceRef{{
				Name:      "official docs",
				URL:       "https://tailwindcss.com/docs/installation",
				Version:   "4.1.0",
				CheckedAt: checked,
			}},
		}},
	})

	require.NoError(t, err)
}

func TestRenderMarkdownIncludesVersionEvidence(t *testing.T) {
	t.Parallel()

	checked := time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC)
	body := RenderMarkdown(Decision{
		Mode:        ModeGreenfield,
		GeneratedAt: checked,
		Selected: []Candidate{{
			Name:      "Vite",
			Version:   "7.0.0",
			Stability: "stable",
			SourceRefs: []SourceRef{{
				Name:      "official docs",
				Version:   "7.0.0",
				CheckedAt: checked,
			}},
		}},
	})

	assert.Contains(t, body, "## Technology Stack Decision")
	assert.Contains(t, body, "greenfield")
	assert.Contains(t, body, "Vite")
	assert.Contains(t, body, "official docs@7.0.0 checked 2026-05-07")
}
