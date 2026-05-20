package spec

import (
	"fmt"
	"strconv"
)

// MergeDeterministicFindings keeps static findings outside provider
// supermajority math while preserving stable IDs across review revisions.
func MergeDeterministicFindings(current, deterministic, prior []ReviewFinding, revision int) []ReviewFinding {
	result := make([]ReviewFinding, 0, len(current)+len(deterministic))
	for _, f := range current {
		if f.Provider == staticContractProvider {
			continue
		}
		result = append(result, f)
	}
	priorByScope := map[string]ReviewFinding{}
	for _, f := range prior {
		if f.Provider == staticContractProvider {
			priorByScope[f.ScopeRef] = f
		}
	}

	nextID := nextFindingID(current, prior)
	activeScopes := map[string]struct{}{}
	for _, f := range deterministic {
		activeScopes[f.ScopeRef] = struct{}{}
		if priorFinding, ok := priorByScope[f.ScopeRef]; ok {
			f.ID = priorFinding.ID
			f.FirstSeenRev = priorFinding.FirstSeenRev
		} else {
			f.ID = fmt.Sprintf("F-%03d", nextID)
			f.FirstSeenRev = revision
			nextID++
		}
		f.Provider = staticContractProvider
		f.Status = FindingStatusOpen
		f.LastSeenRev = revision
		result = append(result, f)
	}

	for scope, priorFinding := range priorByScope {
		if _, stillActive := activeScopes[scope]; stillActive {
			continue
		}
		priorFinding.Status = FindingStatusResolved
		priorFinding.LastSeenRev = revision
		result = append(result, priorFinding)
	}
	return result
}

func nextFindingID(groups ...[]ReviewFinding) int {
	maxID := 0
	for _, group := range groups {
		for _, f := range group {
			match := reFindingID.FindStringSubmatch(f.ID)
			if len(match) != 2 {
				continue
			}
			n, err := strconv.Atoi(match[1])
			if err == nil && n > maxID {
				maxID = n
			}
		}
	}
	return maxID + 1
}
