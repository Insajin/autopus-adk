package skillevolve

import (
	"os"
	"path"
	"path/filepath"
	"strings"
)

func cleanRelPath(rel string) string {
	rel = strings.TrimSpace(strings.ReplaceAll(rel, "\\", "/"))
	rel = strings.TrimPrefix(rel, "./")
	if rel == "" {
		return ""
	}
	return path.Clean(rel)
}

// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-SKILL-EVOLVE-001: generated/root harness paths are excluded from skill evolution writes.
// @AX:REASON: Safety and promotion gates rely on this policy to keep generated platform surfaces out of candidate application.
func isGeneratedSurfacePath(rel string) bool {
	raw := strings.TrimSpace(strings.ReplaceAll(rel, "\\", "/"))
	if raw == "" {
		return false
	}
	if isUnsafeCandidatePath(raw) {
		return true
	}
	rel = cleanRelPath(raw)
	generatedPrefixes := []string{
		".agents/",
		".codex/",
		".opencode/",
		".claude/",
		".gemini/",
		".autopus/plugins/",
		".autopus/runtime/",
		".autopus/orchestra/",
		".autopus/brainstorms/",
		".autopus/canary/",
	}
	for _, prefix := range generatedPrefixes {
		base := strings.TrimSuffix(prefix, "/")
		if rel == base || strings.HasPrefix(rel, prefix) || strings.HasSuffix(rel, "/"+base) || strings.Contains(rel, "/"+prefix) {
			return true
		}
	}
	if rel == ".autopus/context/signatures.md" || strings.HasSuffix(rel, "/.autopus/context/signatures.md") || rel == "config.toml" || strings.HasSuffix(rel, "/config.toml") {
		return true
	}
	if (strings.HasPrefix(rel, ".autopus/") || strings.Contains(rel, "/.autopus/")) && strings.HasSuffix(rel, "-manifest.json") {
		return true
	}
	return strings.Contains(rel, "/plugins/cache/") || strings.Contains(rel, "/.codex/plugins/cache/")
}

func isUnsafeCandidatePath(rel string) bool {
	if path.IsAbs(rel) || filepath.IsAbs(rel) || strings.Contains(rel, "\x00") {
		return true
	}
	if len(rel) >= 2 && rel[1] == ':' {
		return true
	}
	for _, segment := range strings.Split(rel, "/") {
		if segment == ".." {
			return true
		}
	}
	return false
}

func isADKSourceOfTruthPath(rel string) bool {
	rel = cleanRelPath(rel)
	if rel == "" || strings.HasPrefix(rel, "../") || path.IsAbs(rel) {
		return false
	}
	if isGeneratedSurfacePath(rel) {
		return false
	}
	sourcePrefixes := []string{
		"autopus-adk/content/",
		"autopus-adk/templates/",
		"autopus-adk/pkg/adapter/",
	}
	for _, prefix := range sourcePrefixes {
		if strings.HasPrefix(rel, prefix) {
			return true
		}
	}
	return false
}

func pathWithinOwnedPaths(rel string, ownedPaths []string) bool {
	if len(ownedPaths) == 0 {
		return true
	}
	rel = cleanRelPath(rel)
	for _, owned := range ownedPaths {
		owned = cleanRelPath(owned)
		if owned == "" {
			continue
		}
		if strings.HasSuffix(owned, "/**") {
			prefix := strings.TrimSuffix(owned, "/**") + "/"
			if rel == strings.TrimSuffix(prefix, "/") || strings.HasPrefix(rel, prefix) {
				return true
			}
			continue
		}
		if strings.HasSuffix(owned, "/*") {
			prefix := strings.TrimSuffix(owned, "/*") + "/"
			if strings.HasPrefix(rel, prefix) && !strings.Contains(strings.TrimPrefix(rel, prefix), "/") {
				return true
			}
			continue
		}
		if rel == owned {
			return true
		}
	}
	return false
}

func safeFileName(value string) string {
	value = strings.ToLower(value)
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_':
			b.WriteRune(r)
		case r == '.' || r == '/':
			b.WriteRune('-')
		default:
			b.WriteRune('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "candidate"
	}
	return out
}

func pathWithinProjectForRead(projectDir, target string) bool {
	if projectDir == "" || target == "" {
		return true
	}
	root, err := filepath.Abs(projectDir)
	if err != nil {
		return false
	}
	if resolvedRoot, err := filepath.EvalSymlinks(root); err == nil {
		root = resolvedRoot
	}
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return false
	}
	if resolved, err := filepath.EvalSymlinks(targetAbs); err == nil {
		targetAbs = resolved
	}
	rel, err := filepath.Rel(root, targetAbs)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func pathWithinProjectForWrite(projectDir, target string) bool {
	if projectDir == "" || target == "" {
		return true
	}
	root, err := filepath.Abs(projectDir)
	if err != nil {
		return false
	}
	if resolvedRoot, err := filepath.EvalSymlinks(root); err == nil {
		root = resolvedRoot
	}
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return false
	}
	if _, err := os.Lstat(targetAbs); err == nil {
		resolved, err := filepath.EvalSymlinks(targetAbs)
		if err != nil {
			return false
		}
		rel, err := filepath.Rel(root, resolved)
		return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
	}
	parent, missing := filepath.Dir(targetAbs), []string{filepath.Base(targetAbs)}
	for parent != filepath.Dir(parent) {
		if _, err := os.Lstat(parent); err == nil {
			resolved, err := filepath.EvalSymlinks(parent)
			if err != nil {
				return false
			}
			for i := len(missing) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, missing[i])
			}
			rel, err := filepath.Rel(root, resolved)
			return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
		}
		missing = append(missing, filepath.Base(parent))
		parent = filepath.Dir(parent)
	}
	return false
}
