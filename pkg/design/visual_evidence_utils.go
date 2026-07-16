package design

import (
	"sort"
	"strings"
)

func hasVisualVolumePrefix(value string) bool {
	return len(value) >= 2 && ((value[0] >= 'a' && value[0] <= 'z') || (value[0] >= 'A' && value[0] <= 'Z')) && value[1] == ':'
}

func hasPassedAssertion(assertions []VisualAssertion) bool {
	for _, assertion := range assertions {
		if assertion.Status == "PASS" {
			return true
		}
	}
	return false
}

func hasFailedAssertion(assertions []VisualAssertion) bool {
	for _, assertion := range assertions {
		if assertion.Status == "FAIL" {
			return true
		}
	}
	return false
}

func sortedUnique(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			seen[value] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for value := range seen {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func sanitizeSnapshotProof(proof SnapshotComparisonProof) SnapshotComparisonProof {
	proof.Status = strings.ToLower(strings.TrimSpace(proof.Status))
	proof.Diagnostic = strings.TrimSpace(proof.Diagnostic)
	proof.PlaywrightVersion = strings.TrimSpace(proof.PlaywrightVersion)
	proof.UpdateSnapshots = strings.TrimSpace(proof.UpdateSnapshots)
	for i := range proof.Projects {
		proof.Projects[i].Name = strings.TrimSpace(proof.Projects[i].Name)
		proof.Projects[i].ComparisonStatus = strings.ToLower(strings.TrimSpace(proof.Projects[i].ComparisonStatus))
	}
	sort.Slice(proof.Projects, func(i, j int) bool { return proof.Projects[i].Name < proof.Projects[j].Name })
	return proof
}
