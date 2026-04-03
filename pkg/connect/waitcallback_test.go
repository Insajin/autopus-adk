package connect

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// freePort returns an available TCP port for testing.
func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	return port
}

func TestWaitForCallback_SuccessPath(t *testing.T) {
	t.Parallel()

	// Given: a mock token server
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token": "at-waitcb-success",
			"expires_in":   3600,
		})
	}))
	defer tokenSrv.Close()

	cfg := OAuthConfig{Port: freePort(t),
		ClientID: "test-client",
		TokenURL: tokenSrv.URL,
		AuthURL:  "https://auth.example.com/authorize",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// First, get the port that WaitForCallback would use
	flow, err := StartOAuthFlow(ctx, cfg)
	require.NoError(t, err)
	port := flow.Port

	// Manually set up the same server WaitForCallback would create
	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/auth/callback", callbackHandler(flow.State, codeCh, errCh))

	server := &http.Server{
		Addr:    fmt.Sprintf("127.0.0.1:%d", port),
		Handler: mux,
	}
	go func() {
		if sErr := server.ListenAndServe(); sErr != nil && sErr != http.ErrServerClosed {
			errCh <- fmt.Errorf("callback server: %w", sErr)
		}
	}()
	defer server.Close()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// When: sending a callback with a code
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/auth/callback?code=test-code-xyz&state=%s", port, flow.State))
	require.NoError(t, err)
	resp.Body.Close()

	// Then: code should arrive on channel
	select {
	case code := <-codeCh:
		assert.Equal(t, "test-code-xyz", code)

		// Now simulate what WaitForCallback does with the code
		req := CallbackRequest{
			Code:        code,
			Verifier:    flow.Verifier,
			RedirectURI: flow.RedirectURI,
			ClientID:    cfg.clientID(),
			TokenURL:    cfg.tokenURL(),
		}
		result, err := ExchangeAuthCode(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, "at-waitcb-success", result.AccessToken)
	case err := <-errCh:
		t.Fatalf("unexpected error: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for code")
	}
}

func TestWaitForCallback_ErrorPath(t *testing.T) {
	t.Parallel()

	cfg := OAuthConfig{Port: freePort(t),
		ClientID: "test-client",
		AuthURL:  "https://auth.example.com/authorize",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Get a port
	flow, err := StartOAuthFlow(ctx, cfg)
	require.NoError(t, err)
	port := flow.Port

	// Set up the callback server manually
	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/auth/callback", callbackHandler(flow.State, codeCh, errCh))

	server := &http.Server{
		Addr:    fmt.Sprintf("127.0.0.1:%d", port),
		Handler: mux,
	}
	go func() {
		if sErr := server.ListenAndServe(); sErr != nil && sErr != http.ErrServerClosed {
			errCh <- fmt.Errorf("callback server: %w", sErr)
		}
	}()
	defer server.Close()

	time.Sleep(100 * time.Millisecond)

	// When: sending a callback WITHOUT code
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/auth/callback?state=%s", port, flow.State))
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	// Then: error should arrive on errCh
	select {
	case err := <-errCh:
		assert.ErrorContains(t, err, "callback missing authorization code")
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for error")
	}
}

func TestWaitForCallback_ContextCancelled(t *testing.T) {
	t.Parallel()

	cfg := OAuthConfig{Port: freePort(t),
		ClientID: "test-client",
		AuthURL:  "https://auth.example.com/authorize",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// When: context expires before callback arrives (REQ-OAUTH-04)
	_, err := WaitForCallback(ctx, cfg)

	// Then: context deadline error
	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestExchangeAuthCode_DefaultsUsed(t *testing.T) {
	t.Parallel()

	// Given: empty TokenURL/ClientID — defaults apply, cancelled ctx prevents network call
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	req := CallbackRequest{
		Code:     "test-code",
		Verifier: "test-verifier",
	}

	_, err := ExchangeAuthCode(ctx, req)
	require.Error(t, err)
}

func TestStartOAuthFlow_PortIsValid(t *testing.T) {
	t.Parallel()

	cfg := OAuthConfig{Port: freePort(t),ClientID: "test"}

	results := make([]*OAuthFlowResult, 3)
	for i := range results {
		r, err := StartOAuthFlow(context.Background(), cfg)
		require.NoError(t, err)
		results[i] = r
	}

	for i, r := range results {
		assert.Greater(t, r.Port, 1024, "port %d should be unprivileged", i)
		assert.Equal(t, fmt.Sprintf("http://localhost:%d/auth/callback", r.Port), r.RedirectURI)
	}
}
