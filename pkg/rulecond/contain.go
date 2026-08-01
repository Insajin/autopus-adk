package rulecond

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Reason is a containment or size violation reason code. It is the only detail
// a fail-closed suppression is allowed to report (REQ-CONDRULE-FIRE-10).
//
// @AX:NOTE [AUTO] @AX:SPEC: SPEC-CONDRULE-001: these reason strings are operator-visible stderr evidence values; renaming one breaks diagnostics and the fail-closed oracle tests.
type Reason string

const (
	// ReasonAbsolutePath is a manifest body location that is absolute.
	ReasonAbsolutePath Reason = "absolute_path"
	// ReasonPathEscape is a body location carrying a `..` component.
	ReasonPathEscape Reason = "path_escape"
	// ReasonSymlinkEscape is a body that resolves outside the body root.
	ReasonSymlinkEscape Reason = "symlink_escape"
	// ReasonNotRegularFile is a body that is not a regular file.
	ReasonNotRegularFile Reason = "not_regular_file"
	// ReasonBadExtension is a body that does not end in `.md`.
	ReasonBadExtension Reason = "bad_extension"
	// ReasonBodyTooLarge is a body above MaxBodyBytes.
	ReasonBodyTooLarge Reason = "body_too_large"
	// ReasonBadName is a manifest rule Name that is not a bare, bounded
	// identifier. The manifest is untrusted repo-supplied input and its Name
	// reaches the injected context header, so it is contained like the body
	// location is.
	ReasonBadName Reason = "bad_name"
	// ReasonSymlinkRoot is a symlink among the in-project components leading to
	// the conditional surface. A cloned repository can ship `.claude` or the
	// conditional body root as a symlink (git mode 120000 survives a clone),
	// which moves the containment frame itself outside the checkout.
	ReasonSymlinkRoot Reason = "symlink_root"
	// ReasonHardLink is a body file with more than one directory entry. A
	// hardlink is indistinguishable from the real file after path resolution,
	// so link count is the only signal that the bytes also live out of root.
	ReasonHardLink Reason = "hard_link"
)

// ViolationError is a fail-closed containment or size failure. It carries the
// reason code and nothing else: neither the requested location, the resolved
// path, nor any byte of the target file may appear in its message.
type ViolationError struct {
	Reason Reason
}

// Error renders the reason code without any path or file content.
func (e *ViolationError) Error() string {
	return "conditional rule body refused: " + string(e.Reason)
}

// violation builds a fail-closed error for a reason code.
func violation(reason Reason) error {
	return &ViolationError{Reason: reason}
}

// ReadBody reads one manifest-named rule body under the conditional body root.
//
// root MUST come from ResolveBodyRoot. That is what makes the frame trustworthy:
// ResolveBodyRoot has already refused a symlink at every in-project component,
// so the EvalSymlinks below is an identity on the root and cannot be steered by
// a cloned repository. Passing a raw path here compares the body against
// whatever frame an attacker relocated the root to.
//
// The manifest is untrusted repo-supplied input, so the location is contained
// before the filesystem is touched: an absolute value or any `..` component is
// refused up front, the join is then resolved with filepath.EvalSymlinks, and
// the resolved path must still sit under the resolved root. A regular-file
// check, a link-count check, an `.md` check, and the per-body size cap follow,
// and the cap is applied before any byte is read (REQ-CONDRULE-FIRE-07,
// REQ-CONDRULE-FIRE-08).
//
// A containment or size failure returns a *ViolationError. A body that simply
// does not exist returns the underlying fs.ErrNotExist, which is benign absence
// rather than a violation (REQ-CONDRULE-FIRE-09).
//
// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-CONDRULE-001: read-side trust boundary — the only place a manifest-supplied location reaches the filesystem, and the check order is the containment.
// @AX:REASON: Reordering or dropping any of absolute → `..` → EvalSymlinks → withinRoot → regular → `.md` → size re-opens the dispatcher as an arbitrary file-read primitive.
// @AX:WARN [AUTO] @AX:SPEC: SPEC-CONDRULE-001: stat-then-read TOCTOU window between os.Stat and the open on the resolved path.
// @AX:REASON: The read is bounded by io.LimitReader(MaxBodyBytes+1) and rechecked afterwards, which is the mitigation for a file swapped after the stat; reverting to os.ReadFile restores an unbounded read.
func ReadBody(root, location string) ([]byte, error) {
	if isAbsoluteLocation(location) {
		return nil, violation(ReasonAbsolutePath)
	}
	if hasParentComponent(location) {
		return nil, violation(ReasonPathEscape)
	}

	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	resolved, err := filepath.EvalSymlinks(filepath.Join(root, location))
	if err != nil {
		return nil, err
	}
	if !withinRoot(resolvedRoot, resolved) {
		return nil, violation(ReasonSymlinkEscape)
	}

	info, err := os.Stat(resolved)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, violation(ReasonNotRegularFile)
	}
	if hasMultipleLinks(info) {
		return nil, violation(ReasonHardLink)
	}
	if !strings.HasSuffix(location, mdSuffix) || !strings.HasSuffix(resolved, mdSuffix) {
		return nil, violation(ReasonBadExtension)
	}
	if info.Size() > MaxBodyBytes {
		return nil, violation(ReasonBodyTooLarge)
	}

	file, err := os.Open(resolved)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, MaxBodyBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxBodyBytes {
		return nil, violation(ReasonBodyTooLarge)
	}
	return data, nil
}

// mdSuffix is the only body file extension the dispatcher will open.
const mdSuffix = ".md"

// isAbsoluteLocation reports whether a manifest body location is absolute on
// any host, so a POSIX-rooted value is refused on Windows too.
func isAbsoluteLocation(location string) bool {
	return filepath.IsAbs(location) ||
		strings.HasPrefix(location, "/") ||
		strings.HasPrefix(location, `\`)
}

// hasParentComponent reports whether a location carries a `..` component.
func hasParentComponent(location string) bool {
	for _, part := range strings.Split(filepath.ToSlash(location), "/") {
		if part == ".." {
			return true
		}
	}
	return false
}

// withinRoot reports whether a resolved path is the root itself or sits under
// it, comparing whole path components rather than a bare string prefix.
//
// @AX:NOTE [AUTO] @AX:SPEC: SPEC-CONDRULE-001: the trailing separator is what rejects a sibling such as `<root>-evil`; a bare strings.HasPrefix(path, root) would accept it.
func withinRoot(root, path string) bool {
	if path == root {
		return true
	}
	return strings.HasPrefix(path, root+string(os.PathSeparator))
}

// checkCompiledBody refuses to emit a manifest entry the dispatcher would later
// refuse, so a rule never stops firing silently (REQ-CONDRULE-COMPILE-06). The
// returned error names the rule and the reason code, and never echoes body
// content.
func checkCompiledBody(name string, body []byte) error {
	location := name + mdSuffix
	switch {
	case strings.TrimSpace(name) == "":
		return fmt.Errorf("rule %q: %w", name, violation(ReasonBadExtension))
	case isAbsoluteLocation(location):
		return fmt.Errorf("rule %s: %w", name, violation(ReasonAbsolutePath))
	case hasParentComponent(location) || strings.ContainsAny(name, `/\`):
		return fmt.Errorf("rule %s: %w", name, violation(ReasonPathEscape))
	case len(body) > MaxBodyBytes:
		return fmt.Errorf("rule %s: %w", name, violation(ReasonBodyTooLarge))
	}
	return nil
}
