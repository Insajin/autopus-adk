package orchestra

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/insajin/autopus-adk/pkg/terminal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// paneCapableRow describes one row of the shared-predicate truth table (S9).
type paneCapableRow struct {
	name       string
	termName   string // "" with nilTerm=true means a literal nil terminal
	nilTerm    bool
	subprocess bool
	wantPane   bool
}

func paneCapableTruthTable() []paneCapableRow {
	return []paneCapableRow{
		{name: "R1 cmux/false", termName: "cmux", wantPane: true},
		{name: "R2 tmux/false", termName: "tmux", wantPane: true},
		{name: "R3 plain/false", termName: "plain", wantPane: false},
		{name: "R4 nil/false", nilTerm: true, wantPane: false},
		{name: "R5 cmux/true", termName: "cmux", subprocess: true, wantPane: false},
		{name: "R6 tmux/true", termName: "tmux", subprocess: true, wantPane: false},
	}
}

func termForRow(r paneCapableRow) terminal.Terminal {
	if r.nilTerm {
		return nil
	}
	return &mockTerminal{name: r.termName}
}

// TestPaneCapable_TruthTable verifies the shared predicate (REQ-007).
func TestPaneCapable_TruthTable(t *testing.T) {
	t.Parallel()
	for _, r := range paneCapableTruthTable() {
		r := r
		t.Run(r.name, func(t *testing.T) {
			t.Parallel()
			got := paneCapable(termForRow(r), r.subprocess)
			assert.Equal(t, r.wantPane, got)
		})
	}
}

// TestSelectBackend_PerRow asserts SelectBackend agrees with the truth table
// (S3 plain->subprocess, S5 nil->subprocess, S7 cmux+subprocess->subprocess).
func TestSelectBackend_PerRow(t *testing.T) {
	t.Parallel()
	for _, r := range paneCapableTruthTable() {
		r := r
		t.Run(r.name, func(t *testing.T) {
			t.Parallel()
			cfg := OrchestraConfig{Terminal: termForRow(r), SubprocessMode: r.subprocess}
			backend := SelectBackend(cfg)
			require.NotNil(t, backend)
			if r.wantPane {
				assert.Equal(t, "pane", backend.Name())
			} else {
				assert.Equal(t, "subprocess", backend.Name())
			}
		})
	}
}

// TestPredicateAgreement is the S9 oracle: SelectBackend returns the pane
// backend EXACTLY when paneCapable is true, and that matches the runner guard
// (paneCapable IS the runner guard after the refactor).
func TestPredicateAgreement(t *testing.T) {
	t.Parallel()
	for _, r := range paneCapableTruthTable() {
		r := r
		t.Run(r.name, func(t *testing.T) {
			t.Parallel()
			term := termForRow(r)
			predicate := paneCapable(term, r.subprocess)
			cfg := OrchestraConfig{Terminal: term, SubprocessMode: r.subprocess}
			selectsPane := SelectBackend(cfg).Name() == "pane"
			// The runner.go dispatch guard is literally paneCapable(...).
			assert.Equal(t, predicate, selectsPane,
				"SelectBackend pane decision must equal paneCapable")
			assert.Equal(t, r.wantPane, predicate)
		})
	}
}

// reviewerJSON is a valid reviewer verdict payload for S8a.
const reviewerJSON = `{"verdict":"PASS","summary":"ok","findings":[]}`

// debaterJSON is a valid debater round-1 payload for S8b.
const debaterJSON = `{"ideas":[{"title":"x","detail":"y"}]}`

// judgeJSON is a valid judge payload for S8c.
const judgeJSON = `{"recommendation":"adopt idea x"}`

// noisyScreen wraps a JSON payload with ANSI escapes and a CLI banner line so
// the sanitizer has real noise to strip (S8a/S8b/S8c).
func noisyScreen(jsonPayload string) string {
	return "\x1b[2J\x1b[H" + // ANSI clear + home
		"▐▛███▜▌ Claude Code v1.2.3\n" + // CLI banner (Unicode block chars)
		"\x1b[32msome status\x1b[0m\n" +
		jsonPayload + "\n" +
		"❯\n"
}

