package omp

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/config"
	pkgcontent "github.com/insajin/autopus-adk/pkg/content"
)

func TestOMP_S1_S2_RuleTransformation(t *testing.T) {
	// S1, S2, E1: frontmatter filtering and body validation
	srcEmptyFm := `category: workflow
# Body content`
	_, err := pkgcontent.TransformRuleForOMP(srcEmptyFm)
	assert.NoError(t, err) // category is dropped, fm omitted, body non-empty so passes

	srcWithFm := `---
description: "Test Rule"
category: workflow
condition: "tool:bash"
---
# Body content`
	transformed, err := pkgcontent.TransformRuleForOMP(srcWithFm)
	assert.NoError(t, err)
	assert.Contains(t, transformed, "description: Test Rule")
	assert.Contains(t, transformed, "condition: tool:bash")
	assert.NotContains(t, transformed, "category:")

	srcEmptyBody := `---
description: "Empty"
---`
	_, err = pkgcontent.TransformRuleForOMP(srcEmptyBody)
	assert.Error(t, err) // empty body throws error
}

func TestOMP_S3_E2_AgentTransformation(t *testing.T) {
	// S3, E2
	src := pkgcontent.AgentSource{
		Meta: pkgcontent.AgentSourceMeta{
			Name:        "executor",
			Description: "Run code",
			Model:       "sonnet",
			Tools:       "Read, Write, Edit, Grep, Glob, Bash, TodoWrite",
		},
		Body: "Agent body with .claude/skills/autopus/ax-annotation.md reference",
	}

	transformed := pkgcontent.TransformAgentForOMP(src)
	assert.Contains(t, transformed, "name: executor")
	assert.Contains(t, transformed, "model: sonnet")
	assert.Contains(t, transformed, "tools:")
	assert.Contains(t, transformed, "- bash")
	assert.Contains(t, transformed, "- edit")
	assert.Contains(t, transformed, "- todo")
	assert.Contains(t, transformed, "- write")
	assert.Contains(t, transformed, ".agents/skills/ax-annotation/SKILL.md")
	assert.NotContains(t, transformed, "yield")
	assert.NotContains(t, transformed, "spawns")

	srcNoTools := pkgcontent.AgentSource{
		Meta: pkgcontent.AgentSourceMeta{
			Name:        "executor",
			Description: "No tools",
		},
		Body: "Body",
	}
	transformedNoTools := pkgcontent.TransformAgentForOMP(srcNoTools)
	assert.NotContains(t, transformedNoTools, "tools:")
}

func TestOMP_S4_S5_S6_Lifecycle(t *testing.T) {
	// S4, S5, S6
	dir := t.TempDir()
	a := NewWithRoot(dir)

	cfg := config.DefaultFullConfig("omp-test")
	cfg.Platforms = []string{"omp"}

	pf, err := a.Generate(context.Background(), cfg)
	require.NoError(t, err)
	assert.NotNil(t, pf)

	// Check S6: generated files
	assert.FileExists(t, filepath.Join(dir, configFile))
	assert.FileExists(t, filepath.Join(dir, ".omp", "agents", "executor.md"))
	assert.FileExists(t, filepath.Join(dir, ".agents", "rules", "autopus", "branding.md"))

	// Check S5: no CLAUDE.md shadowed
	assert.NoFileExists(t, filepath.Join(dir, ".claude", "CLAUDE.md"))

	// Check S4: Clean
	err = a.Clean(context.Background())
	require.NoError(t, err)
	assert.NoFileExists(t, filepath.Join(dir, ".omp", "agents", "executor.md"))
	assert.NoFileExists(t, filepath.Join(dir, ".agents", "rules", "autopus", "branding.md"))
}

func TestOMP_S10_S11_Ownership(t *testing.T) {
	// S10: Skill ownership yield
	cfgA := &config.HarnessConfig{Platforms: []string{"omp"}}
	assert.True(t, ompOwnsSharedSkillSurface(cfgA))
	assert.True(t, ompOwnsCommandSurface(cfgA))

	cfgB := &config.HarnessConfig{Platforms: []string{"opencode", "omp"}}
	assert.False(t, ompOwnsSharedSkillSurface(cfgB))
	assert.False(t, ompOwnsCommandSurface(cfgB))

	cfgC := &config.HarnessConfig{Platforms: []string{"codex", "omp"}}
	assert.False(t, ompOwnsSharedSkillSurface(cfgC))
	assert.True(t, ompOwnsCommandSurface(cfgC))
}

func TestOMP_E6_StaleManifestClean(t *testing.T) {
	dir := t.TempDir()
	a := NewWithRoot(dir)

	cfg := config.DefaultFullConfig("omp-test")
	cfg.Platforms = []string{"omp"}

	_, err := a.Generate(context.Background(), cfg)
	require.NoError(t, err)

	// Skill file is generated since omp owns it
	skillPath := filepath.Join(dir, ".agents", "skills", "auto", "SKILL.md")
	assert.FileExists(t, skillPath)

	// Switch configuration to mixed, now opencode owns skill.
	// Clean re-reads the on-disk config, so the mixed config must be persisted.
	cfgMixed := config.DefaultFullConfig("omp-test")
	cfgMixed.Platforms = []string{"opencode", "omp"}
	require.NoError(t, config.Save(dir, cfgMixed))

	// Overwrite skill file content (as opencode would do)
	err = os.WriteFile(skillPath, []byte("opencode content"), 0644)
	require.NoError(t, err)

	// Clean omp adapter.
	// OMP manifest still tracks .agents/skills/auto/SKILL.md,
	// but ompClean should re-evaluate ownership and SKIP skillPath because omp mixed configuration yields skill surface.
	err = a.Clean(context.Background())
	require.NoError(t, err)

	// Skill path remains untouched, and no backup was made
	assert.FileExists(t, skillPath)
	data, err := os.ReadFile(skillPath)
	require.NoError(t, err)
	assert.Equal(t, "opencode content", string(data))

	// A yielded surface is skipped before the checksum comparison, so the
	// overwritten file must not be backed up.
	assert.NoDirExists(t, filepath.Join(dir, ".autopus", "backup"))

	// Surfaces omp still owns are removed as usual.
	assert.NoFileExists(t, filepath.Join(dir, ".agents", "rules", "autopus", "branding.md"))
	assert.NoFileExists(t, filepath.Join(dir, ".omp", "agents", "executor.md"))
}
