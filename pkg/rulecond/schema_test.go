package rulecond_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/rulecond"
)

// triggeredSource carries every trigger field named by REQ-CONDRULE-SCHEMA-01.
const triggeredSource = `---
name: lore-commit
description: Lore commit format rules for structured commit messages
category: workflow
condition: "\\bgit\\s+commit\\b"
scope: tool:bash
interruptMode: immediate
astCondition: "(call_expression) @call"
---

# Lore Commit

Sign with 🐙 Autopus <noreply@autopus.co>
`

// TestParseRule_TriggerFieldsParsed covers REQ-CONDRULE-SCHEMA-01 and
// REQ-CONDRULE-SCHEMA-04: every trigger field is parsed and interruptMode /
// astCondition are preserved verbatim without interpretation.
func TestParseRule_TriggerFieldsParsed(t *testing.T) {
	t.Parallel()

	rule, err := rulecond.ParseRule("lore-commit.md", []byte(triggeredSource))
	require.NoError(t, err)

	assert.Equal(t, "lore-commit", rule.Name)
	assert.Equal(t, "workflow", rule.Category)
	assert.Equal(t, []string{`\bgit\s+commit\b`}, rule.Conditions)
	assert.Equal(t, []string{"tool:bash"}, rule.Scopes)
	assert.Equal(t, "immediate", rule.InterruptMode)
	assert.Equal(t, "(call_expression) @call", rule.AstCondition)
	assert.Empty(t, rule.Globs)
	assert.False(t, rule.AlwaysApply)

	assert.Contains(t, rule.Body, loreCommitSignature)
	assert.NotContains(t, rule.Body, "condition:", "body must exclude the frontmatter block")
	assert.False(t, strings.HasPrefix(strings.TrimSpace(rule.Body), "---"))
}

// TestParseRule_ConditionAcceptsStringOrList covers the T1 contract that
// condition is accepted as a single string or a string list.
func TestParseRule_ConditionAcceptsStringOrList(t *testing.T) {
	t.Parallel()

	listSource := `---
name: worktree-safety
description: Worktree safety rules
category: workflow
condition:
  - "\\bgit\\s+gc\\b"
  - "\\bgit\\s+repack\\b"
scope: tool:bash
---

# Worktree Safety
`
	rule, err := rulecond.ParseRule("worktree-safety.md", []byte(listSource))
	require.NoError(t, err)
	assert.Equal(t, []string{`\bgit\s+gc\b`, `\bgit\s+repack\b`}, rule.Conditions)
}

// TestParseRule_AlwaysApplyAndGlobsParsed covers the paths-scoped and sticky
// fields consumed by REQ-CONDRULE-COMPILE-01 and the sibling SPEC.
func TestParseRule_AlwaysApplyAndGlobsParsed(t *testing.T) {
	t.Parallel()

	source := `---
name: file-size-limit
description: Hard limit of 300 lines per source code file
category: structure
globs:
  - "**/*.go"
  - "**/*.ts"
alwaysApply: true
---

# File Size Limit
`
	rule, err := rulecond.ParseRule("file-size-limit.md", []byte(source))
	require.NoError(t, err)
	assert.Equal(t, []string{"**/*.go", "**/*.ts"}, rule.Globs)
	assert.True(t, rule.AlwaysApply)
	assert.Empty(t, rule.Conditions)
}

