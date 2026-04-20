package setup

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

// WorkerStatus holds machine-readable worker readiness state.
type WorkerStatus struct {
	Configured    bool   `json:"configured"`
	AuthValid     bool   `json:"auth_valid"`
	DaemonRunning bool   `json:"daemon_running"`
	WorkspaceID   string `json:"workspace_id"`
	BackendURL    string `json:"backend_url"`
	AuthType      string `json:"auth_type"` // "jwt", "api_key", or "none"
}

// rawCredentials is used for flexible JSON parsing of both credential formats.
type rawCredentials struct {
	// JWT format fields
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    string `json:"expires_at"` // RFC3339 or ISO8601 string — lenient parsing
	// API key format fields
	APIKey   string `json:"api_key"`
	AuthType string `json:"auth_type"`
	// Shared fields
	Workspace string `json:"workspace"`
}

// DefaultCredentialsPath returns the path to ~/.config/autopus/credentials.json.
func DefaultCredentialsPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", "credentials.json")
	}
	return filepath.Join(home, ".config", "autopus", "credentials.json")
}

// loadRawCredentials reads and parses credentials.json without strict type constraints.
func loadRawCredentials() (*rawCredentials, error) {
	return loadRawCredentialsFromPath("")
}

// checkAuthValidity determines auth_valid and auth_type from credentials.
func checkAuthValidity() (authValid bool, authType string) {
	creds, err := loadRawCredentials()
	if err != nil {
		return false, "none"
	}

	// API key format: has api_key field (does not expire client-side).
	if creds.APIKey != "" {
		return true, "api_key"
	}

	// JWT format: has access_token field. Valid if ExpiresAt is empty or after now+5min.
	if creds.AccessToken != "" {
		if creds.ExpiresAt == "" {
			return true, "jwt"
		}
		expiry, err := time.Parse(time.RFC3339, creds.ExpiresAt)
		if err != nil {
			// Best-effort: treat unparseable expiry as valid (let server reject if expired).
			return true, "jwt"
		}
		return time.Until(expiry) > 5*time.Minute, "jwt"
	}

	return false, "none"
}

// checkDaemonRunning probes the OS daemon manager with a 3-second timeout.
func checkDaemonRunning() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "darwin" {
		cmd = exec.CommandContext(ctx, "launchctl", "list", "co.autopus.worker")
	} else {
		cmd = exec.CommandContext(ctx, "systemctl", "--user", "is-active", "autopus-worker.service")
	}

	return cmd.Run() == nil
}

// CollectStatus reads config files and daemon state to produce a WorkerStatus.
func CollectStatus() WorkerStatus {
	status := WorkerStatus{}

	// Check configuration.
	configPath := DefaultWorkerConfigPath()
	if _, err := os.Stat(configPath); err == nil {
		cfg, err := LoadWorkerConfig()
		if err == nil {
			status.Configured = true
			status.WorkspaceID = cfg.WorkspaceID
			status.BackendURL = cfg.BackendURL
		}
	}

	// Check auth validity.
	status.AuthValid, status.AuthType = checkAuthValidity()

	// Check daemon running state.
	status.DaemonRunning = checkDaemonRunning()

	return status
}
