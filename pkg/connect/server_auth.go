package connect

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/insajin/autopus-adk/pkg/worker/setup"
)

// apiEnvelope is the standard backend response wrapper: { success, data }.
type apiEnvelope struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
}

// unwrapJSON extracts the data field from the standard backend response.
func unwrapJSON[T any](body []byte) (*T, error) {
	var env apiEnvelope
	if err := json.Unmarshal(body, &env); err == nil && env.Data != nil {
		var result T
		if err := json.Unmarshal(env.Data, &result); err != nil {
			return nil, fmt.Errorf("decode data: %w", err)
		}
		return &result, nil
	}
	var result T
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &result, nil
}

// Workspace represents an Autopus workspace returned by the server.
type Workspace struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// ServerAuthConfig holds configuration for server authentication.
type ServerAuthConfig struct {
	ServerURL string
}

// AuthResult holds the result of a device code authentication flow.
type AuthResult struct {
	UserCode        string
	VerificationURI string
	Token           string
}

// Client communicates with the Autopus backend API.
type Client struct {
	serverURL  string
	authToken  string
	httpClient *http.Client
}

// NewClient creates a new API client with the given auth token.
func NewClient(authToken string) *Client {
	return &Client{
		authToken:  authToken,
		httpClient: &http.Client{},
	}
}

// WithServerURL sets the server URL on the client and returns it for chaining.
func (c *Client) WithServerURL(serverURL string) *Client {
	c.serverURL = strings.TrimRight(serverURL, "/")
	return c
}

// AuthDeps abstracts external authentication operations for testability.
type AuthDeps interface {
	GeneratePKCE() (verifier, challenge string, err error)
	RequestDeviceCode(backendURL, codeVerifier string) (*setup.DeviceCode, error)
	PollForToken(ctx context.Context, backendURL, deviceCode, codeVerifier string, interval int) (*setup.TokenResponse, error)
	OpenBrowser(url string) error
	SaveCredentials(creds map[string]any) error
}

// defaultAuthDeps delegates to the real setup package functions.
type defaultAuthDeps struct{}

func (d defaultAuthDeps) GeneratePKCE() (string, string, error) {
	return setup.GeneratePKCE()
}

func (d defaultAuthDeps) RequestDeviceCode(backendURL, codeVerifier string) (*setup.DeviceCode, error) {
	return setup.RequestDeviceCode(backendURL, codeVerifier)
}

func (d defaultAuthDeps) PollForToken(ctx context.Context, backendURL, deviceCode, codeVerifier string, interval int) (*setup.TokenResponse, error) {
	return setup.PollForToken(ctx, backendURL, deviceCode, codeVerifier, interval)
}

func (d defaultAuthDeps) OpenBrowser(url string) error {
	return setup.OpenBrowser(url)
}

func (d defaultAuthDeps) SaveCredentials(creds map[string]any) error {
	return setup.SaveCredentials(creds)
}

// AuthenticateServer runs the device code auth flow against the Autopus backend.
// If deps is nil, real setup package implementations are used.
func AuthenticateServer(ctx context.Context, cfg ServerAuthConfig, deps AuthDeps) (*AuthResult, error) {
	if deps == nil {
		deps = defaultAuthDeps{}
	}

	verifier, _, err := deps.GeneratePKCE()
	if err != nil {
		return nil, fmt.Errorf("generate PKCE: %w", err)
	}

	dc, err := deps.RequestDeviceCode(cfg.ServerURL, verifier)
	if err != nil {
		return nil, fmt.Errorf("request device code: %w", err)
	}

	fmt.Printf("Visit %s and enter code: %s\n", dc.VerificationURI, dc.UserCode)
	_ = deps.OpenBrowser(dc.VerificationURI)

	tokenResp, err := deps.PollForToken(ctx, cfg.ServerURL, dc.DeviceCode, verifier, dc.Interval)
	if err != nil {
		return nil, fmt.Errorf("poll for token: %w", err)
	}

	if err := deps.SaveCredentials(map[string]any{
		"access_token":  tokenResp.AccessToken,
		"refresh_token": tokenResp.RefreshToken,
		"token_type":    tokenResp.TokenType,
	}); err != nil {
		return nil, fmt.Errorf("save credentials: %w", err)
	}

	return &AuthResult{
		UserCode:        dc.UserCode,
		VerificationURI: dc.VerificationURI,
		Token:           tokenResp.AccessToken,
	}, nil
}

// ListWorkspaces retrieves the user's workspace list from the Autopus backend.
func (c *Client) ListWorkspaces(ctx context.Context) ([]Workspace, error) {
	url := c.serverURL + "/api/v1/workspaces"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.authToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("list workspaces: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("unauthorized: invalid or expired token")
	}
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list workspaces failed (%d): %s", resp.StatusCode, body)
	}

	result, err := unwrapJSON[[]Workspace](body)
	if err != nil {
		return nil, fmt.Errorf("decode workspaces: %w", err)
	}
	return *result, nil
}
