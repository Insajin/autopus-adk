package setup

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// EnsureResult is the structured output from EnsureWorker.
type EnsureResult struct {
	Action string            `json:"action"` // "ready", "login_required", "starting_daemon", "error"
	Data   map[string]string `json:"data,omitempty"`
}

// EnsureWorker checks worker state and takes action to bring it to ready.
// Returns an EnsureResult describing the current action.
//
// Exit semantics (for CLI use):
//   - action="ready"           → exit 0
//   - action="login_required"  → exit 2 (human needed)
//   - action="error"           → exit 1
//   - action="starting_daemon" → exit 0
func EnsureWorker(ctx context.Context, backendURL, workspaceID string) (*EnsureResult, error) {
	if backendURL == "" {
		backendURL = "https://api.autopus.co"
	}

	status := CollectStatus()

	// Step 1: not configured → trigger device auth flow.
	if !status.Configured {
		return ensureDeviceAuth(ctx, backendURL)
	}

	// Step 2: configured but auth invalid → try refresh, else device auth.
	if !status.AuthValid {
		if !tryRefreshCredentials(ctx, backendURL) {
			return ensureDeviceAuth(ctx, backendURL)
		}
	}

	// Step 3: auth valid but daemon not running → install and start daemon.
	if !status.DaemonRunning {
		if err := ensureInstallDaemon(); err != nil {
			return &EnsureResult{
				Action: "error",
				Data:   map[string]string{"message": err.Error()},
			}, err
		}
		return &EnsureResult{Action: "starting_daemon"}, nil
	}

	// Step 4: all good.
	wsID := status.WorkspaceID
	if wsID == "" {
		wsID = workspaceID
	}
	return &EnsureResult{
		Action: "ready",
		Data:   map[string]string{"workspace_id": wsID},
	}, nil
}

// ensureDeviceAuth initiates PKCE device code flow and blocks until token obtained.
// It outputs login_required JSON immediately so callers see it before polling begins.
func ensureDeviceAuth(ctx context.Context, backendURL string) (*EnsureResult, error) {
	verifier, _, err := GeneratePKCE()
	if err != nil {
		return nil, fmt.Errorf("generate PKCE: %w", err)
	}

	dc, err := RequestDeviceCode(backendURL, verifier)
	if err != nil {
		return nil, fmt.Errorf("request device code: %w", err)
	}

	verifyURL := dc.VerificationURIComplete
	if verifyURL == "" {
		verifyURL = dc.VerificationURI
	}

	result := &EnsureResult{
		Action: "login_required",
		Data: map[string]string{
			"url":        verifyURL,
			"code":       dc.UserCode,
			"expires_in": fmt.Sprintf("%d", dc.ExpiresIn),
		},
	}

	// Output login_required immediately before blocking on poll.
	if out, err := json.Marshal(result); err == nil {
		fmt.Println(string(out))
	}

	tokenResp, err := PollForToken(ctx, backendURL, dc.DeviceCode, verifier, dc.Interval)
	if err != nil {
		// Context cancelled or expired — caller already saw login_required JSON.
		return result, nil
	}

	if err := saveTokenCredentials(tokenResp, backendURL); err != nil {
		return nil, fmt.Errorf("save credentials: %w", err)
	}

	// After successful auth, ensure daemon is running.
	if !checkDaemonRunning() {
		if err := ensureInstallDaemon(); err != nil {
			return &EnsureResult{
				Action: "error",
				Data:   map[string]string{"message": err.Error()},
			}, err
		}
		return &EnsureResult{Action: "starting_daemon"}, nil
	}

	return &EnsureResult{Action: "ready"}, nil
}

// tryRefreshCredentials attempts JWT refresh. Returns true on success.
func tryRefreshCredentials(ctx context.Context, backendURL string) bool {
	creds, err := loadRawCredentials()
	if err != nil || creds.RefreshToken == "" {
		return false
	}

	tokenResp, err := RefreshToken(ctx, backendURL, creds.RefreshToken)
	if err != nil {
		return false
	}

	return saveTokenCredentials(tokenResp, backendURL) == nil
}

// saveTokenCredentials persists a TokenResponse to credentials.json.
func saveTokenCredentials(tokenResp *TokenResponse, backendURL string) error {
	creds := map[string]any{
		"auth_type":     "jwt",
		"access_token":  tokenResp.AccessToken,
		"refresh_token": tokenResp.RefreshToken,
		"backend_url":   backendURL,
		"created_at":    time.Now().Format(time.RFC3339),
	}
	if tokenResp.ExpiresIn > 0 {
		creds["expires_at"] = time.Now().
			Add(time.Duration(tokenResp.ExpiresIn) * time.Second).
			Format(time.RFC3339)
	}
	return SaveCredentials(creds)
}

// ensureInstallDaemon resolves the current binary path and installs the daemon.
func ensureInstallDaemon() error {
	return installAndStartDaemon()
}
