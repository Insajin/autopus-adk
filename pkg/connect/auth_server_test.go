package connect_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/insajin/autopus-adk/pkg/connect"
	"github.com/insajin/autopus-adk/pkg/worker/setup"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockAuthDeps implements connect.AuthDeps for testing.
type mockAuthDeps struct {
	generatePKCE      func() (string, string, error)
	requestDeviceCode func(string, string) (*setup.DeviceCode, error)
	pollForToken      func(context.Context, string, string, string, int) (*setup.TokenResponse, error)
	openBrowser       func(string) error
	saveCredentials   func(map[string]any) error
	printLoginPrompt  func(uri, code string)
}

func (m *mockAuthDeps) GeneratePKCE() (string, string, error) {
	return m.generatePKCE()
}

func (m *mockAuthDeps) RequestDeviceCode(backendURL, codeVerifier string) (*setup.DeviceCode, error) {
	return m.requestDeviceCode(backendURL, codeVerifier)
}

func (m *mockAuthDeps) PollForToken(ctx context.Context, backendURL, deviceCode, codeVerifier string, interval int) (*setup.TokenResponse, error) {
	return m.pollForToken(ctx, backendURL, deviceCode, codeVerifier, interval)
}

func (m *mockAuthDeps) OpenBrowser(url string) error {
	return m.openBrowser(url)
}

func (m *mockAuthDeps) SaveCredentials(creds map[string]any) error {
	return m.saveCredentials(creds)
}

// PrintLoginPrompt is a no-op in the mock unless overridden.
func (m *mockAuthDeps) PrintLoginPrompt(uri, code string) {
	if m.printLoginPrompt != nil {
		m.printLoginPrompt(uri, code)
	}
}

func newSuccessDeps() *mockAuthDeps {
	return &mockAuthDeps{
		generatePKCE: func() (string, string, error) {
			return "test-verifier", "test-challenge", nil
		},
		requestDeviceCode: func(url, verifier string) (*setup.DeviceCode, error) {
			return &setup.DeviceCode{
				DeviceCode:      "dev-code-123",
				UserCode:        "ABCD-1234",
				VerificationURI: "https://auth.example.com/verify",
				ExpiresIn:       900,
				Interval:        5,
			}, nil
		},
		pollForToken: func(ctx context.Context, url, code, verifier string, interval int) (*setup.TokenResponse, error) {
			return &setup.TokenResponse{
				AccessToken:  "at-server-test",
				RefreshToken: "rt-server-test",
				ExpiresIn:    3600,
				TokenType:    "Bearer",
			}, nil
		},
		openBrowser:     func(url string) error { return nil },
		saveCredentials: func(creds map[string]any) error { return nil },
	}
}

func TestAuthenticateServer_Success(t *testing.T) {
	t.Parallel()

	// Given: mock deps that simulate a successful device code flow
	deps := newSuccessDeps()
	cfg := connect.ServerAuthConfig{ServerURL: "https://api.example.com"}

	// When: authenticating
	result, err := connect.AuthenticateServer(context.Background(), cfg, deps)

	// Then: auth result should contain user code and token
	require.NoError(t, err)
	assert.Equal(t, "ABCD-1234", result.UserCode)
	assert.Equal(t, "https://auth.example.com/verify", result.VerificationURI)
	assert.Equal(t, "at-server-test", result.Token)
}

func TestAuthenticateServer_PKCEError(t *testing.T) {
	t.Parallel()

	// Given: PKCE generation fails
	deps := newSuccessDeps()
	deps.generatePKCE = func() (string, string, error) {
		return "", "", fmt.Errorf("entropy exhausted")
	}
	cfg := connect.ServerAuthConfig{ServerURL: "https://api.example.com"}

	// When: authenticating
	_, err := connect.AuthenticateServer(context.Background(), cfg, deps)

	// Then: PKCE error should be returned
	require.Error(t, err)
	assert.ErrorContains(t, err, "generate PKCE")
	assert.ErrorContains(t, err, "entropy exhausted")
}

func TestAuthenticateServer_DeviceCodeError(t *testing.T) {
	t.Parallel()

	// Given: device code request fails
	deps := newSuccessDeps()
	deps.requestDeviceCode = func(url, verifier string) (*setup.DeviceCode, error) {
		return nil, fmt.Errorf("server unreachable")
	}
	cfg := connect.ServerAuthConfig{ServerURL: "https://api.example.com"}

	// When: authenticating
	_, err := connect.AuthenticateServer(context.Background(), cfg, deps)

	// Then: device code error should be returned
	require.Error(t, err)
	assert.ErrorContains(t, err, "request device code")
	assert.ErrorContains(t, err, "server unreachable")
}

func TestAuthenticateServer_PollTimeout(t *testing.T) {
	t.Parallel()

	// Given: token polling fails (timeout or user denied)
	deps := newSuccessDeps()
	deps.pollForToken = func(ctx context.Context, url, code, verifier string, interval int) (*setup.TokenResponse, error) {
		return nil, fmt.Errorf("polling timed out")
	}
	cfg := connect.ServerAuthConfig{ServerURL: "https://api.example.com"}

	// When: authenticating
	_, err := connect.AuthenticateServer(context.Background(), cfg, deps)

	// Then: poll error should be returned
	require.Error(t, err)
	assert.ErrorContains(t, err, "poll for token")
	assert.ErrorContains(t, err, "polling timed out")
}

func TestAuthenticateServer_SaveCredentialsError(t *testing.T) {
	t.Parallel()

	// Given: credential saving fails
	deps := newSuccessDeps()
	deps.saveCredentials = func(creds map[string]any) error {
		return fmt.Errorf("disk full")
	}
	cfg := connect.ServerAuthConfig{ServerURL: "https://api.example.com"}

	// When: authenticating
	_, err := connect.AuthenticateServer(context.Background(), cfg, deps)

	// Then: save error should be returned
	require.Error(t, err)
	assert.ErrorContains(t, err, "save credentials")
	assert.ErrorContains(t, err, "disk full")
}

func TestAuthenticateServer_BrowserOpenError(t *testing.T) {
	t.Parallel()

	// Given: browser open fails (non-fatal, should continue)
	deps := newSuccessDeps()
	deps.openBrowser = func(url string) error {
		return fmt.Errorf("no display")
	}
	cfg := connect.ServerAuthConfig{ServerURL: "https://api.example.com"}

	// When: authenticating
	result, err := connect.AuthenticateServer(context.Background(), cfg, deps)

	// Then: should succeed despite browser error (non-fatal)
	require.NoError(t, err)
	assert.Equal(t, "at-server-test", result.Token)
}

func TestAuthenticateServer_NilDepsCompiles(t *testing.T) {
	t.Parallel()

	// Given: nil deps — verifies the nil guard works (doesn't panic)
	// We can't actually run this without hitting real services,
	// so just verify it doesn't panic on the nil check.
	// The real call would fail on PKCE generation needing crypto/rand.
	cfg := connect.ServerAuthConfig{ServerURL: "http://192.0.2.1:1"}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// When: calling with nil deps and cancelled context
	_, err := connect.AuthenticateServer(ctx, cfg, nil)

	// Then: should get an error (not a panic)
	require.Error(t, err)
}
