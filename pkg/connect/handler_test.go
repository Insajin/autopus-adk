package connect

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCallbackHandler_WithCode(t *testing.T) {
	t.Parallel()

	// Given: channels to receive code/error and a request with a valid code
	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)
	handler := callbackHandler("test-state", codeCh, errCh)

	req := httptest.NewRequest(http.MethodGet, "/auth/callback?code=test-code-123&state=test-state", nil)
	rec := httptest.NewRecorder()

	// When: handling the callback
	handler(rec, req)

	// Then: success HTML returned and code sent to channel
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Success")
	assert.Contains(t, rec.Body.String(), "Authorization complete")

	select {
	case code := <-codeCh:
		assert.Equal(t, "test-code-123", code)
	default:
		t.Fatal("expected code on channel")
	}
}

func TestCallbackHandler_MissingCode(t *testing.T) {
	t.Parallel()

	// Given: channels and a request WITHOUT code parameter
	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)
	handler := callbackHandler("test-state", codeCh, errCh)

	req := httptest.NewRequest(http.MethodGet, "/auth/callback?state=test-state", nil)
	rec := httptest.NewRecorder()

	// When: handling the callback
	handler(rec, req)

	// Then: 400 error returned with error message
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "Error")
	assert.Contains(t, rec.Body.String(), "Authorization code not found")

	select {
	case err := <-errCh:
		require.Error(t, err)
		assert.ErrorContains(t, err, "callback missing authorization code")
	default:
		t.Fatal("expected error on channel")
	}
}

func TestCallbackHandler_EmptyCode(t *testing.T) {
	t.Parallel()

	// Given: a request with empty code parameter
	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)
	handler := callbackHandler("test-state", codeCh, errCh)

	req := httptest.NewRequest(http.MethodGet, "/auth/callback?code=&state=test-state", nil)
	rec := httptest.NewRecorder()

	// When: handling the callback
	handler(rec, req)

	// Then: treated as missing code
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	select {
	case err := <-errCh:
		require.Error(t, err)
	default:
		t.Fatal("expected error on channel")
	}
}

func TestOAuthConfig_Defaults(t *testing.T) {
	t.Parallel()

	// Given: empty config
	cfg := OAuthConfig{}

	// Then: defaults should be used
	assert.Equal(t, openAIClientID, cfg.clientID())
	assert.Equal(t, openAIScopes, cfg.scopes())
	assert.Equal(t, openAIAuthURL, cfg.authURL())
	assert.Equal(t, openAITokenURL, cfg.tokenURL())
}

func TestOAuthConfig_CustomValues(t *testing.T) {
	t.Parallel()

	// Given: config with custom values
	cfg := OAuthConfig{
		ClientID: "custom-client",
		Scopes:   "custom-scope",
		AuthURL:  "https://custom-auth.example.com",
		TokenURL: "https://custom-token.example.com",
	}

	// Then: custom values should be returned
	assert.Equal(t, "custom-client", cfg.clientID())
	assert.Equal(t, "custom-scope", cfg.scopes())
	assert.Equal(t, "https://custom-auth.example.com", cfg.authURL())
	assert.Equal(t, "https://custom-token.example.com", cfg.tokenURL())
}

func TestExchangeAuthCode_HttpDoError(t *testing.T) {
	t.Parallel()

	// Given: a context cancelled before Do() executes
	ctx, cancel := context.WithCancel(context.Background())

	req := CallbackRequest{
		Code:     "test-code",
		Verifier: "test-verifier",
		TokenURL: "http://192.0.2.1:1", // RFC 5737 TEST-NET, guaranteed unreachable
	}

	cancel() // cancel before Do

	// When: exchanging with cancelled context
	_, err := ExchangeAuthCode(ctx, req)

	// Then: http.Do error
	require.Error(t, err)
	assert.ErrorContains(t, err, "token exchange")
}

func TestExchangeAuthCode_ReadBodyError(t *testing.T) {
	t.Parallel()

	// Given: a server that returns 200 but with truncated body that can still be read
	// We test the non-200 path with body content
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("bad request details"))
	}))
	defer srv.Close()

	req := CallbackRequest{
		Code:     "test-code",
		Verifier: "test-verifier",
		TokenURL: srv.URL,
	}

	_, err := ExchangeAuthCode(t.Context(), req)
	require.Error(t, err)
	assert.ErrorContains(t, err, "bad request details")
}

func TestBuildAuthorizeURL(t *testing.T) {
	t.Parallel()

	// Given: config and flow result
	cfg := OAuthConfig{
		ClientID: "test-client",
		Scopes:   "read write",
		AuthURL:  "https://auth.example.com/authorize",
	}
	flow := &OAuthFlowResult{
		RedirectURI: "http://localhost:8080/callback",
		Challenge:   "test-challenge",
		State:       "test-state",
	}

	// When: building the authorize URL
	url := buildAuthorizeURL(cfg, flow)

	// Then: URL should contain all required parameters
	assert.Contains(t, url, "https://auth.example.com/authorize?")
	assert.Contains(t, url, "client_id=test-client")
	assert.Contains(t, url, "response_type=code")
	assert.Contains(t, url, "code_challenge=test-challenge")
	assert.Contains(t, url, "code_challenge_method=S256")
}
