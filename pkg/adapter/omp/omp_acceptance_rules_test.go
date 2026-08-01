package omp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	contentfs "github.com/insajin/autopus-adk/content"
	"github.com/insajin/autopus-adk/pkg/config"
	pkgcontent "github.com/insajin/autopus-adk/pkg/content"
)

// generateOMPOnly runs Generate for an omp-only platform list and persists the
// config so that ownership re-evaluation in Clean reads the same platform set.
func generateOMPOnly(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cfg := config.DefaultFullConfig("omp-acceptance")
	cfg.Platforms = []string{"omp"}
	require.NoError(t, config.Save(dir, cfg))
	_, err := NewWithRoot(dir).Generate(context.Background(), cfg)
	require.NoError(t, err)
	return dir
}

// splitEmittedFrontmatter separates the leading `---` block from the body.
func splitEmittedFrontmatter(t *testing.T, content string) (frontmatter, body string) {
	t.Helper()
	if !strings.HasPrefix(content, "---\n") {
		return "", content
	}
	rest := content[len("---\n"):]
	end := strings.Index(rest, "\n---\n")
	require.GreaterOrEqual(t, end, 0, "frontmatter block must be terminated")
	return rest[:end], rest[end+len("\n---\n"):]
}

func sourceRuleNames(t *testing.T) []string {
	t.Helper()
	entries, err := contentfs.FS.ReadDir("rules")
	require.NoError(t, err)
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			names = append(names, e.Name())
		}
	}
	return names
}

// TestOMPAcceptance_S1_RuleFrontmatterPassthrough covers REQ-002/REQ-003/REQ-004.
func TestOMPAcceptance_S1_RuleFrontmatterPassthrough(t *testing.T) {
	dir := generateOMPOnly(t)
	ruleDir := filepath.Join(dir, ".agents", "rules", "autopus")

	loreRaw, err := os.ReadFile(filepath.Join(ruleDir, "lore-commit.md"))
	require.NoError(t, err)
	lore := string(loreRaw)
	assert.True(t, strings.HasPrefix(lore, "---\n"),
		"lore-commit.md must open with a frontmatter delimiter, got first line %q",
		strings.SplitN(lore, "\n", 2)[0])

	loreFM, loreBody := splitEmittedFrontmatter(t, lore)
	assert.Contains(t, loreFM,
		"description: Lore commit format rules for structured, traceable commit messages")
	assert.NotContains(t, loreFM, "category:")
	assert.NotContains(t, loreFM, "name:")
	assert.NotEmpty(t, strings.TrimSpace(loreBody))

	// branding.md carries no frontmatter at source, so the emitted copy gains a
	// synthesized description from its title. Without one omp writes the file
	// but never surfaces the rule in a session.
	brandingRaw, err := os.ReadFile(filepath.Join(ruleDir, "branding.md"))
	require.NoError(t, err)
	branding := string(brandingRaw)
	brandingFM, brandingBody := splitEmittedFrontmatter(t, branding)
	assert.Equal(t, "description: Autopus Branding", strings.TrimSpace(brandingFM))
	assert.Equal(t, "# Autopus Branding", strings.SplitN(strings.TrimSpace(brandingBody), "\n", 2)[0],
		"the body keeps its original title")

	// Every emitted rule carries a non-empty body (REQ-004) and reaches an omp
	// session: it is listed in <domain-rules> when it declares a description, or
	// registered with the TTSR engine when it declares a trigger key. A rule with
	// neither lands on disk and is never discovered.
	names := sourceRuleNames(t)
	assert.Len(t, names, 14, "content/rules must hold exactly 14 rule sources")
	for _, name := range names {
		data, readErr := os.ReadFile(filepath.Join(ruleDir, name))
		require.NoError(t, readErr, "rule %s must be emitted", name)
		fm, body := splitEmittedFrontmatter(t, string(data))
		hasContentLine := false
		for _, line := range strings.Split(body, "\n") {
			if strings.TrimSpace(line) != "" {
				hasContentLine = true
				break
			}
		}
		assert.True(t, hasContentLine, "rule %s body must hold a non-blank line", name)

		discoverable := strings.Contains(fm, "description:") ||
			strings.Contains(fm, "condition:") ||
			strings.Contains(fm, "astCondition:") ||
			strings.Contains(fm, "scope:")
		assert.True(t, discoverable,
			"rule %s must be discoverable by an omp session, frontmatter was %q", name, fm)
	}

	// Nothing is written at the `.agents/rules/` top level (REQ-002 namespace).
	topLevel, err := os.ReadDir(filepath.Join(dir, ".agents", "rules"))
	require.NoError(t, err)
	for _, e := range topLevel {
		assert.True(t, e.IsDir(), "omp must not write %q at .agents/rules/ top level", e.Name())
	}
}

