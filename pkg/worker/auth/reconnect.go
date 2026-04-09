package auth

import (
	"context"
	"fmt"
	"log"
	"sync"
)

// ServerReconnecter abstracts the a2a.Server methods needed for coordinated reconnection.
type ServerReconnecter interface {
	ReconnectTransport(ctx context.Context) error
	SetAuthToken(token string)
}

// Reconnector coordinates token refresh and WebSocket reconnection in a single sequence.
// It prevents duplicate concurrent reconnection attempts via mutex.
type Reconnector struct {
	refresher  *TokenRefresher
	server     ServerReconnecter
	mu         sync.Mutex
	inProgress bool
}

// NewReconnector creates a Reconnector that coordinates token refresh → auth update → transport reconnect.
func NewReconnector(refresher *TokenRefresher, server ServerReconnecter) *Reconnector {
	return &Reconnector{refresher: refresher, server: server}
}

// Reconnect executes the coordinated reconnection sequence (FR-AUTH-04):
//  1. Refresh token via CredentialStore (with backoff)
//  2. Update server auth token
//  3. Reconnect WebSocket transport
//
// Concurrent calls are deduplicated — only the first caller executes, others return nil.
func (r *Reconnector) Reconnect(ctx context.Context) error {
	r.mu.Lock()
	if r.inProgress {
		r.mu.Unlock()
		log.Printf("[reconnect] already in progress, skipping duplicate")
		return nil // duplicate call, skip
	}
	r.inProgress = true
	r.mu.Unlock()

	defer func() {
		r.mu.Lock()
		r.inProgress = false
		r.mu.Unlock()
	}()

	// Step 1: Refresh token with backoff retries.
	newToken, err := r.refresher.ForceRefresh(ctx)
	if err != nil {
		return fmt.Errorf("token refresh: %w", err)
	}

	// Step 2: Update auth token on transport.
	r.server.SetAuthToken(newToken)

	// Step 3: Re-establish transport connection.
	if err := r.server.ReconnectTransport(ctx); err != nil {
		return fmt.Errorf("transport reconnect: %w", err)
	}

	log.Printf("[reconnect] coordinated reconnection completed")
	return nil
}
