package orchestra

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/insajin/autopus-adk/pkg/terminal"
)

// surfaceMock extends mockTerminal with per-pane ReadScreen error control.
type surfaceMock struct {
	mockTerminal
	stalePane map[terminal.PaneID]bool // panes that return ReadScreen error
}

func (m *surfaceMock) ReadScreen(ctx context.Context, paneID terminal.PaneID, opts terminal.ReadScreenOpts) (string, error) {
	if m.stalePane != nil && m.stalePane[paneID] {
		return "", fmt.Errorf("surface stale: %s", paneID)
	}
	return m.mockTerminal.ReadScreen(ctx, paneID, opts)
}

func TestNeedsSurfaceCheck(t *testing.T) {
	tests := []struct {
		name     string
		provider ProviderConfig
		want     bool
	}{
		{
			name:     "claude needs check",
			provider: ProviderConfig{Name: "claude", Binary: "claude"},
			want:     true,
		},
		{
			name:     "opencode needs check",
			provider: ProviderConfig{Name: "opencode", Binary: "opencode"},
			want:     true,
		},
		{
			name:     "gemini needs check",
			provider: ProviderConfig{Name: "gemini", Binary: "gemini"},
			want:     true,
		},
		{
			name:     "codex needs check",
			provider: ProviderConfig{Name: "codex", Binary: "codex"},
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := needsSurfaceCheck(tt.provider)
			if got != tt.want {
				t.Errorf("needsSurfaceCheck(%s) = %v, want %v", tt.provider.Binary, got, tt.want)
			}
		})
	}
}

func TestValidateSurface(t *testing.T) {
	ctx := context.Background()

	t.Run("valid surface", func(t *testing.T) {
		mock := newCmuxMock()
		if !validateSurface(ctx, mock, "pane-1") {
			t.Error("expected valid surface, got invalid")
		}
	})

	t.Run("stale surface", func(t *testing.T) {
		mock := &surfaceMock{
			mockTerminal: mockTerminal{name: "cmux"},
			stalePane:    map[terminal.PaneID]bool{"pane-1": true},
		}
		if validateSurface(ctx, mock, "pane-1") {
			t.Error("expected invalid surface, got valid")
		}
	})
}

func TestRecreatePane_Success(t *testing.T) {
	ctx := context.Background()
	mock := newCmuxMock()
	mock.readScreenOutput = "Ask anything"

	pi := paneInfo{
		paneID:     "old-pane",
		outputFile: t.TempDir() + "/old-output.txt",
		provider:   ProviderConfig{Name: "opencode", Binary: "opencode"},
	}

	cfg := OrchestraConfig{Terminal: mock}
	newPI, err := recreatePane(ctx, cfg, pi, 1)
	if err != nil {
		t.Fatalf("recreatePane failed: %v", err)
	}
	if newPI.paneID == pi.paneID {
		t.Error("expected new paneID, got same as old")
	}
	if newPI.skipWait {
		t.Error("expected skipWait=false after successful recreation")
	}
	if newPI.provider.Name != "opencode" {
		t.Errorf("expected provider opencode, got %s", newPI.provider.Name)
	}
	// Verify cleanup of old pane was called.
	if len(mock.closeCalls) < 1 {
		t.Error("expected Close call for old pane")
	}
}

func TestRecreatePane_SplitPaneError(t *testing.T) {
	ctx := context.Background()
	mock := newCmuxMock()
	mock.splitPaneErr = fmt.Errorf("no space for pane")

	pi := paneInfo{
		paneID:     "old-pane",
		outputFile: t.TempDir() + "/old-output.txt",
		provider:   ProviderConfig{Name: "opencode", Binary: "opencode"},
	}

	cfg := OrchestraConfig{Terminal: mock}
	_, err := recreatePane(ctx, cfg, pi, 1)
	if err == nil {
		t.Fatal("expected error from recreatePane when SplitPane fails")
	}
}

