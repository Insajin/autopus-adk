package orchestra

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newPaneBoundaryMarkerProvider creates a harmless local executable whose only
// observable side effect is a marker file. If the marker appears, a subprocess
// path ran after the pane-provisioning commit point.
func newPaneBoundaryMarkerProvider(t *testing.T, name, binaryName string) (ProviderConfig, string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("marker fixture requires a POSIX shell")
	}
	if binaryName == "" {
		binaryName = "provider-fixture"
	}
	dir := t.TempDir()
	binary := filepath.Join(dir, binaryName)
	marker := filepath.Join(dir, "subprocess-called")
	script := "#!/bin/sh\ncat >/dev/null\n: > \"$1\"\nprintf '%s\\n' '{\"recommendation\":\"subprocess fixture judge\"}'\n"
	require.NoError(t, os.WriteFile(binary, []byte(script), 0o700))
	return ProviderConfig{
		Name:         name,
		Binary:       binary,
		Args:         []string{marker},
		OutputFormat: "text",
	}, marker
}

func TestInteractivePaneBackend_PostSplitFailuresNeverExecuteSubprocess(t *testing.T) {
	tests := []struct {
		name              string
		binaryName        string
		providerName      string
		longTextErrAt     int
		commandErrAt      int
		screen            string
		startupTimeout    time.Duration
		invalidWorkingDir bool
		wantError         string
	}{
		{
			name:       "launch command build",
			binaryName: "agy", providerName: "gemini", invalidWorkingDir: true,
			wantError: "launch command error",
		},
		{
			name: "launch send", providerName: "claude", longTextErrAt: 1,
			wantError: "launch send error",
		},
		{
			name: "launch enter", providerName: "claude", commandErrAt: 1,
			wantError: "launch enter error",
		},
		{
			name: "ready timeout", providerName: "claude", screen: "still loading",
			startupTimeout: 10 * time.Millisecond, wantError: "session never became ready",
		},
		{
			name: "prompt send", providerName: "claude", screen: readyScreen, longTextErrAt: 2,
			startupTimeout: time.Second, wantError: "prompt send error",
		},
		{
			name: "prompt enter", providerName: "claude", screen: readyScreen, commandErrAt: 2,
			startupTimeout: time.Second, wantError: "prompt enter error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, marker := newPaneBoundaryMarkerProvider(t, tt.providerName, tt.binaryName)
			provider.StartupTimeout = tt.startupTimeout
			term := &paneCommitTerminal{
				longTextErrAt: tt.longTextErrAt,
				commandErrAt:  tt.commandErrAt,
				screen:        tt.screen,
			}
			workingDir := t.TempDir()
			if tt.invalidWorkingDir {
				workingDir = filepath.Join(t.TempDir(), "not-a-directory")
				require.NoError(t, os.WriteFile(workingDir, []byte("fixture"), 0o600))
			}

			backend := NewInteractivePaneBackend(OrchestraConfig{
				Terminal:   term,
				WorkingDir: workingDir,
			})
			resp, err := backend.Execute(context.Background(), ProviderRequest{
				Provider: provider.Name,
				Config:   provider,
				Prompt:   "review the provisioning boundary",
				Timeout:  3 * time.Second,
			})

			if assert.Error(t, err) {
				assert.Contains(t, err.Error(), tt.wantError)
			}
			if assert.NotNil(t, resp) {
				assert.Equal(t, paneBackendName, resp.ExecutedBackend)
			}
			assert.Equal(t, 1, term.splitCalls, "the pane must be committed before the failure")
			assert.Contains(t, term.closed, string(committedPaneID), "the committed pane must be cleaned up")
			_, statErr := os.Stat(marker)
			assert.ErrorIs(t, statErr, os.ErrNotExist, "post-split failure must not execute subprocess")
		})
	}
}

func TestInteractivePaneBackend_NonEmptySplitErrorCommitsPane(t *testing.T) {
	isolateSurfaceTracker(t)
	provider, marker := newPaneBoundaryMarkerProvider(t, "claude", "")
	term := &paneCommitTerminal{
		splitID:  committedPaneID,
		splitErr: errors.New("split reported a late error"),
	}
	backend := NewInteractivePaneBackend(OrchestraConfig{Terminal: term})

	resp, err := backend.Execute(context.Background(), ProviderRequest{
		Provider: provider.Name,
		Config:   provider,
		Prompt:   "do not cross the committed pane boundary",
		Timeout:  3 * time.Second,
	})

	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "SplitPane")
	}
	if assert.NotNil(t, resp) {
		assert.Equal(t, paneBackendName, resp.ExecutedBackend)
	}
	assert.Contains(t, term.closed, string(committedPaneID))
	_, statErr := os.Stat(marker)
	assert.ErrorIs(t, statErr, os.ErrNotExist, "a non-empty pane ID commits pane transport even with an error")
}

func TestInteractivePaneBackend_CompletionCollectionAndCleanupFailuresStayPane(t *testing.T) {
	isolateSurfaceTracker(t)
	provider, marker := newPaneBoundaryMarkerProvider(t, "claude", "")
	provider.InteractiveInput = "args"
	term := &paneCommitTerminal{
		readErr:  errors.New("injected collection read failure"),
		closeErr: errors.New("injected persistent close failure"),
	}
	backend := NewInteractivePaneBackend(OrchestraConfig{
		Terminal:           term,
		WorkingDir:         t.TempDir(),
		InitialDelay:       time.Millisecond,
		CompletionDetector: &stubCompletionDetector{completed: false},
	})

	// The budget must clear the two mandatory production sleeps inside Execute — the
	// promptRegisterDelay timer between paste and Enter plus the post-launch
	// registration sleep — before the completion wait can consume the rest of it.
	// A 300ms budget left only 100ms of slack for those 200ms of real sleeps, so a
	// loaded runner failed launch with "launch enter error: context deadline
	// exceeded" instead of reaching the TimedOut assertion below. 2s matches
	// TestExecute_S12_TimeoutDeterministic and keeps every assertion identical:
	// the stub detector never completes, so the request still times out.
	resp, err := backend.Execute(context.Background(), ProviderRequest{
		Provider: provider.Name,
		Config:   provider,
		Prompt:   "exercise terminal-only failure stages",
		Timeout:  2 * time.Second,
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, paneBackendName, resp.ExecutedBackend)
	assert.True(t, resp.TimedOut)
	assert.Len(t, term.closed, closePaneSurfaceAttempts, "cleanup close failures must stay bounded")
	_, statErr := os.Stat(marker)
	assert.ErrorIs(t, statErr, os.ErrNotExist, "completion, collection, and cleanup failures must not execute subprocess")
}
