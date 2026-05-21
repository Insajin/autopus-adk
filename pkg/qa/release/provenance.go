package release

import (
	"path/filepath"
	"regexp"
	"strings"
)

var sourceRefSegmentRe = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

func workspaceRef(projectDir string) WorkspaceRef {
	abs, err := filepath.Abs(projectDir)
	if err != nil {
		abs = projectDir
	}
	repoID := sourceRefSegment(filepath.Base(abs))
	if repoID == "" || repoID == "." || repoID == string(filepath.Separator) {
		repoID = "project"
	}
	return WorkspaceRef{
		WorkspaceID: repoID,
		RepoID:      repoID,
		RepoRoot:    ".",
	}
}

func planSourceRefs(projectDir string, plan Plan) []string {
	refs := []string{}
	for _, pack := range plan.JourneyPacks {
		refs = appendUniqueStrings(refs, sourceRefForSpec(projectDir, pack.SourceSpec))
	}
	for _, gap := range plan.SetupGaps {
		refs = appendUniqueStrings(refs, sourceRefForSpec(projectDir, gap.OwnerSpec))
	}
	if len(refs) == 0 {
		refs = append(refs, sourceRefForSpec(projectDir, "SPEC-QAMESH-004"))
	}
	return refs
}

func sourceRefForSpec(projectDir, spec string) string {
	if strings.TrimSpace(spec) == "" {
		spec = "SPEC-QAMESH-004"
	}
	return "qamesh://source/" + workspaceRef(projectDir).RepoID + "/specs/" + sourceRefSegment(spec)
}

func sourceRefSegment(value string) string {
	return strings.Trim(sourceRefSegmentRe.ReplaceAllString(strings.TrimSpace(value), "-"), "-")
}

func appendUniqueStrings(base []string, values ...string) []string {
	seen := make(map[string]struct{}, len(base)+len(values))
	out := make([]string, 0, len(base)+len(values))
	for _, value := range append(base, values...) {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
