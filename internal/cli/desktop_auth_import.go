package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/worker/setup"
)

type desktopAuthImportPayload struct {
	BackendURL   string `json:"backend_url"`
	WorkspaceID  string `json:"workspace_id"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresAt    string `json:"expires_at,omitempty"`
}

type desktopAuthImportResult struct {
	OK          bool   `json:"ok"`
	BackendURL  string `json:"backend_url"`
	WorkspaceID string `json:"workspace_id"`
}

var (
	desktopAuthSaveCredentials = setup.SaveCredentials
	desktopAuthSaveConfig      = setup.SaveWorkerConfig
	desktopAuthLoadConfig      = setup.LoadWorkerConfig
)

func newDesktopAuthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "auth",
		Short:  "Desktop auth helper surfaces",
		Hidden: true,
	}
	cmd.AddCommand(newDesktopAuthImportCmd())
	return cmd
}

func newDesktopAuthImportCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "import",
		Short:  "Persist desktop runtime auth payload from stdin JSON",
		Hidden: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			result, err := importDesktopAuthPayload(cmd.InOrStdin())
			if err != nil {
				return err
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(result)
		},
	}
}

func importDesktopAuthPayload(r io.Reader) (*desktopAuthImportResult, error) {
	var payload desktopAuthImportPayload
	if err := json.NewDecoder(r).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode desktop auth payload: %w", err)
	}
	if payload.BackendURL == "" {
		return nil, fmt.Errorf("backend_url is required")
	}
	if payload.WorkspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	if payload.AccessToken == "" {
		return nil, fmt.Errorf("access_token is required")
	}

	expiresAt := payload.ExpiresAt
	if expiresAt == "" {
		expiresAt = time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	}

	if err := desktopAuthSaveCredentials(map[string]any{
		"access_token":  payload.AccessToken,
		"refresh_token": payload.RefreshToken,
		"token_type":    "Bearer",
		"expires_at":    expiresAt,
	}); err != nil {
		return nil, fmt.Errorf("persist desktop credentials: %w", err)
	}

	cfg, err := desktopAuthLoadConfig()
	if err != nil {
		cfg = &setup.WorkerConfig{}
	}
	cfg.BackendURL = payload.BackendURL
	cfg.WorkspaceID = payload.WorkspaceID

	if err := desktopAuthSaveConfig(*cfg); err != nil {
		return nil, fmt.Errorf("persist desktop runtime config: %w", err)
	}

	return &desktopAuthImportResult{
		OK:          true,
		BackendURL:  cfg.BackendURL,
		WorkspaceID: cfg.WorkspaceID,
	}, nil
}
