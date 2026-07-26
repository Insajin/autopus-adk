package orchestra

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/terminal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const participantJudgeBoundaryPane terminal.PaneID = "participant-pane"

type judgeBoundaryTerminal struct {
	*paneCommitTerminal
	events                 []string
	participantPipeStopErr error
	participantCloseErr    error
}

func (m *judgeBoundaryTerminal) SplitPane(
	ctx context.Context,
	direction terminal.Direction,
) (terminal.PaneID, error) {
	m.events = append(m.events, "split:judge")
	return m.paneCommitTerminal.SplitPane(ctx, direction)
}

func (m *judgeBoundaryTerminal) PipePaneStop(
	_ context.Context,
	paneID terminal.PaneID,
) error {
	m.events = append(m.events, "pipe-stop:"+string(paneID))
	if paneID == participantJudgeBoundaryPane && m.participantPipeStopErr != nil {
		return m.participantPipeStopErr
	}
	return nil
}

func (m *judgeBoundaryTerminal) Close(_ context.Context, ref string) error {
	m.events = append(m.events, "close:"+ref)
	if ref == string(participantJudgeBoundaryPane) && m.participantCloseErr != nil {
		return m.participantCloseErr
	}
	return nil
}

func TestRunJudgeRound_FreshHookSessionProjectsRedactedReceiptEvidence(t *testing.T) {
	isolateSurfaceTracker(t)
	t.Setenv("TMPDIR", t.TempDir())
	provider, _ := newPaneBoundaryMarkerProvider(t, "claude", "")
	const participantSessionID = "participant-session-sensitive"
	participantSession, err := NewHookSession(participantSessionID)
	require.NoError(t, err)
	term := &judgeSessionRecordingTerminal{paneCommitTerminal: &paneCommitTerminal{
		splitID: "surface:9301",
		screen:  `{"recommendation":"isolated judge session"}` + "\n❯\n",
	}}
	cfg := OrchestraConfig{
		Providers:          []ProviderConfig{provider},
		Strategy:           StrategyDebate,
		JudgeProvider:      provider.Name,
		Terminal:           term,
		WorkingDir:         t.TempDir(),
		TimeoutSeconds:     3,
		InitialDelay:       time.Millisecond,
		CompletionDetector: &stubCompletionDetector{completed: true},
		HookMode:           true,
		SessionID:          participantSessionID,
	}

	resp := runJudgeRound(context.Background(), cfg, nil, participantSession, []ProviderResponse{
		{Provider: "claude", Output: "candidate answer"},
	}, 1)

	require.NotNil(t, resp)
	require.NotNil(t, resp.freshJudgeSession)
	evidence := resp.freshJudgeSession
	assert.True(t, evidence.Required)
	assert.True(t, evidence.Isolated)
	assert.True(t, evidence.Verified)
	assert.True(t, evidence.ParticipantsTerminated)
	assert.Equal(t, "isolated_hook_session", evidence.Mechanism)
	assert.Equal(t, testSessionFingerprint(participantSessionID), evidence.ParticipantSessionFingerprint)
	judgeSessionID := exportedJudgeSessionID(term.commands)
	require.NotEmpty(t, judgeSessionID)
	assert.Equal(t, testSessionFingerprint(judgeSessionID), evidence.JudgeSessionFingerprint)
	assert.NotEqual(t, evidence.ParticipantSessionFingerprint, evidence.JudgeSessionFingerprint)
	_, statErr := os.Stat(participantSession.Dir())
	assert.ErrorIs(t, statErr, os.ErrNotExist)

	result := FinalizeOrchestrationResult(&OrchestraResult{
		Strategy:          StrategyDebate,
		Responses:         []ProviderResponse{*resp},
		FreshJudgeSession: evidence,
	}, cfg)
	require.NotNil(t, result.RunReceipt)
	require.Equal(t, evidence, result.RunReceipt.FreshJudgeSession)
	wire, err := json.Marshal(result)
	require.NoError(t, err)
	assert.Contains(t, string(wire), `"fresh_judge_session"`)
	assert.NotContains(t, string(wire), participantSessionID)
	assert.NotContains(t, string(wire), judgeSessionID)
}

