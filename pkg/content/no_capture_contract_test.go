package content_test

import (
	"testing"

	contentfs "github.com/insajin/autopus-adk/content"
	"github.com/insajin/autopus-adk/pkg/content"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// noCaptureContractTokens are the strings pkg/qa/capture actually enforces. A
// surface that drops one instructs the agent to collect three oracles for a
// gate that requires four, or to report a kind conformance does not recognise,
// which fails the run for a reason the prompt never mentioned.
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

// TestNoCaptureContractReachesEveryUXSkillSurface pins the contract on the
// frontend-verify skill for every platform that compiles it.
func TestNoCaptureContractReachesEveryUXSkillSurface(t *testing.T) {
	t.Parallel()

	transformer, err := content.NewSkillTransformerFromFS(contentfs.FS, "skills")
	require.NoError(t, err)

	for _, platform := range []string{"claude", "codex", "gemini", "opencode"} {
		platform := platform
		t.Run(platform, func(t *testing.T) {
			t.Parallel()

			skills, _, err := transformer.TransformForPlatform(platform)
			require.NoError(t, err)

			verifySkill := findTransformedSkill(t, skills, "frontend-verify")
			for _, token := range noCaptureContractTokens {
				assert.Contains(t, verifySkill.Content, token)
			}
			assert.Contains(t, verifySkill.Content,
				`must be "screenshot" or "no-capture"`,
				"the skill must name the exact config error a bad capture value produces")
		})
	}
}

// TestNoCaptureContractReachesEveryUXAgentSurface pins the contract on both UX
// agents, in the embedded source and in the omp projection.
func TestNoCaptureContractReachesEveryUXAgentSurface(t *testing.T) {
	t.Parallel()

	sources := ompAgentSources(t)
	for _, name := range []string{"frontend-specialist", "ux-validator"} {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			src, ok := sources[name]
			require.True(t, ok, "agent source %s must exist in the embedded content FS", name)

			_, _, body := parseOMPAgentOutput(t, content.TransformAgentForOMP(src))
			for _, token := range noCaptureContractTokens {
				assert.Contains(t, src.Body, token, "content source must carry %q", token)
				assert.Contains(t, body, token, "omp projection must carry %q", token)
			}
			assert.Contains(t, src.Body, "verify.capture",
				"the branch must name the config key that selects it")
		})
	}
}

// TestFrontendSpecialistForbidsScreenshotsInNoCaptureMode keeps the branch
// actionable: the agent has to be told which call to stop making, otherwise
// "no-capture" reads as advice and the capture contract rejects the run.
func TestFrontendSpecialistForbidsScreenshotsInNoCaptureMode(t *testing.T) {
	t.Parallel()

	body := ompAgentSource(t, "frontend-specialist").Body
	assert.Contains(t, body, "do NOT call `page.screenshot`")
	assert.Contains(t, body, "`Status: BLOCKED`")
}
