package codex

import (
	"strings"
	"testing"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/stretchr/testify/assert"
)

func TestSkillCompilerExplicitlySelects_NilConfig(t *testing.T) {
	t.Parallel()
	assert.False(t, skillCompilerExplicitlySelects(nil, "x"))
}

func TestSkillCompilerExplicitlySelects_Matches(t *testing.T) {
	t.Parallel()
	cfg := config.DefaultFullConfig("demo")
	cfg.Skills.Compiler.ExplicitSkills = []string{"foo", "bar"}
	assert.True(t, skillCompilerExplicitlySelects(cfg, "foo"))
	assert.False(t, skillCompilerExplicitlySelects(cfg, "baz"))
}

func TestEnsureCodexSkillFrontmatter_NotSkillFile(t *testing.T) {
	t.Parallel()
	body := "raw body"
	out := ensureCodexSkillFrontmatter("rules/x.md", "n", "d", body)
	assert.Equal(t, body, out)
}

func TestEnsureCodexSkillFrontmatter_ExistingFrontmatter(t *testing.T) {
	t.Parallel()
	body := "---\nname: keep\n---\n\nthe body"
	out := ensureCodexSkillFrontmatter("skills/x/SKILL.md", "n", "d", body)
	assert.Contains(t, out, "name: keep")
	assert.Contains(t, out, "the body")
}

func TestEnsureCodexSkillFrontmatter_GeneratesWithDescription(t *testing.T) {
	t.Parallel()
	out := ensureCodexSkillFrontmatter("skills/x/SKILL.md", "myskill", "My description", "the body")
	assert.Contains(t, out, "name: myskill")
	assert.Contains(t, out, "My description")
	assert.True(t, strings.HasPrefix(out, "---\n"))
}

func TestEnsureCodexSkillFrontmatter_EmptyDescriptionFallsBackToName(t *testing.T) {
	t.Parallel()
	out := ensureCodexSkillFrontmatter("skills/x/SKILL.md", "myskill", "   ", "body")
	// description defaults to name when blank.
	assert.Contains(t, out, "name: myskill")
	assert.Contains(t, out, "description: >\n  myskill")
}

func TestLogTransformReport_NilSafe(t *testing.T) {
	t.Parallel()
	// Must not panic on nil report.
	assert.NotPanics(t, func() { logTransformReport(nil) })
}
