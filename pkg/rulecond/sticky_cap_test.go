package rulecond_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/rulecond"
)

// syntheticStickyFile renders a rule file whose injectable body — the body with
// the frontmatter removed — measures exactly bodyBytes.
func syntheticStickyFile(name, marker string, bodyBytes int) string {
	head := "# " + marker + "\n"
	var body string
	if len(head) >= bodyBytes {
		body = head[:bodyBytes]
	} else {
		body = head + strings.Repeat("x", bodyBytes-len(head))
	}
	return "---\nname: " + name + "\ncategory: synthetic\nalwaysApply: true\n---\n" + body
}

// linesMentioning returns the context lines that name a rule.
func linesMentioning(ctx, name string) []string {
	var out []string
	for _, line := range strings.Split(ctx, "\n") {
		if strings.Contains(line, name) {
			out = append(out, line)
		}
	}
	return out
}

// TestStickyFire_ShippedPairFitsUnderTheAggregateCap is the first half of the
// S6 oracle: the measured pair injects whole and carries no frontmatter.
func TestStickyFire_ShippedPairFitsUnderTheAggregateCap(t *testing.T) {
	assert.Equal(t, 6000, rulecond.MaxStickyContextBytes,
		"the aggregate cap admits the shipped pair, which a 4000-byte cap would have truncated")
	assert.Equal(t, 4000, rulecond.MaxStickyBodyBytes)

	f := newStickyFixture(t)
	f.writeShippedStickyPair()

	stdout, stderr := f.submit("S-alpha")
	require.Empty(t, stderr)
	ctx := stickyContext(t, stdout)

	language := injectableBody(t, ruleLanguagePolicy)
	objective := injectableBody(t, ruleObjectiveReasoning)
	assert.Contains(t, ctx, language, "the language-policy body must inject in full")
	assert.Contains(t, ctx, objective, "the objective-reasoning body must inject in full")
	assert.LessOrEqual(t, len(language)+len(objective), rulecond.MaxStickyContextBytes,
		"the shipped injectable pair must fit the aggregate cap")
	assert.LessOrEqual(t, len(ctx), rulecond.MaxStickyContextBytes)

	for _, frontmatterLine := range []string{
		"name: " + ruleLanguagePolicy,
		"name: " + ruleObjectiveReasoning,
		"category: workflow",
		"category: quality",
		"alwaysApply: true",
	} {
		assert.NotContains(t, strings.Split(ctx, "\n"), frontmatterLine,
			"only the injectable body enters context, never the frontmatter")
	}
	assert.NotContains(t, ctx, rulecond.TruncationPrefix,
		"nothing is dropped when the pair fits")
}

// TestStickyFire_AggregateCapDropsWholeRulesInReverseNameOrder is the second
// half of the S6 oracle for REQ-STICKYRULE-FIRE-05 and REQ-STICKYRULE-FIRE-10.
func TestStickyFire_AggregateCapDropsWholeRulesInReverseNameOrder(t *testing.T) {
	f := newStickyFixture(t)
	f.writeStickyBody("aaa-rule.md", syntheticStickyFile("aaa-rule", "AAA-MARKER", 3000))
	f.writeStickyBody("mmm-rule.md", syntheticStickyFile("mmm-rule", "MMM-MARKER", 2000))
	f.writeStickyBody("zzz-rule.md", syntheticStickyFile("zzz-rule", "ZZZ-MARKER", 2000))
	f.writeStickyManifest(
		rulecond.StickyRule{Name: "aaa-rule", Body: "aaa-rule.md"},
		rulecond.StickyRule{Name: "mmm-rule", Body: "mmm-rule.md"},
		rulecond.StickyRule{Name: "zzz-rule", Body: "zzz-rule.md"},
	)

	stdout, stderr := f.submitRaw(
		stickyPayloadFull("S-cap", "deploy with "+decoySecret, "/t.jsonl", "/c"), 1)
	require.Empty(t, stderr, "an aggregate-cap drop is not a fail-closed suppression")
	ctx := stickyContext(t, stdout)

	assert.Contains(t, ctx, "AAA-MARKER", "aaa-rule is kept")
	assert.Contains(t, ctx, "MMM-MARKER", "mmm-rule is kept")
	assert.NotContains(t, ctx, "ZZZ-MARKER",
		"zzz-rule is dropped whole; a partial body is never injected")
	assert.LessOrEqual(t, len(ctx), rulecond.MaxStickyContextBytes,
		"additionalContext must stay at or under the aggregate cap")

	notice := linesMentioning(ctx, "zzz-rule")
	require.Len(t, notice, 1,
		"a dropped rule is named on exactly one truncation notice line")
	assert.NotContains(t, notice[0], "ZZZ-MARKER")
	assert.NotContains(t, ctx, decoySecret, "the notice carries no prompt text")
	assert.NotContains(t, ctx, "deploy with")

	// The payload is a pure function of the sticky set, so a second injecting
	// prompt reproduces it byte for byte.
	repeat, _ := f.submitRaw(
		stickyPayloadFull("S-cap", "another prompt", "/t.jsonl", "/c"), 1)
	assert.Equal(t, stdout, repeat, "repeating the run must produce a byte-identical payload")
}

