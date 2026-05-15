package run

import (
	"path/filepath"
	"strings"
)

func generatedSurfaceIn(rel string) (string, bool) {
	parts := strings.Split(strings.ToLower(filepath.ToSlash(filepath.Clean(rel))), "/")
	for index, part := range parts {
		switch part {
		case ".codex", ".claude", ".gemini", ".opencode":
			return part, true
		case ".autopus":
			if index+1 < len(parts) && parts[index+1] == "plugins" {
				return ".autopus/plugins", true
			}
		}
	}
	return "", false
}

func pathWithin(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}
