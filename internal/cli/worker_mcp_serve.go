package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/worker/mcpserver"
	"github.com/insajin/autopus-adk/pkg/worker/setup"
)

func newWorkerMCPServerCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "mcp-server",
		Aliases: []string{"mcp-serve"},
		Hidden:  true,
		Short:   "Run the worker MCP server over stdio",
		Long:    "Compatibility shim for the legacy worker-owned MCP surface. Prefer `auto mcp server`.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWorkerMCPServe()
		},
	}
}

func runWorkerMCPServe() error {
	backendURL := defaultServerURL
	workspaceID := ""

	cfg, err := setup.LoadWorkerConfig()
	if err == nil {
		if cfg.BackendURL != "" {
			backendURL = cfg.BackendURL
		}
		workspaceID = cfg.WorkspaceID
	}

	authToken := ""
	token, err := setup.LoadAuthToken()
	if err == nil {
		authToken = token
	}

	srv := mcpserver.NewMCPServer(backendURL, authToken, workspaceID)
	if err := srv.Start(context.Background()); err != nil {
		return fmt.Errorf("worker mcp server: %w", err)
	}

	return nil
}
