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
