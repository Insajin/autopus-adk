package readiness

import (
	"path/filepath"
	"strings"
)

// workspacePathNormalizer rewrites absolute paths that lie inside the workspace
// root back into workspace-relative form before any safety classifier runs.
//
// The evidence producers record manifest, run-index, and feedback refs using
// whatever project directory they were handed, so an absolute --project-dir
// (the documented way to target another project, and the norm in CI) puts
// absolute paths into the very indexes this package consumes. Normalizing at
// the consumer boundary fixes every such ref for whichever producer wrote it,
// and guarantees the projection itself can never republish an absolute local
// path. Paths outside the root are left untouched, so a genuine
// /Users/<name>/... leak still reaches the classifiers and is still rejected.
type workspacePathNormalizer struct {
	roots []string
}

// newWorkspacePathNormalizer resolves the workspace root into the forms a
// producer may have written. Both the lexical and the symlink-evaluated form
// are kept because the two sides of the comparison are produced independently:
// on macOS a project reached through $TMPDIR is /var/folders/... to the shell
// that ran the producer and /private/var/folders/... to Go's os.Getwd in the
// consumer, and a compare in one form alone rejects the harness's own output.
func newWorkspacePathNormalizer(root string) workspacePathNormalizer {
	abs, err := filepath.Abs(root)
	if err != nil {
		return workspacePathNormalizer{}
	}
	return workspacePathNormalizer{roots: pathForms(abs)}
}

// normalizeDoc rewrites every in-workspace absolute path string in doc, in
// place, including nested maps and arrays. Only whole strings that parse as
// absolute paths are considered, so free text that merely mentions a path is
// left for the redaction classifiers.
func (n workspacePathNormalizer) normalizeDoc(doc map[string]any) {
	for key, value := range doc {
		doc[key] = n.normalizeValue(value)
	}
}

func (n workspacePathNormalizer) normalizeValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		n.normalizeDoc(typed)
		return typed
	case []any:
		for i, item := range typed {
			typed[i] = n.normalizeValue(item)
		}
		return typed
	case string:
		return n.normalizePath(typed)
	default:
		return value
	}
}

func (n workspacePathNormalizer) normalizePath(value string) string {
	if !filepath.IsAbs(value) {
		return value
	}
	// The stored path is resolved as well as the root: normalizing only one
	// side leaves the two in opposite symlink forms and containment fails.
	for _, candidate := range pathForms(filepath.Clean(value)) {
		for _, root := range n.roots {
			rel, err := filepath.Rel(root, candidate)
			if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				continue
			}
			return filepath.ToSlash(rel)
		}
	}
	return value
}

// pathForms returns the lexical path plus its symlink-resolved form when they
// differ. Resolution never widens the guard: a ref that resolves out of the
// workspace simply fails to match and stays subject to the classifiers.
func pathForms(path string) []string {
	forms := []string{path}
	if resolved := resolveExistingPrefix(path); resolved != path {
		forms = append(forms, resolved)
	}
	return forms
}

// resolveExistingPrefix symlink-resolves the longest existing ancestor of path
// and re-appends the remainder, so a ref naming a file that was deleted (or
// never written) still resolves through the real directory chain.
func resolveExistingPrefix(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	parent := filepath.Dir(path)
	if parent == path {
		return path
	}
	return filepath.Join(resolveExistingPrefix(parent), filepath.Base(path))
}
