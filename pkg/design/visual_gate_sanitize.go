package design

import (
	"path/filepath"
	"strings"
)

func sanitizePathList(paths []string) []string {
	out := make([]string, 0, len(paths))
	for _, candidate := range paths {
		candidate = filepath.ToSlash(strings.TrimSpace(candidate))
		if candidate != "" {
			out = append(out, candidate)
		}
	}
	return out
}

func sanitizeVisualCritic(critic VisualCriticReport) VisualCriticReport {
	if critic.Status == "" {
		return VisualCriticReport{}
	}
	critic.Status = strings.ToUpper(strings.TrimSpace(critic.Status))
	for i := range critic.Findings {
		critic.Findings[i].Screenshot = filepath.ToSlash(strings.TrimSpace(critic.Findings[i].Screenshot))
	}
	return critic
}

func RedactVisualPath(root, rawPath string) string {
	candidate := strings.TrimSpace(rawPath)
	if candidate == "" {
		return ""
	}
	if !filepath.IsAbs(candidate) {
		if !hasParentTraversal(candidate) {
			return filepath.ToSlash(candidate)
		}
		return externalVisualPath(candidate)
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return externalVisualPath(candidate)
	}
	if evaluatedRoot, evalErr := filepath.EvalSymlinks(rootAbs); evalErr == nil {
		rootAbs = evaluatedRoot
	}
	if rel, relErr := filepath.Rel(rootAbs, candidate); relErr == nil && isInsideRoot(rootAbs, candidate) {
		return filepath.ToSlash(rel)
	}
	return externalVisualPath(candidate)
}

func externalVisualPath(candidate string) string {
	base := filepath.Base(filepath.Clean(candidate))
	if base == "." || base == string(filepath.Separator) || base == ".." {
		base = "artifact"
	}
	return filepath.ToSlash("external:" + shortHash(candidate) + ":" + base)
}
