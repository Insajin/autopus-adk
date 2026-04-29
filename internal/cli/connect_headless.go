package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/connect"
	"github.com/insajin/autopus-adk/pkg/worker/setup"
)

// headlessAuthDeps implements connect.AuthDeps for headless mode.
// It emits NDJSON events instead of opening a browser, and skips credential saving
// (the token is used transiently within the headless flow).
type headlessAuthDeps struct {
	deviceCode *setup.DeviceCode
}

func (h *headlessAuthDeps) GeneratePKCE() (string, string, error) {
	return setup.GeneratePKCE()
}

func (h *headlessAuthDeps) RequestDeviceCode(backendURL, codeVerifier string) (*setup.DeviceCode, error) {
	dc, err := setup.RequestDeviceCode(backendURL, codeVerifier)
	if err != nil {
		return nil, err
	}
	h.deviceCode = dc
	// Emit login_required before returning so the caller can act on the URL/code.
	connect.EmitEvent(connect.HeadlessEvent{
		Step:      "server_auth",
		Action:    "login_required",
		URL:       dc.VerificationURI,
		Code:      dc.UserCode,
		ExpiresIn: dc.ExpiresIn,
	})
	return dc, nil
}

func (h *headlessAuthDeps) PollForToken(ctx context.Context, backendURL, deviceCode, codeVerifier string, interval int) (*setup.TokenResponse, error) {
	return setup.PollForToken(ctx, backendURL, deviceCode, codeVerifier, interval)
}

// OpenBrowser is a no-op in headless mode — agent handles URL navigation.
func (h *headlessAuthDeps) OpenBrowser(_ string) error { return nil }

// SaveCredentials is a no-op in headless mode — token used transiently.
func (h *headlessAuthDeps) SaveCredentials(_ map[string]any) error { return nil }

// PrintLoginPrompt is a no-op in headless mode — the NDJSON event is emitted from
// RequestDeviceCode instead, which runs before this method would be called.
func (h *headlessAuthDeps) PrintLoginPrompt(_, _ string) {}

// runHeadlessConnect executes the 3-step OAuth connection in non-interactive (headless) mode.
// All output is written as NDJSON to stdout; debug/progress to stderr.
func runHeadlessConnect(cmd *cobra.Command, serverURL, workspaceID string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
	defer cancel()

	// Step 1: Autopus server authentication via device code flow.
	authResult, err := headlessServerAuth(ctx, serverURL)
	if err != nil {
		connect.EmitEvent(connect.HeadlessEvent{
			Step:   "server_auth",
			Status: "error",
			Error:  err.Error(),
		})
		return fmt.Errorf("server auth: %w", err)
	}
	connect.EmitEvent(connect.HeadlessEvent{
		Step:   "server_auth",
		Status: "success",
	})

	// Step 2: Validate workspace ID against the server.
	client := connect.NewClient(authResult.Token).WithServerURL(serverURL)
	wsID, wsName, err := headlessWorkspaceValidate(ctx, client, workspaceID)
	if err != nil {
		connect.EmitEvent(connect.HeadlessEvent{
			Step:   "workspace",
			Status: "error",
			Error:  err.Error(),
		})
		return fmt.Errorf("workspace validation: %w", err)
	}
	connect.EmitEvent(connect.HeadlessEvent{
		Step:          "workspace",
		Status:        "success",
		WorkspaceID:   wsID,
		WorkspaceName: wsName,
	})

	// Step 3: OpenAI OAuth via server-proxy device code flow.
	if err := headlessOpenAIOAuth(ctx, serverURL, authResult.Token, wsID); err != nil {
		connect.EmitEvent(connect.HeadlessEvent{
			Step:   "openai_oauth",
			Status: "error",
			Error:  err.Error(),
		})
		return fmt.Errorf("openai oauth: %w", err)
	}
	connect.EmitEvent(connect.HeadlessEvent{
		Step:   "openai_oauth",
		Status: "success",
	})

	// Final: emit completion event.
	connect.EmitEvent(connect.HeadlessEvent{
		Step:        "complete",
		Status:      "success",
		WorkspaceID: wsID,
		Provider:    "openai",
	})
	return nil
}

// headlessServerAuth runs the Autopus device code flow without opening a browser.
// It emits a login_required event with the verification URL and user code.
func headlessServerAuth(ctx context.Context, serverURL string) (*connect.AuthResult, error) {
	deps := &headlessAuthDeps{}
	cfg := connect.ServerAuthConfig{ServerURL: serverURL}
	return connect.AuthenticateServer(ctx, cfg, deps)
}

// headlessWorkspaceValidate confirms the given workspace ID exists on the server.
func headlessWorkspaceValidate(ctx context.Context, client *connect.Client, workspaceID string) (id, name string, err error) {
	workspaces, err := client.ListWorkspaces(ctx)
	if err != nil {
		return "", "", err
	}
	for _, ws := range workspaces {
		if ws.ID == workspaceID {
			return ws.ID, ws.Name, nil
		}
	}
	return "", "", fmt.Errorf("workspace not found")
}

// headlessOpenAIOAuth requests a server-proxy device code and polls until completion.
func headlessOpenAIOAuth(ctx context.Context, serverURL, authToken, workspaceID string) error {
	dc, err := connect.RequestOpenAIDeviceCode(ctx, serverURL, authToken, workspaceID)
	if err != nil {
		return fmt.Errorf("server does not support device code flow: %w", err)
	}

	// Notify caller to direct the user to the verification URL.
	connect.EmitEvent(connect.HeadlessEvent{
		Step:      "openai_oauth",
		Action:    "login_required",
		URL:       dc.VerificationURI,
		Code:      dc.UserCode,
		ExpiresIn: dc.ExpiresIn,
	})

	tokenResp, err := connect.PollOpenAIDeviceToken(ctx, serverURL, authToken, workspaceID, dc.DeviceCode, dc.Interval)
	if err != nil {
		return err
	}
	if tokenResp.AccessToken == "" {
		return fmt.Errorf("completed openai oauth response missing access token")
	}

	client := connect.NewClient(authToken).WithServerURL(serverURL)
	if err := client.SubmitToken(ctx, connect.SubmitTokenRequest{
		ProviderToken: tokenResp.AccessToken,
		RefreshToken:  tokenResp.RefreshToken,
		WorkspaceID:   workspaceID,
		Provider:      "openai",
	}); err != nil {
		return fmt.Errorf("submit openai token: %w", err)
	}
	return nil
}
