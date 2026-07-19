package orchestra

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollectResponse_UsesHookResultWhenResponseFileMissing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		round int
	}{
		{name: "unscoped attempt", round: 0},
		{name: "round-scoped attempt", round: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			session := testHookSessionAt(t.TempDir())
			writePaneHookResult(t, session, "codex", tt.round, HookResult{
				Output:   `{"verdict":"PASS","summary":"hook result","findings":[]}`,
				ExitCode: 0,
			})

			term := newCmuxMock()
			term.readScreenOutput = `{"verdict":"REVISE","summary":"screen must not win","findings":[]}`
			backend := NewInteractivePaneBackend(OrchestraConfig{Terminal: term})

			response := backend.collectResponse(context.Background(), ProviderRequest{
				Provider: "codex",
				Role:     "reviewer",
				Round:    tt.round,
			}, paneInfo{
				paneID:       "pane-1",
				role:         "reviewer",
				responseFile: filepath.Join(t.TempDir(), "missing-response.md"),
			}, false, session)

			require.NotNil(t, response)
			assert.Equal(t, "codex", response.Provider)
			assert.Contains(t, response.Output, "hook result")
			assert.False(t, response.TimedOut)
			assert.False(t, response.EmptyOutput)
			assert.Equal(t, paneBackendName, response.ExecutedBackend)
			assert.Zero(t, term.readScreenCalls, "a valid hook result must avoid screen scraping")
		})
	}
}

func TestCollectResponse_ResponseFilePrecedesHookResult(t *testing.T) {
	t.Parallel()

	session := testHookSessionAt(t.TempDir())
	writePaneHookResult(t, session, "codex", 3, HookResult{
		Output:   `{"verdict":"REVISE","summary":"hook must not win","findings":[]}`,
		ExitCode: 0,
	})

	responsePath := filepath.Join(t.TempDir(), "response.md")
	responseBody := responseBeginMarker + "\n" +
		`{"verdict":"PASS","summary":"response file wins","findings":[]}` + "\n" +
		responseEndMarker + "\n"
	require.NoError(t, os.WriteFile(responsePath, []byte(responseBody), 0o600))

	term := newCmuxMock()
	term.readScreenOutput = `{"verdict":"REVISE","summary":"screen must not win","findings":[]}`
	backend := NewInteractivePaneBackend(OrchestraConfig{Terminal: term})

	response := backend.collectResponse(context.Background(), ProviderRequest{
		Provider: "codex",
		Role:     "reviewer",
		Round:    3,
	}, paneInfo{
		paneID:       "pane-1",
		role:         "reviewer",
		responseFile: responsePath,
	}, false, session)

	require.NotNil(t, response)
	assert.Contains(t, response.Output, "response file wins")
	assert.NotContains(t, response.Output, "hook must not win")
	assert.False(t, response.TimedOut)
	assert.False(t, response.EmptyOutput)
	assert.Equal(t, paneBackendName, response.ExecutedBackend)
	assert.Zero(t, term.readScreenCalls, "response-file collection must remain the highest priority")
}

func TestCollectResponse_EmptyOrMalformedHookResultUsesFinalScreenFallback(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name    string
		content string
	}{
		{name: "empty output", content: `{"output":"  ","exit_code":0}`},
		{name: "malformed result", content: `{broken`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			session := testHookSessionAt(t.TempDir())
			resultPath := filepath.Join(session.Dir(), "codex-result.json")
			require.NoError(t, os.WriteFile(resultPath, []byte(tt.content), 0o600))
			term := newCmuxMock()
			term.readScreenOutput = `{"verdict":"PASS","summary":"screen fallback","findings":[]}`
			backend := NewInteractivePaneBackend(OrchestraConfig{Terminal: term})

			response := backend.collectResponse(context.Background(), ProviderRequest{
				Provider: "codex",
				Role:     "reviewer",
			}, paneInfo{
				paneID:       "pane-1",
				role:         "reviewer",
				responseFile: filepath.Join(t.TempDir(), "missing-response.md"),
			}, false, session)

			require.NotNil(t, response)
			assert.False(t, response.EmptyOutput)
			assert.Contains(t, response.Output, "screen fallback")
			assert.Empty(t, response.Error)
			assert.Equal(t, 1, term.readScreenCalls)
		})
	}
}

func testHookSessionAt(dir string) *HookSession {
	return &HookSession{
		sessionID:     "pane-hook-collect-test",
		sessionDir:    dir,
		hookProviders: DefaultHookProviders(),
	}
}

func writePaneHookResult(t *testing.T, session *HookSession, provider string, round int, result HookResult) {
	t.Helper()

	name := sanitizeProviderName(provider) + "-result.json"
	if round > 0 {
		name = RoundSignalName(provider, round, "result.json")
	}
	data, err := json.Marshal(result)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(session.Dir(), name), data, 0o600))
}
