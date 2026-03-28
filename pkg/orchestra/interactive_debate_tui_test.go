package orchestra

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SPEC-ORCH-014 R3: opencode with empty InteractiveInput must NOT skip
// SendLongText in round 1. The args-based skip logic only applies when
// InteractiveInput == "args".

// TestExecuteRound_OpencodeTUI_SendLongTextRound1 verifies that opencode
// with InteractiveInput="" receives SendLongText in round 1 (not skipped).
// RED: if opencode defaults still have InteractiveInput="args", this test
// ensures the new TUI-mode behavior works correctly.
func TestExecuteRound_OpencodeTUI_SendLongTextRound1(t *testing.T) {
	t.Parallel()

	mock := newCmuxMock()
	mock.readScreenOutput = "❯\n"

	opencodeProvider := ProviderConfig{
		Name:             "opencode",
		Binary:           "opencode",
		PaneArgs:         []string{"-m", "openai/gpt-5.4"},
		InteractiveInput: "", // TUI mode: prompt via sendkeys
	}

	cfg := OrchestraConfig{
		Providers:      []ProviderConfig{opencodeProvider},
		Strategy:       StrategyDebate,
		Prompt:         "review this code",
		TimeoutSeconds: 5,
		Terminal:       mock,
		Interactive:    true,
		InitialDelay:   time.Millisecond,
	}
	panes := []paneInfo{{provider: opencodeProvider, paneID: "pane-1"}}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_ = executeRound(ctx, cfg, panes, nil, 1, nil)

	// TUI mode opencode MUST receive SendLongText in round 1
	require.NotEmpty(t, mock.sendLongTextCalls,
		"opencode TUI mode must receive SendLongText in round 1 (R3)")
	assert.Contains(t, mock.sendLongTextCalls[0].Text, "review this code",
		"SendLongText must contain the original prompt (R3)")
}

// TestExecuteRound_OpencodeTUI_SendLongTextRound2 verifies that opencode
// with empty InteractiveInput receives SendLongText in round 2+ as well.
// This ensures session persistence — opencode stays alive between rounds.
func TestExecuteRound_OpencodeTUI_SendLongTextRound2(t *testing.T) {
	t.Parallel()

	mock := newCmuxMock()
	mock.readScreenOutput = "❯\n"

	opencodeProvider := ProviderConfig{
		Name:             "opencode",
		Binary:           "opencode",
		PaneArgs:         []string{"-m", "openai/gpt-5.4"},
		InteractiveInput: "", // TUI mode
	}

	cfg := OrchestraConfig{
		Providers:      []ProviderConfig{opencodeProvider},
		Strategy:       StrategyDebate,
		Prompt:         "discuss architecture",
		TimeoutSeconds: 5,
		Terminal:       mock,
		Interactive:    true,
		InitialDelay:   time.Millisecond,
	}
	panes := []paneInfo{{provider: opencodeProvider, paneID: "pane-1"}}

	prevResponses := []ProviderResponse{
		{Provider: "claude", Output: "claude's analysis"},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_ = executeRound(ctx, cfg, panes, nil, 2, prevResponses)

	// Round 2 must also send via SendLongText (session persists)
	require.NotEmpty(t, mock.sendLongTextCalls,
		"opencode TUI mode must receive SendLongText in round 2 (R3)")
}

// TestExecuteRound_OpencodeTUI_NoSkipVsArgsMode verifies the contrast:
// args mode skips SendLongText in round 1, TUI mode does not.
func TestExecuteRound_OpencodeTUI_NoSkipVsArgsMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		interactiveInput string
		expectSendCalls  bool
	}{
		{
			name:             "TUI mode sends prompt via SendLongText",
			interactiveInput: "",
			expectSendCalls:  true,
		},
		{
			name:             "args mode skips SendLongText in round 1",
			interactiveInput: "args",
			expectSendCalls:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mock := newCmuxMock()
			mock.readScreenOutput = "❯\n"

			provider := ProviderConfig{
				Name:             "opencode",
				Binary:           "opencode",
				PaneArgs:         []string{"-m", "openai/gpt-5.4"},
				InteractiveInput: tt.interactiveInput,
			}

			cfg := OrchestraConfig{
				Providers:      []ProviderConfig{provider},
				Strategy:       StrategyDebate,
				Prompt:         "test prompt",
				TimeoutSeconds: 5,
				Terminal:       mock,
				Interactive:    true,
				InitialDelay:   time.Millisecond,
			}
			panes := []paneInfo{{provider: provider, paneID: "pane-1"}}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			_ = executeRound(ctx, cfg, panes, nil, 1, nil)

			if tt.expectSendCalls {
				assert.NotEmpty(t, mock.sendLongTextCalls,
					"TUI mode must call SendLongText in round 1")
			} else {
				assert.Empty(t, mock.sendLongTextCalls,
					"args mode must skip SendLongText in round 1")
			}
		})
	}
}
