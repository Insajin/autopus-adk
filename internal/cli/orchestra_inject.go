package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/orchestra"
	"github.com/insajin/autopus-adk/pkg/terminal"
)

// newOrchestraInjectCmd creates the "orchestra inject" subcommand.
// Sends a prompt to a specific provider's pane in an existing session.
func newOrchestraInjectCmd() *cobra.Command {
	var (
		sessionID string
		provider  string
	)

	cmd := &cobra.Command{
		Use:   "inject <prompt>",
		Short: "세션의 특정 pane에 프롬프트를 주입한다",
		Long:  "yield-rounds로 생성된 세션의 프로바이더 pane에 SendLongText로 프롬프트를 전송합니다.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if sessionID == "" {
				return fmt.Errorf("--session-id is required")
			}
			if provider == "" {
				return fmt.Errorf("--provider is required")
			}
			return runOrchestraInject(cmd, sessionID, provider, args[0])
		},
	}

	cmd.Flags().StringVar(&sessionID, "session-id", "", "세션 ID")
	cmd.Flags().StringVar(&provider, "provider", "", "프롬프트를 보낼 프로바이더 이름")
	_ = cmd.MarkFlagRequired("session-id")
	_ = cmd.MarkFlagRequired("provider")

	return cmd
}

// runOrchestraInject loads a session, finds the provider's pane, and sends the prompt.
func runOrchestraInject(cmd *cobra.Command, sessionID, provider, prompt string) error {
	session, err := orchestra.LoadSession(sessionID)
	if err != nil {
		return fmt.Errorf("session %s not found: %w", sessionID, err)
	}

	paneID, ok := session.Panes[provider]
	if !ok {
		available := make([]string, 0, len(session.Panes))
		for name := range session.Panes {
			available = append(available, name)
		}
		return fmt.Errorf("provider %q not found in session (available: %v)", provider, available)
	}

	term := terminal.DetectTerminal()
	if term == nil {
		return fmt.Errorf("no terminal multiplexer detected — inject requires cmux or tmux")
	}

	ctx := cmd.Context()

	// Send the prompt via SendLongText
	if err := term.SendLongText(ctx, terminal.PaneID(paneID), prompt); err != nil {
		return fmt.Errorf("SendLongText to %s failed: %w", provider, err)
	}

	// Send Enter to submit the prompt
	time.Sleep(500 * time.Millisecond)
	if err := term.SendCommand(ctx, terminal.PaneID(paneID), "\n"); err != nil {
		fmt.Fprintf(os.Stderr, "[inject] SendCommand (Enter) failed: %v — prompt may need manual submit\n", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Injected prompt to %s (pane %s)\n", provider, paneID)
	return nil
}
