package orchestra

import "github.com/insajin/autopus-adk/pkg/terminal"

// paneCapable is the single source of truth for whether interactive-pane
// execution is possible for the given terminal and mode (REQ-007).
// It returns true only when subprocess mode is NOT forced, a terminal is
// attached, and that terminal is not the non-interactive "plain" terminal.
//
// This predicate is shared by the RunOrchestra dispatch guard and SelectBackend
// so the pane/subprocess decision is identical everywhere (F-002, F-006).
func paneCapable(term terminal.Terminal, subprocessMode bool) bool {
	return !subprocessMode && term != nil && term.Name() != "plain"
}
