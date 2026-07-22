package orchestra

import (
	"context"
	"fmt"
	"os"

	"github.com/insajin/autopus-adk/pkg/terminal"
)

func recoveryHookSession(cfg OrchestraConfig) (*HookSession, error) {
	if !cfg.HookMode {
		return nil, nil
	}
	if cfg.SessionID == "" {
		return nil, fmt.Errorf("hook recovery requires a session ID")
	}
	if cfg.SurfaceMgr == nil || cfg.SurfaceMgr.hookSession == nil {
		return nil, fmt.Errorf("hook recovery requires the active hook session")
	}
	session := cfg.SurfaceMgr.hookSession
	if session.SessionID() != cfg.SessionID {
		return nil, fmt.Errorf(
			"active hook session %q does not match config session %q",
			session.SessionID(), cfg.SessionID,
		)
	}
	return session, nil
}

// launchRecoveryProvider installs fresh-pane hook coordinates before launching
// the provider. A partially installed hook environment is fatal because the
// provider would otherwise run without a usable completion protocol.
func launchRecoveryProvider(
	ctx context.Context,
	cfg OrchestraConfig,
	paneID terminal.PaneID,
	provider ProviderConfig,
	round int,
) error {
	hookSession, err := recoveryHookSession(cfg)
	if err != nil {
		return err
	}
	hasHookCapability := hookSession != nil &&
		(hookSession.HasHook(provider.Name) || hookSession.HasStartupHook(provider.Name))
	if hasHookCapability {
		if err := hookSession.ResetAttempt(provider.Name, round); err != nil {
			return fmt.Errorf("reset hook attempt for %s: %w", provider.Name, err)
		}
		sendErr, enterErr := sendPaneInputAndEnterSerialized(ctx, cfg.Terminal, paneID, 0, func() error {
			return SendSessionEnvToPane(ctx, cfg.Terminal, paneID, cfg.SessionID)
		})
		if sendErr != nil {
			return fmt.Errorf("export hook session failed: %w", sendErr)
		}
		if enterErr != nil {
			return fmt.Errorf("commit hook session export failed: %w", enterErr)
		}

		roundErr, roundEnterErr := sendPaneInputAndEnterSerialized(ctx, cfg.Terminal, paneID, 0, func() error {
			return SendRoundEnvToPane(ctx, cfg.Terminal, paneID, round)
		})
		if roundErr != nil {
			return fmt.Errorf("export hook round failed: %w", roundErr)
		}
		if roundEnterErr != nil {
			return fmt.Errorf("commit hook round export failed: %w", roundEnterErr)
		}
	}

	cmd := buildInteractiveLaunchCmdWithCWD(provider, "", cfg.WorkingDir)
	launchErr, launchEnterErr := sendPaneInputAndEnterSerialized(ctx, cfg.Terminal, paneID, promptRegisterDelay, func() error {
		return cfg.Terminal.SendLongText(ctx, paneID, cmd)
	})
	if launchErr != nil {
		return fmt.Errorf("launch for %s failed: %w", provider.Name, launchErr)
	}
	if launchEnterErr != nil {
		return fmt.Errorf("launch enter for %s failed: %w", provider.Name, launchEnterErr)
	}
	return nil
}

func waitForRecoveredProviderReady(
	ctx context.Context,
	cfg OrchestraConfig,
	paneID terminal.PaneID,
	provider ProviderConfig,
	round int,
) bool {
	hookSession, err := recoveryHookSession(cfg)
	if err != nil {
		return false
	}
	if !waitForPaneReady(
		ctx,
		cfg.Terminal,
		paneID,
		SessionReadyPatterns(),
		startupTimeoutFor(provider),
		hookSession,
		provider.Name,
		round,
	) {
		return false
	}
	if hookSession == nil || !hookSession.HasStartupHook(provider.Name) {
		return true
	}
	readyName := RoundSignalName(provider.Name, round, "ready")
	if err := hookSession.removeArtifact(readyName); err != nil && !os.IsNotExist(err) {
		return false
	}
	return true
}
