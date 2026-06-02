package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/insajin/autopus-adk/pkg/spec"
)

func mergeVerifyFindings(
	providerFindings [][]spec.ReviewFinding,
	priorFindings []spec.ReviewFinding,
	totalProviders int,
	threshold float64,
) []spec.ReviewFinding {
	if len(providerFindings) == 0 {
		return append([]spec.ReviewFinding(nil), priorFindings...)
	}

	priorIDs := make(map[string]struct{}, len(priorFindings))
	for _, f := range priorFindings {
		if f.ID != "" {
			priorIDs[f.ID] = struct{}{}
		}
	}

	priorStatusInputs := make([][]spec.ReviewFinding, 0, len(providerFindings))
	var newFindings []spec.ReviewFinding
	for _, findings := range providerFindings {
		var priorStatuses []spec.ReviewFinding
		for _, f := range findings {
			if _, ok := priorIDs[f.ID]; ok {
				priorStatuses = append(priorStatuses, f)
				continue
			}
			if matchesPriorFindingContent(f, priorFindings) {
				continue
			}
			newFindings = append(newFindings, f)
		}
		priorStatusInputs = append(priorStatusInputs, priorStatuses)
	}

	merged := spec.MergeFindingStatuses(priorStatusInputs, threshold)
	mergedNew := spec.MergeSupermajority(newFindings, totalProviders, threshold)
	mergedNew = spec.DeduplicateFindings(mergedNew)
	assignNewFindingIDs(mergedNew, maxFindingNumber(priorFindings)+1)
	return append(merged, mergedNew...)
}

type findingContentKey struct {
	scopeRef    string
	category    spec.FindingCategory
	description string
}

func matchesPriorFindingContent(f spec.ReviewFinding, priorFindings []spec.ReviewFinding) bool {
	k := findingKey(f)
	for _, prior := range priorFindings {
		if findingKey(prior) == k {
			return true
		}
	}
	return false
}

func findingKey(f spec.ReviewFinding) findingContentKey {
	return findingContentKey{
		scopeRef:    spec.NormalizeScopeRef(f.ScopeRef, ""),
		category:    f.Category,
		description: normalizedFindingDescription(f.Description),
	}
}

func normalizedFindingDescription(description string) string {
	return strings.ToLower(strings.Join(strings.Fields(description), " "))
}

func assignNewFindingIDs(findings []spec.ReviewFinding, start int) {
	for i := range findings {
		findings[i].ID = fmt.Sprintf("F-%03d", start+i)
	}
}

func maxFindingNumber(findings []spec.ReviewFinding) int {
	maxID := 0
	for _, f := range findings {
		raw := strings.TrimPrefix(f.ID, "F-")
		n, err := strconv.Atoi(raw)
		if err != nil {
			continue
		}
		if n > maxID {
			maxID = n
		}
	}
	return maxID
}
