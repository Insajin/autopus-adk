package cli

import (
	"regexp"
	"testing"

	"github.com/insajin/autopus-adk/pkg/terminal"
)

// sessionIDPattern mirrors the safe pattern enforced by NewHookSession and
// SendSessionEnvToPane; a generated session ID that violates it would be
// rejected at runtime and silently disable hook collection.
var sessionIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

func TestHookCollectionEligible(t *testing.T) {
	cmux := stubTerminal{name: "cmux"}
	tmux := stubTerminal{name: "tmux"}
	plain := stubTerminal{name: "plain"}

	tests := []struct {
		name      string
		term      terminal.Terminal
		subproc   bool
		hookAvail bool
		want      bool
	}{
		{"cmux + hook installed", cmux, false, true, true},
		{"tmux + hook installed", tmux, false, true, true},
		{"cmux without hooks installed", cmux, false, false, false},
		{"plain terminal stays floor", plain, false, true, false},
		{"subprocess forced stays floor", cmux, true, true, false},
		{"nil terminal stays floor", nil, false, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hookCollectionEligible(tt.term, tt.subproc, tt.hookAvail); got != tt.want {
				t.Fatalf("hookCollectionEligible(%s, subproc=%v, hook=%v) = %v, want %v",
					tt.name, tt.subproc, tt.hookAvail, got, tt.want)
			}
		})
	}
}

func TestNewOrchSessionID_MatchesSafePattern(t *testing.T) {
	if got := newOrchSessionID(); !sessionIDPattern.MatchString(got) {
		t.Fatalf("newOrchSessionID() = %q does not match %s", got, sessionIDPattern)
	}
}
