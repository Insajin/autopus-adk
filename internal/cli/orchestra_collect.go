package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/orchestra"
	"github.com/insajin/autopus-adk/pkg/terminal"
)

// CollectResult is the JSON output structure for the collect command.
type CollectResult struct {
	SessionID string                   `json:"session_id"`
	Round     int                      `json:"round"`
	Responses []CollectProviderResult  `json:"responses"`
}

// CollectProviderResult holds one provider's collected screen output.
type CollectProviderResult struct {
	Provider string `json:"provider"`
	Output   string `json:"output"`
	Error    string `json:"error,omitempty"`
}

// newOrchestraCollectCmd creates the "orchestra collect" subcommand.
// Loads a persisted session, reads each provider's pane screen, and outputs JSON.
func newOrchestraCollectCmd() *cobra.Command {
	var round int

	cmd := &cobra.Command{
		Use:   "collect <session-id>",
		Short: "Collect provider responses from a yield-rounds session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sessionID := args[0]

			session, err := orchestra.LoadSession(sessionID)
			if err != nil {
				return fmt.Errorf("session %q not found: %w", sessionID, err)
			}

			// Default to the latest round if not specified
			targetRound := round
			if targetRound <= 0 {
				targetRound = len(session.Rounds)
			}

			term := terminal.DetectTerminal()
			if term == nil {
				return fmt.Errorf("no terminal multiplexer detected — collect requires cmux or tmux")
			}

			ctx := cmd.Context()
			var responses []CollectProviderResult

			for _, p := range session.Providers {
				paneID, ok := session.Panes[p.Name]
				if !ok {
					responses = append(responses, CollectProviderResult{
						Provider: p.Name,
						Error:    "pane not found in session",
					})
					continue
				}

				screen, err := term.ReadScreen(ctx, terminal.PaneID(paneID), terminal.ReadScreenOpts{
					Scrollback: true,
				})
				if err != nil {
					responses = append(responses, CollectProviderResult{
						Provider: p.Name,
						Error:    fmt.Sprintf("ReadScreen failed: %v", err),
					})
					continue
				}

				responses = append(responses, CollectProviderResult{
					Provider: p.Name,
					Output:   screen,
				})
			}

			result := CollectResult{
				SessionID: sessionID,
				Round:     targetRound,
				Responses: responses,
			}

			data, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				return fmt.Errorf("marshal result: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(data))
			return nil
		},
	}

	cmd.Flags().IntVar(&round, "round", 0, "Round number to collect (0 = latest)")
	return cmd
}
