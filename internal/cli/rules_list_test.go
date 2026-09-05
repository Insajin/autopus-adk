package cli

// `auto rules list` failure and rendering oracles.
//
// The happy path is covered by rules_conditional_test.go (classification) and
// rules_sticky_test.go (sticky column, cadence line). What is left is what the
// inspection command does when the project config cannot be read and when the
// output sink refuses a write. Both must surface: a listing that prints a
// fabricated cadence, or a table truncated without a word, is worse for an
// operator than one that refuses outright.

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// errWriteRefused is what refusingWriter returns, so a test can assert the
// renderer propagated the sink's own error rather than one of its own.
var errWriteRefused = errors.New("sink refused the write")

// refusingWriter refuses every write whose payload contains stop, and records
// what it accepted. An empty stop refuses everything.
type refusingWriter struct {
	stop     string
	accepted strings.Builder
}

func (w *refusingWriter) Write(p []byte) (int, error) {
	if w.stop == "" || strings.Contains(string(p), w.stop) {
		return 0, errWriteRefused
	}
	return w.accepted.Write(p)
}

// listFixtureRows is one row per classification, plus a second sticky-bearing
// always rule, so counts and the sticky tally are all distinguishable.
func listFixtureRows() []ruleListRow {
	return []ruleListRow{
		{Name: "branding", Class: "always", Trigger: "-",
			Destination: ".claude/rules/autopus/branding.md"},
		{Name: "language-policy", Class: "always", Sticky: true, Trigger: "-",
			Destination: ".claude/rules/autopus/language-policy.md"},
		{Name: "file-size-limit", Class: "paths-scoped", Trigger: "**/*.go",
			Destination: ".claude/rules/autopus/file-size-limit.md"},
		{Name: "lore-commit", Class: "hook-fired", Trigger: "tool:bash",
			Destination: ".claude/hooks/autopus/conditional/lore-commit.md"},
		{Name: "spec-quality", Class: "skill-scoped", Trigger: "-",
			Destination: ".claude/hooks/autopus/conditional/spec-quality.md"},
	}
}

// TestRulesList_UnreadableConfigRefusesInsteadOfFabricatingCadence is the
// effectiveStickyCadence failure oracle. sticky_cadence is a typed int, so a
// non-integer scalar fails config load. The sticky runtime degrades to the
// default there, but an inspection command must not: reporting 8 would tell an
// operator the project is configured for a cadence nobody wrote.
func TestRulesList_UnreadableConfigRefusesInsteadOfFabricatingCadence(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".claude"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "autopus.yaml"),
		[]byte("hooks:\n  sticky_cadence: every-other-prompt\n"), 0o644))
	t.Chdir(dir)

	cmd := newRulesCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"list"})

	err := cmd.Execute()

	require.Error(t, err, "an unreadable cadence must not be reported as a value")
	assert.Contains(t, err.Error(), "resolve sticky cadence")
	assert.Contains(t, err.Error(), "parse config",
		"the underlying config failure must stay attributable")
	assert.Empty(t, out.String(),
		"the refusal is total: no table, no counts, and above all no cadence line")
}

// TestRenderRuleListRows_WriteFailuresReachTheCaller pins that every write the
// renderer performs is checked, so a sink that stops accepting output can never
// leave a silently truncated listing behind.
//
// The form feed in the "during a row" case is what makes that case reachable:
// tabwriter buffers a tab-separated line until a section break, so a row write
// only touches the sink when the row itself forces a flush.
func TestRenderRuleListRows_WriteFailuresReachTheCaller(t *testing.T) {
	tests := []struct {
		name        string
		stop        string
		forceFlush  bool
		wantAccepts string // substring the sink must have accepted before failing
	}{
		{name: "during a row", stop: "", forceFlush: true},
		{name: "at the table flush", stop: ""},
		{name: "at the summary", stop: "rules:", wantAccepts: "language-policy"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rows := listFixtureRows()
			if tt.forceFlush {
				rows[0].Trigger = "-\f"
			}
			sink := &refusingWriter{stop: tt.stop}

			err := renderRuleListRows(sink, rows, 3)

			require.ErrorIs(t, err, errWriteRefused,
				"the sink's own error must reach the caller unwrapped")
			assert.NotContains(t, sink.accepted.String(), "re-attached",
				"the renderer stops at the first refusal, so the sticky summary is never emitted")
			if tt.wantAccepts == "" {
				return
			}
			assert.Contains(t, sink.accepted.String(), tt.wantAccepts,
				"a summary-only refusal still leaves the aligned table on the sink")
		})
	}
}

// TestRenderRuleListRows_SummarisesCountsAndAlignsColumns is the rendering
// oracle: the totals line is derived from the rows rather than restated, and the
// cadence it reports is the resolved one the caller passed in.
func TestRenderRuleListRows_SummarisesCountsAndAlignsColumns(t *testing.T) {
	var out bytes.Buffer

	require.NoError(t, renderRuleListRows(&out, listFixtureRows(), 3))

	assert.True(t, strings.HasSuffix(out.String(),
		"\n5 rules: 2 always, 1 paths-scoped, 1 hook-fired, 1 skill-scoped\n"+
			"1 sticky, re-attached on an effective cadence of 3 prompts\n"),
		"totals must count the rows and echo the resolved cadence:\n%s", out.String())

	lines := strings.Split(out.String(), "\n")
	require.Greater(t, len(lines), 5)
	assert.True(t, strings.HasPrefix(lines[0], "RULE"),
		"the header names the rule column first:\n%s", out.String())

	branding := rowFor(t, out.String(), "branding")
	languagePolicy := rowFor(t, out.String(), "language-policy")
	assert.Equal(t, strings.Index(languagePolicy, "always"), strings.Index(branding, "always"),
		"equal-width columns must start at the same offset:\n%s", out.String())

	assert.Contains(t, branding, "false", "branding carries no alwaysApply flag")
	assert.Contains(t, languagePolicy, "true", "language-policy is one of the two sticky rules")
}
