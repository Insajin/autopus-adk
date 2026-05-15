package run

import (
	"path/filepath"
	"strings"

	qaevidence "github.com/insajin/autopus-adk/pkg/qa/evidence"
)

const redactedPublicPath = "[REDACTED_LOCAL_PATH]"

// @AX:NOTE [AUTO] [downgraded from ANCHOR - fan_in < 3] @AX:SPEC: SPEC-QAMESH-006: public plan previews must not expose absolute project, manifest, or artifact paths.
// @AX:REASON: Dry-run JSON and cross-agent feedback can persist these previews, so roots and artifact refs must stay project-relative or redacted.
func publicPlan(plan Plan, projectDir string) Plan {
	plan.OutputRoot = publicProjectPath(projectDir, plan.OutputRoot)
	plan.RunIndexPreviewPath = publicProjectPath(projectDir, plan.RunIndexPreviewPath)
	plan.HarnessContract.JourneyPackRoot = publicProjectPath(projectDir, plan.HarnessContract.JourneyPackRoot)
	plan.HarnessContract.RuntimeArtifactRoot = publicProjectPath(projectDir, plan.HarnessContract.RuntimeArtifactRoot)
	for i := range plan.ManifestOutputPreviewPaths {
		plan.ManifestOutputPreviewPaths[i] = publicProjectPath(projectDir, plan.ManifestOutputPreviewPaths[i])
	}
	for i := range plan.ArtifactPreviewRefs {
		plan.ArtifactPreviewRefs[i].Path = publicPreviewPath(projectDir, plan.ArtifactPreviewRefs[i].Path)
	}
	return plan
}

func publicProjectPath(projectDir, path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if strings.Contains(path, "://") {
		return redactedPublicPath
	}
	root, rootErr := filepath.Abs(projectDir)
	target, targetErr := filepath.Abs(path)
	if rootErr == nil && targetErr == nil {
		if rel, err := filepath.Rel(root, target); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return filepath.ToSlash(filepath.Clean(rel))
		}
		if filepath.IsAbs(path) {
			return redactedPublicPath
		}
	}
	return filepath.ToSlash(filepath.Clean(qaevidence.RedactText(path)))
}

func publicPreviewPath(projectDir, path string) string {
	if strings.Contains(path, "://") {
		return redactedPublicPath
	}
	return publicProjectPath(projectDir, path)
}
