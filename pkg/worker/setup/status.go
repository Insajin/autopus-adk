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
	Configured          bool   `json:"configured"`
	AuthValid           bool   `json:"auth_valid"`
	DaemonRunning       bool   `json:"daemon_running"`
	WorkspaceID         string `json:"workspace_id"`
	BackendURL          string `json:"backend_url"`
	AuthType            string `json:"auth_type"` // "jwt", "api_key", or "none"
	CredentialBackend   string `json:"credential_backend"`
	SecureStorageReady  bool   `json:"secure_storage_ready"`
	DesktopSessionReady bool   `json:"desktop_session_ready"`
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
	snapshot, err := loadCredentialSnapshot()
	if err != nil {
		return nil, err
	}
	return snapshot.creds, nil
}

// checkAuthValidity determines auth_valid and auth_type from credentials.
func checkAuthValidity() (authValid bool, authType string) {
	snapshot, err := loadCredentialSnapshot()
	if err != nil {
		return false, "none"
	}
	return authStateFromCredentials(snapshot.creds)
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

func detectCredentialBackend() (backend string, secure bool) {
	snapshot, err := loadCredentialSnapshot()
	if err != nil {
		if data, readErr := os.ReadFile(DefaultCredentialsPath()); readErr == nil && len(data) > 0 {
			return "plaintext_file", false
		}
		return "none", false
	}
	return snapshot.backend, snapshot.secure
}

// CollectStatus reads config files and daemon state to produce a WorkerStatus.
func CollectStatus() WorkerStatus {
	snapshot, _ := loadCredentialSnapshot()
	return collectStatusFromSnapshot(snapshot)
}

func collectStatusFromSnapshot(snapshot credentialSnapshot) WorkerStatus {
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
	status.AuthValid, status.AuthType = authStateFromCredentials(snapshot.creds)
	if snapshot.backend == "" {
		status.CredentialBackend = "none"
	} else {
		status.CredentialBackend = snapshot.backend
	}
	status.SecureStorageReady = snapshot.secure
	status.DesktopSessionReady = status.Configured &&
		status.AuthValid &&
		status.AuthType == "jwt" &&
		status.WorkspaceID != "" &&
		status.BackendURL != "" &&
		status.SecureStorageReady

	// Check daemon running state.
	status.DaemonRunning = checkDaemonRunning()

	return status
}
