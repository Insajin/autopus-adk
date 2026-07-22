package orchestra

import (
	"io/fs"
	"os"
	"strings"

	"github.com/insajin/autopus-adk/pkg/terminal"
)

const maxSessionClaimFileSize = 64 << 20

type sessionSurfaceClaims struct {
	exact          map[surfaceIdentity]struct{}
	legacyTmuxRefs map[string]struct{}
}

func newSessionSurfaceClaims() sessionSurfaceClaims {
	return sessionSurfaceClaims{
		exact:          make(map[surfaceIdentity]struct{}),
		legacyTmuxRefs: make(map[string]struct{}),
	}
}

func (claims sessionSurfaceClaims) owns(tracked trackedSurface, current terminal.Terminal) bool {
	identity, err := trackedSurfaceIdentity(tracked)
	if err == nil {
		_, ok := claims.exact[identity]
		return ok
	}
	if tracked.TerminalKind != "" || current == nil || current.Name() != "tmux" ||
		!hasOnlyGlobalTmuxPanes(map[string]string{"tracked": tracked.Ref}) {
		return false
	}
	if _, ok := currentTmuxServerRef(); !ok {
		return false
	}
	_, ok := claims.legacyTmuxRefs[tracked.Ref]
	return ok
}

func loadPersistedSessionSurfaceClaims() sessionSurfaceClaims {
	claims := newSessionSurfaceClaims()
	loadPrivateSessionSurfaceClaims(claims)
	loadLegacySessionSurfaceClaims(claims)
	return claims
}

func loadPrivateSessionSurfaceClaims(claims sessionSurfaceClaims) {
	root, err := openSessionRoot(false)
	if err != nil {
		return
	}
	defer func() { _ = root.Close() }()
	if !surfaceDirSecure(sessionDirectoryPath()) {
		return
	}
	loadSessionSurfaceClaimsFromRoot(claims, root, func(name string) (string, bool) {
		if !strings.HasSuffix(name, ".json") {
			return "", false
		}
		id := strings.TrimSuffix(name, ".json")
		expected, err := sessionFilename(id)
		return id, err == nil && expected == name
	})
}

func loadLegacySessionSurfaceClaims(claims sessionSurfaceClaims) {
	root, err := openLegacySessionRoot()
	if err != nil {
		return
	}
	defer func() { _ = root.Close() }()
	loadSessionSurfaceClaimsFromRoot(claims, root, func(name string) (string, bool) {
		if !strings.HasPrefix(name, legacySessionFilenamePrefix) || !strings.HasSuffix(name, ".json") {
			return "", false
		}
		id := strings.TrimSuffix(strings.TrimPrefix(name, legacySessionFilenamePrefix), ".json")
		expected, err := legacySessionFilename(id)
		return id, err == nil && expected == name
	})
}

func loadSessionSurfaceClaimsFromRoot(
	claims sessionSurfaceClaims,
	root *os.Root,
	parseName func(string) (string, bool),
) {
	entries, err := fs.ReadDir(root.FS(), ".")
	if err != nil {
		return
	}
	for _, entry := range entries {
		name := entry.Name()
		id, ok := parseName(name)
		if entry.IsDir() || !ok {
			continue
		}
		info, err := root.Lstat(name)
		if err != nil || !sessionClaimFileInfoSecure(info) {
			continue
		}
		session, openedInfo, err := readSessionEntry(root, name, id)
		if err != nil || !sessionClaimFileInfoSecure(openedInfo) ||
			!os.SameFile(info, openedInfo) {
			continue
		}
		claims.addSession(session)
	}
}

func sessionClaimFileInfoSecure(info fs.FileInfo) bool {
	return info != nil && info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0 &&
		info.Size() >= 0 && info.Size() <= maxSessionClaimFileSize &&
		trackerInfoOwnedByCurrentUser(info) && trackerModeSecure(info, 0o600)
}

func (claims sessionSurfaceClaims) addSession(session *OrchestraSession) {
	if session == nil || len(session.Panes) == 0 {
		return
	}
	switch session.TerminalKind {
	case "cmux":
		if session.TmuxServerRef != "" {
			return
		}
		workspaceRef, err := canonicalCmuxWorkspaceRef(session.WorkspaceRef)
		if err != nil || !hasOnlyCmuxPanes(session.Panes) {
			return
		}
		for _, ref := range session.Panes {
			claims.exact[surfaceIdentity{
				Ref: ref, TerminalKind: "cmux", WorkspaceRef: workspaceRef,
			}] = struct{}{}
		}
	case "tmux":
		if session.WorkspaceRef != "" || !validTmuxServerRef.MatchString(session.TmuxServerRef) ||
			!hasOnlyGlobalTmuxPanes(session.Panes) {
			return
		}
		for _, ref := range session.Panes {
			claims.exact[surfaceIdentity{
				Ref: ref, TerminalKind: "tmux", TmuxServerRef: session.TmuxServerRef,
			}] = struct{}{}
		}
	case "":
		if session.WorkspaceRef != "" || session.TmuxServerRef != "" ||
			!hasOnlyGlobalTmuxPanes(session.Panes) {
			return
		}
		for _, ref := range session.Panes {
			claims.legacyTmuxRefs[ref] = struct{}{}
		}
	}
}
