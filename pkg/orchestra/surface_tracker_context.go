package orchestra

import (
	"errors"
	"strings"

	"github.com/insajin/autopus-adk/pkg/terminal"
)

var newTrackedCmuxTerminal = func(workspaceRef string) (terminal.Terminal, error) {
	return terminal.NewCmuxAdapterWithWorkspace(workspaceRef)
}

func resolveTrackedSurfaceTerminal(current terminal.Terminal, tracked trackedSurface) (terminal.Terminal, error) {
	if current == nil {
		return nil, errors.New("no terminal context available")
	}
	if tracked.TerminalKind == "" {
		if strings.HasPrefix(tracked.Ref, "%") {
			if current.Name() != "tmux" {
				return nil, errors.New("legacy tmux surface requires an active tmux context")
			}
			if _, ok := currentTmuxServerRef(); !ok {
				return nil, errors.New("active tmux server identity is unavailable")
			}
			return current, nil
		}
		return nil, errors.New("legacy surface has no proven workspace context")
	}
	if tracked.TerminalKind == "tmux" {
		identity, err := trackedSurfaceIdentity(tracked)
		if err != nil {
			return nil, err
		}
		if current.Name() != "tmux" {
			return nil, errors.New("tracked tmux surface requires an active tmux context")
		}
		currentRef, ok := currentTmuxServerRef()
		if !ok {
			return nil, errors.New("active tmux server identity is unavailable")
		}
		if currentRef != identity.TmuxServerRef {
			return nil, errors.New("tracked tmux surface belongs to a different server")
		}
		return current, nil
	}
	if tracked.TerminalKind != "cmux" {
		return nil, errors.New("unsupported tracked terminal backend")
	}
	identity, err := trackedSurfaceIdentity(tracked)
	if err != nil {
		return nil, err
	}
	if current.Name() == "cmux" {
		provider, ok := current.(terminal.WorkspaceContextProvider)
		if !ok {
			return nil, errors.New("cmux terminal cannot restore tracked workspace context")
		}
		return provider.WithWorkspaceRef(identity.WorkspaceRef)
	}
	return newTrackedCmuxTerminal(identity.WorkspaceRef)
}
