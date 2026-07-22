package orchestra

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/terminal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var responseOnlyPathPattern = regexp.MustCompile(`response file: ([^ ]+?\.md)`)

type responseOnlyHookTerminal struct {
	mockTerminal
	responseMu     sync.Mutex
	responseWrites int
	maxResponses   int
	afterResponse  func(int)
}

func newResponseOnlyHookTerminal() *responseOnlyHookTerminal {
	return &responseOnlyHookTerminal{mockTerminal: mockTerminal{
		name: "cmux", readScreenOutput: "codex>\n",
	}}
}

func (t *responseOnlyHookTerminal) SplitPane(_ context.Context, dir terminal.Direction) (terminal.PaneID, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.splitPaneCalls = append(t.splitPaneCalls, dir)
	t.nextPaneID++
	id := terminal.PaneID(fmt.Sprintf("surface:%d", t.nextPaneID))
	t.createdPanes = append(t.createdPanes, id)
	return id, nil
}

func (t *responseOnlyHookTerminal) SendCommand(ctx context.Context, paneID terminal.PaneID, cmd string) error {
	if err := t.mockTerminal.SendCommand(ctx, paneID, cmd); err != nil {
		return err
	}
	return t.writeResponseOnly(cmd)
}

func (t *responseOnlyHookTerminal) SendLongText(ctx context.Context, paneID terminal.PaneID, text string) error {
	if err := t.mockTerminal.SendLongText(ctx, paneID, text); err != nil {
		return err
	}
	return t.writeResponseOnly(text)
}

func (t *responseOnlyHookTerminal) WorkspaceRef() (string, error) {
	return "workspace:1", nil
}

func (t *responseOnlyHookTerminal) WithWorkspaceRef(string) (terminal.Terminal, error) {
	return t, nil
}

func (t *responseOnlyHookTerminal) writeResponseOnly(text string) error {
	match := responseOnlyPathPattern.FindStringSubmatch(text)
	if len(match) != 2 {
		return nil
	}
	t.responseMu.Lock()
	if t.maxResponses > 0 && t.responseWrites >= t.maxResponses {
		t.responseMu.Unlock()
		return nil
	}
	t.responseWrites++
	writeNumber := t.responseWrites
	afterResponse := t.afterResponse
	t.responseMu.Unlock()
	output := fmt.Sprintf("response-only round %d", writeNumber)
	if err := os.WriteFile(match[1], []byte(markedResponse(output)), 0o600); err != nil {
		return err
	}
	if afterResponse != nil {
		afterResponse(writeNumber)
	}
	return nil
}

func (t *responseOnlyHookTerminal) writes() int {
	t.responseMu.Lock()
	defer t.responseMu.Unlock()
	return t.responseWrites
}

func TestRunPaneDebate_ResponseOnlyHookFallsBackToDirectPolling(t *testing.T) {
	isolateSurfaceTracker(t)
	t.Setenv("TMPDIR", t.TempDir())
	term := newResponseOnlyHookTerminal()
	cfg := responseOnlyHookDebateConfig(t, term, false)
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	result, err := runPaneDebate(ctx, cfg, 2, 5*time.Second, time.Now())

	require.NoError(t, err)
	require.Len(t, result.RoundHistory, 2)
	require.Len(t, result.RoundHistory[0], 1)
	require.Len(t, result.RoundHistory[1], 1)
	assert.Equal(t, "response-only round 1", result.RoundHistory[0][0].Output)
	assert.Equal(t, "response-only round 2", result.RoundHistory[1][0].Output)
	assert.Equal(t, 2, term.writes(), "round 2 must be delivered directly to the stable prompt")
}

func TestRunPaneDebate_ResponseOnlyHookYieldsWithoutAbortWaiter(t *testing.T) {
	isolateSurfaceTracker(t)
	t.Setenv("TMPDIR", t.TempDir())
	term := newResponseOnlyHookTerminal()
	cfg := responseOnlyHookDebateConfig(t, term, true)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	result, err := runPaneDebate(ctx, cfg, 2, 3*time.Second, time.Now())

	require.NoError(t, err)
	require.NotNil(t, result.Yield)
	t.Cleanup(func() { _ = RemoveSession(result.Yield.SessionID) })
	assert.Equal(t, "response-only round 1", result.RoundHistory[0][0].Output)
	assert.Equal(t, 1, term.writes())
	assert.Empty(t, term.closeCalls, "the durable yield session must own the pane")
}

func TestCollectRoundHookResults_DoneProvenanceKeepsHookActive(t *testing.T) {
	session, err := NewHookSession("hook-done-health-" + NewSessionID())
	require.NoError(t, err)
	defer session.Cleanup()
	provider := ProviderConfig{Name: "codex", ExecutionTimeout: time.Second}
	responsePath := t.TempDir() + "/response.md"
	writeMarkedResponse(t, responsePath, "normal hook response")
	require.NoError(t, session.writeArtifact(RoundSignalName("codex", 1, "done"), nil, 0o600))

	responses := collectRoundHookResults(
		context.Background(), OrchestraConfig{Providers: []ProviderConfig{provider}},
		session, 1, []paneInfo{{provider: provider, responseFile: responsePath}},
	)

	require.Len(t, responses, 1)
	assert.Equal(t, "normal hook response", responses[0].Output)
	assert.True(t, session.HasHook("codex"), "done provenance must retain the normal hook contract")
}

func TestCollectRoundHookResults_NonCodexResponseBeforeDoneKeepsHookActive(t *testing.T) {
	for _, providerName := range []string{"claude", "gemini"} {
		t.Run(providerName, func(t *testing.T) {
			session, err := NewHookSession("non-codex-health-" + providerName + "-" + NewSessionID())
			require.NoError(t, err)
			defer session.Cleanup()
			provider := ProviderConfig{Name: providerName, ExecutionTimeout: time.Second}
			responsePath := t.TempDir() + "/response.md"
			writeMarkedResponse(t, responsePath, "response before done")
			doneWritten := make(chan error, 1)
			go func() {
				time.Sleep(75 * time.Millisecond)
				doneWritten <- session.writeArtifact(RoundSignalName(providerName, 1, "done"), nil, 0o600)
			}()

			responses := collectRoundHookResults(
				context.Background(), OrchestraConfig{Providers: []ProviderConfig{provider}},
				session, 1, []paneInfo{{provider: provider, responseFile: responsePath}},
			)

			require.NoError(t, <-doneWritten)
			require.Len(t, responses, 1)
			assert.Equal(t, "response before done", responses[0].Output)
			assert.True(t, session.HasHook(providerName),
				"non-Codex response-first ordering must retain the normal hook contract")
		})
	}
}

func responseOnlyHookDebateConfig(t *testing.T, term terminal.Terminal, yield bool) OrchestraConfig {
	t.Helper()
	provider := ProviderConfig{
		Name: "codex", Binary: "codex", InteractiveInput: "args", PromptViaArgs: true,
		ExecutionTimeout: 3 * time.Second,
	}
	return OrchestraConfig{
		Providers: []ProviderConfig{provider}, Strategy: StrategyDebate,
		Prompt: "prove response-only hook fallback", TimeoutSeconds: 4,
		Terminal: term, Interactive: true, HookMode: true,
		SessionID: "response-only-" + NewSessionID(), YieldRounds: yield,
		NoJudge: true, InitialDelay: time.Millisecond, WorkingDir: t.TempDir(),
	}
}
