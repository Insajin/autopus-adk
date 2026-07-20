package orchestra

import (
	"fmt"
	"sort"
	"strings"
)

func findingStatusMap(statuses []ReviewerFindingStatusOut) map[string]string {
	result := make(map[string]string, len(statuses))
	for _, status := range statuses {
		id := normalizeLine(status.ID)
		if id != "" {
			result[id] = strings.ToLower(strings.TrimSpace(status.Status))
		}
	}
	return result
}

func findingEvidenceStatus(finding Finding, identity string, statuses map[string]string) string {
	keys := []string{normalizeLine(finding.ID), normalizeLine(identity)}
	for _, key := range keys {
		if status := statuses[key]; status != "" {
			return status
		}
	}
	return "open"
}

func preferredFinding(current Finding, currentStatus string, candidate Finding, candidateStatus string) (Finding, string) {
	currentRank := effectiveFindingRank(current.Severity, currentStatus)
	candidateRank := effectiveFindingRank(candidate.Severity, candidateStatus)
	if candidateRank > currentRank {
		return candidate, candidateStatus
	}
	if candidateRank == currentRank && stableFindingSortKey(candidate) < stableFindingSortKey(current) {
		return candidate, candidateStatus
	}
	return current, currentStatus
}

func effectiveFindingRank(severity, status string) int {
	if status == "resolved" {
		return -1
	}
	return findingSeverityRank(severity)
}

func findingSeverityRank(severity string) int {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case "critical":
		return 4
	case "major":
		return 3
	case "minor":
		return 2
	case "suggestion":
		return 1
	default:
		return 0
	}
}

func stableFindingSortKey(finding Finding) string {
	return strings.Join([]string{finding.Severity, finding.Category, finding.ScopeRef, finding.Location, finding.Description, finding.Suggestion}, "|")
}

func findingClaimHasCriticalVeto(claim findingClaim) bool {
	for _, evidence := range claim.evidence {
		if evidence.Status == "resolved" {
			continue
		}
		if isCriticalVetoFinding(Finding{Severity: evidence.Severity, Category: evidence.Category}) {
			return true
		}
	}
	return false
}

func formatFindingEvidence(evidence []FindingClaimEvidence) string {
	ordered := append([]FindingClaimEvidence(nil), evidence...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Provider < ordered[j].Provider })
	var lines []string
	for _, item := range ordered {
		detail := fmt.Sprintf("\n  - %s: %s/%s", item.Provider, strings.ToLower(item.Severity), item.Status)
		if suggestion := strings.TrimSpace(item.Suggestion); suggestion != "" {
			detail += " — " + suggestion
		}
		lines = append(lines, detail)
	}
	return strings.Join(lines, "")
}
