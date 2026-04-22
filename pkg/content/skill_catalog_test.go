package content_test

import (
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/content"
)

func TestSkillCatalog_RegisteredDoesNotImplyCompiledOrVisible(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"skills/shared-core.md": &fstest.MapFile{
			Data: []byte(`---
name: shared-core
description: Shared visible skill
visibility: shared
platforms:
  - opencode
---
# Shared core
`),
		},
		"skills/explicit-only.md": &fstest.MapFile{
			Data: []byte(`---
name: explicit-only
description: Explicit long-tail skill
visibility: explicit-only
bundles:
  - long-tail
platforms:
  - opencode
---
# Explicit only
`),
		},
	}

	registry := &content.SkillRegistry{}
	require.NoError(t, registry.LoadFromFS(fsys, "skills"))

	registered, err := registry.Get("explicit-only")
	require.NoError(t, err, "acceptance S3b: explicit-only skill must stay registered even before any compiler target selects it")
	assert.Equal(t, "explicit-only", registered.Name, "acceptance S3b: catalog registration must survive separately from compile or visibility decisions")

	transformer, err := content.NewSkillTransformerFromFS(fsys, "skills")
	require.NoError(t, err)

	compiled, report, err := transformer.TransformForPlatform("opencode")
	require.NoError(t, err)

	assert.Contains(t, report.Compatible, "shared-core", "acceptance S3b: shared skill must still compile for the shared visible surface")
	assert.NotContains(t, report.Compatible, "explicit-only", "acceptance S3b: registered explicit-only skill must not become compiled just because it exists in the catalog")
	assert.NotContains(t, transformedSkillNames(compiled), "explicit-only", "acceptance S3b: visible surface must not include explicit-only skill until an explicit compiler target opts in")
}

func transformedSkillNames(skills []content.TransformedSkill) []string {
	names := make([]string, 0, len(skills))
	for _, skill := range skills {
		names = append(names, skill.Name)
	}
	return names
}
