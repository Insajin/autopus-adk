package rulecond_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/rulecond"
)

// parseStickySource parses an inline rule source into a *rulecond.Rule.
func parseStickySource(t *testing.T, filename, frontmatter, body string) *rulecond.Rule {
	t.Helper()
	rule, err := rulecond.ParseRule(filename, []byte("---\n"+frontmatter+"---\n"+body))
	require.NoError(t, err)
	return rule
}

// TestIsSticky_IsOrthogonalToClassification is the S7 orthogonality oracle for
// REQ-STICKYRULE-SCHEMA-01: sticky is a flag beside the three-value partition,
// never a fourth value, and it never moves a rule.
func TestIsSticky_IsOrthogonalToClassification(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		frontmatter string
		wantClass   rulecond.Class
		wantSticky  bool
	}{
		{
			name:        "always rule with alwaysApply",
			frontmatter: "name: language-policy\nalwaysApply: true\n",
			wantClass:   rulecond.ClassAlways,
			wantSticky:  true,
		},
		{
			name:        "always rule without alwaysApply",
			frontmatter: "name: branding\n",
			wantClass:   rulecond.ClassAlways,
			wantSticky:  false,
		},
		{
			name:        "paths-scoped rule with alwaysApply",
			frontmatter: "name: file-size-limit\nglobs: '*.go'\nalwaysApply: true\n",
			wantClass:   rulecond.ClassPathsScoped,
			wantSticky:  true,
		},
		{
			name:        "hook-fired rule without alwaysApply",
			frontmatter: "name: lore-commit\ncondition: 'git commit'\nscope: tool:bash\n",
			wantClass:   rulecond.ClassHookFired,
			wantSticky:  false,
		},
		{
			name:        "alwaysApply false is not sticky",
			frontmatter: "name: doc-storage\nalwaysApply: false\n",
			wantClass:   rulecond.ClassAlways,
			wantSticky:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rule := parseStickySource(t, "rule.md", tt.frontmatter, "# Body\n\nText.\n")
			assert.Equal(t, tt.wantClass, rulecond.Classify(rule),
				"the sticky flag must not change the classification")
			assert.Equal(t, tt.wantSticky, rulecond.IsSticky(rule))
		})
	}
}

// TestValidate_HookFiredPlusAlwaysApplyIsRejected is the S7 rejection oracle for
// REQ-STICKYRULE-SCHEMA-03. SPEC-CONDRULE-001 relocates a hook-fired body out of
// the sticky body root, so the combination has no body this runtime can resolve.
func TestValidate_HookFiredPlusAlwaysApplyIsRejected(t *testing.T) {
	t.Parallel()

	rule := parseStickySource(t, "lore-commit.md",
		"name: lore-commit\ncondition: '\\bgit\\s+commit\\b'\nscope: tool:bash\nalwaysApply: true\n",
		"# Lore Commit\n\nText.\n")
	require.Equal(t, rulecond.ClassHookFired, rulecond.Classify(rule))

	err := rulecond.Validate(rule)

	require.Error(t, err, "the unrepresentable combination must be refused at validation")
	message := err.Error()
	assert.Contains(t, message, "lore-commit", "the error must name the rule")
	assert.Contains(t, message, "hook-fired", "the error must name the classification")
	assert.Contains(t, message, "alwaysApply", "the error must name the attribute")
}

