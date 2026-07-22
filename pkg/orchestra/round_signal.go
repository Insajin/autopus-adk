package orchestra

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/insajin/autopus-adk/pkg/terminal"
)

// RoundSignalName generates a round-scoped signal filename.
// Format: "{provider}-round{N}-{suffix}" (e.g., "claude-round2-done").
func RoundSignalName(provider string, round int, suffix string) string {
	return fmt.Sprintf("%s-round%d-%s", sanitizeProviderName(provider), round, suffix)
}

// CleanRoundSignals removes signal files for the given round,
// preserving result files. Cleans done, input.json, ready, and abort files.
func CleanRoundSignals(session *HookSession, round int) {
	patterns := []string{
		fmt.Sprintf("*-round%d-done", round),
		fmt.Sprintf("*-round%d-input.json", round),
		fmt.Sprintf("*-round%d-ready", round),
		fmt.Sprintf("*-round%d-abort", round),
	}
	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(session.Dir(), pattern))
		if err != nil {
			continue
		}
		for _, m := range matches {
			_ = os.Remove(m)
		}
	}
}

// SetRoundEnv sets the AUTOPUS_ROUND environment variable to the current round number.
// @AX:WARN [AUTO] global state mutation via os.Setenv — affects all goroutines; safe only when called from single-threaded debate loop
func SetRoundEnv(round int) {
	_ = os.Setenv("AUTOPUS_ROUND", fmt.Sprintf("%d", round))
}

// SendRoundEnvToPane sends "export AUTOPUS_ROUND=N" to the specified terminal pane.
func SendRoundEnvToPane(ctx context.Context, term terminal.Terminal, paneID terminal.PaneID, round int) error {
	cmd := fmt.Sprintf("export AUTOPUS_ROUND=%d", round)
	return term.SendCommand(ctx, paneID, cmd)
}

// SendSessionEnvToPane sends "export AUTOPUS_SESSION_ID=<sid>" to the specified
// terminal pane. This ensures the hook script (hook-claude-stop.sh) receives the
// session ID via the pane's shell environment, not just the orchestrator process env.
// IDs must use the canonical lowercase artifact form to prevent shell injection
// and case-insensitive filesystem aliases.
func SendSessionEnvToPane(ctx context.Context, term terminal.Terminal, paneID terminal.PaneID, sessionID string) error {
	if err := validateHookSessionID(sessionID); err != nil {
		return fmt.Errorf("SendSessionEnvToPane: invalid session ID: %w", err)
	}
	// AUTOPUS_SESSION_DIR mirrors NewHookSession's path (os.TempDir based) so the
	// provider's hooks write the ready/done signals to the exact directory the
	// orchestrator watches. Without it the hooks hardcode /tmp/autopus and diverge
	// from os.TempDir() whenever $TMPDIR is not /tmp (e.g. sandboxed runners),
	// silently no-op-ing every hook (the dir-existence guard fails).
	dir := filepath.Join(os.TempDir(), "autopus", sessionID)
	cmd := fmt.Sprintf("export AUTOPUS_SESSION_ID=%s AUTOPUS_SESSION_DIR=%s", sessionID, shellQuote(dir))
	return term.SendCommand(ctx, paneID, cmd)
}
