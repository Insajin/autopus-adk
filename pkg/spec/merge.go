package spec

// MergeFindingStatuses applies supermajority merge across providers.
// threshold: fraction of providers that must agree (e.g., 0.67 for 2/3).
// resolved requires >= threshold agreement; regressed > open in priority.
func MergeFindingStatuses(providerResults [][]ReviewFinding, threshold float64) []ReviewFinding {
	if len(providerResults) == 0 {
		return nil
	}

	// Flatten and group by finding ID
	byID := make(map[string][]ReviewFinding)
	for _, findings := range providerResults {
		for _, f := range findings {
			byID[f.ID] = append(byID[f.ID], f)
		}
	}

	total := float64(len(providerResults))
	var merged []ReviewFinding

	for _, group := range byID {
		if len(group) == 0 {
			continue
		}
		base := group[0]

		resolvedCount := 0
		regressedCount := 0
		for _, f := range group {
			if f.Status == FindingStatusResolved {
				resolvedCount++
			}
			if f.Status == FindingStatusRegressed {
				regressedCount++
			}
		}

		if float64(resolvedCount)/total >= threshold {
			base.Status = FindingStatusResolved
		} else if regressedCount > 0 {
			base.Status = FindingStatusRegressed
		} else {
			base.Status = FindingStatusOpen
		}

		merged = append(merged, base)
	}

	return merged
}

// ShouldTripCircuitBreaker returns true if the review loop should halt.
// Compares open+regressed counts (excluding escape hatch and out_of_scope/deferred).
// If new escape hatch findings were introduced in curr, the breaker does NOT trip —
// a newly discovered critical/security issue is considered progress.
func ShouldTripCircuitBreaker(prev, curr []ReviewFinding) bool {
	prevCount := countActiveFindings(prev, true)
	currCount := countActiveFindings(curr, true)

	// New escape hatch findings indicate newly discovered critical issues — not stalling.
	if countEscapeHatch(curr) > countEscapeHatch(prev) {
		return false
	}

	return currCount >= prevCount
}

// countActiveFindings counts open+regressed findings, always excluding escape hatch.
func countActiveFindings(findings []ReviewFinding, excludeEscapeHatch bool) int {
	count := 0
	for _, f := range findings {
		if f.Status == FindingStatusOutOfScope || f.Status == FindingStatusDeferred {
			continue
		}
		if excludeEscapeHatch && f.EscapeHatch {
			continue
		}
		if f.Status == FindingStatusOpen || f.Status == FindingStatusRegressed {
			count++
		}
	}
	return count
}

// countEscapeHatch returns the number of escape hatch findings.
func countEscapeHatch(findings []ReviewFinding) int {
	count := 0
	for _, f := range findings {
		if f.EscapeHatch {
			count++
		}
	}
	return count
}

// MergeVerdicts combines multiple review results using a supermajority threshold.
// REJECT is a security gate — one REJECT immediately returns VerdictReject.
// totalProviders is the configured provider count (denominator for threshold math).
// When no supermajority is reached, any REVISE vote keeps the result as REVISE.
func MergeVerdicts(results []ReviewResult, threshold float64, totalProviders int) ReviewVerdict {
	if len(results) == 0 {
		return VerdictPass
	}

	// REJECT is a security gate — one provider is enough.
	for _, r := range results {
		if r.Verdict == VerdictReject {
			return VerdictReject
		}
	}

	denom := float64(totalProviders)
	if denom <= 0 {
		denom = float64(len(results))
	}
	passCount := 0
	reviseCount := 0
	for _, r := range results {
		switch r.Verdict {
		case VerdictPass:
			passCount++
		case VerdictRevise:
			reviseCount++
		}
	}

	// Small tolerance aligns with MergeSupermajority: 2/3=0.6667 qualifies for threshold=0.67.
	const tol = 0.005
	// PASS requires a strict supermajority; any doubt falls back to REVISE.
	if float64(passCount)/denom+tol >= threshold && reviseCount == 0 {
		return VerdictPass
	}
	if float64(passCount)/denom+tol >= threshold && float64(reviseCount)/denom+tol < threshold {
		return VerdictPass
	}
	if reviseCount > 0 {
		return VerdictRevise
	}
	return VerdictPass
}