// TestStickyFire_PerBodyCapMeasuresTheWholeSourceFile is a characterization
// oracle for the measurement basis of the per-body cap.
//
// REQ-STICKYRULE-FIRE-07 states the cap over the injectable body, but the
// enforced quantity is the whole emitted rule file, frontmatter included:
// ReadBody refuses on info.Size(), and checkStickyBody gates generation on the
// same source-file size on purpose, so a rule the compiler admits is a rule the
// runtime can read. The effective allowance for an injectable body is therefore
// MaxStickyBodyBytes minus its frontmatter, not MaxStickyBodyBytes.
//
// The deviation is fail-closed and self-consistent, so this test pins it rather
// than asserting the looser SPEC wording. It fails if either half moves: if
// ReadBody switched to the injectable body the rule below would inject, and if
// splitFrontmatter changed shape the compile-time gate would stop agreeing with
// the runtime.
func TestStickyFire_PerBodyCapMeasuresTheWholeSourceFile(t *testing.T) {
	const frontmatter = "---\nname: band-rule\ncategory: synthetic\nalwaysApply: true\n---\n"
	// An injectable body deliberately inside the cap whose source file, once the
	// frontmatter is counted, lands outside it.
	body := "# BAND-MARKER\n" + strings.Repeat("x", 3990-len("# BAND-MARKER\n"))
	require.Less(t, len(body), rulecond.MaxStickyBodyBytes,
		"the injectable body must sit inside the cap for this case to be meaningful")
	require.Greater(t, len(frontmatter)+len(body), rulecond.MaxStickyBodyBytes,
		"the source file must sit outside the cap for this case to be meaningful")

	f := newStickyFixture(t)
	f.writeStickyBody("band-rule.md", frontmatter+body)
	f.writeStickyManifest(rulecond.StickyRule{Name: "band-rule", Body: "band-rule.md"})

	stdout, stderr := f.submitAt("S-band", 1)

	assert.NotContains(t, stickyContext(t, stdout), "BAND-MARKER",
		"the enforced cap is the source-file size, so frontmatter consumes the allowance")
	assert.Equal(t, []string{"band-rule " + string(rulecond.ReasonBodyTooLarge)},
		stderrLines(stderr),
		"the refusal is fail-closed and reports the rule name and reason code only")
}

// TestStickySourceFiles_KeepHeadroomUnderThePerBodyCap is the tripwire the
// measurement basis above implies: because the cap is enforced over the whole
// source file, growing a shipped sticky rule past MaxStickyBodyBytes stops it
// injecting. Deriving the set from the shipped rule names keeps this from
// rotting into a hardcoded file list.
func TestStickySourceFiles_KeepHeadroomUnderThePerBodyCap(t *testing.T) {
	t.Parallel()

	total := 0
	for _, name := range []string{ruleLanguagePolicy, ruleObjectiveReasoning} {
		source := sourceRuleFile(t, name)
		assert.LessOrEqual(t, len(source), rulecond.MaxStickyBodyBytes,
			"%s measures %d bytes with frontmatter and would be refused body_too_large",
			name, len(source))
		total += len(injectableBody(t, name))
	}

	// The pin moved 4492 -> 4628 when language-policy.md gained its "Enforcement"
	// section, which replaced a claim of strict enforcement with the fact that
	// nothing in the harness inspects language. That content is the point of the
	// rule, so the pin absorbed it rather than the rule being trimmed back to a
	// false claim. The tripwire still bites: 4628 leaves 1372 bytes under
	// MaxStickyContextBytes, and the next growth has to justify itself here too.
	assert.LessOrEqual(t, total, 4628,
		"REQ-STICKYRULE-FIRE-05 pins the shipped pair at 4628 injectable bytes or less")
	assert.LessOrEqual(t, total, rulecond.MaxStickyContextBytes,
		"the shipped pair must inject whole, with no truncation notice")
}

// TestStickyFire_PerBodyCapPrecedesTheAggregateCap is the REQ-STICKYRULE-FIRE-07
// ordering oracle: an oversize body is refused outright rather than counted
// toward the aggregate and then dropped silently.
func TestStickyFire_PerBodyCapPrecedesTheAggregateCap(t *testing.T) {
	f := newStickyFixture(t)
	f.writeStickyBody("aaa-huge.md", syntheticStickyFile("aaa-huge", "HUGE-MARKER", 4001))
	f.writeStickyBody("bbb-small.md", syntheticStickyFile("bbb-small", "SMALL-MARKER", 500))
	f.writeStickyManifest(
		rulecond.StickyRule{Name: "aaa-huge", Body: "aaa-huge.md"},
		rulecond.StickyRule{Name: "bbb-small", Body: "bbb-small.md"},
	)

	stdout, stderr := f.submitAt("S-percap", 1)
	ctx := stickyContext(t, stdout)

	assert.NotContains(t, ctx, "HUGE-MARKER",
		"a body above the 4000-byte per-body cap injects no part of itself")
	assert.Contains(t, ctx, "SMALL-MARKER",
		"suppressing one rule must not suppress the contained ones")
	assert.Equal(t, []string{"aaa-huge " + string(rulecond.ReasonBodyTooLarge)},
		stderrLines(stderr),
		"a size violation is fail-closed and reports the rule name and reason code only")
}
