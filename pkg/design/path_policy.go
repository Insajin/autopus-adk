package design

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// @AX:NOTE [AUTO]: Extension allow-list and sensitive path deny-lists define what local design context may be read.
var allowedDesignExts = map[string]bool{
	".md":       true,
	".markdown": true,
	".txt":      true,
}

var sensitiveNames = map[string]bool{
	".env": true, ".npmrc": true, ".pypirc": true,
	"credentials": true, "credentials.json": true,
	"secrets": true, "secrets.json": true, "id_rsa": true,
}

var sensitiveDirs = map[string]bool{
	".git": true, ".ssh": true, ".aws": true, ".gnupg": true,
	"node_modules": true, "vendor": true, "dist": true, ".cache": true,
}

// @AX:WARN [AUTO]: Path traversal and symlink containment check has high branch count by design.
// @AX:REASON: Local DESIGN.md paths are user-controlled; removing any normalization or root check can expose sensitive files.
func ResolveDesignPath(root, rawPath string) (string, *Diagnostic) {
	if strings.TrimSpace(rawPath) == "" {
		return "", &Diagnostic{Path: rawPath, Category: CategoryMissingPath, Message: "empty design path"}
	}
	if hasParentTraversal(rawPath) {
		return "", &Diagnostic{Path: rawPath, Category: CategoryParentTraversal, Message: "parent traversal is not allowed"}
	}
	cleanRaw := filepath.Clean(rawPath)

	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", &Diagnostic{Path: rawPath, Category: CategoryOutsideRoot, Message: err.Error()}
	}
	if evaluatedRoot, err := filepath.EvalSymlinks(rootAbs); err == nil {
		rootAbs = evaluatedRoot
	}
	candidate := cleanRaw
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(rootAbs, candidate)
	}
	abs, err := filepath.Abs(candidate)
	if err != nil {
		return "", &Diagnostic{Path: rawPath, Category: CategoryOutsideRoot, Message: err.Error()}
	}
	if !isInsideRoot(rootAbs, abs) {
		return "", &Diagnostic{Path: rawPath, Category: CategoryOutsideRoot, Message: "path resolves outside project root"}
	}
	if diag := rejectSensitive(rootAbs, abs, rawPath); diag != nil {
		return "", diag
	}
	if !allowedDesignExts[strings.ToLower(filepath.Ext(abs))] {
		return "", &Diagnostic{Path: rawPath, Category: CategoryUnsupportedExtension, Message: "design context must be markdown or text"}
	}
	evaluated, err := filepath.EvalSymlinks(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return "", &Diagnostic{Path: rawPath, Category: CategoryMissingPath, Message: "path does not exist"}
		}
		return "", &Diagnostic{Path: rawPath, Category: CategoryOutsideRoot, Message: err.Error()}
	}
	if !isInsideRoot(rootAbs, evaluated) {
		return "", &Diagnostic{Path: rawPath, Category: CategorySymlinkEscape, Message: "symlink resolves outside project root"}
	}
	return evaluated, nil
}

func relPath(root, abs string) string {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return filepath.ToSlash(abs)
	}
	if evaluatedRoot, err := filepath.EvalSymlinks(rootAbs); err == nil {
		rootAbs = evaluatedRoot
	}
	rel, err := filepath.Rel(rootAbs, abs)
	if err != nil {
		return filepath.ToSlash(abs)
	}
	return filepath.ToSlash(rel)
}

func hasParentTraversal(path string) bool {
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		if part == ".." {
			return true
		}
	}
	return false
}

func isInsideRoot(rootAbs, pathAbs string) bool {
	rel, err := filepath.Rel(rootAbs, pathAbs)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != "..")
}

func rejectSensitive(rootAbs, abs, rawPath string) *Diagnostic {
	rel, err := filepath.Rel(rootAbs, abs)
	if err != nil {
		return &Diagnostic{Path: rawPath, Category: CategoryOutsideRoot, Message: err.Error()}
	}
	for _, part := range strings.Split(filepath.ToSlash(rel), "/") {
		lower := strings.ToLower(part)
		if sensitiveNames[lower] || sensitiveDirs[lower] {
			return &Diagnostic{Path: rawPath, Category: CategorySensitivePath, Message: fmt.Sprintf("sensitive path segment %q", part)}
		}
	}
	return nil
}
