package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// exitCodeUnsupportedTopology separates "sync verify does not apply to this
// directory" from the exit code a classification violation produces under
// --strict. An agent reading only the exit code must still tell them apart.
const exitCodeUnsupportedTopology = 2

// syncTopologyError reports a directory sync verify cannot plan for at all:
// no Git repository, or a Git repository that is not an Autopus workspace.
// It is deliberately not a classification result — nothing was classified.
type syncTopologyError struct{ reason string }

func (e *syncTopologyError) Error() string { return "unsupported topology: " + e.reason }

// ExitCode satisfies exitCoder so the process exit distinguishes an
// inapplicable workspace from errSyncVerifyStrict's blocked-path failure.
func (e *syncTopologyError) ExitCode() int { return exitCodeUnsupportedTopology }

// syncTopology is the resolved workspace layout the plan is built against.
type syncTopology struct {
	// Root is the absolute directory every relative path is resolved from.
	Root string
	// Single marks a lone Git repository with no module boundary.
	Single bool
}

// line is the topology banner printed above every plan. The meta root is named
// by its workspace-relative label so no absolute path reaches the output.
func (t syncTopology) line() string {
	if t.Single {
		return "topology: single-repo"
	}
	return "topology: multi-repo (meta root .)"
}

// resolveSyncTopology prefers a multi-repo meta root and otherwise falls back
// to the single Git repository containing startDir. A linked worktree resolves
// to its own worktree root, which is where its dirty paths live.
//
// Workspace detection only stats a ".git" entry, so a stale or half-created
// one still anchors a meta root. Git cannot plan a Phase B commit there, so
// the meta root is probed before it is trusted; otherwise the run dies later
// with an opaque "git status failed" instead of a topology diagnostic.
func resolveSyncTopology(startDir string) (syncTopology, error) {
	metaRoot, metaErr := resolveMetaRoot(startDir)
	if metaErr != nil {
		return resolveSingleRepoTopology(startDir)
	}
	if _, err := resolveGitWorktreeRoot(metaRoot); err == nil {
		return syncTopology{Root: metaRoot}, nil
	}
	// A single repository under the cursor is still plannable, so prefer it and
	// only blame the meta root when nothing here can be planned at all.
	if single, err := resolveSingleRepoTopology(startDir); err == nil {
		return single, nil
	}
	return syncTopology{}, &syncTopologyError{reason: fmt.Sprintf(
		"multi-repo meta root %s is not a Git repository (initialize it or run from a module)",
		displayPath(filepath.Base(metaRoot)))}
}

// resolveSingleRepoTopology accepts the lone Git repository containing
// startDir when its worktree root is an Autopus workspace.
func resolveSingleRepoTopology(startDir string) (syncTopology, error) {
	repoRoot, err := resolveGitWorktreeRoot(startDir)
	if err != nil {
		return syncTopology{}, err
	}
	if _, err := os.Stat(filepath.Join(repoRoot, "autopus.yaml")); err != nil {
		return syncTopology{}, &syncTopologyError{reason: "Git repository has no autopus.yaml " +
			"at its worktree root, so it is not an Autopus workspace"}
	}
	return syncTopology{Root: repoRoot, Single: true}, nil
}

// resolveGitWorktreeRoot asks Git for the enclosing worktree root, which is the
// only answer that stays correct for a linked worktree whose .git is a file.
func resolveGitWorktreeRoot(startDir string) (string, error) {
	raw, err := runSyncGit(".", startDir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", &syncTopologyError{reason: "no multi-repo workspace and no Git " +
			"repository found from the current directory"}
	}
	root := strings.TrimSpace(string(raw))
	if root == "" {
		return "", &syncTopologyError{reason: "Git reported no worktree root for the current directory"}
	}
	return root, nil
}

// collectTopologyDirty gathers the dirty inventory for the resolved topology.
func collectTopologyDirty(topology syncTopology) ([]repoDirty, error) {
	if topology.Single {
		return collectSingleRepoDirty(topology.Root)
	}
	return collectDirty(topology.Root)
}

// collectSingleRepoDirty inventories one repository as a whole. There is no
// module boundary, so every dirty path shares a single commit group while the
// generated/runtime and tracked-but-ignored exclusions still apply.
func collectSingleRepoDirty(root string) ([]repoDirty, error) {
	snapshot, err := capturePorcelainSnapshot(".", root)
	if err != nil {
		return nil, err
	}
	ignoredRaw, err := runSyncGit(".", root, "ls-files", "-c", "-i", "--exclude-standard", "-z")
	if err != nil {
		return nil, err
	}
	ignored, err := parseNULPaths(ignoredRaw)
	if err != nil {
		return nil, fmt.Errorf("malformed tracked-but-ignored inventory for repo %s", diagnosticRepoLabel("."))
	}
	return []repoDirty{{
		Path:           ".",
		AbsPath:        root,
		Role:           repoRoleSingle,
		Files:          snapshot.Files,
		TrackedIgnored: ignored,
	}}, nil
}
