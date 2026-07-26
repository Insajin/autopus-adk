package orchestra

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/terminal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type judgeSessionRecordingTerminal struct {
	*paneCommitTerminal
	commands []string
}

func (m *judgeSessionRecordingTerminal) SendCommand(
	ctx context.Context,
	paneID terminal.PaneID,
	command string,
) error {
	m.commands = append(m.commands, command)
	return m.paneCommitTerminal.SendCommand(ctx, paneID, command)
}

func TestRunJudgeRound_PaneCapableTerminalDoesNotExecuteSubprocess(t *testing.T) {
	provider, marker := newPaneBoundaryMarkerProvider(t, "claude", "")
	term := &paneCommitTerminal{
		screen: `{"recommendation":"keep the judge in a pane"}` + "\n❯\n",
	}
	cfg := OrchestraConfig{
		Providers:          []ProviderConfig{provider},
		JudgeProvider:      provider.Name,
		Terminal:           term,
		WorkingDir:         t.TempDir(),
		TimeoutSeconds:     3,
		InitialDelay:       time.Millisecond,
		CompletionDetector: &stubCompletionDetector{completed: true},
	}

	resp := runJudgeRound(context.Background(), cfg, nil, nil, []ProviderResponse{
		{Provider: "claude", Output: "candidate answer"},
	}, 1)

	require.NotNil(t, resp)
	assert.Equal(t, 1, term.splitCalls, "judge execution must provision a pane")
	assert.Equal(t, paneBackendName, resp.ExecutedBackend)
	assert.Contains(t, term.closed, string(committedPaneID), "judge pane must be cleaned up")
	_, statErr := os.Stat(marker)
	assert.ErrorIs(t, statErr, os.ErrNotExist, "pane judge must not execute the subprocess fixture")
}

func TestRunJudgeRound_ClaudeJudgeUsesFreshHookSession(t *testing.T) {
	isolateSurfaceTracker(t)
	t.Setenv("TMPDIR", t.TempDir())
	provider, _ := newPaneBoundaryMarkerProvider(t, "claude", "")
	startupHook := false
	provider.HasStartupHook = &startupHook
	const participantSessionID = "participant-session"
	term := &judgeSessionRecordingTerminal{paneCommitTerminal: &paneCommitTerminal{
		splitID: "surface:9201",
		screen:  `{"recommendation":"isolated judge session"}` + "\n❯\n",
	}}
	cfg := OrchestraConfig{
		Providers:          []ProviderConfig{provider},
		JudgeProvider:      provider.Name,
		Terminal:           term,
		WorkingDir:         t.TempDir(),
		TimeoutSeconds:     3,
		InitialDelay:       time.Millisecond,
		CompletionDetector: &stubCompletionDetector{completed: true},
		HookMode:           true,
		SessionID:          participantSessionID,
	}

	resp := runJudgeRound(context.Background(), cfg, nil, nil, []ProviderResponse{
		{Provider: "claude", Output: "candidate answer"},
	}, 1)

	require.NotNil(t, resp)
	var judgeSessionID string
	for _, command := range term.commands {
		const prefix = "export AUTOPUS_SESSION_ID="
		if !strings.HasPrefix(command, prefix) {
			continue
		}
		fields := strings.Fields(strings.TrimPrefix(command, prefix))
		if len(fields) > 0 {
			judgeSessionID = fields[0]
		}
		break
	}
	assert.NotEmpty(t, judgeSessionID, "Claude judge must receive hook coordinates in its pane")
	assert.NotEqual(t, participantSessionID, judgeSessionID,
		"Claude judge must not reuse the participant hook session")
}

func TestRunJudgeRound_CodexJudgeUsesDirectReadinessWhenFreshArtifactMissing(t *testing.T) {
	isolateSurfaceTracker(t)
	t.Setenv("TMPDIR", t.TempDir())
	provider, _ := newPaneBoundaryMarkerProvider(t, "codex", "")
	term := &judgeSessionRecordingTerminal{paneCommitTerminal: &paneCommitTerminal{
		splitID: "surface:9203",
		screen:  `{"recommendation":"codex direct readiness"}` + "\n" + codexStablePrompt,
	}}
	cfg := OrchestraConfig{
		Providers:          []ProviderConfig{provider},
		JudgeProvider:      provider.Name,
		Terminal:           term,
		WorkingDir:         t.TempDir(),
		TimeoutSeconds:     60,
		InitialDelay:       time.Millisecond,
		CompletionDetector: &stubCompletionDetector{completed: true},
		HookMode:           true,
		SessionID:          "participant-session",
	}

	resp := runJudgeRound(context.Background(), cfg, nil, nil, []ProviderResponse{
		{Provider: "codex", Output: "candidate answer"},
	}, 1)

	require.NotNil(t, resp)
	assert.Empty(t, resp.Error)
	assert.Equal(t, TerminalCompleted, resp.TerminalState)
	require.NotNil(t, resp.freshJudgeSession)
	assert.True(t, resp.freshJudgeSession.Isolated)
	assert.True(t, resp.freshJudgeSession.Verified)
}

