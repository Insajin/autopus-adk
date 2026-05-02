package evidence

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func sanitizeArtifact(artifact ArtifactRef, outputDir string) (ArtifactRef, error) {
	if !isTextArtifactPath(artifact.Path) {
		return ArtifactRef{}, fmt.Errorf("binary artifact %s must be represented by a sanitized text summary before publication", artifact.Path)
	}
	body, err := os.ReadFile(artifact.Path)
	if err != nil {
		return ArtifactRef{}, fmt.Errorf("read artifact %s: %w", RedactText(artifact.Path), err)
	}
	redacted := RedactText(string(body))
	if err := AssertSafeText(redacted, artifact.Path); err != nil {
		return ArtifactRef{}, err
	}
	artifactDir := filepath.Join(outputDir, "artifacts", safePathSegment(artifact.Kind))
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		return ArtifactRef{}, err
	}
	target := filepath.Join(artifactDir, safePathSegment(filepath.Base(artifact.Path)))
	if err := os.WriteFile(target, []byte(redacted), 0o644); err != nil {
		return ArtifactRef{}, err
	}
	artifact.Path = filepath.ToSlash(filepath.Join("artifacts", safePathSegment(artifact.Kind), filepath.Base(target)))
	return artifact, nil
}

func realAbsPath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err == nil {
		return resolved, nil
	}
	return filepath.Clean(abs), nil
}

func isPathWithin(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func isTextArtifactPath(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".json", ".log", ".txt", ".md", ".yml", ".yaml":
		return true
	default:
		return strings.HasSuffix(strings.ToLower(path), ".aria.yml") ||
			strings.HasSuffix(strings.ToLower(path), ".stdout") ||
			strings.HasSuffix(strings.ToLower(path), ".stderr")
	}
}

func safePathSegment(value string) string {
	cleaned := strings.ToLower(strings.TrimSpace(value))
	cleaned = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '.' || r == '_' || r == '-' {
			return r
		}
		return '-'
	}, cleaned)
	cleaned = strings.Trim(cleaned, "-")
	if cleaned == "" {
		return "artifact"
	}
	if len(cleaned) > 100 {
		return cleaned[:100]
	}
	return cleaned
}
