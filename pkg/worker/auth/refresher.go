// Package auth provides token lifecycle management for autopus workers.
package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/insajin/autopus-adk/pkg/worker/setup"
)

// Credentials holds the authentication tokens.
type Credentials struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	Email        string    `json:"email,omitempty"`
	Workspace    string    `json:"workspace,omitempty"`
}

// TokenRefresher monitors token expiry and auto-refreshes using CredentialStore.
type TokenRefresher struct {
	backendURL           string
	store                setup.CredentialStore
	onReauthNeeded       func()
	onTokenRefresh       func(newToken string)
	onPermanentFailureFn func(event string)
	client               *http.Client
	mu                   sync.RWMutex
	creds                *Credentials
}

// NewTokenRefresher creates a refresher backed by a CredentialStore.
// Credentials are stored under the "autopus-worker" service key.
func NewTokenRefresher(
	backendURL string,
	store setup.CredentialStore,
	onReauthNeeded func(),
	onTokenRefresh func(string),
) *TokenRefresher {
	return &TokenRefresher{
		backendURL:     backendURL,
		store:          store,
		onReauthNeeded: onReauthNeeded,
		onTokenRefresh: onTokenRefresh,
		client:         &http.Client{Timeout: 10 * time.Second},
	}
}

// Start runs the background token-check loop. It blocks until ctx is cancelled.
func (t *TokenRefresher) Start(ctx context.Context) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	// Initial check immediately.
	t.checkAndRefresh(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			t.checkAndRefresh(ctx)
		}
	}
}

// LoadCredentials reads and parses credentials from the CredentialStore.
func (t *TokenRefresher) LoadCredentials() (*Credentials, error) {
	raw, err := t.store.Load("autopus-worker")
	if err != nil {
		return nil, fmt.Errorf("load credentials from store: %w", err)
	}
	var creds Credentials
	if err := json.Unmarshal([]byte(raw), &creds); err != nil {
		return nil, fmt.Errorf("parse credentials: %w", err)
	}
	t.mu.Lock()
	t.creds = &creds
	t.mu.Unlock()
	return &creds, nil
}

// SaveCredentials persists credentials to the CredentialStore.
func (t *TokenRefresher) SaveCredentials(creds *Credentials) error {
	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal credentials: %w", err)
	}
	if err := t.store.Save("autopus-worker", string(data)); err != nil {
		return fmt.Errorf("save credentials to store: %w", err)
	}
	t.mu.Lock()
	t.creds = creds
	t.mu.Unlock()
	return nil
}

// ForceRefresh attempts an immediate token refresh with backoff retries.
// Returns the new access token on success, or an error if all retries fail.
func (t *TokenRefresher) ForceRefresh(ctx context.Context) (string, error) {
	creds, err := t.LoadCredentials()
	if err != nil {
		return "", fmt.Errorf("load credentials: %w", err)
	}

	for attempt := 0; attempt < DefaultMaxRetries; attempt++ {
		if attempt > 0 {
			delay := Backoff(attempt-1, DefaultBackoffBase, DefaultBackoffFactor, DefaultBackoffJitter)
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(delay):
			}
		}

		ok, sc := t.doRefresh(ctx, creds)
		if ok {
			t.mu.RLock()
			token := t.creds.AccessToken
			t.mu.RUnlock()
			return token, nil
		}

		// Non-retryable (401/403): emit permanent failure and stop immediately.
		if !IsRetryableStatus(sc) {
			t.emitPermanentFailure(attempt+1, nil)
			t.onReauthNeeded()
			return "", fmt.Errorf("non-retryable auth error: HTTP %d", sc)
		}
	}

	// All retries exhausted.
	t.emitPermanentFailure(DefaultMaxRetries, nil)
	t.onReauthNeeded()
	return "", fmt.Errorf("token refresh failed after %d attempts", DefaultMaxRetries)
}

func (t *TokenRefresher) checkAndRefresh(ctx context.Context) {
	creds, err := t.LoadCredentials()
	if err != nil {
		log.Printf("[auth] check: load credentials failed: %v", err)
		return
	}
	// Refresh 5 minutes before expiry.
	if time.Until(creds.ExpiresAt) > 5*time.Minute {
		return
	}

	for attempt := 0; attempt < DefaultMaxRetries; attempt++ {
		if attempt > 0 {
			delay := Backoff(attempt-1, DefaultBackoffBase, DefaultBackoffFactor, DefaultBackoffJitter)
			select {
			case <-ctx.Done():
				return
			case <-time.After(delay):
			}
		}

		ok, sc := t.doRefresh(ctx, creds)
		if ok {
			return // refresh succeeded
		}

		// Non-retryable (401/403): emit permanent failure and stop immediately (FR-AUTH-11).
		if !IsRetryableStatus(sc) {
			t.emitPermanentFailure(attempt+1, nil)
			t.onReauthNeeded()
			return
		}
	}

	// All retries exhausted.
	t.emitPermanentFailure(DefaultMaxRetries, nil)
	t.onReauthNeeded()
}

// OnPermanentFailure registers a callback invoked when a non-retryable auth
// failure occurs (e.g. 401 Unauthorized). The callback receives the event name
// "auth.permanent_failure" (FR-AUTH-11).
func (t *TokenRefresher) OnPermanentFailure(fn func(event string)) {
	t.mu.Lock()
	t.onPermanentFailureFn = fn
	t.mu.Unlock()
}

// emitPermanentFailure logs a structured permanent failure event (FR-AUTH-11)
// and invokes the registered OnPermanentFailure callback.
func (t *TokenRefresher) emitPermanentFailure(attempts int, lastErr error) {
	log.Printf("[auth.permanent_failure] reason=%v attempts=%d last_error=%v",
		"token refresh exhausted", attempts, lastErr)
	t.mu.RLock()
	fn := t.onPermanentFailureFn
	t.mu.RUnlock()
	if fn != nil {
		fn("auth.permanent_failure")
	}
}

// doRefresh attempts a single token refresh. Returns whether the refresh
// succeeded and the HTTP status code (0 if the request itself failed).
func (t *TokenRefresher) doRefresh(ctx context.Context, creds *Credentials) (ok bool, statusCode int) {
	body, _ := json.Marshal(map[string]string{
		"refresh_token": creds.RefreshToken,
	})
	// Use cli-refresh endpoint — it accepts refresh_token in the body
	// (standard /auth/refresh reads from httpOnly cookies, which CLI clients cannot use).
	endpoint := t.backendURL + "/api/v1/auth/cli-refresh"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		log.Printf("[auth] refresh: create request failed: %v", err)
		return false, 0
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		log.Printf("[auth] refresh: request failed: %v", err)
		return false, 0
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[auth] refresh: server returned %d", resp.StatusCode)
		return false, resp.StatusCode
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
	if err := json.NewDecoder(resp.Body).Decode(&wrapper); err != nil {
		log.Printf("[auth] refresh: decode response failed: %v", err)
		return false, resp.StatusCode
	}
	if !wrapper.Success {
		log.Printf("[auth] refresh: server returned success=false")
		return false, resp.StatusCode
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
	if err := t.SaveCredentials(newCreds); err != nil {
		log.Printf("[auth] refresh: save credentials failed: %v", err)
		return false, http.StatusOK
	}
	if t.onTokenRefresh != nil {
		t.onTokenRefresh(newCreds.AccessToken)
	}
	return true, http.StatusOK
}