// TestCompileClaude_StickySetReachesTheManifest is the S7 compilation oracle for
// REQ-STICKYRULE-COMPILE-01 and REQ-STICKYRULE-COMPILE-04.
func TestCompileClaude_StickySetReachesTheManifest(t *testing.T) {
	t.Parallel()

	rules := []*rulecond.Rule{
		parseStickySource(t, "language-policy.md", "name: language-policy\nalwaysApply: true\n", "# LP\n\nText.\n"),
		parseStickySource(t, "objective-reasoning.md", "name: objective-reasoning\nalwaysApply: true\n", "# OR\n\nText.\n"),
		parseStickySource(t, "branding.md", "name: branding\n", "# B\n\nText.\n"),
		parseStickySource(t, "lore-commit.md",
			"name: lore-commit\ncondition: 'git commit'\nscope: tool:bash\n", "# LC\n\nText.\n"),
	}

	compiled, err := rulecond.CompileClaude(rules)
	require.NoError(t, err)

	assert.Equal(t, []rulecond.StickyRule{
		{Name: ruleLanguagePolicy, Body: ruleLanguagePolicy + ".md"},
		{Name: ruleObjectiveReasoning, Body: ruleObjectiveReasoning + ".md"},
	}, compiled.Sticky,
		"sticky body locations are recorded relative to the sticky body root")

	var manifest rulecond.Manifest
	require.NoError(t, json.Unmarshal(compiled.Manifest.Content, &manifest))
	names := make([]string, 0, len(manifest.Sticky))
	for _, entry := range manifest.Sticky {
		names = append(names, entry.Name)
	}
	assert.Equal(t, []string{ruleLanguagePolicy, ruleObjectiveReasoning}, names,
		"the compiled manifest lists exactly the two sticky rules")

	// REQ-STICKYRULE-SCHEMA-01: baseline placement is unchanged, so an `always`
	// sticky rule is still absent from the compiler's own file mappings and
	// keeps flowing through the verbatim copy path.
	for _, mapping := range append(compiled.RuleFiles, compiled.Bodies...) {
		assert.NotContains(t, mapping.TargetPath, ruleLanguagePolicy)
		assert.NotContains(t, mapping.TargetPath, ruleObjectiveReasoning)
	}
}

// TestCompileClaude_StickyBodyAboveThePerBodyCapFailsGeneration keeps the
// compiler from minting an entry the runtime would refuse
// (REQ-STICKYRULE-COMPILE-04).
func TestCompileClaude_StickyBodyAboveThePerBodyCapFailsGeneration(t *testing.T) {
	t.Parallel()

	oversize := "# Huge\n" + strings.Repeat("x", rulecond.MaxStickyBodyBytes)
	rules := []*rulecond.Rule{
		parseStickySource(t, "huge-sticky.md", "name: huge-sticky\nalwaysApply: true\n", oversize),
	}

	_, err := rulecond.CompileClaude(rules)

	require.Error(t, err, "a sticky body above the per-body cap must fail generation")
	assert.Contains(t, err.Error(), "huge-sticky", "the error must name the rule")
	assert.NotContains(t, err.Error(), "xxxxxxxx", "the error must not echo body content")
}

// TestCompileClaude_StickySetAboveTheAggregateCapFailsGeneration covers the
// whole-set half of REQ-STICKYRULE-COMPILE-04: the compiler refuses a set the
// runtime would silently truncate.
func TestCompileClaude_StickySetAboveTheAggregateCapFailsGeneration(t *testing.T) {
	t.Parallel()

	big := "# Big\n" + strings.Repeat("y", 3500)
	rules := []*rulecond.Rule{
		parseStickySource(t, "aaa-big.md", "name: aaa-big\nalwaysApply: true\n", big),
		parseStickySource(t, "bbb-big.md", "name: bbb-big\nalwaysApply: true\n", big),
	}

	_, err := rulecond.CompileClaude(rules)

	require.Error(t, err,
		"7000 bytes of injectable body exceeds the 6000-byte aggregate cap")
	assert.True(t,
		strings.Contains(err.Error(), "aaa-big") || strings.Contains(err.Error(), "bbb-big"),
		"the error must name the offending rule, got %q", err.Error())
}

// TestCompileClaude_NoStickyRuleRecordsNoStickySet is the negative control for
// REQ-STICKYRULE-COMPILE-01: the manifest carries a sticky set only when one
// exists.
func TestCompileClaude_NoStickyRuleRecordsNoStickySet(t *testing.T) {
	t.Parallel()

	rules := []*rulecond.Rule{
		parseStickySource(t, "branding.md", "name: branding\n", "# B\n\nText.\n"),
	}

	compiled, err := rulecond.CompileClaude(rules)
	require.NoError(t, err)

	assert.Empty(t, compiled.Sticky)
	assert.NotContains(t, string(compiled.Manifest.Content), `"sticky":[`,
		"an empty sticky set must not add a populated array to the manifest")
}
