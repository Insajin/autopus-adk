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
	sb.WriteString(verdictLine)
	sb.WriteString("\n")
	fmt.Fprintf(&sb, "**Revision**: %d\n", r.Revision)
	fmt.Fprintf(&sb, "**Date**: %s\n\n", time.Now().Format("2006-01-02 15:04:05"))

	if section := RenderProviderHealthSection(r.ProviderStatuses, totalConfigured); section != "" {
		sb.WriteString(section)
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
