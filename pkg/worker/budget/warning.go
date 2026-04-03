package budget

import (
	"fmt"
	"io"
	"log"
)

// WarningInjector sends budget warnings to the subprocess stdin pipe.
type WarningInjector struct {
	writer io.Writer
}

// NewWarningInjector creates a WarningInjector that writes to w.
func NewWarningInjector(w io.Writer) *WarningInjector {
	return &WarningInjector{writer: w}
}

// Inject sends a warning message based on the IncrementResult.
// Only sends a message when the threshold level has changed.
func (wi *WarningInjector) Inject(r IncrementResult) {
	if !r.Changed {
		return
	}

	var msg string
	switch r.Level {
	case LevelWarn:
		msg = formatWarning(r.Count, r.Budget.Limit)
	case LevelDanger:
		msg = formatDanger(r.Count, r.Budget.Limit)
	default:
		return
	}

	if _, err := fmt.Fprintln(wi.writer, msg); err != nil {
		log.Printf("[budget] failed to inject warning: %v", err)
	}
}

func formatWarning(count, limit int) string {
	return fmt.Sprintf(
		"[BUDGET WARNING] Tool call %d/%d (%.0f%%). "+
			"You are approaching the iteration budget limit. "+
			"Prioritize completing the current task efficiently.",
		count, limit, float64(count)/float64(limit)*100,
	)
}

func formatDanger(count, limit int) string {
	return fmt.Sprintf(
		"[BUDGET CRITICAL] Tool call %d/%d (%.0f%%). "+
			"You are about to exceed the iteration budget. "+
			"Wrap up immediately — the process will be terminated at %d calls.",
		count, limit, float64(count)/float64(limit)*100, limit,
	)
}
