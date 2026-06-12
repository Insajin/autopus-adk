package gemini

import (
	"strings"
	"testing"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
	pkgcontent "github.com/insajin/autopus-adk/pkg/content"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Extended Skills ---

func TestRenderExtendedSkills(t *testing.T) {
	t.Parallel()
	files, err := NewWithRoot(t.TempDir()).renderExtendedSkills(config.DefaultFullConfig("test"))
	require.NoError(t, err)
	assert.NotEmpty(t, files)
	for _, f := range files {
		assert.Contains(t, f.TargetPath, ".gemini/skills/autopus/")
		assert.Equal(t, adapter.OverwriteAlways, f.OverwritePolicy)
	}
}

// S4: Gemini extended skill에 Claude 전용 경로 참조가 남지 않음 (Must, REQ-004).
func TestAcceptance_S4_GeminiSkillRefsResolved(t *testing.T) {
	t.Parallel()
	files, err := NewWithRoot(t.TempDir()).renderExtendedSkills(config.DefaultFullConfig("test"))
	require.NoError(t, err)

	var agentPipeline string
	found := false
	for _, f := range files {
		if strings.Contains(f.TargetPath, "/agent-pipeline/SKILL.md") {
			agentPipeline = string(f.Content)
			found = true
			break
		}
	}
	require.True(t, found, "agent-pipeline skill must be generated for gemini")
	assert.NotContains(t, agentPipeline, ".claude/skills/autopus/",
		"no Claude-only skill path should leak into gemini output")
	assert.Contains(t, agentPipeline, ".gemini/skills/autopus/worktree-isolation/SKILL.md",
		"canonical ref resolves to gemini native path")
}

func TestLogTransformReport_Nil(t *testing.T) {
	t.Parallel()
	logTransformReport("gemini", nil)
}

func TestLogTransformReport_WithData(t *testing.T) {
	t.Parallel()
	report := &pkgcontent.TransformReport{
		Compatible:   []string{"a", "b"},
		Incompatible: []string{"c"},
	}
	logTransformReport("gemini", report)
}