// TestRecreatePane_PipePaneStartError verifies that recreatePane succeeds even when
// PipePaneStart fails — pipe capture is non-fatal; the pane is still usable for
// interactive I/O. The outputFile should be empty to disable idle fallback.
func TestRecreatePane_PipePaneStartError(t *testing.T) {
	ctx := context.Background()
	mock := &pipePaneErrorMock{mockTerminal: mockTerminal{name: "cmux", readScreenOutput: "Ask anything"}}

	pi := paneInfo{
		paneID:     "old-pane",
		outputFile: t.TempDir() + "/old-output.txt",
		provider:   ProviderConfig{Name: "opencode", Binary: "opencode"},
	}

	cfg := OrchestraConfig{Terminal: mock}
	newPI, err := recreatePane(ctx, cfg, pi, 1)
	if err != nil {
		t.Fatalf("recreatePane should succeed even when PipePaneStart fails, got: %v", err)
	}
	if newPI.outputFile != "" {
		t.Errorf("expected empty outputFile when PipePaneStart fails, got: %s", newPI.outputFile)
	}
	if newPI.paneID == pi.paneID {
		t.Error("expected new paneID after recreation")
	}
}

// TestRecreatePane_LaunchError verifies that recreatePane returns an error when
// the CLI launch (SendLongText) fails on the new pane.
func TestRecreatePane_LaunchError(t *testing.T) {
	ctx := context.Background()
	mock := &sendLongTextErrorMock{mockTerminal: mockTerminal{name: "cmux"}}

	pi := paneInfo{
		paneID:     "old-pane",
		outputFile: t.TempDir() + "/old-output.txt",
		provider:   ProviderConfig{Name: "opencode", Binary: "opencode"},
	}

	cfg := OrchestraConfig{Terminal: mock}
	_, err := recreatePane(ctx, cfg, pi, 1)
	if err == nil {
		t.Fatal("expected error from recreatePane when SendLongText (launch) fails")
	}
	if !strings.Contains(err.Error(), "launch") {
		t.Errorf("expected 'launch' in error, got: %v", err)
	}
}

// TestRecreatePane_RoundEnvSet verifies that SendRoundEnvToPane is called with
// the correct round number when recreating an args-mode provider in round > 1.
func TestRecreatePane_RoundEnvSet(t *testing.T) {
	ctx := context.Background()
	mock := newCmuxMock()
	mock.readScreenOutput = "Ask anything"

	pi := paneInfo{
		paneID:     "old-pane",
		outputFile: t.TempDir() + "/old-output.txt",
		provider:   ProviderConfig{Name: "opencode", Binary: "opencode", InteractiveInput: "args"},
	}

	cfg := OrchestraConfig{Terminal: mock, Prompt: "test prompt"}
	_, err := recreatePane(ctx, cfg, pi, 3)
	if err != nil {
		t.Fatalf("recreatePane failed: %v", err)
	}

	// Verify SendRoundEnvToPane was called — it uses SendCommand with "export AUTOPUS_ROUND=3".
	mock.mu.Lock()
	defer mock.mu.Unlock()
	found := false
	for _, call := range mock.sendCommandCalls {
		if strings.Contains(call.Cmd, "AUTOPUS_ROUND=3") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected SendRoundEnvToPane call with round=3, not found in sendCommandCalls")
	}
}

// TestRecreatePane_ArgsProvider_NoPrompt verifies that args-mode providers are
// relaunched in REPL mode (without the original prompt) when recreated.
func TestRecreatePane_ArgsProvider_NoPrompt(t *testing.T) {
	ctx := context.Background()
	mock := newCmuxMock()
	mock.readScreenOutput = "Ask anything"

	pi := paneInfo{
		paneID:     "old-pane",
		outputFile: t.TempDir() + "/old-output.txt",
		provider:   ProviderConfig{Name: "opencode", Binary: "opencode", InteractiveInput: "args"},
	}

	cfg := OrchestraConfig{Terminal: mock, Prompt: "do not include this prompt"}
	_, err := recreatePane(ctx, cfg, pi, 2)
	if err != nil {
		t.Fatalf("recreatePane failed: %v", err)
	}

	// Verify the launch command (sent via SendLongText) does NOT contain the prompt.
	mock.mu.Lock()
	defer mock.mu.Unlock()
	for _, call := range mock.sendLongTextCalls {
		if strings.Contains(call.Text, "do not include this prompt") {
			t.Error("args provider should be relaunched WITHOUT the original prompt in round > 1")
		}
	}
}
