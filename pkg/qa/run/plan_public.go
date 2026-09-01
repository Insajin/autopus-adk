package run

import (
	"path/filepath"
	"strings"

	qaevidence "github.com/insajin/autopus-adk/pkg/qa/evidence"
)

// RedactedLocalPath replaces a filesystem path that cannot be expressed
// relative to the project root. It is a bracketed sentinel, matching the
// pkg/qa/evidence family, and deliberately free of the shell metacharacters
// (`$`, `<`, `>`) that downstream ref validation rejects.
const RedactedLocalPath = "[REDACTED_LOCAL_PATH]"

// @AX:NOTE [AUTO] [downgraded from ANCHOR - fan_in < 3] @AX:SPEC: SPEC-QAMESH-006: public plan previews must not expose absolute project, manifest, or artifact paths.
// @AX:REASON: Dry-run JSON and cross-agent feedback can persist these previews, so roots and artifact refs must stay project-relative or redacted.
func publicPlan(plan Plan, projectDir string) Plan {
	plan.OutputRoot = PublicProjectPath(projectDir, plan.OutputRoot)
	plan.RunIndexPreviewPath = PublicProjectPath(projectDir, plan.RunIndexPreviewPath)
	plan.HarnessContract.JourneyPackRoot = PublicProjectPath(projectDir, plan.HarnessContract.JourneyPackRoot)
	plan.HarnessContract.RuntimeArtifactRoot = PublicProjectPath(projectDir, plan.HarnessContract.RuntimeArtifactRoot)
	for i := range plan.ManifestOutputPreviewPaths {
		plan.ManifestOutputPreviewPaths[i] = PublicProjectPath(projectDir, plan.ManifestOutputPreviewPaths[i])
	}
	for i := range plan.ArtifactPreviewRefs {
		plan.ArtifactPreviewRefs[i].Path = publicPreviewPath(projectDir, plan.ArtifactPreviewRefs[i].Path)
	}
	return plan
}

// PublicProjectPath renders a path as a project-root-relative ref. Publishing
// refs relative to the project root is what keeps an absolute --project-dir
// from producing paths that later redaction has to destroy; a path genuinely
// outside the project collapses to RedactedLocalPath rather than to "".
func PublicProjectPath(projectDir, path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if strings.Contains(path, "://") {
		return RedactedLocalPath
	}
	root, rootErr := filepath.Abs(projectDir)
	target, targetErr := filepath.Abs(path)
	if rootErr == nil && targetErr == nil {
		if rel, err := filepath.Rel(root, target); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return filepath.ToSlash(filepath.Clean(rel))
		}
		if filepath.IsAbs(path) {
			return RedactedLocalPath
		}
	}
	return filepath.ToSlash(filepath.Clean(qaevidence.RedactText(path)))
}

func publicPreviewPath(projectDir, path string) string {
	if strings.Contains(path, "://") {
		return RedactedLocalPath
	}
	return PublicProjectPath(projectDir, path)
}