func TestRunJudgeRound_PreservesConcretePaneFailure(t *testing.T) {
	isolateSurfaceTracker(t)
	provider, _ := newPaneBoundaryMarkerProvider(t, "claude", "")
	term := &paneCommitTerminal{splitID: "surface:9202", longTextErrAt: 1}
	cfg := OrchestraConfig{
		Providers:      []ProviderConfig{provider},
		JudgeProvider:  provider.Name,
		Terminal:       term,
		WorkingDir:     t.TempDir(),
		TimeoutSeconds: 3,
		InitialDelay:   time.Millisecond,
	}

	resp := runJudgeRound(context.Background(), cfg, nil, nil, []ProviderResponse{
		{Provider: "claude", Output: "candidate answer"},
	}, 1)

	if assert.NotNil(t, resp, "failed judge attempt must remain observable to the caller") {
		assert.Contains(t, resp.Error, "launch send error: injected SendLongText failure")
		assert.Equal(t, paneBackendName, resp.ExecutedBackend)
	}
}

func TestRunJudgeRound_FreshJudgeStartupBudgetExceedsParticipantBudget(t *testing.T) {
	tests := []struct {
		name       string
		provider   string
		readyFrame string
	}{
		{name: "claude", provider: "claude", readyFrame: "❯\n"},
		{name: "codex", provider: "codex", readyFrame: codexStablePrompt},
		{name: "gemini", provider: "gemini", readyFrame: "> Type your message\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isolateSurfaceTracker(t)
			provider, _ := newPaneBoundaryMarkerProvider(t, tt.provider, "")
			provider.StartupTimeout = 250 * time.Millisecond
			term := &seqScreenMock{
				name: "cmux",
				screens: []string{
					"starting fresh judge...\n",
					tt.readyFrame,
					`{"recommendation":"fresh judge ready"}` + "\n" + tt.readyFrame,
				},
			}
			cfg := OrchestraConfig{
				Providers:          []ProviderConfig{provider},
				JudgeProvider:      provider.Name,
				Terminal:           term,
				WorkingDir:         t.TempDir(),
				TimeoutSeconds:     60,
				InitialDelay:       time.Millisecond,
				CompletionDetector: &stubCompletionDetector{completed: true},
			}

			assert.Equal(t, 250*time.Millisecond, startupTimeoutFor(provider),
				"the ordinary participant budget must remain unchanged")
			resp := runJudgeRound(context.Background(), cfg, nil, nil, []ProviderResponse{
				{Provider: tt.provider, Output: "candidate answer"},
			}, 1)

			require.NotNil(t, resp)
			assert.Empty(t, resp.Error,
				"the fresh judge must outlive the ordinary participant startup budget")
			assert.Equal(t, TerminalCompleted, resp.TerminalState)
			require.NotNil(t, resp.freshJudgeSession)
			assert.True(t, resp.freshJudgeSession.Isolated)
		})
	}
}

func TestFreshJudgeStartupTimeout_AppliesFloorAndRequestCap(t *testing.T) {
	tests := []struct {
		name           string
		provider       ProviderConfig
		ordinary       time.Duration
		requestTimeout time.Duration
		want           time.Duration
	}{
		{
			name: "claude floor", provider: ProviderConfig{Name: "claude"},
			ordinary: 30 * time.Second, requestTimeout: 5 * time.Minute, want: time.Minute,
		},
		{
			name: "codex floor", provider: ProviderConfig{Name: "codex"},
			ordinary: 30 * time.Second, requestTimeout: 5 * time.Minute, want: time.Minute,
		},
		{
			name: "gemini floor", provider: ProviderConfig{Name: "gemini"},
			ordinary: 10 * time.Second, requestTimeout: 5 * time.Minute, want: time.Minute,
		},
		{
			name: "larger provider budget",
			provider: ProviderConfig{
				Name:           "claude",
				StartupTimeout: 90 * time.Second,
			},
			requestTimeout: 2 * time.Minute,
			want:           90 * time.Second,
		},
		{
			name: "request cap",
			provider: ProviderConfig{
				Name:           "codex",
				StartupTimeout: 90 * time.Second,
			},
			requestTimeout: 45 * time.Second,
			want:           45 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.ordinary > 0 {
				assert.Equal(t, tt.ordinary, startupTimeoutFor(tt.provider),
					"the ordinary participant default must remain provider-specific")
			}
			assert.Equal(t, tt.want, freshJudgeStartupTimeout(tt.provider, tt.requestTimeout))
		})
	}
}
