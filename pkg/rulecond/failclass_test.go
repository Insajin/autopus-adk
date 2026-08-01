package rulecond_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/rulecond"
)

const failClassSubject = "git gc --prune=now && git commit -m x"

// containedPair writes the two legitimate bodies and returns their manifest
// entries.
func containedPair(t *testing.T, f *fixture) []rulecond.ManifestRule {
	t.Helper()
	f.writeBody("lore-commit.md", sourceRuleBody(t, "lore-commit"))
	f.writeBody("worktree-safety.md", sourceRuleBody(t, "worktree-safety"))
	return []rulecond.ManifestRule{
		bashRule("lore-commit", condLoreCommit, "lore-commit.md"),
		bashRule("worktree-safety", condWorktreeSafety, "worktree-safety.md"),
	}
}

func evilRule() rulecond.ManifestRule {
	return bashRule("evil", `\bgit\b`, "/etc/passwd")
}

// TestFire_FailOpenAndFailClosedStayDistinct is the S15 partition oracle for
// REQ-CONDRULE-FIRE-09 and REQ-CONDRULE-FIRE-10.
func TestFire_FailOpenAndFailClosedStayDistinct(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		class        string
		setup        func(t *testing.T, f *fixture)
		stdin        string
		wantInjected []string
		wantStdout   bool
		wantStderr   string
	}{
		{
			name:  "fail-open manifest absent",
			class: "fail-open",
			setup: func(t *testing.T, f *fixture) {
				containedPair(t, f)
				f.removeManifest()
			},
			stdin:        bashPayload(failClassSubject),
			wantInjected: []string{},
		},
		{
			name:  "fail-open stdin unparseable",
			class: "fail-open",
			setup: func(t *testing.T, f *fixture) {
				f.writeManifest(append(containedPair(t, f), evilRule())...)
			},
			stdin:        "{not json",
			wantInjected: []string{},
		},
		{
			name:  "fail-open no condition match",
			class: "fail-open",
			setup: func(t *testing.T, f *fixture) {
				f.writeManifest(containedPair(t, f)...)
			},
			stdin:        bashPayload("ls -la"),
			wantInjected: []string{},
		},
		{
			name:  "fail-open body file deleted skips only that rule",
			class: "fail-open",
			setup: func(t *testing.T, f *fixture) {
				f.writeManifest(containedPair(t, f)...)
				require.NoError(t, os.Remove(filepath.Join(f.bodyRoot(), "worktree-safety.md")))
			},
			stdin:        bashPayload(failClassSubject),
			wantInjected: []string{"lore-commit"},
			wantStdout:   true,
		},
		{
			name:  "fail-closed absolute body suppresses only the offender",
			class: "fail-closed",
			setup: func(t *testing.T, f *fixture) {
				f.writeManifest(append(containedPair(t, f), evilRule())...)
			},
			stdin:        bashPayload(failClassSubject),
			wantInjected: []string{"lore-commit", "worktree-safety"},
			wantStdout:   true,
			wantStderr:   "evil " + string(rulecond.ReasonAbsolutePath),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := newFixture(t)
			tt.setup(t, f)

			stdout, stderr := f.fire(tt.stdin)

			assert.Equal(t, tt.wantInjected, firedRuleNames(t, stdout), "injected rule set")
			if tt.wantStdout {
				assert.NotEmpty(t, stdout)
			} else {
				assert.Empty(t, stdout)
			}
			assert.Equal(t, tt.wantStderr, strings.TrimSpace(stderr), "%s stderr", tt.class)

			assert.NotContains(t, stdout, "permissionDecision")
			assert.NotContains(t, stdout, `"continue"`)
			assert.NotContains(t, stderr, "root:x:0:0")
			assert.NotContains(t, stderr, "/etc/passwd")
		})
	}
}

// TestFire_FailClosedDiagnosticIsNameAndReasonOnly pins the REQ-CONDRULE-FIRE-10
// stderr shape.
func TestFire_FailClosedDiagnosticIsNameAndReasonOnly(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	f.writeManifest(append(containedPair(t, f), evilRule())...)

	stdout, stderr := f.fire(bashPayload(failClassSubject))

	assert.Equal(t, []string{"evil " + string(rulecond.ReasonAbsolutePath)}, stderrLines(stderr))
	assert.NotContains(t, injectedContext(t, stdout), "evil",
		"the fail-closed diagnostic must not reach model context")
}
