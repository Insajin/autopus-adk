package rulecond_test

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/rulecond"
)

func hookFiredRule(name, condition string) *rulecond.Rule {
	return &rulecond.Rule{
		Name:       name,
		SourceFile: name + ".md",
		Conditions: []string{condition},
		Scopes:     []string{"tool:bash"},
		Body:       "# " + name + "\n\nbody of " + name + "\n",
	}
}

func compiledRules(t *testing.T) []*rulecond.Rule {
	t.Helper()
	editRule := &rulecond.Rule{
		Name:       "fixture-go-edit",
		SourceFile: "fixture-go-edit.md",
		Conditions: []string{condGoEditFixture},
		Scopes:     []string{"tool:edit(*.go)"},
		Body:       "# fixture-go-edit\n",
	}
	return []*rulecond.Rule{
		hookFiredRule("worktree-safety", condWorktreeSafety),
		hookFiredRule("lore-commit", condLoreCommit),
		editRule,
		hookFiredRule("shell-portability", condShellPortable),
	}
}

func parseManifest(t *testing.T, file adapter.FileMapping) rulecond.Manifest {
	t.Helper()
	var m rulecond.Manifest
	require.NoError(t, json.Unmarshal(file.Content, &m))
	return m
}

// TestCompileClaude_ManifestIsDeterministic is the S9 determinism oracle for
// REQ-CONDRULE-COMPILE-04.
func TestCompileClaude_ManifestIsDeterministic(t *testing.T) {
	t.Parallel()

	first, err := rulecond.CompileClaude(compiledRules(t))
	require.NoError(t, err)
	second, err := rulecond.CompileClaude(compiledRules(t))
	require.NoError(t, err)

	assert.Equal(t, string(first.Manifest.Content), string(second.Manifest.Content),
		"regeneration must produce byte-identical manifests")
	assert.Equal(t, ".claude/hooks/autopus/conditional-rules.json",
		filepath.ToSlash(first.Manifest.TargetPath))

	manifest := parseManifest(t, first.Manifest)
	names := make([]string, 0, len(manifest.Rules))
	for _, rule := range manifest.Rules {
		names = append(names, rule.Name)
	}
	assert.Equal(t,
		[]string{"fixture-go-edit", "lore-commit", "shell-portability", "worktree-safety"},
		names, "manifest rules are ordered by rule name")
}

// TestCompileClaude_BodyLocationsAreRootRelative is the REQ-CONDRULE-COMPILE-07
// oracle.
func TestCompileClaude_BodyLocationsAreRootRelative(t *testing.T) {
	t.Parallel()

	compiled, err := rulecond.CompileClaude(compiledRules(t))
	require.NoError(t, err)

	manifest := parseManifest(t, compiled.Manifest)
	require.Len(t, manifest.Rules, 4)
	for _, rule := range manifest.Rules {
		assert.Equal(t, rule.Name+".md", rule.Body, "body is named relative to the body root")
		assert.False(t, filepath.IsAbs(rule.Body), "no absolute path may be stored")
		assert.NotContains(t, rule.Body, "..")
	}

	bodyPaths := make([]string, 0, len(compiled.Bodies))
	for _, body := range compiled.Bodies {
		bodyPaths = append(bodyPaths, filepath.ToSlash(body.TargetPath))
	}
	assert.ElementsMatch(t, []string{
		".claude/hooks/autopus/conditional/fixture-go-edit.md",
		".claude/hooks/autopus/conditional/lore-commit.md",
		".claude/hooks/autopus/conditional/shell-portability.md",
		".claude/hooks/autopus/conditional/worktree-safety.md",
	}, bodyPaths)
}

