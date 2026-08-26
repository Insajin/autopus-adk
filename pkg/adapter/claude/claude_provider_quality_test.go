package claude

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/config"
)

func TestClaudeWorkflowUsesClaudeProviderQualityBeforeGlobalDefault(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultFullConfig("claude-provider-quality")
	cfg.Quality.Providers = map[string]string{
		config.QualityProviderClaude: "ultra",
		config.QualityProviderCodex:  "balanced",
	}
	files, err := NewWithRoot(t.TempDir()).prepareWorkflowSkillMappings(cfg)
	require.NoError(t, err)

	var goSkill string
	for _, file := range files {
		if file.TargetPath == ".claude/skills/auto-go/SKILL.md" {
			goSkill = string(file.Content)
			break
		}
	}
	require.NotEmpty(t, goSkill)
	assert.Contains(t, goSkill, "quality.providers.claude")
	assert.Contains(t, goSkill, "quality.default")
	assert.Contains(t, goSkill, "provider override")
	assert.Less(t,
		indexOrFail(t, goSkill, "quality.providers.claude"),
		indexOrFail(t, goSkill, "quality.default"),
	)
}

func indexOrFail(t *testing.T, value, needle string) int {
	t.Helper()
	index := -1
	for i := 0; i+len(needle) <= len(value); i++ {
		if value[i:i+len(needle)] == needle {
			index = i
			break
		}
	}
	require.NotEqual(t, -1, index, needle)
	return index
}
