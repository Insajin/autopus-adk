package design

import (
	"cmp"
	"fmt"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

type imagePairV2 struct {
	actual   VisualArtifactV2
	expected VisualArtifactV2
}

func BuildScreenshotDiffStatsV2(artifacts []VisualArtifactV2) ScreenshotDiffStats {
	stats := ScreenshotDiffStats{DeterministicMode: "actual_expected_pixel_compare_v2"}
	for _, artifact := range artifacts {
		if artifact.Kind == "diff" {
			stats.DiffArtifactRefs = append(stats.DiffArtifactRefs, artifact.Path)
		}
	}
	pairs := pairActualExpectedV2(artifacts)
	if len(pairs) > maxVisualComparisonPairs {
		stats.ComparisonErrors = append(stats.ComparisonErrors, fmt.Sprintf("comparison pair budget exceeded: %d > %d", len(pairs), maxVisualComparisonPairs))
		pairs = pairs[:maxVisualComparisonPairs]
	}
	for _, pair := range pairs {
		diff, err := compareImageFilesV2(pair.actual.LocalPath, pair.expected.LocalPath)
		if err != nil {
			stats.ComparisonErrors = append(stats.ComparisonErrors, redactImageDiffError(err, pair.actual.Path, pair.actual.LocalPath, pair.expected.Path, pair.expected.LocalPath))
			continue
		}
		accumulateImageDiff(&stats, diff)
	}
	sort.Strings(stats.DiffArtifactRefs)
	sort.Strings(stats.ComparisonErrors)
	return stats
}

func pairActualExpectedV2(artifacts []VisualArtifactV2) []imagePairV2 {
	groups := map[string]map[string]VisualArtifactV2{}
	for _, artifact := range artifacts {
		if artifact.LocalPath == "" || (artifact.Kind != "actual" && artifact.Kind != "expected") {
			continue
		}
		identity := visualArtifactPairIdentity(artifact)
		key := fmt.Sprintf("%s\x00%s\x00%d", identity, artifact.ResultID, artifact.Retry)
		if groups[key] == nil {
			groups[key] = map[string]VisualArtifactV2{}
		}
		groups[key][artifact.Kind] = artifact
	}
	pairs := make([]imagePairV2, 0, len(groups))
	for _, group := range groups {
		actual, hasActual := group["actual"]
		expected, hasExpected := group["expected"]
		if hasActual && hasExpected {
			pairs = append(pairs, imagePairV2{actual: actual, expected: expected})
		}
	}
	sortImagePairsV2(pairs)
	return pairs
}

func sanitizeArtifactsV2(artifacts []VisualArtifactV2) []VisualArtifactV2 {
	out := make([]VisualArtifactV2, 0, len(artifacts))
	for _, artifact := range artifacts {
		artifact.Name = sanitizeVisualEvidenceName(artifact.Name)
		artifact.Path = filepath.ToSlash(strings.TrimSpace(artifact.Path))
		artifact.LocalPath = strings.TrimSpace(artifact.LocalPath)
		artifact.ComparisonID = filepath.ToSlash(strings.TrimSpace(artifact.ComparisonID))
		artifact.ResultID = strings.TrimSpace(artifact.ResultID)
		if artifact.Retry < 0 {
			artifact.Retry = 0
		}
		if artifact.Path == "" {
			continue
		}
		if artifact.Kind == "" {
			artifact.Kind = ClassifyVisualArtifact(artifact.Name, artifact.Path)
		}
		out = append(out, artifact)
	}
	return out
}

func visualArtifactPairIdentity(artifact VisualArtifactV2) string {
	if artifact.ComparisonID != "" {
		return artifact.ComparisonID
	}
	for _, candidate := range []string{artifact.Path, artifact.Name} {
		if identity := visualArtifactStemIdentity(candidate); identity != "" {
			return identity
		}
	}
	return "unidentified"
}

func visualArtifactStemIdentity(value string) string {
	value = strings.ReplaceAll(strings.TrimSpace(value), "\\", "/")
	if value == "" {
		return ""
	}
	cleaned := path.Clean(value)
	dir, base := path.Dir(cleaned), path.Base(cleaned)
	ext := path.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	lower := strings.ToLower(stem)
	for _, suffix := range []string{"-expected", "-actual", "_expected", "_actual", ".expected", ".actual"} {
		if strings.HasSuffix(lower, suffix) {
			stem = stem[:len(stem)-len(suffix)]
			break
		}
	}
	if strings.EqualFold(stem, "expected") || strings.EqualFold(stem, "actual") || stem == "" {
		stem = "screenshot"
	}
	identity := stem + ext
	if dir != "." {
		identity = path.Join(dir, identity)
	}
	return identity
}

func sortImagePairsV2(pairs []imagePairV2) {
	sort.Slice(pairs, func(i, j int) bool {
		if order := compareVisualArtifactV2(pairs[i].actual, pairs[j].actual); order != 0 {
			return order < 0
		}
		return compareVisualArtifactV2(pairs[i].expected, pairs[j].expected) < 0
	})
}

func compareVisualArtifactV2(left, right VisualArtifactV2) int {
	for _, values := range [][2]string{
		{left.Path, right.Path},
		{left.ComparisonID, right.ComparisonID},
		{left.ResultID, right.ResultID},
		{left.Name, right.Name},
		{left.Kind, right.Kind},
		{left.ContentType, right.ContentType},
		{left.LocalPath, right.LocalPath},
	} {
		if order := cmp.Compare(values[0], values[1]); order != 0 {
			return order
		}
	}
	return cmp.Compare(left.Retry, right.Retry)
}

func sanitizeAssertions(assertions []VisualAssertion) []VisualAssertion {
	out := make([]VisualAssertion, 0, len(assertions))
	for _, assertion := range assertions {
		assertion.Name = sanitizeVisualEvidenceName(assertion.Name)
		assertion.TestID = strings.TrimSpace(assertion.TestID)
		assertion.Project = strings.TrimSpace(assertion.Project)
		assertion.Status = strings.ToUpper(strings.TrimSpace(assertion.Status))
		assertion.ComparisonID = filepath.ToSlash(strings.TrimSpace(assertion.ComparisonID))
		assertion.ResultID = strings.TrimSpace(assertion.ResultID)
		assertion.Diagnostic = strings.TrimSpace(assertion.Diagnostic)
		if assertion.Retry < 0 {
			assertion.Retry = 0
		}
		assertion.BaselinePath = strings.TrimSpace(assertion.BaselinePath)
		if assertion.Name != "" && assertion.Status != "" {
			out = append(out, assertion)
		}
	}
	return out
}

func sanitizeVisualEvidenceName(value string) string {
	value = strings.ReplaceAll(strings.TrimSpace(value), "\\", "/")
	if value == "" {
		return ""
	}
	cleaned := path.Clean(value)
	if strings.HasPrefix(value, "/") || hasVisualVolumePrefix(value) || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return path.Base(cleaned)
	}
	return cleaned
}

func publicArtifactsV2(artifacts []VisualArtifactV2) []VisualArtifactV2 {
	out := make([]VisualArtifactV2, len(artifacts))
	copy(out, artifacts)
	for i := range out {
		out[i].LocalPath = ""
	}
	return out
}
