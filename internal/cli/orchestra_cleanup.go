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
func runOrchestraCleanup(ctx context.Context, sessionID string) error {
	session, err := orchestra.LoadSession(sessionID)
	if err != nil {
		return fmt.Errorf("load session: %w", err)
	}

	term := terminal.DetectTerminal()
	if term == nil {
		return fmt.Errorf("no terminal multiplexer detected")
	}

	// Kill each pane referenced by the session.
	killed := 0
	for provider, paneID := range session.Panes {
		if err := term.Close(ctx, paneID); err != nil {
			fmt.Fprintf(os.Stderr, "[cleanup] %s (pane %s) close failed: %v\n", provider, paneID, err)
			continue
		}
		killed++
		fmt.Fprintf(os.Stderr, "[cleanup] %s (pane %s) closed\n", provider, paneID)
	}

	// Remove the session persistence file.
	if err := orchestra.RemoveSession(sessionID); err != nil {
		return fmt.Errorf("remove session file: %w", err)
	}

	fmt.Fprintf(os.Stderr, "[cleanup] session %s removed (%d/%d panes closed)\n",
		sessionID, killed, len(session.Panes))
	return nil
}