// TestCompileClaude_OneHookPerEventMatcherPair is the REQ-CONDRULE-COMPILE-03
// oracle: three Bash rules collapse to one dispatcher entry.
func TestCompileClaude_OneHookPerEventMatcherPair(t *testing.T) {
	t.Parallel()

	compiled, err := rulecond.CompileClaude(compiledRules(t))
	require.NoError(t, err)

	require.Len(t, compiled.Hooks, 2, "one entry per distinct event and matcher pair")

	matchers := map[string]adapter.HookConfig{}
	for _, hook := range compiled.Hooks {
		assert.Equal(t, preToolUse, hook.Event)
		assert.Equal(t, "command", hook.Type)
		assert.Contains(t, hook.Command, "auto rules fire")
		matchers[hook.Matcher] = hook
	}
	assert.Contains(t, matchers, bashMatcher)
	assert.Contains(t, matchers, editMatcher)
}

// TestCompileClaude_PathsScopedRuleStaysInBaselineDir is the compiler half of
// S13 for REQ-CONDRULE-COMPILE-01.
func TestCompileClaude_PathsScopedRuleStaysInBaselineDir(t *testing.T) {
	t.Parallel()

	rule := &rulecond.Rule{
		Name:       "file-size-limit",
		SourceFile: "file-size-limit.md",
		Globs:      []string{"**/*.go", "**/*.ts"},
		Body:       "# File Size Limit\n\n300 lines is the hard limit.\n",
	}

	compiled, err := rulecond.CompileClaude([]*rulecond.Rule{rule})
	require.NoError(t, err)

	assert.Empty(t, compiled.Hooks, "a paths-scoped rule registers no hook entry")
	assert.Empty(t, compiled.Bodies, "a paths-scoped rule is not relocated")
	assert.Empty(t, parseManifest(t, compiled.Manifest).Rules,
		"a paths-scoped rule gets no manifest entry")

	require.Len(t, compiled.RuleFiles, 1)
	emitted := compiled.RuleFiles[0]
	assert.Equal(t, ".claude/rules/autopus/file-size-limit.md",
		filepath.ToSlash(emitted.TargetPath))

	content := string(emitted.Content)
	assert.Contains(t, content, "paths:", "globs compile to a native paths: list")
	assert.Contains(t, content, "**/*.go")
	assert.Contains(t, content, "**/*.ts")
	assert.Contains(t, content, "300 lines is the hard limit.")
}

// TestCompileClaude_SkillScopedRuleRelocatesWithoutATrigger is the issue #185
// oracle: a skill-scoped rule leaves baseline context exactly like a hook-fired
// one, but earns no manifest rule and no dispatcher entry, so the manifest bytes
// are the ones a harness with no skill-scoped rule already had.
func TestCompileClaude_SkillScopedRuleRelocatesWithoutATrigger(t *testing.T) {
	t.Parallel()

	rule := &rulecond.Rule{
		Name:        "spec-quality",
		SourceFile:  "spec-quality.md",
		SkillScoped: true,
		Body:        "# SPEC Quality Checklist\n\nRun the checklist before review.\n",
	}

	compiled, err := rulecond.CompileClaude([]*rulecond.Rule{rule})
	require.NoError(t, err)

	require.Len(t, compiled.Bodies, 1)
	assert.Equal(t, ".claude/hooks/autopus/conditional/spec-quality.md",
		filepath.ToSlash(compiled.Bodies[0].TargetPath))
	assert.Equal(t, rule.Body, string(compiled.Bodies[0].Content),
		"the relocated body is the source body, frontmatter already removed by ParseRule")

	assert.Empty(t, compiled.RuleFiles, "a skill-scoped rule keeps no baseline file")
	assert.Empty(t, compiled.Hooks, "a skill-scoped rule registers no dispatcher entry")
	assert.Empty(t, parseManifest(t, compiled.Manifest).Rules,
		"a skill-scoped rule gets no manifest entry")

	empty, err := rulecond.CompileClaude(nil)
	require.NoError(t, err)
	assert.Equal(t, string(empty.Manifest.Content), string(compiled.Manifest.Content),
		"a skill-scoped rule must not move conditional-rules.json bytes")

	assert.False(t, rulecond.IsSticky(&rulecond.Rule{
		Name: "spec-quality", SkillScoped: true, AlwaysApply: true,
	}), "a relocated body can never be resolved by the sticky runtime")
}

