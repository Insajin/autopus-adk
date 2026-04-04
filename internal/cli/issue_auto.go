package cli

import (
	"log"

	"github.com/insajin/autopus-adk/pkg/config"
)

// triggerAutoIssueReport checks config and triggers automatic issue reporting on pipeline failure.
// It is a fire-and-forget helper called after pipeline errors when auto_submit is enabled.
func triggerAutoIssueReport(cfg *config.HarnessConfig, errMsg, command string, exitCode int) {
	if cfg == nil || !cfg.IssueReport.AutoSubmit {
		return
	}
	log.Printf("[auto-issue] auto-submitting issue report for: %s", errMsg)
	// Full implementation requires refactoring runIssueReport to not depend on cobra.Command.
	// For now, log the intent. The hook point is established here for future wiring.
}
