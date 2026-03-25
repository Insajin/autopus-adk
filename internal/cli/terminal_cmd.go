// Package cli provides the terminal command group for pipeline integration.
package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/terminal"
)

// newTerminalCmd creates the `auto terminal` parent command with subcommands.
// @AX:NOTE [AUTO] DetectTerminal() is called once per handler invocation — no caching; acceptable for CLI but not for long-running services
func newTerminalCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "terminal",
		Short: "Terminal multiplexer operations",
		Long:  "Detect, create workspaces, split panes, send commands, and notify via terminal multiplexers.",
	}

	cmd.AddCommand(newTerminalDetectCmd())
	cmd.AddCommand(newTerminalWorkspaceCmd())
	cmd.AddCommand(newTerminalSplitCmd())
	cmd.AddCommand(newTerminalSendCmd())
	cmd.AddCommand(newTerminalNotifyCmd())

	return cmd
}

// newTerminalDetectCmd creates the `auto terminal detect` subcommand.
func newTerminalDetectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "detect",
		Short: "Detect terminal multiplexer",
		Long:  "Detect the best available terminal multiplexer and print its name.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			adapter := terminal.DetectTerminal()
			fmt.Fprintln(cmd.OutOrStdout(), adapter.Name())
			return nil
		},
	}
}

// newTerminalWorkspaceCmd creates the `auto terminal workspace` command group.
func newTerminalWorkspaceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workspace",
		Short: "Manage terminal workspaces",
	}

	cmd.AddCommand(newWorkspaceCreateCmd())
	cmd.AddCommand(newWorkspaceCloseCmd())

	return cmd
}

// newWorkspaceCreateCmd creates the `auto terminal workspace create <name>` subcommand.
func newWorkspaceCreateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "create <name>",
		Short: "Create a terminal workspace",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			adapter := terminal.DetectTerminal()
			ctx := context.Background()
			if err := adapter.CreateWorkspace(ctx, args[0]); err != nil {
				return fmt.Errorf("create workspace %q: %w", args[0], err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "workspace %q created (%s)\n", args[0], adapter.Name())
			return nil
		},
	}
}

// newWorkspaceCloseCmd creates the `auto terminal workspace close <name>` subcommand.
func newWorkspaceCloseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "close <name>",
		Short: "Close a terminal workspace",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			adapter := terminal.DetectTerminal()
			ctx := context.Background()
			if err := adapter.Close(ctx, args[0]); err != nil {
				return fmt.Errorf("close workspace %q: %w", args[0], err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "workspace %q closed (%s)\n", args[0], adapter.Name())
			return nil
		},
	}
}

// newTerminalSplitCmd creates the `auto terminal split <h|v>` subcommand.
func newTerminalSplitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "split <h|v>",
		Short: "Split the current pane",
		Long:  "Split the current terminal pane. Use 'h' for horizontal, 'v' for vertical.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := parseDirection(args[0])
			if err != nil {
				return err
			}
			adapter := terminal.DetectTerminal()
			ctx := context.Background()
			paneID, err := adapter.SplitPane(ctx, dir)
			if err != nil {
				return fmt.Errorf("split pane: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(paneID))
			return nil
		},
	}
}

// newTerminalSendCmd creates the `auto terminal send <pane-id> <command>` subcommand.
func newTerminalSendCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "send <pane-id> <command>",
		Short: "Send a command to a pane",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			adapter := terminal.DetectTerminal()
			ctx := context.Background()
			paneID := terminal.PaneID(args[0])
			if err := adapter.SendCommand(ctx, paneID, args[1]); err != nil {
				return fmt.Errorf("send command to pane %q: %w", args[0], err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "sent to %s\n", args[0])
			return nil
		},
	}
}

// newTerminalNotifyCmd creates the `auto terminal notify <message>` subcommand.
func newTerminalNotifyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "notify <message>",
		Short: "Display a notification",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			adapter := terminal.DetectTerminal()
			ctx := context.Background()
			if err := adapter.Notify(ctx, args[0]); err != nil {
				return fmt.Errorf("notify: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "notified")
			return nil
		},
	}
}

// parseDirection converts a string ("h" or "v") to a terminal.Direction.
func parseDirection(s string) (terminal.Direction, error) {
	switch s {
	case "h":
		return terminal.Horizontal, nil
	case "v":
		return terminal.Vertical, nil
	default:
		return 0, fmt.Errorf("invalid direction %q: use 'h' or 'v'", s)
	}
}