// TestOMPAcceptance_S2_ConditionalFieldPassthrough covers REQ-003 trigger fields.
func TestOMPAcceptance_S2_ConditionalFieldPassthrough(t *testing.T) {
	conditional := `---
description: demo
condition: tool:bash
scope:
  - tool:edit
alwaysApply: false
interruptMode: prose-only
---

# Conditional Demo

Body line.
`
	out, err := pkgcontent.TransformRuleForOMP(conditional)
	require.NoError(t, err)

	fm, body := splitEmittedFrontmatter(t, out)
	require.NotEmpty(t, fm, "conditional-demo must retain a frontmatter block")

	var parsed map[string]any
	require.NoError(t, yaml.Unmarshal([]byte(fm), &parsed))
	assert.Equal(t, "tool:bash", parsed["condition"])
	assert.Equal(t, []any{"tool:edit"}, parsed["scope"])
	assert.Equal(t, false, parsed["alwaysApply"])
	assert.Equal(t, "prose-only", parsed["interruptMode"])
	assert.Contains(t, body, "# Conditional Demo")

	plain := `# Plain Demo

Body line.
`
	plainOut, err := pkgcontent.TransformRuleForOMP(plain)
	require.NoError(t, err)
	plainFM, plainBody := splitEmittedFrontmatter(t, plainOut)
	assert.Equal(t, "description: Plain Demo", strings.TrimSpace(plainFM),
		"plain-demo gains only a synthesized description")
	assert.NotContains(t, plainOut, "condition:",
		"synthesis must not invent a trigger that would reroute the rule to TTSR")
	assert.NotContains(t, plainOut, "scope:")
	assert.NotContains(t, plainOut, "alwaysApply:")
	assert.Contains(t, plainBody, "# Plain Demo")
}

// TestOMPAcceptance_E1_UnrecognizedKeysOnly covers REQ-003 drop behavior and
// the synthesis that keeps such a rule discoverable. The original E1 contract
// (emit no frontmatter block) was falsified by live measurement against omp
// 17.1.8: the bare body is written but no session ever lists the rule, which
// defeats the purpose of emitting it.
func TestOMPAcceptance_E1_UnrecognizedKeysOnly(t *testing.T) {
	src := `---
category: workflow
---

# Body Heading

Body text.
`
	out, err := pkgcontent.TransformRuleForOMP(src)
	require.NoError(t, err)
	fm, body := splitEmittedFrontmatter(t, out)
	assert.Equal(t, "description: Body Heading", strings.TrimSpace(fm),
		"the unrecognized key is dropped and replaced by a synthesized description")
	assert.NotContains(t, out, "category:")
	assert.Equal(t, "# Body Heading\n\nBody text.", strings.TrimSpace(body),
		"the body stays byte-identical")
}

// TestOMPAcceptance_S1_EmptyBodyRejected covers REQ-004 fail-closed behavior.
func TestOMPAcceptance_S1_EmptyBodyRejected(t *testing.T) {
	_, err := pkgcontent.TransformRuleForOMP("---\ndescription: Empty\n---\n")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}
