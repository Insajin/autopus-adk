package connect_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStartOAuthFlow_GeneratesPKCE(t *testing.T) {
	t.Parallel()

	// Given: a valid OAuth configuration
	cfg := connect.OAuthConfig{ClientID: "test-client"}

	// When: starting the OAuth flow
	result, err := connect.StartOAuthFlow(context.Background(), cfg)

	// Then: PKCE verifier and challenge should be generated
	require.NoError(t, err)
	assert.NotEmpty(t, result.Verifier, "PKCE verifier must be generated")
	assert.NotEmpty(t, result.Challenge, "PKCE challenge must be generated")
	assert.Len(t, result.Verifier, 43, "RFC 7636: base64url(32 bytes) = 43 chars")
	assert.Len(t, result.Challenge, 43, "SHA256 challenge should be 43 chars base64url")
}

func TestStartOAuthFlow_StartsLocalServer(t *testing.T) {
	t.Parallel()

	// Given: a valid OAuth configuration
	cfg := connect.OAuthConfig{ClientID: "test-client"}

	// When: starting the OAuth flow
	result, err := connect.StartOAuthFlow(context.Background(), cfg)

	// Then: a local server port should be allocated
	require.NoError(t, err)
	assert.Greater(t, result.Port, 0, "port must be positive")
	assert.Contains(t, result.RedirectURI, fmt.Sprintf("http://localhost:%d", result.Port))
}

func TestStartOAuthFlow_UniquePerCall(t *testing.T) {
	t.Parallel()

	// Given: the same config used twice
	cfg := connect.OAuthConfig{ClientID: "test-client"}

	// When: starting two separate flows
	r1, err1 := connect.StartOAuthFlow(context.Background(), cfg)
	r2, err2 := connect.StartOAuthFlow(context.Background(), cfg)

	// Then: each flow generates unique PKCE credentials
	require.NoError(t, err1)
	require.NoError(t, err2)
	assert.NotEqual(t, r1.Verifier, r2.Verifier, "each flow must have a unique verifier")
}

func TestExchangeAuthCode_Success(t *testing.T) {
	t.Parallel()

	// Given: a mock token endpoint that returns valid tokens
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))

		err := r.ParseForm()
		require.NoError(t, err)
		assert.Equal(t, "authorization_code", r.FormValue("grant_type"))
		assert.Equal(t, "test-code", r.FormValue("code"))
		assert.NotEmpty(t, r.FormValue("code_verifier"))

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "at-12345",
			"refresh_token": "rt-67890",
			"expires_in":    3600,
			"scope":         "openai.chat.completions.create",
		})
	}))
	defer srv.Close()

	req := connect.CallbackRequest{
		Code:        "test-code",
		Verifier:    "test-verifier-43-chars-long-padded-to-match",
		RedirectURI: "http://localhost:9999/auth/callback",
		ClientID:    "test-client",
		TokenURL:    srv.URL,
	}

	// When: exchanging the authorization code
	result, err := connect.ExchangeAuthCode(context.Background(), req)

	// Then: valid tokens should be returned
	require.NoError(t, err)
	assert.Equal(t, "at-12345", result.AccessToken)
	assert.Equal(t, "rt-67890", result.RefreshToken)
	assert.Equal(t, 3600, result.ExpiresIn)
	assert.False(t, result.ExpiresAt.IsZero(), "ExpiresAt should be set")
}

func TestExchangeAuthCode_EmptyCode(t *testing.T) {
	t.Parallel()

	// Given: a callback request with NO authorization code (REQ-OAUTH-03)
	req := connect.CallbackRequest{
		Code:     "",
		Verifier: "test-verifier",
	}

	// When: processing the callback
	_, err := connect.ExchangeAuthCode(context.Background(), req)

	// Then: an error about missing code should be returned
	require.Error(t, err)
	assert.ErrorContains(t, err, "code")
}

func TestExchangeAuthCode_ServerError(t *testing.T) {
	t.Parallel()

	// Given: a token endpoint that returns 500
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "internal error")
	}))
	defer srv.Close()

	req := connect.CallbackRequest{
		Code:     "test-code",
		Verifier: "test-verifier",
		TokenURL: srv.URL,
	}

	// When: exchanging the code
	_, err := connect.ExchangeAuthCode(context.Background(), req)

	// Then: a descriptive error should be returned
	require.Error(t, err)
	assert.ErrorContains(t, err, "token exchange failed")
}

func TestExchangeAuthCode_InvalidJSON(t *testing.T) {
	t.Parallel()

	// Given: a token endpoint that returns invalid JSON
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, "not-json")
	}))
	defer srv.Close()

	req := connect.CallbackRequest{
		Code:     "test-code",
		Verifier: "test-verifier",
		TokenURL: srv.URL,
	}

	// When: exchanging the code
	_, err := connect.ExchangeAuthCode(context.Background(), req)

	// Then: a decode error should be returned
	require.Error(t, err)
	assert.ErrorContains(t, err, "decode token response")
}

func TestWaitForCallback_Timeout(t *testing.T) {
	t.Parallel()

	// Given: a context that expires immediately (REQ-OAUTH-04)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	cfg := connect.OAuthConfig{ClientID: "test-client"}

	// When: waiting for a callback that never arrives
	_, err := connect.WaitForCallback(ctx, cfg)

	// Then: deadline exceeded error should be returned
	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestDefaultClientID(t *testing.T) {
	t.Parallel()

	// When: getting the default client ID
	id := connect.DefaultClientID()

	// Then: it should return the expected OpenAI app ID
	assert.Equal(t, "app_EMoamEEZ73f0CkXaXp7hrann", id)
}
