package orchestra

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/insajin/autopus-adk/pkg/terminal"
)

const tmuxServerRefPrefix = "tmux-sha256:"

var validTmuxServerRef = regexp.MustCompile(`^tmux-sha256:[0-9a-f]{64}$`)

type surfaceIdentity struct {
	Ref           string
	TerminalKind  string
	WorkspaceRef  string
	TmuxServerRef string
}

// currentTmuxServerRef returns a non-routable identity derived from the tmux
// socket and server PID. The identity is only compared and is never passed to a
// command line.
func currentTmuxServerRef() (string, bool) {
	raw := strings.TrimSpace(os.Getenv("TMUX"))
	lastComma := strings.LastIndexByte(raw, ',')
	if lastComma <= 0 || lastComma == len(raw)-1 {
		return "", false
	}
	secondComma := strings.LastIndexByte(raw[:lastComma], ',')
	if secondComma <= 0 || secondComma == lastComma-1 {
		return "", false
	}
	socket := filepath.Clean(raw[:secondComma])
	pidText := raw[secondComma+1 : lastComma]
	sessionText := raw[lastComma+1:]
	pid, err := strconv.ParseUint(pidText, 10, 64)
	if err != nil || pid == 0 || !filepath.IsAbs(socket) || !hasOnlyDecimalDigits(sessionText) {
		return "", false
	}
	digest := sha256.Sum256([]byte(socket + "\x00" + strconv.FormatUint(pid, 10)))
	return tmuxServerRefPrefix + hex.EncodeToString(digest[:]), true
}

func trackedSurfaceForTerminal(term terminal.Terminal, ref string) (trackedSurface, error) {
	if term == nil {
		return trackedSurface{}, errors.New("nil terminal")
	}
	switch term.Name() {
	case "cmux":
		if !hasOnlyCmuxPanes(map[string]string{"tracked": ref}) {
			return trackedSurface{}, fmt.Errorf("invalid cmux surface ref %q", ref)
		}
		provider, ok := term.(terminal.WorkspaceContextProvider)
		if !ok {
			return trackedSurface{}, errors.New("cmux terminal has no workspace context")
		}
		workspaceRef, err := provider.WorkspaceRef()
		if err != nil {
			return trackedSurface{}, fmt.Errorf("resolve cmux workspace: %w", err)
		}
		workspaceRef, err = canonicalCmuxWorkspaceRef(workspaceRef)
		if err != nil {
			return trackedSurface{}, err
		}
		return trackedSurface{Ref: ref, TerminalKind: "cmux", WorkspaceRef: workspaceRef}, nil
	case "tmux":
		if !hasOnlyGlobalTmuxPanes(map[string]string{"tracked": ref}) {
			return trackedSurface{}, fmt.Errorf("invalid tmux pane ref %q", ref)
		}
		serverRef, ok := currentTmuxServerRef()
		if !ok {
			return trackedSurface{}, errors.New("tmux server identity is unavailable")
		}
		return trackedSurface{Ref: ref, TerminalKind: "tmux", TmuxServerRef: serverRef}, nil
	default:
		return trackedSurface{}, fmt.Errorf("unsupported terminal kind %q", term.Name())
	}
}

func trackedSurfaceIdentity(tracked trackedSurface) (surfaceIdentity, error) {
	switch tracked.TerminalKind {
	case "cmux":
		if tracked.TmuxServerRef != "" {
			return surfaceIdentity{}, errors.New("tracked cmux surface has conflicting tmux context")
		}
		if !hasOnlyCmuxPanes(map[string]string{"tracked": tracked.Ref}) {
			return surfaceIdentity{}, errors.New("tracked cmux surface reference is invalid")
		}
		workspaceRef, err := canonicalCmuxWorkspaceRef(tracked.WorkspaceRef)
		if err != nil {
			return surfaceIdentity{}, err
		}
		return surfaceIdentity{Ref: tracked.Ref, TerminalKind: "cmux", WorkspaceRef: workspaceRef}, nil
	case "tmux":
		if tracked.WorkspaceRef != "" {
			return surfaceIdentity{}, errors.New("tracked tmux surface has conflicting cmux context")
		}
		if !hasOnlyGlobalTmuxPanes(map[string]string{"tracked": tracked.Ref}) {
			return surfaceIdentity{}, errors.New("tracked tmux pane reference is invalid")
		}
		if !validTmuxServerRef.MatchString(tracked.TmuxServerRef) {
			return surfaceIdentity{}, errors.New("tracked tmux server identity is invalid")
		}
		return surfaceIdentity{
			Ref: tracked.Ref, TerminalKind: "tmux", TmuxServerRef: tracked.TmuxServerRef,
		}, nil
	default:
		return surfaceIdentity{}, errors.New("tracked surface has no proven backend identity")
	}
}

func canonicalCmuxWorkspaceRef(workspaceRef string) (string, error) {
	if _, err := terminal.NewCmuxAdapterWithWorkspace(workspaceRef); err != nil {
		return "", fmt.Errorf("invalid cmux workspace context: %w", err)
	}
	if strings.HasPrefix(workspaceRef, "workspace:") {
		index, err := strconv.ParseUint(strings.TrimPrefix(workspaceRef, "workspace:"), 10, 64)
		if err != nil {
			return "", fmt.Errorf("invalid cmux workspace context: %w", err)
		}
		return "workspace:" + strconv.FormatUint(index, 10), nil
	}
	return strings.ToLower(workspaceRef), nil
}

func sameTrackedSurface(left, right trackedSurface) bool {
	leftIdentity, leftErr := trackedSurfaceIdentity(left)
	rightIdentity, rightErr := trackedSurfaceIdentity(right)
	return leftErr == nil && rightErr == nil && leftIdentity == rightIdentity
}
