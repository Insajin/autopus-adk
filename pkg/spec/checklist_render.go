package spec

import (
	"fmt"
	"strings"
)

// CountChecklistStatuses returns the per-status totals from a slice of
// ChecklistOutcome. Statuses outside PASS/FAIL/N/A are silently ignored so
// callers can rely on (pass + fail + na) <= len(outcomes).
func CountChecklistStatuses(outcomes []ChecklistOutcome) (pass, fail, na int) {
	for _, o := range outcomes {
		switch o.Status {
		case ChecklistStatusPass:
			pass++
		case ChecklistStatusFail:
			fail++
		case ChecklistStatusNA:
			na++
		}
	}
	return pass, fail, na
}

// RenderChecklistSection renders the markdown section for checklist outcomes
// using the same column-aligned table pattern as RenderProviderHealthSection.
// Empty outcomes produce an empty string so the section is omitted from
// review.md when no checklist data is available.
//
// SPEC-SPECREV-001 follow-up: surfaces orchestra parser checklist data
// (including N/A items) into review.md as a standardized table section,
// mirroring the Provider Health pattern.
func RenderChecklistSection(outcomes []ChecklistOutcome) string {
	if len(outcomes) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("## Checklist Summary\n\n")
	sb.WriteString("| ID | Status | Provider | Reason |\n")
	sb.WriteString("| --- | --- | --- | --- |\n")
	for _, o := range outcomes {
		reason := sanitizeNote(o.Reason)
		if reason == "" {
			reason = emptyNotePlaceholder
		}
		fmt.Fprintf(&sb, "| %s | %s | %s | %s |\n", o.ID, o.Status, o.Provider, reason)
	}
	pass, fail, na := CountChecklistStatuses(outcomes)
	fmt.Fprintf(&sb, "\nTotal: %d (PASS: %d, FAIL: %d, N/A: %d)\n\n", len(outcomes), pass, fail, na)
	return sb.String()
}