func TestRunJudgeRound_TerminatesParticipantsBeforeJudgeDispatch(t *testing.T) {
	isolateSurfaceTracker(t)
	provider, _ := newPaneBoundaryMarkerProvider(t, "claude", "")
	term := &judgeBoundaryTerminal{paneCommitTerminal: &paneCommitTerminal{
		splitID: "surface:9302",
		screen:  `{"recommendation":"judge after termination"}` + "\n❯\n",
	}}
	panes := []paneInfo{{paneID: participantJudgeBoundaryPane, provider: provider}}
	cfg := judgeBoundaryConfig(t, provider, term)

	resp := runJudgeRound(context.Background(), cfg, panes, nil, []ProviderResponse{
		{Provider: "claude", Output: "candidate answer"},
	}, 1)

	require.NotNil(t, resp)
	assert.Empty(t, resp.Error)
	require.NotNil(t, resp.freshJudgeSession)
	assert.True(t, resp.freshJudgeSession.ParticipantsTerminated)
	assert.Equal(t, "fresh_backend_execution", resp.freshJudgeSession.Mechanism)
	assert.Empty(t, panes[0].paneID, "successfully closed panes must not be closed again by deferred cleanup")
	events := strings.Join(term.events, ",")
	require.Contains(t, events, "pipe-stop:participant-pane")
	require.Contains(t, events, "close:participant-pane")
	require.Contains(t, events, "split:judge")
	assert.Less(t, strings.Index(events, "pipe-stop:participant-pane"), strings.Index(events, "close:participant-pane"))
	assert.Less(t, strings.Index(events, "close:participant-pane"), strings.Index(events, "split:judge"))
}

func TestRunJudgeRound_ParticipantCloseFailureBlocksJudgeDispatch(t *testing.T) {
	isolateSurfaceTracker(t)
	provider, _ := newPaneBoundaryMarkerProvider(t, "claude", "")
	term := &judgeBoundaryTerminal{
		paneCommitTerminal:  &paneCommitTerminal{splitID: "surface:9303"},
		participantCloseErr: errors.New("participant close failed"),
	}
	panes := []paneInfo{{paneID: participantJudgeBoundaryPane, provider: provider}}
	cfg := judgeBoundaryConfig(t, provider, term)

	resp := runJudgeRound(context.Background(), cfg, panes, nil, []ProviderResponse{
		{Provider: "claude", Output: "candidate answer"},
	}, 1)

	require.NotNil(t, resp)
	assert.Contains(t, resp.Error, "participant pane termination failed")
	assert.Equal(t, TerminalBlocked, resp.TerminalState)
	assert.Equal(t, noneBackendMarker, resp.ExecutedBackend)
	require.NotNil(t, resp.freshJudgeSession)
	assert.False(t, resp.freshJudgeSession.Isolated)
	assert.False(t, resp.freshJudgeSession.Verified)
	assert.False(t, resp.freshJudgeSession.ParticipantsTerminated)
	assert.Contains(t, resp.freshJudgeSession.Reason, "participant pane termination failed")
	assert.NotContains(t, term.events, "split:judge")
	assert.Equal(t, participantJudgeBoundaryPane, panes[0].paneID,
		"failed close must remain owned by deferred cleanup")

	result := FinalizeOrchestrationResult(&OrchestraResult{
		Strategy: StrategyDebate,
		FailedProviders: []FailedProvider{{
			Name: provider.Name, Role: "judge", Error: resp.Error,
		}},
		FreshJudgeSession: resp.freshJudgeSession,
	}, cfg)
	require.NotNil(t, result.RunReceipt)
	require.NotNil(t, result.RunReceipt.FreshJudgeSession)
	assert.False(t, result.RunReceipt.FreshJudgeSession.ParticipantsTerminated)
	wire, err := json.Marshal(result.RunReceipt)
	require.NoError(t, err)
	assert.Contains(t, string(wire), `"participant_session_fingerprint":""`)
	assert.Contains(t, string(wire), `"judge_session_fingerprint":""`)
}

