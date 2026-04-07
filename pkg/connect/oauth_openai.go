package connect

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/insajin/autopus-adk/pkg/worker/setup"
)

// @AX:NOTE [AUTO] @AX:REASON: hardcoded OpenAI OAuth endpoints and client ID — update if OpenAI changes auth infrastructure
const (
	openAIAuthURL  = "https://auth.openai.com/oauth/authorize"
	openAITokenURL = "https://auth.openai.com/oauth/token"
	openAIClientID = "app_EMoamEEZ73f0CkXaXp7hrann"
	openAIScopes   = "openid profile email offline_access api.connectors.read api.connectors.invoke"
	openAIPort     = 1455
	// @AX:NOTE [AUTO] @AX:REASON: 5-minute timeout for OAuth flow — user must complete browser auth within this window
	oauthTimeout = 5 * time.Minute
)

// DefaultClientID returns the OpenAI PKCE client ID.
func DefaultClientID() string { return openAIClientID }

// OAuthConfig holds configuration for the OpenAI OAuth flow.
type OAuthConfig struct {
	ClientID string
	Scopes   string
	AuthURL  string
	TokenURL string
	Port     int // Callback port. 0 = use default (1455).
}

// OAuthFlowResult contains the PKCE pair and local server info from StartOAuthFlow.
type OAuthFlowResult struct {
	Verifier    string
	Challenge   string
	State       string
	Port        int
	RedirectURI string
}

// OAuthResult holds tokens returned from the token exchange.
type OAuthResult struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	ExpiresAt    time.Time
	Scopes       string `json:"scope"`
}

// CallbackRequest represents the data needed to exchange an authorization code.
type CallbackRequest struct {
	Code        string
	Verifier    string
	RedirectURI string
	ClientID    string
	TokenURL    string
}

func (c OAuthConfig) clientID() string {
	if c.ClientID != "" {
		return c.ClientID
	}
	return openAIClientID
}

func (c OAuthConfig) scopes() string {
	if c.Scopes != "" {
		return c.Scopes
	}
	return openAIScopes
}

func (c OAuthConfig) authURL() string {
	if c.AuthURL != "" {
		return c.AuthURL
	}
	return openAIAuthURL
}

func (c OAuthConfig) tokenURL() string {
	if c.TokenURL != "" {
		return c.TokenURL
	}
	return openAITokenURL
}

func (c OAuthConfig) port() int {
	if c.Port > 0 {
		return c.Port
	}
	return openAIPort
}

// StartOAuthFlow generates PKCE credentials and starts a local callback server.
// It does NOT open the browser or wait for callback — use WaitForCallback for that.
func StartOAuthFlow(ctx context.Context, cfg OAuthConfig) (*OAuthFlowResult, error) {
	verifier, challenge, err := setup.GeneratePKCE()
	if err != nil {
		return nil, fmt.Errorf("generate PKCE: %w", err)
	}

	// Generate CSRF state parameter.
	stateBuf := make([]byte, 16)
	if _, err := rand.Read(stateBuf); err != nil {
		return nil, fmt.Errorf("generate state: %w", err)
	}
	state := base64.RawURLEncoding.EncodeToString(stateBuf)

	// Use configured port, defaulting to 1455 (Codex CLI's registered redirect_uri).
	port := cfg.port()
	redirectURI := fmt.Sprintf("http://localhost:%d/auth/callback", port)
	return &OAuthFlowResult{
		Verifier:    verifier,
		Challenge:   challenge,
		State:       state,
		Port:        port,
		RedirectURI: redirectURI,
	}, nil
}

// WaitForCallback starts a local HTTP server and waits for the OAuth callback.
// It respects the provided context for cancellation/timeout.
func WaitForCallback(ctx context.Context, cfg OAuthConfig) (*OAuthResult, error) {
	flow, err := StartOAuthFlow(ctx, cfg)
	if err != nil {
		return nil, err
	}

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/auth/callback", callbackHandler(flow.State, codeCh, errCh))

	server := &http.Server{
		Addr:    fmt.Sprintf("127.0.0.1:%d", flow.Port),
		Handler: mux,
	}

	// @AX:WARN [AUTO] @AX:REASON: goroutine outlives function scope — server.Close in defer handles cleanup
	go func() {
		if sErr := server.ListenAndServe(); sErr != nil && !errors.Is(sErr, http.ErrServerClosed) {
			errCh <- fmt.Errorf("callback server: %w", sErr)
		}
	}()
	defer server.Close()

	authorizeURL := buildAuthorizeURL(cfg, flow)
	_ = setup.OpenBrowser(authorizeURL)

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case err := <-errCh:
		return nil, err
	case code := <-codeCh:
		req := CallbackRequest{
			Code:        code,
			Verifier:    flow.Verifier,
			RedirectURI: flow.RedirectURI,
			ClientID:    cfg.clientID(),
			TokenURL:    cfg.tokenURL(),
		}
		return ExchangeAuthCode(ctx, req)
	}
}

// @AX:NOTE [AUTO] @AX:REASON: public API — fan_in < 3, downgraded from ANCHOR during sync
// ExchangeAuthCode exchanges an authorization code for tokens.
func ExchangeAuthCode(ctx context.Context, req CallbackRequest) (*OAuthResult, error) {
	if req.Code == "" {
		return nil, fmt.Errorf("authorization code is required")
	}

	tokenURL := req.TokenURL
	if tokenURL == "" {
		tokenURL = openAITokenURL
	}
	clientID := req.ClientID
	if clientID == "" {
		clientID = openAIClientID
	}

	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {req.Code},
		"code_verifier": {req.Verifier},
		"redirect_uri":  {req.RedirectURI},
		"client_id":     {clientID},
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create token request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("token exchange: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token exchange failed (%d): %s", resp.StatusCode, body)
	}

	var result OAuthResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode token response: %w", err)
	}
	if result.ExpiresIn > 0 {
		result.ExpiresAt = time.Now().Add(time.Duration(result.ExpiresIn) * time.Second)
	}
	return &result, nil
}

func buildAuthorizeURL(cfg OAuthConfig, flow *OAuthFlowResult) string {
	params := url.Values{
		"client_id":                  {cfg.clientID()},
		"redirect_uri":              {flow.RedirectURI},
		"response_type":             {"code"},
		"code_challenge":            {flow.Challenge},
		"code_challenge_method":     {"S256"},
		"scope":                     {cfg.scopes()},
		"state":                     {flow.State},
		"codex_cli_simplified_flow": {"true"},
		"prompt":                    {"consent"},
		"access_type":               {"offline"},
	}
	return cfg.authURL() + "?" + params.Encode()
}

func callbackHandler(expectedState string, codeCh chan<- string, errCh chan<- error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		// Validate CSRF state parameter.
		if state := r.URL.Query().Get("state"); state != expectedState {
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprint(w, "<h1>Error</h1><p>Invalid state parameter — possible CSRF attack.</p>")
			errCh <- fmt.Errorf("callback state mismatch: possible CSRF")
			return
		}

		code := r.URL.Query().Get("code")
		if code == "" {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, "<h1>Error</h1><p>Authorization code not found.</p>")
			errCh <- fmt.Errorf("callback missing authorization code")
			return
		}

		fmt.Fprint(w, "<h1>Success</h1><p>Authorization complete. You can close this window.</p>")
		codeCh <- code
	}
}
