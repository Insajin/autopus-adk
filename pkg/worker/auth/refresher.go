// Package auth provides token lifecycle management for autopus workers.
package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Credentials holds the authentication tokens.
type Credentials struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	Email        string    `json:"email,omitempty"`
	Workspace    string    `json:"workspace,omitempty"`
}

// TokenRefresher monitors token expiry and auto-refreshes.
type TokenRefresher struct {
	backendURL      string
	credentialsPath string
	onReauthNeeded  func()
	onTokenRefresh  func(newToken string)
	client          *http.Client
	mu              sync.RWMutex
	creds           *Credentials
}

// NewTokenRefresher creates a refresher that watches the given credentials file
// and refreshes tokens via backendURL before they expire.
func NewTokenRefresher(
	backendURL, credentialsPath string,
	onReauthNeeded func(),
	onTokenRefresh func(string),
) *TokenRefresher {
	return &TokenRefresher{
		backendURL:      backendURL,
		credentialsPath: credentialsPath,
		onReauthNeeded:  onReauthNeeded,
		onTokenRefresh:  onTokenRefresh,
		client:          &http.Client{Timeout: 10 * time.Second},
	}
}

// Start runs the background token-check loop. It blocks until ctx is cancelled.
func (r *TokenRefresher) Start(ctx context.Context) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	// Initial check immediately.
	r.checkAndRefresh(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.checkAndRefresh(ctx)
		}
	}
}

// LoadCredentials reads and parses the credentials file from disk.
func (r *TokenRefresher) LoadCredentials() (*Credentials, error) {
	data, err := os.ReadFile(r.credentialsPath)
	if err != nil {
		return nil, fmt.Errorf("read credentials: %w", err)
	}
	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("parse credentials: %w", err)
	}
	r.mu.Lock()
	r.creds = &creds
	r.mu.Unlock()
	return &creds, nil
}

// SaveCredentials writes credentials to disk atomically.
func (r *TokenRefresher) SaveCredentials(creds *Credentials) error {
	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal credentials: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(r.credentialsPath), 0o700); err != nil {
		return fmt.Errorf("create credentials dir: %w", err)
	}
	tmp := r.credentialsPath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write temp credentials: %w", err)
	}
	if err := os.Rename(tmp, r.credentialsPath); err != nil {
		return fmt.Errorf("rename credentials: %w", err)
	}
	r.mu.Lock()
	r.creds = creds
	r.mu.Unlock()
	return nil
}

func (r *TokenRefresher) checkAndRefresh(ctx context.Context) {
	creds, err := r.LoadCredentials()
	if err != nil {
		return
	}
	// Refresh 5 minutes before expiry.
	if time.Until(creds.ExpiresAt) > 5*time.Minute {
		return
	}
	if r.doRefresh(ctx, creds) {
		return
	}
	// Retry once: reload from disk in case another process refreshed.
	creds, err = r.LoadCredentials()
	if err != nil {
		r.onReauthNeeded()
		return
	}
	if time.Until(creds.ExpiresAt) > 5*time.Minute {
		return // Another process already refreshed.
	}
	if !r.doRefresh(ctx, creds) {
		r.onReauthNeeded()
	}
}

// doRefresh attempts a single token refresh. Returns true on success.
func (r *TokenRefresher) doRefresh(ctx context.Context, creds *Credentials) bool {
	body, _ := json.Marshal(map[string]string{
		"refresh_token": creds.RefreshToken,
	})
	// Use cli-refresh endpoint — it accepts refresh_token in the body
	// (standard /auth/refresh reads from httpOnly cookies, which CLI clients cannot use).
	url := r.backendURL + "/api/v1/auth/cli-refresh"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return false
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false
	}

	// cli-refresh returns { success, data: { access_token, refresh_token, expires_in } }.
	var wrapper struct {
		Success bool `json:"success"`
		Data    struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
			ExpiresIn    int64  `json:"expires_in"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&wrapper); err != nil || !wrapper.Success {
		return false
	}

	newCreds := &Credentials{
		AccessToken:  wrapper.Data.AccessToken,
		RefreshToken: wrapper.Data.RefreshToken,
		Email:        creds.Email,
		Workspace:    creds.Workspace,
	}
	if wrapper.Data.ExpiresIn > 0 {
		newCreds.ExpiresAt = time.Now().Add(time.Duration(wrapper.Data.ExpiresIn) * time.Second)
	}
	if err := r.SaveCredentials(newCreds); err != nil {
		return false
	}
	if r.onTokenRefresh != nil {
		r.onTokenRefresh(newCreds.AccessToken)
	}
	return true
}
