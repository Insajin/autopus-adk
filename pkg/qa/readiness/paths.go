package readiness

import (
	"path/filepath"
	"strings"
)

func safeManifestRefs(root string, refs []string) ([]string, string) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return nil, "invalid_ref:workspace_root"
	}
	out := make([]string, 0, len(refs))
	for _, ref := range refs {
		clean, blocker := safeManifestRef(rootAbs, ref)
		if blocker != "" {
			return nil, blocker
		}
		out = append(out, clean)
	}
	return out, ""
}

func safeManifestRef(rootAbs, ref string) (string, string) {
	text := strings.TrimSpace(ref)
	if text == "" {
		return "", "invalid_ref:manifest_path"
	}
	if class := unsafeStringClass(text, "manifest_paths"); class != "" {
		return "", class
	}
	if filepath.IsAbs(text) || filepath.IsAbs(filepath.FromSlash(text)) {
		return "", "unsafe_ref:absolute_local_user_path"
	}
	if strings.ContainsAny(text, "\x00\r\n\t:;&|$`<>") {
		return "", "unsafe_ref:invalid_manifest_path"
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
