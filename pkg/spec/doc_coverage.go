package spec

import (
	"fmt"
	"strings"
)

// DegradedReasonPartialDocContext is the machine-readable degraded reason
// recorded on the verdict when a required auxiliary document was injected below
// 100% coverage because the document set exceeded the total context budget. The
// promotion gate reads this code to block auto-promotion (REQ-RINT-TRUNC-04).
// Its quorum counterpart lives in provider_health.go as DegradedReasonProviderQuorum.
const DegradedReasonPartialDocContext = "partial_doc_context"

// DocCoverage records how completely one auxiliary document was injected into a
// review prompt (SPEC-ADK-REVIEW-INTEGRITY-001 REQ-RINT-COV-01). It is the
// observation-integrity evidence that lets the promotion gate refuse to
// approve a SPEC that a reviewer never fully saw.
// @AX:ANCHOR: [AUTO] shared JSON schema — DocCoverage crosses the review-findings.json sidecar (findings.go), ReviewResult (types.go), and review.md rendering (review_persist.go); field renames require updating all three plus REQ-RINT-COMPAT-10 back-compat handling
// @AX:REASON: fan-out across persistence, in-memory result, and render layers; a silent field rename breaks JSON round-tripping for on-disk review-findings.json without a compile error
type DocCoverage struct {
	Name     string `json:"name"`     // auxiliary document file name (plan.md, research.md, acceptance.md)
	Injected int    `json:"injected"` // source lines actually injected into the prompt
	Total    int    `json:"total"`    // source total line count
	Percent  int    `json:"percent"`  // floor(injected*100/total)
	Complete bool   `json:"complete"` // injected >= total
}

// ComputeCoverage derives the coverage record for a single document.
// percent = floor(injected*100/total); complete = injected >= total (INV-001).
// An empty document (total == 0) is treated as fully observed.
// @AX:NOTE: [AUTO] subtle invariant — total==0 short-circuits to percent=100/complete=true so a missing aux doc never blocks AllDocumentsFullyObserved; do not change this default without checking REQ-RINT-TRUNC-04 promotion-gate impact
func ComputeCoverage(name string, injected, total int) DocCoverage {
	percent := 100
	complete := true
	if total > 0 {
		percent = injected * 100 / total
		if percent > 100 {
			percent = 100
		}
		complete = injected >= total
	}
	return DocCoverage{Name: name, Injected: injected, Total: total, Percent: percent, Complete: complete}
}

// AuxDocCoverages computes the per-document coverage the review prompt would
// produce for the auxiliary documents in specDir under totalBudget lines. It is
// deterministic and shares the same planning core as prompt injection so the
// CLI loop can aggregate coverage without rebuilding the prompt.
func AuxDocCoverages(specDir string, totalBudget int) []DocCoverage {
	plans := planAuxDocInjection(specDir, totalBudget)
	coverages := make([]DocCoverage, 0, len(plans))
	for _, plan := range plans {
		coverages = append(coverages, plan.coverage)
	}
	return coverages
}

// RenderObservationCoverage renders the review.md `## Observation Coverage`
// section from a coverage set. It returns an empty string when there are no
// rows so callers can omit the section entirely.
func RenderObservationCoverage(coverages []DocCoverage) string {
	if len(coverages) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("## Observation Coverage\n\n")
	sb.WriteString("| Document | Injected | Total | Coverage | Complete |\n")
	sb.WriteString("|----------|----------|-------|----------|----------|\n")
	for _, cov := range coverages {
		complete := "no"
		if cov.Complete {
			complete = "yes"
		}
		fmt.Fprintf(&sb, "| %s | %d | %d | %d%% | %s |\n",
			cov.Name, cov.Injected, cov.Total, cov.Percent, complete)
	}
	return sb.String()
}

// AllDocumentsFullyObserved reports whether every coverage row is complete.
// An empty set is considered fully observed (no aux docs to truncate).
func AllDocumentsFullyObserved(coverages []DocCoverage) bool {
	for _, cov := range coverages {
		if !cov.Complete {
			return false
		}
	}
	return true
}
