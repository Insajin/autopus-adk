package cli

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/worker/mcpserver"
	"github.com/insajin/autopus-adk/pkg/worker/setup"
)

func runRuntimeMCPServe(cmd *cobra.Command) error {
	if err := delegateRuntimeHelperStream(cmd, []string{"mcp", "server"}); err != nil {
		if !errors.Is(err, errRuntimeHelperNotFound) {
			return err
		}
		return runEmbeddedMCPServer(cmd)
	}
	return nil
}

func runEmbeddedMCPServer(cmd *cobra.Command) error {
	var backendURL, workspaceID string
	if cfg, err := setup.LoadWorkerConfig(); err == nil && cfg != nil {
		backendURL = cfg.BackendURL
		workspaceID = cfg.WorkspaceID
	}
	token, _ := setup.LoadAuthToken()

	srv := mcpserver.NewMCPServer(backendURL, token, workspaceID)
	srv.SetIO(cmd.InOrStdin(), cmd.OutOrStdout())
	return srv.Start(cmd.Context())
}
