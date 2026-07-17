package spec

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// PersistReview writes a ReviewResult to review.md in the given directory.
func PersistReview(dir string, result *ReviewResult) error {
	content := formatReviewMd(result)
	path := filepath.Join(dir, "review.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("failed to write review.md: %w", err)
	}
	return nil
}

// formatReviewMd formats a ReviewResult as Markdown.
//
// SPEC-SPECREV-001 REQ-VERD-1 / REQ-VERD-2 / REQ-VERD-4: when ProviderStatuses
// is populated, the verdict line carries the degraded suffix (if applicable)
// and a Provider Health section is rendered between the verdict header and
// the Findings table. Legacy callers that leave ProviderStatuses empty get
// the original output verbatim.
func formatReviewMd(r *ReviewResult) string {
	var sb strings.Builder

	totalConfigured := len(r.ProviderStatuses)

	fmt.Fprintf(&sb, "# Review: %s\n\n", r.SpecID)
	verdictLine := fmt.Sprintf("**Verdict**: %s", r.Verdict)
	verdictLine += DegradedLabel(r.ProviderStatuses, totalConfigured)
	verdictLine += degradedReasonsSuffix(r.DegradedReasons)
	sb.WriteString(verdictLine)
	sb.WriteString("\n")
	fmt.Fprintf(&sb, "**Revision**: %d\n", r.Revision)
	fmt.Fprintf(&sb, "**Date**: %s\n\n", time.Now().Format("2006-01-02 15:04:05"))

	if line := renderPromotionOverrideLine(r); line != "" {
		sb.WriteString(line)
	}

	if section := RenderProviderHealthSection(r.ProviderStatuses, totalConfigured); section != "" {
		sb.WriteString(section)
	}

	if section := RenderObservationCoverage(r.DocCoverages); section != "" {
		sb.WriteString(section)
		// RenderObservationCoverage ends with a single newline; add one more so
		// the section is separated from what follows by a blank line, matching
		// the trailing-blank-line convention of the other rendered sections.
		sb.WriteString("\n")
	}

	if section := RenderChecklistSection(r.ChecklistOutcomes); section != "" {
		sb.WriteString(section)
	}

	if len(r.Findings) > 0 {
		sb.WriteString("## Findings\n\n")
		sb.WriteString("| Provider | Severity | Description |\n")
		sb.WriteString("|----------|----------|-------------|\n")
		for _, f := range r.Findings {
			fmt.Fprintf(&sb, "| %s | %s | %s |\n", f.Provider, f.Severity, f.Description)
		}
		sb.WriteString("\n")
	}

	if len(r.Responses) > 0 {
		sb.WriteString("## Provider Responses\n\n")
		for i, resp := range r.Responses {
			fmt.Fprintf(&sb, "### Response %d\n\n", i+1)
			sb.WriteString(resp)
			sb.WriteString("\n\n")
		}
	}

	return sb.String()
}

// degradedReasonsSuffix renders the observation-integrity degraded reasons as a
// verdict-line parenthetical, e.g. " (degraded: partial_doc_context, provider_quorum)"
// (SPEC-ADK-REVIEW-INTEGRITY-001 REQ-RINT-TRUNC-04 / REQ-RINT-QUORUM-05 render
// half). It is a separate axis from DegradedLabel, which reports the
// provider-response ratio: the two can co-occur on one verdict line. Reasons are
// joined in caller order for determinism, and an empty set yields the empty
// string so the verdict line stays byte-identical to the pre-integrity output.
func degradedReasonsSuffix(reasons []string) string {
	if len(reasons) == 0 {
		return ""
	}
	return fmt.Sprintf(" (degraded: %s)", strings.Join(reasons, ", "))
}

// renderPromotionOverrideLine renders the audit line recorded when an operator
// promotes a degraded-observation PASS with --allow-degraded (REQ-RINT-OVERRIDE-07
// render half). The line is deterministic and diff-stable: reasons are joined in
// caller order and an empty reason set falls back to a stable phrase. It returns
// the empty string (no trailing block) when no override occurred so legacy output
// stays byte-stable.
func renderPromotionOverrideLine(r *ReviewResult) string {
	if !r.OverridePromotion {
		return ""
	}
	detail := strings.Join(r.DegradedReasons, ", ")
	if detail == "" {
		detail = "degraded observation"
	}
	return fmt.Sprintf("**Promotion Override**: --allow-degraded accepted by operator despite %s\n\n", detail)
}