// TestCompileClaude_SkillScopedBodyIsNotBoundByTheInjectionBudget separates the
// two size contracts that share MaxBodyBytes. The cap bounds what a dispatcher
// pastes into a PreToolUse payload; a skill-scoped body is read by the skill
// that references it and is never pasted anywhere, so capping it would refuse
// generation over a budget the class never spends. Every shipped skill-scoped
// rule is already larger than the cap, so this is the difference between a
// working harness and a hard generation failure.
func TestCompileClaude_SkillScopedBodyIsNotBoundByTheInjectionBudget(t *testing.T) {
	t.Parallel()

	oversize := padTo("SKILL-SCOPED-BODY", rulecond.MaxBodyBytes+1)
	skillScoped := &rulecond.Rule{
		Name: "spec-quality", SourceFile: "spec-quality.md", SkillScoped: true, Body: oversize,
	}

	compiled, err := rulecond.CompileClaude([]*rulecond.Rule{skillScoped})

	require.NoError(t, err, "the injection budget must not gate a body nothing injects")
	require.Len(t, compiled.Bodies, 1)
	assert.Equal(t, oversize, string(compiled.Bodies[0].Content))

	hookFired := hookFiredRule("worktree-safety", condWorktreeSafety)
	hookFired.Body = oversize
	_, err = rulecond.CompileClaude([]*rulecond.Rule{hookFired})
	require.Error(t, err, "the same body still fails for a class the dispatcher injects")
	assert.Contains(t, err.Error(), string(rulecond.ReasonBodyTooLarge))
}

// TestValidate_SkillScopedRefusesEveryOtherTriggerField pins the refusals that
// keep `skillScoped` a standalone trigger: each combination would classify or
// place the rule somewhere the author did not ask for.
func TestValidate_SkillScopedRefusesEveryOtherTriggerField(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		mutit func(*rulecond.Rule)
		field string
	}{
		{name: "condition", field: "condition", mutit: func(r *rulecond.Rule) {
			r.Conditions = []string{`\bgit\s+commit\b`}
			r.Scopes = []string{"tool:bash"}
		}},
		{name: "globs", field: "globs", mutit: func(r *rulecond.Rule) {
			r.Globs = []string{"**/*.go"}
		}},
		{name: "alwaysApply", field: "alwaysApply", mutit: func(r *rulecond.Rule) {
			r.AlwaysApply = true
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rule := &rulecond.Rule{
				Name: "spec-quality", SourceFile: "spec-quality.md", SkillScoped: true,
			}
			tt.mutit(rule)

			err := rulecond.Validate(rule)

			require.Error(t, err)
			assert.Contains(t, err.Error(), "spec-quality.md", "the error must name the rule")
			assert.Contains(t, err.Error(), tt.field, "the error must name the offending field")
			assert.Contains(t, err.Error(), "skillScoped")
		})
	}

	assert.NoError(t, rulecond.Validate(&rulecond.Rule{
		Name: "spec-quality", SourceFile: "spec-quality.md", SkillScoped: true,
	}), "skillScoped alone is the representable shape")
}

// TestCompileClaude_OversizeBodyFailsGeneration is the compile-time half of S14
// for REQ-CONDRULE-COMPILE-06.
func TestCompileClaude_OversizeBodyFailsGeneration(t *testing.T) {
	t.Parallel()

	oversize := hookFiredRule("worktree-safety", condWorktreeSafety)
	oversize.Body = padTo("HUGE-BODY", rulecond.MaxBodyBytes+1)

	compiled, err := rulecond.CompileClaude([]*rulecond.Rule{
		hookFiredRule("lore-commit", condLoreCommit),
		oversize,
	})

	require.Error(t, err, "generation must fail rather than emit an entry the dispatcher refuses")
	assert.Contains(t, err.Error(), "worktree-safety", "the error must name the offending rule")
	assert.Contains(t, err.Error(), string(rulecond.ReasonBodyTooLarge))
	assert.Nil(t, compiled)
	assert.False(t, strings.Contains(err.Error(), "HUGE-BODY"),
		"the error must not echo body content")
}
