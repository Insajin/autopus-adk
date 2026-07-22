package orchestra

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/insajin/autopus-adk/pkg/terminal"
)

var yieldSurfaceUntracker = untrackSurfaceForTerminal

func buildYieldSession(sessionID string, term terminal.Terminal, panes []paneInfo, responses []ProviderResponse, createdAt time.Time) (OrchestraSession, error) {
	terminalKind, workspaceRef, tmuxServerRef, err := sessionTerminalContext(term)
	if err != nil {
		return OrchestraSession{}, err
	}
	session := OrchestraSession{
		ID: sessionID, TerminalKind: terminalKind, WorkspaceRef: workspaceRef,
		TmuxServerRef: tmuxServerRef, Panes: make(map[string]string, len(panes)),
		Providers: make([]SessionProviderConfig, 0, len(panes)),
		Rounds:    make([][]SessionProviderResponse, 0, 1), CreatedAt: createdAt,
	}
	for _, pi := range panes {
		session.Panes[pi.provider.Name] = string(pi.paneID)
		session.Providers = append(session.Providers, SessionProviderConfig{
			Name: pi.provider.Name, Binary: pi.provider.Binary,
		})
	}
	round := make([]SessionProviderResponse, 0, len(responses))
	for _, response := range responses {
		round = append(round, SessionProviderResponse{
			Provider: response.Provider, Output: response.Output,
			DurationMs: response.Duration.Milliseconds(), TimedOut: response.TimedOut,
			Usage: response.Usage, UsageCapability: response.UsageCapability,
		})
	}
	session.Rounds = append(session.Rounds, round)
	return session, nil
}

func handoffYieldPanes(sessionID string, term terminal.Terminal, panes []paneInfo) error {
	var handoffErrors []error
	for _, pane := range panes {
		ref := string(pane.paneID)
		if err := yieldSurfaceUntracker(term, ref); err != nil {
			log.Printf("[yield] session %s is durable but tracker handoff failed for %s: %v", sessionID, ref, err)
			handoffErrors = append(handoffErrors, fmt.Errorf("untrack %s: %w", ref, err))
		}
	}
	if len(handoffErrors) == 0 {
		return nil
	}
	return fmt.Errorf("yield session %s is durable but tracker handoff failed; recover with auto orchestra cleanup --session-id %s: %w",
		sessionID, sessionID, errors.Join(handoffErrors...))
}
