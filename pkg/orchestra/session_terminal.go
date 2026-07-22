package orchestra

import (
	"errors"
	"fmt"
	"strings"

	"github.com/insajin/autopus-adk/pkg/terminal"
)

// ResolveSessionTerminal restores the terminal context persisted with a yield
// session. Legacy cmux sessions fail closed because surface refs are only
// meaningful inside their original workspace. Legacy tmux panes are accepted
// only from an actively detected tmux context.
func ResolveSessionTerminal(session *OrchestraSession, detected terminal.Terminal) (terminal.Terminal, error) {
	if session == nil {
		return nil, errors.New("nil orchestra session")
	}
	if len(session.Panes) == 0 {
		return detected, nil
	}
	switch session.TerminalKind {
	case "":
		if hasOnlyGlobalTmuxPanes(session.Panes) {
			if detected == nil || detected.Name() != "tmux" {
				return nil, errors.New("legacy tmux session requires an active tmux context")
			}
			return detected, nil
		}
		return nil, errors.New("legacy session has no proven terminal workspace context")
	case "cmux":
		return resolveCmuxSessionTerminal(session, detected)
	case "tmux":
		return resolveTmuxSessionTerminal(session, detected)
	case "plain":
		return nil, errors.New("plain terminal cannot operate persisted panes")
	default:
		return nil, fmt.Errorf("unsupported persisted terminal kind %q", session.TerminalKind)
	}
}

// ResolveSessionTerminalWithWorkspace applies an explicit recovery capability
// only to metadata-less cmux sessions created before workspace persistence.
func ResolveSessionTerminalWithWorkspace(session *OrchestraSession, detected terminal.Terminal, workspaceRef string) (terminal.Terminal, error) {
	if workspaceRef == "" {
		return ResolveSessionTerminal(session, detected)
	}
	if session == nil {
		return nil, errors.New("nil orchestra session")
	}
	if session.TerminalKind != "" || session.WorkspaceRef != "" || session.TmuxServerRef != "" {
		return nil, errors.New("workspace override is only valid for a legacy cmux session")
	}
	if len(session.Panes) == 0 {
		return terminal.NewCmuxAdapterWithWorkspace(workspaceRef)
	}
	if !hasOnlyCmuxPanes(session.Panes) {
		return nil, errors.New("workspace override requires legacy cmux pane references")
	}
	legacy := *session
	legacy.TerminalKind = "cmux"
	legacy.WorkspaceRef = workspaceRef
	term, err := resolveCmuxSessionTerminal(&legacy, detected)
	if err != nil {
		return nil, fmt.Errorf("invalid workspace override: %w", err)
	}
	return term, nil
}

func resolveCmuxSessionTerminal(session *OrchestraSession, detected terminal.Terminal) (terminal.Terminal, error) {
	if session.TmuxServerRef != "" {
		return nil, errors.New("cmux session contains conflicting tmux server context")
	}
	if !hasOnlyCmuxPanes(session.Panes) {
		return nil, errors.New("cmux session contains an incompatible pane reference")
	}
	if session.WorkspaceRef == "" {
		return nil, errors.New("cmux session has no persisted workspace context")
	}
	if _, err := terminal.NewCmuxAdapterWithWorkspace(session.WorkspaceRef); err != nil {
		return nil, fmt.Errorf("invalid persisted cmux workspace context: %w", err)
	}
	if detected != nil && detected.Name() == "cmux" {
		provider, ok := detected.(terminal.WorkspaceContextProvider)
		if !ok {
			return nil, errors.New("cmux terminal cannot restore persisted workspace context")
		}
		return provider.WithWorkspaceRef(session.WorkspaceRef)
	}
	return terminal.NewCmuxAdapterWithWorkspace(session.WorkspaceRef)
}

func resolveTmuxSessionTerminal(session *OrchestraSession, detected terminal.Terminal) (terminal.Terminal, error) {
	if !hasOnlyGlobalTmuxPanes(session.Panes) {
		return nil, errors.New("tmux session contains an incompatible pane reference")
	}
	if session.WorkspaceRef != "" {
		return nil, errors.New("tmux session contains conflicting cmux workspace context")
	}
	if detected == nil || detected.Name() != "tmux" {
		return nil, errors.New("persisted tmux session requires an active tmux context")
	}
	currentRef, ok := currentTmuxServerRef()
	if !ok || session.TmuxServerRef == "" {
		return nil, errors.New("tmux server identity cannot be proven")
	}
	if !validTmuxServerRef.MatchString(session.TmuxServerRef) {
		return nil, errors.New("persisted tmux server identity is invalid")
	}
	if currentRef != session.TmuxServerRef {
		return nil, errors.New("tmux server identity does not match persisted session")
	}
	return detected, nil
}

func hasOnlyCmuxPanes(panes map[string]string) bool {
	if len(panes) == 0 {
		return false
	}
	for _, ref := range panes {
		prefix := "surface:"
		if strings.HasPrefix(ref, "pane:") {
			prefix = "pane:"
		}
		if !strings.HasPrefix(ref, prefix) || !hasOnlyDecimalDigits(ref[len(prefix):]) {
			return false
		}
	}
	return true
}

func hasOnlyGlobalTmuxPanes(panes map[string]string) bool {
	if len(panes) == 0 {
		return false
	}
	for _, ref := range panes {
		if len(ref) < 2 || ref[0] != '%' || !hasOnlyDecimalDigits(ref[1:]) {
			return false
		}
	}
	return true
}

func hasOnlyDecimalDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

func sessionTerminalContext(term terminal.Terminal) (string, string, string, error) {
	if term == nil || term.Name() == "plain" {
		return "", "", "", errors.New("yield session requires a pane-capable terminal")
	}
	kind := term.Name()
	if kind == "tmux" {
		serverRef, ok := currentTmuxServerRef()
		if !ok {
			return "", "", "", errors.New("cannot persist tmux server identity")
		}
		return kind, "", serverRef, nil
	}
	if kind != "cmux" {
		return "", "", "", fmt.Errorf("unsupported yield terminal kind %q", kind)
	}
	provider, ok := term.(terminal.WorkspaceContextProvider)
	if !ok {
		return "", "", "", errors.New("cmux terminal cannot persist workspace context")
	}
	workspaceRef, err := provider.WorkspaceRef()
	if err != nil {
		return "", "", "", fmt.Errorf("resolve cmux workspace context: %w", err)
	}
	if _, err := terminal.NewCmuxAdapterWithWorkspace(workspaceRef); err != nil {
		return "", "", "", fmt.Errorf("validate cmux workspace context: %w", err)
	}
	return kind, workspaceRef, "", nil
}
