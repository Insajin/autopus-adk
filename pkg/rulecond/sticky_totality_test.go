package rulecond_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/rulecond"
)

// totalityFixture is the S13 base layout: two contained sticky rules plus one
// hostile entry whose body location is absolute.
func totalityFixture(t *testing.T) *stickyFixture {
	t.Helper()
	f := newStickyFixture(t)
	f.writeStickyBody(ruleLanguagePolicy+".md", sourceRuleFile(t, ruleLanguagePolicy))
	f.writeStickyBody(ruleObjectiveReasoning+".md", sourceRuleFile(t, ruleObjectiveReasoning))
	f.writeStickyManifest(
		rulecond.StickyRule{Name: ruleLanguagePolicy, Body: ruleLanguagePolicy + ".md"},
		rulecond.StickyRule{Name: ruleObjectiveReasoning, Body: ruleObjectiveReasoning + ".md"},
		rulecond.StickyRule{Name: "evil", Body: "/etc/passwd"},
	)
	return f
}

// assertInjectedSet fixes exactly which shipped bodies reached the context.
func assertInjectedSet(t *testing.T, ctx string, want []string) {
	t.Helper()
	for _, name := range []string{ruleLanguagePolicy, ruleObjectiveReasoning} {
		body := injectableBody(t, name)
		if containsString(want, name) {
			assert.Contains(t, ctx, body, "%s must inject", name)
			continue
		}
		assert.NotContains(t, ctx, body, "%s must not inject", name)
	}
}

func containsString(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}

// TestStickyFire_EveryConditionLandsInExactlyOneCase is the S13 totality oracle
// for REQ-STICKYRULE-FIRE-08 and INV-109. Collapsing these cases would make an
// escape attempt indistinguishable from a missing file.
func TestStickyFire_EveryConditionLandsInExactlyOneCase(t *testing.T) {
	tests := []struct {
		name         string
		partition    string
		setup        func(t *testing.T, f *stickyFixture)
		cadence      int
		stdin        string
		wantInjected []string
		wantStderr   []string
	}{
		{
			name:      "manifest absent",
			partition: "whole-run benign",
			setup:     func(_ *testing.T, f *stickyFixture) { f.removeStickyManifest() },
			cadence:   1,
		},
		{
			name:      "manifest is not json",
			partition: "whole-run benign",
			setup:     func(_ *testing.T, f *stickyFixture) { f.writeRawManifestFile("{not json") },
			cadence:   1,
		},
		{
			name:      "stdin unparseable",
			partition: "whole-run benign",
			cadence:   1,
			stdin:     "not json",
		},
		{
			name:      "stdin empty",
			partition: "whole-run benign",
			cadence:   1,
			stdin:     "",
		},
		{
			name:      "state file holds a non-number",
			partition: "whole-run benign",
			setup: func(t *testing.T, f *stickyFixture) {
				require.NoError(t, os.MkdirAll(f.stateDir(), 0o755))
				require.NoError(t, os.WriteFile(
					filepath.Join(f.stateDir(), rulecond.StickyStateKey("S-total")),
					[]byte("not-a-number"), 0o644))
			},
			cadence: 1,
		},
		{
			name:      "index is off-cadence",
			partition: "whole-run benign",
			setup: func(t *testing.T, f *stickyFixture) {
				require.NoError(t, os.MkdirAll(f.stateDir(), 0o755))
				require.NoError(t, os.WriteFile(
					filepath.Join(f.stateDir(), rulecond.StickyStateKey("S-total")),
					[]byte("1"), 0o644))
			},
			cadence: 8, // the next observed index is 2, which is off-cadence
		},
		{
			// The deleted body contributes no stderr line of its own: the single
			// line below is the base fixture's hostile entry, which stays
			// fail-closed in the same run. That coexistence is the partition
			// proof — a body that is simply gone stays silent while a containment
			// violation is still named, so the two per-rule cases cannot collapse
			// into each other (REQ-STICKYRULE-FIRE-08).
			name:      "per-rule benign: one body file deleted",
			partition: "per-rule benign",
			setup: func(t *testing.T, f *stickyFixture) {
				require.NoError(t, os.Remove(
					filepath.Join(f.stickyRoot(), ruleObjectiveReasoning+".md")))
			},
			cadence:      1,
			wantInjected: []string{ruleLanguagePolicy},
			wantStderr:   []string{"evil " + string(rulecond.ReasonAbsolutePath)},
		},
		{
			name:         "per-rule fail-closed: hostile body location",
			partition:    "per-rule fail-closed",
			cadence:      1,
			wantInjected: []string{ruleLanguagePolicy, ruleObjectiveReasoning},
			wantStderr:   []string{"evil " + string(rulecond.ReasonAbsolutePath)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.partition+": "+tt.name, func(t *testing.T) {
			f := totalityFixture(t)
			if tt.setup != nil {
				tt.setup(t, f)
			}
			stdin := tt.stdin
			if stdin == "" && tt.name != "stdin empty" {
				stdin = stickyPayload("S-total")
			}

			stdout, stderr := f.submitRaw(stdin, tt.cadence)

			if len(tt.wantInjected) == 0 {
				assert.Empty(t, stdout, "%s writes no stdout", tt.partition)
			} else {
				assert.NotEmpty(t, stdout, "%s still injects the contained rules", tt.partition)
			}
			assertInjectedSet(t, stickyContext(t, stdout), tt.wantInjected)
			assert.Equal(t, tt.wantStderr, stderrLines(stderr))

			assert.NotContains(t, stdout, "decision",
				"a decision value of block would erase the user's prompt")
			assert.NotContains(t, stdout, "block")
		})
	}
}

// TestStickyFire_HostileEntryDoesNotSuppressContainedRules pins the per-rule
// scope of a fail-closed refusal: it removes one rule, never the run.
func TestStickyFire_HostileEntryDoesNotSuppressContainedRules(t *testing.T) {
	f := totalityFixture(t)

	stdout, stderr := f.submitAt("S-scope", 1)
	ctx := stickyContext(t, stdout)

	assertInjectedSet(t, ctx, []string{ruleLanguagePolicy, ruleObjectiveReasoning})
	assert.Equal(t, []string{"evil " + string(rulecond.ReasonAbsolutePath)}, stderrLines(stderr))
	assert.NotContains(t, ctx, "evil")
}