// TestValidate_GlobShapedConditionRejected is the S6 oracle for
// REQ-CONDRULE-SCHEMA-03.
func TestValidate_GlobShapedConditionRejected(t *testing.T) {
	t.Parallel()

	globShaped := `---
name: file-size-limit
description: Hard limit of 300 lines per source code file
category: structure
condition: "**/*.go"
scope: tool:edit(**/*.go)
---

# File Size Limit
`
	rule, err := rulecond.ParseRule("file-size-limit.md", []byte(globShaped))
	require.NoError(t, err)

	err = rulecond.Validate(rule)
	require.Error(t, err, "a glob-shaped condition must be rejected")
	msg := err.Error()
	assert.Contains(t, msg, "file-size-limit.md", "error must name the offending rule")
	assert.Contains(t, msg, "condition", "error must name the offending field")
	assert.Contains(t, msg, "globs", "error must state that a file glob belongs in globs")

	corrected := `---
name: file-size-limit
description: Hard limit of 300 lines per source code file
category: structure
condition: "\\.go$"
scope: tool:edit(**/*.go)
globs:
  - "**/*.go"
---

# File Size Limit
`
	fixed, err := rulecond.ParseRule("file-size-limit.md", []byte(corrected))
	require.NoError(t, err)
	assert.NoError(t, rulecond.Validate(fixed), "a regex condition plus globs must validate")
}

// TestValidate_ConditionWithoutToolScopeRejected covers the T2 contract that a
// condition without a tool scope is not a valid hook-fired rule.
func TestValidate_ConditionWithoutToolScopeRejected(t *testing.T) {
	t.Parallel()

	source := `---
name: lore-commit
description: Lore commit format rules
category: workflow
condition: "\\bgit\\s+commit\\b"
---

# Lore Commit
`
	rule, err := rulecond.ParseRule("lore-commit.md", []byte(source))
	require.NoError(t, err)

	err = rulecond.Validate(rule)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "scope")
	assert.Contains(t, err.Error(), "lore-commit.md")
}

// TestValidate_UntriggeredRuleValidates keeps the ten untouched rules valid.
func TestValidate_UntriggeredRuleValidates(t *testing.T) {
	t.Parallel()

	source := `---
name: branding
description: Autopus branding tiers
category: style
---

# Autopus Branding
`
	rule, err := rulecond.ParseRule("branding.md", []byte(source))
	require.NoError(t, err)
	assert.NoError(t, rulecond.Validate(rule))
	assert.Equal(t, rulecond.ClassAlways, rulecond.Classify(rule))
}

// TestValidate_GlobHeuristicBothDirections widens the S6 oracle. The glob
// heuristic guards a Must requirement in both directions: a missed glob lets
// omp silently re-scope the rule, and a false positive rejects a legitimate
// regex condition at validation time. The shipped conditions are included
// because each carries a wildcard that a naive heuristic would flag.
func TestValidate_GlobHeuristicBothDirections(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		condition string
		wantGlob  bool
	}{
		{name: "recursive glob", condition: `**/*.go`, wantGlob: true},
		{name: "bare quantifier glob", condition: `*.go`, wantGlob: true},
		{name: "path glob", condition: `src/*.ts`, wantGlob: true},
		{name: "brace-free nested glob", condition: `pkg/**/testdata/*.json`, wantGlob: true},
		{name: "shipped lore-commit condition", condition: `\bgit\s+commit\b`},
		{name: "shipped worktree-safety condition", condition: `\bgit\s+(gc|prune|repack)\b`},
		{name: "shipped shell-portability condition", condition: `(^|[;&|]\s)\s*g?timeout\s+[0-9]`},
		{name: "anchored extension regex", condition: `\.go$`},
		{name: "path regex with regex-only constructs", condition: `^/repo/.*\.go$`},
		{name: "alternation with wildcard", condition: `(foo|bar)*baz`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rule := &rulecond.Rule{
				Name:       "probe",
				SourceFile: "probe.md",
				Conditions: []string{tt.condition},
				Scopes:     []string{"tool:bash"},
				Body:       "# probe\n",
			}

			err := rulecond.Validate(rule)
			if !tt.wantGlob {
				assert.NoError(t, err,
					"a legitimate regex condition must not be rejected as a glob")
				return
			}
			require.Error(t, err, "a glob-shaped condition must be rejected")
			assert.Contains(t, err.Error(), "probe.md")
			assert.Contains(t, err.Error(), "condition")
			assert.Contains(t, err.Error(), "globs")
			assert.Contains(t, err.Error(), tt.condition,
				"the error must quote the offending value")
		})
	}
}
