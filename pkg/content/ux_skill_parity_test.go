package content_test

import (
	"testing"

	contentfs "github.com/insajin/autopus-adk/content"
	"github.com/insajin/autopus-adk/pkg/content"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUXSkillIntelligencePassTransformsForAllSupportedPlatforms(t *testing.T) {
	t.Parallel()

	transformer, err := content.NewSkillTransformerFromFS(contentfs.FS, "skills")
	require.NoError(t, err)

	platforms := []string{"claude", "codex", "gemini", "opencode"}
	for _, platform := range platforms {
		platform := platform
		t.Run(platform, func(t *testing.T) {
			t.Parallel()

			skills, _, err := transformer.TransformForPlatform(platform)
			require.NoError(t, err)

			frontendSkill := findTransformedSkill(t, skills, "frontend-skill")
			assert.Contains(t, frontendSkill.Content, "## UX Intelligence Pass")
			assert.Contains(t, frontendSkill.Content, "Design Discovery Matrix")
			assert.Contains(t, frontendSkill.Content, "Pre-delivery checklist")

			verifySkill := findTransformedSkill(t, skills, "frontend-verify")
			assert.Contains(t, verifySkill.Content, "Phase 0.6: UX 인텔리전스 기준 합성")
			assert.Contains(t, verifySkill.Content, "## UX Intelligence")
			assert.Contains(t, verifySkill.Content, "--viewport-matrix")
		})
	}
}

func findTransformedSkill(t *testing.T, skills []content.TransformedSkill, name string) content.TransformedSkill {
	t.Helper()

	for _, skill := range skills {
		if skill.Name == name {
			return skill
		}
	}
	require.Failf(t, "missing transformed skill", "skill %q was not generated", name)
	return content.TransformedSkill{}
}
