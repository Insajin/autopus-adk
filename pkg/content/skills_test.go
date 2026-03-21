// Package content_test는 스킬 콘텐츠 패키지의 테스트이다.
package content_test

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/content"
)

func TestSkillRegistry_Load(t *testing.T) {
	t.Parallel()

	// 임시 스킬 디렉토리 생성
	dir := t.TempDir()
	writeSkillFile(t, dir, "planning.md", `---
name: planning
description: 기능 기획 스킬
triggers:
  - "plan"
  - "기획"
category: workflow
level1_metadata: "기획 메타데이터"
level3_resources:
  - "https://example.com/planning"
---

# Planning Skill

기획 단계에서 사용하는 스킬입니다.
`)

	registry := &content.SkillRegistry{}
	err := registry.Load(dir)
	require.NoError(t, err)

	skills := registry.List()
	assert.Len(t, skills, 1)
	assert.Equal(t, "planning", skills[0].Name)
	assert.Equal(t, "기능 기획 스킬", skills[0].Description)
	assert.Equal(t, "workflow", skills[0].Category)
	assert.Equal(t, []string{"plan", "기획"}, skills[0].Triggers)
}

func TestSkillRegistry_Get(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeSkillFile(t, dir, "debugging.md", `---
name: debugging
description: 디버깅 스킬
triggers:
  - "debug"
category: quality
---

# Debugging Skill
`)

	registry := &content.SkillRegistry{}
	require.NoError(t, registry.Load(dir))

	skill, err := registry.Get("debugging")
	require.NoError(t, err)
	assert.Equal(t, "debugging", skill.Name)
	assert.Equal(t, "quality", skill.Category)
}

func TestSkillRegistry_Get_NotFound(t *testing.T) {
	t.Parallel()

	registry := &content.SkillRegistry{}
	_, err := registry.Get("nonexistent")
	assert.Error(t, err)
}

func TestSkillRegistry_ListByCategory(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeSkillFile(t, dir, "tdd.md", `---
name: tdd
description: TDD 스킬
triggers:
  - "tdd"
category: methodology
---
body`)
	writeSkillFile(t, dir, "ddd.md", `---
name: ddd
description: DDD 스킬
triggers:
  - "ddd"
category: methodology
---
body`)
	writeSkillFile(t, dir, "debugging.md", `---
name: debugging
description: 디버깅
triggers:
  - "debug"
category: quality
---
body`)

	registry := &content.SkillRegistry{}
	require.NoError(t, registry.Load(dir))

	methodology := registry.ListByCategory("methodology")
	assert.Len(t, methodology, 2)

	quality := registry.ListByCategory("quality")
	assert.Len(t, quality, 1)
}

func TestSkillConvertToPlatform_Claude(t *testing.T) {
	t.Parallel()

	skill := content.SkillDefinition{
		Name:            "planning",
		Description:     "기획 스킬",
		Level1Metadata:  "메타데이터",
		Level2Body:      "# Planning\n\n기획 내용",
		Level3Resources: []string{"https://example.com"},
		Triggers:        []string{"plan"},
		Category:        "workflow",
	}

	result, err := content.ConvertSkillToPlatform(skill, "claude")
	require.NoError(t, err)
	assert.Contains(t, result, "planning")
	assert.Contains(t, result, "기획 스킬")
}

func TestSkillConvertToPlatform_Codex(t *testing.T) {
	t.Parallel()

	skill := content.SkillDefinition{
		Name:        "planning",
		Description: "기획 스킬",
		Level2Body:  "# Planning",
	}

	result, err := content.ConvertSkillToPlatform(skill, "codex")
	require.NoError(t, err)
	assert.Contains(t, result, "auto-planning")
}

func TestSkillConvertToPlatform_Gemini(t *testing.T) {
	t.Parallel()

	skill := content.SkillDefinition{
		Name:        "planning",
		Description: "기획 스킬",
		Level2Body:  "# Planning",
	}

	result, err := content.ConvertSkillToPlatform(skill, "gemini")
	require.NoError(t, err)
	// gemini는 YAML frontmatter 포함
	assert.Contains(t, result, "name:")
	assert.Contains(t, result, "description:")
}

func TestSkillConvertToPlatform_UnknownPlatform(t *testing.T) {
	t.Parallel()

	skill := content.SkillDefinition{Name: "test"}
	_, err := content.ConvertSkillToPlatform(skill, "unknown")
	assert.Error(t, err)
}

func TestSkillRegistry_LoadFromFS(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"skills/planning.md": &fstest.MapFile{
			Data: []byte(`---
name: planning
description: 기능 기획 스킬
triggers:
  - "plan"
  - "기획"
category: workflow
level1_metadata: "기획 메타데이터"
---

# Planning Skill

기획 단계에서 사용하는 스킬입니다.
`),
		},
		"skills/debugging.md": &fstest.MapFile{
			Data: []byte(`---
name: debugging
description: 디버깅 스킬
triggers:
  - "debug"
category: quality
---

# Debugging Skill
`),
		},
	}

	registry := &content.SkillRegistry{}
	err := registry.LoadFromFS(fsys, "skills")
	require.NoError(t, err)

	skills := registry.List()
	assert.Len(t, skills, 2)

	skill, err := registry.Get("planning")
	require.NoError(t, err)
	assert.Equal(t, "기능 기획 스킬", skill.Description)
	assert.Equal(t, "workflow", skill.Category)
	assert.Equal(t, "기획 메타데이터", skill.Level1Metadata)
	assert.Contains(t, skill.Level2Body, "Planning Skill")
}

func TestSkillRegistry_LoadFromFS_FallbackName(t *testing.T) {
	t.Parallel()

	// Skill without name in frontmatter uses filename as name.
	fsys := fstest.MapFS{
		"skills/no-name.md": &fstest.MapFile{
			Data: []byte(`---
description: 이름 없는 스킬
---

body
`),
		},
	}

	registry := &content.SkillRegistry{}
	err := registry.LoadFromFS(fsys, "skills")
	require.NoError(t, err)

	skill, err := registry.Get("no-name")
	require.NoError(t, err)
	assert.Equal(t, "no-name", skill.Name)
}

func TestSkillRegistry_LoadFromFS_InvalidDir(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{}
	registry := &content.SkillRegistry{}
	err := registry.LoadFromFS(fsys, "nonexistent")
	assert.Error(t, err)
}

// writeSkillFile은 테스트용 스킬 파일을 생성한다.
func writeSkillFile(t *testing.T, dir, name, content string) {
	t.Helper()
	err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644)
	require.NoError(t, err)
}
