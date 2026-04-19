package spec

import (
	"fmt"
	"strings"
)

// FindingsSummary holds a status breakdown of a ReviewFinding slice.
// Total equals len(findings) after deduplication, so it is the "unique" count
// surfaced to operators rather than the raw pre-merge row count.
type FindingsSummary struct {
	Total      int
	Open       int
	Resolved   int
	Regressed  int
	OutOfScope int
}

// SummarizeFindings groups findings by Status for CLI and report output.
// Unknown or empty statuses are counted only in Total.
func SummarizeFindings(findings []ReviewFinding) FindingsSummary {
	s := FindingsSummary{Total: len(findings)}
	for _, f := range findings {
		switch f.Status {
		case FindingStatusOpen:
			s.Open++
		case FindingStatusResolved:
			s.Resolved++
		case FindingStatusRegressed:
			s.Regressed++
		case FindingStatusOutOfScope:
			s.OutOfScope++
		}
	}
	return s
}

// Format renders the summary as "N unique (open: a, resolved: b, ...)".
// Zero-count buckets are omitted so operators see only the statuses that matter.
// Returns "0 unique" when there are no findings.
func (s FindingsSummary) Format() string {
	if s.Total == 0 {
		return "0 unique"
	}

	buckets := []struct {
		label string
		n     int
	}{
		{"open", s.Open},
		{"regressed", s.Regressed},
		{"resolved", s.Resolved},
		{"out_of_scope", s.OutOfScope},
	}

	var parts []string
	for _, b := range buckets {
		if b.n > 0 {
			parts = append(parts, fmt.Sprintf("%s: %d", b.label, b.n))
		}
	}

	if len(parts) == 0 {
		return fmt.Sprintf("%d unique", s.Total)
	}
	return fmt.Sprintf("%d unique (%s)", s.Total, strings.Join(parts, ", "))
}