func TestRunJudgeRound_ParticipantPipeStopFailureStillDispatchesJudge(t *testing.T) {
	isolateSurfaceTracker(t)
	provider, _ := newPaneBoundaryMarkerProvider(t, "claude", "")
	term := &judgeBoundaryTerminal{
		paneCommitTerminal: &paneCommitTerminal{
			splitID: "surface:9304",
			screen:  `{"recommendation":"judge after pane close"}` + "\n❯\n",
		},
		participantPipeStopErr: errors.New("participant pipe already inactive"),
	}
	panes := []paneInfo{{paneID: participantJudgeBoundaryPane, provider: provider}}
	cfg := judgeBoundaryConfig(t, provider, term)

	resp := runJudgeRound(context.Background(), cfg, panes, nil, []ProviderResponse{
		{Provider: "claude", Output: "candidate answer"},
	}, 1)

	require.NotNil(t, resp)
	assert.Empty(t, resp.Error)
	require.NotNil(t, resp.freshJudgeSession)
	assert.True(t, resp.freshJudgeSession.ParticipantsTerminated)
	assert.Empty(t, panes[0].paneID)
	events := strings.Join(term.events, ",")
	require.Contains(t, events, "pipe-stop:participant-pane")
	require.Contains(t, events, "close:participant-pane")
	require.Contains(t, events, "split:judge")
	assert.Less(t, strings.Index(events, "close:participant-pane"), strings.Index(events, "split:judge"))
}

func TestRunJudgeRound_EarlyDeadlinePreservesConcreteCause(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	cfg := OrchestraConfig{
		Providers:      []ProviderConfig{{Name: "claude", Binary: "echo"}},
		Strategy:       StrategyDebate,
		JudgeProvider:  "claude",
		TimeoutSeconds: 60,
	}

	resp := runJudgeRound(ctx, cfg, nil, nil, []ProviderResponse{
		{Provider: "claude", Output: "candidate answer"},
	}, 2)

	require.NotNil(t, resp)
	assert.Equal(t, context.DeadlineExceeded.Error(), resp.Error)
	assert.Equal(t, "judge", resp.Role)
	assert.Equal(t, 3, resp.Attempt)
	assert.Equal(t, TerminalBlocked, resp.TerminalState)
	require.NotNil(t, resp.freshJudgeSession)
	assert.True(t, resp.freshJudgeSession.ParticipantsTerminated)
	assert.False(t, resp.freshJudgeSession.Isolated)
}

func judgeBoundaryConfig(t *testing.T, provider ProviderConfig, term terminal.Terminal) OrchestraConfig {
	t.Helper()
	return OrchestraConfig{
		Providers:          []ProviderConfig{provider},
		Strategy:           StrategyDebate,
		JudgeProvider:      provider.Name,
		Terminal:           term,
		WorkingDir:         t.TempDir(),
		TimeoutSeconds:     3,
		InitialDelay:       time.Millisecond,
		CompletionDetector: &stubCompletionDetector{completed: true},
	}
}

func exportedJudgeSessionID(commands []string) string {
	const prefix = "export AUTOPUS_SESSION_ID="
	for _, command := range commands {
		if !strings.HasPrefix(command, prefix) {
			continue
		}
		fields := strings.Fields(strings.TrimPrefix(command, prefix))
		if len(fields) > 0 {
			return fields[0]
		}
	}
	return ""
}

func testSessionFingerprint(sessionID string) string {
	sum := sha256.Sum256([]byte(sessionID))
	return "sha256:" + hex.EncodeToString(sum[:])
}