// TestBuildResponseFromScreen_Reviewer covers S8a: sanitized output parses as a
// PASS reviewer verdict with no banner/ANSI leakage.
func TestBuildResponseFromScreen_Reviewer(t *testing.T) {
	t.Parallel()
	b := NewInteractivePaneBackend(OrchestraConfig{Terminal: &mockTerminal{name: "cmux"}})
	resp := b.buildResponseFromScreen("claude", noisyScreen(reviewerJSON), false)

	assert.Equal(t, "pane", resp.ExecutedBackend)
	assert.NotContains(t, resp.Output, "\x1b[", "ANSI escapes must be stripped")
	assert.NotContains(t, resp.Output, "Claude Code", "CLI banner must be stripped")

	out, err := (&OutputParser{}).ParseReviewer(resp.Output)
	require.NoError(t, err)
	assert.Equal(t, "PASS", out.Verdict)
}

// TestBuildResponseFromScreen_Debater covers S8b: sanitized output parses as a
// debater round-1 response with >=1 idea.
func TestBuildResponseFromScreen_Debater(t *testing.T) {
	t.Parallel()
	b := NewInteractivePaneBackend(OrchestraConfig{Terminal: &mockTerminal{name: "cmux"}})
	resp := b.buildResponseFromScreen("claude", noisyScreen(debaterJSON), false)

	assert.NotContains(t, resp.Output, "\x1b[")
	assert.NotContains(t, resp.Output, "Claude Code")

	out, err := (&OutputParser{}).ParseDebaterR1(resp.Output)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(out.Ideas), 1)
}

// TestBuildResponseFromScreen_Judge covers S8c: sanitized output parses as a
// judge response with a non-empty recommendation.
func TestBuildResponseFromScreen_Judge(t *testing.T) {
	t.Parallel()
	b := NewInteractivePaneBackend(OrchestraConfig{Terminal: &mockTerminal{name: "cmux"}})
	resp := b.buildResponseFromScreen("claude", noisyScreen(judgeJSON), false)

	assert.NotContains(t, resp.Output, "\x1b[")
	assert.NotContains(t, resp.Output, "Claude Code")

	out, err := (&OutputParser{}).ParseJudge(resp.Output)
	require.NoError(t, err)
	assert.NotEmpty(t, strings.TrimSpace(out.Recommendation))
}

func TestCollectResponse_PrefersResponseFile(t *testing.T) {
	t.Parallel()
	responsePath := filepath.Join(t.TempDir(), "response.md")
	content := responseBeginMarker + "\nfile answer\n" + responseEndMarker + "\n"
	require.NoError(t, os.WriteFile(responsePath, []byte(content), 0o600))

	mock := newCmuxMock()
	mock.readScreenOutput = "screen fallback should not be used"
	b := NewInteractivePaneBackend(OrchestraConfig{Terminal: mock})

	resp := b.collectResponse(context.Background(), ProviderRequest{Provider: "claude"}, paneInfo{
		paneID:       "pane-1",
		responseFile: responsePath,
	}, true)

	require.NotNil(t, resp)
	assert.Equal(t, "file answer", resp.Output)
	assert.False(t, resp.TimedOut)
	assert.False(t, resp.EmptyOutput)
	assert.Equal(t, "pane", resp.ExecutedBackend)
	assert.Zero(t, mock.readScreenCalls, "valid response file should avoid screen collection")
}

// TestInteractivePaneBackend_Name guards the backend identifier.
func TestInteractivePaneBackend_Name(t *testing.T) {
	t.Parallel()
	b := NewInteractivePaneBackend(OrchestraConfig{})
	assert.Equal(t, "pane", b.Name())
}
