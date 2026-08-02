package rulecond

import (
	"os"
	"path/filepath"
	"strings"
)

// The trusted-root contract shared by the dispatcher and by internal/cli.
//
// The project root is the trust anchor. It is discovered from the process
// working directory by internal/cli/rules.go::resolveProjectRoot and is the one
// path in this package that may legitimately sit behind a symlink, because the
// system prefix above a checkout routinely does (macOS resolves /var to
// /private/var, and /tmp to /private/tmp). That prefix is resolved once, for the
// anchor only.
//
// Everything below the anchor is repo-supplied and therefore untrusted. A
// cloned repository can ship `.claude`, or any directory under it, as a symlink
// with git mode 120000, which survives the clone. Resolving those components
// would move the containment frame itself outside the checkout and make every
// later within-root comparison meaningless, so each in-project component is
// inspected with os.Lstat and a symlink is refused outright.
//
// Every resolver here answers a read, and the SPEC-STICKYRULE-001 counter state
// directory is deliberately absent from the list. That directory is the one
// location this runtime creates, writes, and sweeps, and a resolved path cannot
// carry containment across a write: a path states only what a name meant at the
// instant it was resolved, while every later syscall resolves that name again.
// The write side therefore holds a directory handle instead — openStateRoot in
// sticky_state_dir.go — so a path-shaped resolver for the state directory does
// not belong here.
//
// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-CONDRULE-001: trusted-root contract — the single definition of which path components may be symlinks, shared by the manifest read and the body read.
// @AX:REASON: Resolving in-project components instead of refusing them reopens the dispatcher as an out-of-repo file-read primitive; the write side enforces the same rule in pkg/adapter/transaction_helpers.go::rejectTransactionSymlinkComponents.
//
// @AX:NOTE [AUTO] @AX:SPEC: SPEC-STICKYRULE-001: the counter state directory has no resolver in this file by design; its containment frame is the openStateRoot handle in sticky_state_dir.go.
// @AX:REASON: A path-shaped state resolver reads as the write-side guard while proving nothing about where a later write lands, which is what hid a leaf-write escape once already.

// ResolveManifestPath returns the conditional manifest path under projectRoot,
// refusing any symlink among the in-project components leading to it.
func ResolveManifestPath(projectRoot string) (string, error) {
	return containedProjectPath(projectRoot, ManifestRelPath)
}

// ResolveBodyRoot returns the conditional body root under projectRoot, refusing
// any symlink among the in-project components leading to it. The returned path
// is fully resolved at the system prefix and symlink-free below it, so it is a
// sound frame for the within-root comparison ReadBody performs.
func ResolveBodyRoot(projectRoot string) (string, error) {
	return containedProjectPath(projectRoot, BodyRootRelPath)
}

// ResolveStickyBodyRoot returns the SPEC-STICKYRULE-001 sticky body root under
// projectRoot, refusing any symlink among the in-project components leading to
// it.
//
// The sticky body root is the baseline rule directory itself, because a sticky
// rule keeps its baseline placement and is therefore already installed there. It
// is a different root from ResolveBodyRoot but the same contract: the returned
// path is resolved at the system prefix and symlink-free below it, so it is a
// sound frame for the within-root comparison ReadBody performs. A cloned
// repository that ships `.claude/rules/autopus` as a symlink is refused here
// rather than silently relocating the frame.
//
// @AX:NOTE [AUTO] @AX:SPEC: SPEC-STICKYRULE-001: two distinct roots share one contract — ResolveBodyRoot bounds relocated hook-fired bodies under BodyRootRelPath, this one bounds sticky bodies under ClaudeRulesRelDir; swapping them silently widens what either runtime may read.
func ResolveStickyBodyRoot(projectRoot string) (string, error) {
	return containedProjectPath(projectRoot, ClaudeRulesRelDir)
}

// containedProjectPath joins relPath onto the resolved project root and refuses
// a symlink at any component of relPath, including the last. A component that
// does not exist yet is not a violation: absence is reported later by the read
// itself, which keeps a missing manifest fail-open.
func containedProjectPath(projectRoot, relPath string) (string, error) {
	base, err := filepath.EvalSymlinks(projectRoot)
	if err != nil {
		return "", err
	}

	current := base
	for _, component := range strings.Split(filepath.FromSlash(relPath), string(os.PathSeparator)) {
		if component == "" {
			continue
		}
		current = filepath.Join(current, component)
		info, lstatErr := os.Lstat(current)
		if os.IsNotExist(lstatErr) {
			// Nothing is there to be a symlink. Return the intended location so
			// the caller's own open reports the absence.
			return filepath.Join(base, filepath.FromSlash(relPath)), nil
		}
		if lstatErr != nil {
			return "", lstatErr
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", violation(ReasonSymlinkRoot)
		}
	}
	return current, nil
}
