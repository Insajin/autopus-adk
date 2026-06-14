package run

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/insajin/autopus-adk/pkg/qa/journey"
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

func laneSourceRefs(projectDir, lane string) []string {
	return []string{sourceRefForSpec(projectDir, sourceSpecForLane(lane))}
}

func packSourceRefs(projectDir string, pack journey.Pack) []string {
	spec := sourceRefs(pack).SourceSpec
	if spec == "" {
		spec = sourceSpecForLane(firstLane(pack))
	}
	return []string{sourceRefForSpec(projectDir, spec)}
}

func sourceSpecForLane(lane string) string {
	switch strings.TrimSpace(lane) {
	case "browser-staging", "desktop-native":
		return "SPEC-QAMESH-005"
	case "gui-explore":
		return "SPEC-QAMESH-003"
	case "mobile-readiness":
		return "SPEC-QAMESH-006"
	case laneMobileScripted:
		return "SPEC-QAMESH-008"
	case "canary-explicit":
		return "SPEC-QAMESH-004"
	default:
		return "SPEC-QAMESH-002"
	}
}

func sourceRefForSpec(projectDir, spec string) string {
	repoID := workspaceRef(projectDir).RepoID
	return "qamesh://source/" + repoID + "/specs/" + sourceRefSegment(spec)
}

func sourceRefSegment(value string) string {
	return strings.Trim(sourceRefSegmentRe.ReplaceAllString(strings.TrimSpace(value), "-"), "-")
}

func firstLane(pack journey.Pack) string {
	if len(pack.Lanes) == 0 {
		return ""
	}
	return pack.Lanes[0]
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
