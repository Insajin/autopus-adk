package content_test

import (
	"testing"

	contentfs "github.com/insajin/autopus-adk/content"
	"github.com/insajin/autopus-adk/pkg/content"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKoreanWritingRefinerCatalogAndTransforms(t *testing.T) {
	t.Parallel()

	catalog, err := content.LoadSkillCatalogFromFS(contentfs.FS, "skills")
	require.NoError(t, err)

	entry, ok := catalog.Get("korean-writing-refiner")
	require.True(t, ok, "korean writing refiner must be registered as canonical ADK skill content")
	assert.Equal(t, "documentation", entry.Category)
	assert.Equal(t, []string{"research"}, entry.Bundles)
	assert.ElementsMatch(t, []string{"claude", "codex", "gemini", "opencode"}, entry.CompileTargets)
	assert.False(t, content.IsCoreSkill(entry.Name), "copy refinement is useful long-tail guidance, not a core execution skill")

	transformer, err := content.NewSkillTransformerFromFS(contentfs.FS, "skills")
	require.NoError(t, err)

	for _, platform := range []string{"claude", "codex", "gemini", "opencode"} {
		platform := platform
		t.Run(platform, func(t *testing.T) {
			t.Parallel()

			skills, report, err := transformer.TransformForPlatform(platform)
			require.NoError(t, err)
			assert.Contains(t, report.Compatible, "korean-writing-refiner")

			skill := findTransformedSkill(t, skills, "korean-writing-refiner")
			assert.Contains(t, skill.Content, "외부 표면에 노출될 한국어 최종 문안")
			assert.Contains(t, skill.Content, "분석, 리뷰, 전략 정리, 코드 구현")
			assert.Contains(t, skill.Content, "의미 보존")
		})
	}
}
