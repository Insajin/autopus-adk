package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/orchestra"
	"github.com/insajin/autopus-adk/pkg/terminal"
)

// newOrchestraCleanupCmd creates the cleanup subcommand for orchestra.
// Loads a persisted session, kills all panes, and removes the session file.
func newOrchestraCleanupCmd() *cobra.Command {
	var sessionID string

	cmd := &cobra.Command{
		Use:   "cleanup",
		Short: "yield-rounds 세션의 pane을 정리하고 세션 파일을 삭제한다",
		Long:  "지정된 세션 ID로 저장된 yield-rounds 세션의 모든 pane을 종료하고 세션 파일을 제거합니다.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if sessionID == "" {
				return fmt.Errorf("--session-id is required")
			}
			return runOrchestraCleanup(cmd.Context(), sessionID)
		},
	}

	cmd.Flags().StringVar(&sessionID, "session-id", "", "cleanup 대상 세션 ID (필수)")
	_ = cmd.MarkFlagRequired("session-id")

	return cmd
}

// runOrchestraCleanup loads the session, kills panes, and removes the session file.
// Idempotent: returns nil if session file is already missing.
func runOrchestraCleanup(ctx context.Context, sessionID string) error {
	session, err := orchestra.LoadSession(sessionID)
	if err != nil {
		// Session file missing — already cleaned or never created.
		fmt.Fprintf(os.Stderr, "[cleanup] session %s: %v (may already be cleaned)\n", sessionID, err)
		return nil
	}

	// Detect terminal for pane cleanup.
	term := terminal.DetectTerminal()

	// Kill each pane referenced by the session.
	killed := 0
	for provider, paneID := range session.Panes {
		if term != nil {
			if killErr := term.Close(ctx, paneID); killErr != nil {
				fmt.Fprintf(os.Stderr, "[cleanup] %s pane %s kill failed: %v\n", provider, paneID, killErr)
			} else {
				killed++
			}
		}
	}

	// Remove the session persistence file.
	if removeErr := orchestra.RemoveSession(sessionID); removeErr != nil {
		fmt.Fprintf(os.Stderr, "[cleanup] session file removal failed: %v\n", removeErr)
	}

	fmt.Fprintf(os.Stderr, "[cleanup] session %s: %d panes killed, session file removed\n",
		sessionID, killed)
	return nil
}
