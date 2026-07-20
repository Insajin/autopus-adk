package run

import (
	"context"

	"github.com/insajin/autopus-adk/pkg/qa/desktopobserve"
)

func (client *orcaDesktopClient) Handshake(
	ctx context.Context,
) (desktopobserve.ProviderIdentity, error) {
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.identity.Name != "" {
		return client.identity, nil
	}
	raw, err := client.runLocked(ctx, "computer", "capabilities", "--json")
	if err != nil {
		return desktopobserve.ProviderIdentity{}, err
	}
	identity, runtimeID, err := decodeOrcaCapabilities(raw)
	if err != nil {
		return desktopobserve.ProviderIdentity{}, err
	}
	client.identity = identity
	client.runtimeID = runtimeID
	client.capabilities = desktopobserve.ReadOnlyOperations()
	return client.identity, nil
}

func (client *orcaDesktopClient) Capabilities(
	ctx context.Context,
) ([]desktopobserve.Operation, error) {
	client.mu.Lock()
	defer client.mu.Unlock()
	if err := client.requireHandshakeLocked(ctx); err != nil {
		return nil, err
	}
	return append([]desktopobserve.Operation(nil), client.capabilities...), nil
}

func (client *orcaDesktopClient) Permissions(
	ctx context.Context,
) (desktopobserve.PermissionResult, error) {
	client.mu.Lock()
	defer client.mu.Unlock()
	if err := client.requireHandshakeLocked(ctx); err != nil {
		return desktopobserve.PermissionResult{}, err
	}
	raw, err := client.runLocked(ctx, "computer", "permissions", "--json")
	if err != nil {
		return desktopobserve.PermissionResult{}, err
	}
	granted, err := decodeOrcaPermissions(raw, client.runtimeID)
	return desktopobserve.PermissionResult{AccessibilityGranted: granted}, err
}

func (client *orcaDesktopClient) ListApps(
	ctx context.Context,
) ([]desktopobserve.AppSummary, error) {
	client.mu.Lock()
	defer client.mu.Unlock()
	if err := client.requireHandshakeLocked(ctx); err != nil {
		return nil, err
	}
	raw, err := client.runLocked(ctx, "computer", "list-apps", "--json")
	if err != nil {
		return nil, err
	}
	pid, matches, err := decodeOrcaApps(raw, client.runtimeID)
	if err != nil {
		return nil, err
	}
	client.targetPID = 0
	if matches != 1 {
		apps := make([]desktopobserve.AppSummary, matches)
		for index := range apps {
			apps[index] = desktopobserve.AppSummary{AppRef: "autopus-desktop"}
		}
		return apps, nil
	}
	client.targetPID = pid
	return []desktopobserve.AppSummary{{AppRef: "autopus-desktop"}}, nil
}

func (client *orcaDesktopClient) ListWindows(
	ctx context.Context,
	appRef string,
) ([]desktopobserve.WindowSummary, error) {
	client.mu.Lock()
	defer client.mu.Unlock()
	if appRef != "autopus-desktop" || client.targetPID <= 1 {
		return nil, desktopobserve.ErrMalformedEnvelope
	}
	raw, err := client.runLocked(
		ctx, "computer", "list-windows", "--app", orcaAppBundleID, "--json",
	)
	if err != nil {
		return nil, err
	}
	binding, matches, err := decodeOrcaWindows(raw, client.runtimeID, client.targetPID)
	if err != nil {
		return nil, err
	}
	client.window = orcaWindowBinding{}
	if matches != 1 {
		windows := make([]desktopobserve.WindowSummary, matches)
		for index := range windows {
			windows[index] = desktopobserve.WindowSummary{WindowRef: "main-window"}
		}
		return windows, nil
	}
	client.window = binding
	return []desktopobserve.WindowSummary{{WindowRef: "main-window"}}, nil
}

func (client *orcaDesktopClient) GetState(
	ctx context.Context,
	appRef string,
	windowRef string,
) (desktopobserve.SemanticProjection, error) {
	client.mu.Lock()
	defer client.mu.Unlock()
	if appRef != "autopus-desktop" || windowRef != "main-window" ||
		client.window.pid <= 1 || client.window.id <= 0 {
		return desktopobserve.SemanticProjection{}, desktopobserve.ErrMalformedEnvelope
	}
	raw, err := client.runLocked(
		ctx,
		"computer", "get-app-state", "--app", orcaAppBundleID, "--no-screenshot", "--json",
	)
	if err != nil {
		return desktopobserve.SemanticProjection{}, err
	}
	return decodeOrcaState(raw, client.runtimeID, client.window, client.random)
}

func (client *orcaDesktopClient) requireHandshakeLocked(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if client.identity.Name == "" || client.runtimeID == "" || len(client.capabilities) == 0 {
		return errDesktopProviderUnavailable
	}
	return nil
}

func (client *orcaDesktopClient) runLocked(
	ctx context.Context,
	arguments ...string,
) ([]byte, error) {
	if client == nil || client.executor == nil || client.path == "" {
		return nil, errDesktopProviderUnavailable
	}
	return client.executor.Run(ctx, client.path, append([]string(nil), arguments...))
}
