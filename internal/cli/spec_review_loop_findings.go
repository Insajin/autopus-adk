package cli

import "github.com/insajin/autopus-adk/pkg/spec"

func mergeDiscoverFindings(
	findings []spec.ReviewFinding,
	totalProviders int,
	threshold float64,
	verdict spec.ReviewVerdict,
) []spec.ReviewFinding {
	merged := spec.MergeSupermajority(findings, totalProviders, threshold)
	merged = spec.DeduplicateFindings(merged)
	if len(merged) > 0 || verdict != spec.VerdictRevise || !hasBlockingProviderFindings(findings) {
		return merged
	}
	return spec.DeduplicateFindings(findings)
}

func hasBlockingProviderFindings(findings []spec.ReviewFinding) bool {
	for _, finding := range findings {
		if spec.IsActiveBlockingFinding(finding) {
			return true
		}
	}
	return false
}
