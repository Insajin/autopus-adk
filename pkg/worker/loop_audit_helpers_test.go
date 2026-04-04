package worker

const auditEscalationThreshold = 3

// RecordWarning adds a warning entry and escalates to error after 3
// consecutive warnings (matching SPEC-WORKER-003 REQ-AUDIT-02 threshold).
// Uses len(warnings) as the consecutive failure counter since each
// testLogBuffer is fresh per test and only receives failure entries.
func (b *testLogBuffer) RecordWarning(msg string) {
	b.warnings = append(b.warnings, msg)
	if len(b.warnings) >= auditEscalationThreshold {
		b.errors = append(b.errors, "audit write: consecutive failures reached threshold — "+msg)
	}
}

// RecordError adds an error entry.
func (b *testLogBuffer) RecordError(msg string) {
	b.errors = append(b.errors, msg)
}
