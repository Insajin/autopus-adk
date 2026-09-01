package readiness

import (
	"path/filepath"
	"strings"
)

// Ref kinds name the field a rejected path came from so a blocker class points
// at the index entry the operator has to fix.
const (
	refKindManifest = "manifest_path"
	refKindRunIndex = "run_index_path"
)

func safeManifestRefs(root string, refs []string) ([]string, string) {
	return safeWorkspaceRefs(root, refKindManifest, refs)
}

func safeWorkspaceRefs(root, kind string, refs []string) ([]string, string) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return nil, "invalid_ref:workspace_root"
	}
	out := make([]string, 0, len(refs))
	for _, ref := range refs {
		clean, blocker := safeWorkspaceRef(rootAbs, kind, ref)
		if blocker != "" {
			return nil, blocker
		}
		out = append(out, clean)
	}
	return out, ""
}

func safeWorkspaceRef(rootAbs, kind, ref string) (string, string) {
	text := strings.TrimSpace(ref)
	if text == "" {
		return "", "invalid_ref:" + kind
	}
	if class := unsafeStringClass(text, kind+"s"); class != "" {
		return "", class
	}
	// Any absolute ref that survived workspace normalization resolves outside
	// the workspace root, so it can never be republished as portable evidence.
	// The condition is "outside the workspace", not "a user directory": the old
	// class name claimed a /Users or /home segment the path need not contain.
	if filepath.IsAbs(text) || filepath.IsAbs(filepath.FromSlash(text)) {
		return "", "unsafe_ref:" + kind + "_outside_workspace"
	}
	if strings.ContainsAny(text, "\x00\r\n\t:;&|$`<>") {
		return "", "unsafe_ref:invalid_" + kind
	}
	clean := filepath.Clean(filepath.FromSlash(text))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", "unsafe_ref:path_traversal"
	}
	abs := filepath.Join(rootAbs, clean)
	rel, err := filepath.Rel(rootAbs, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", "unsafe_ref:path_traversal"
	}
	return filepath.ToSlash(clean), ""
}
