// Package pipeline provides team pane management for pipeline monitoring.
package pipeline

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/insajin/autopus-adk/pkg/terminal"
)

// TeammatePaneInfo tracks a teammate's terminal pane and log file.
type TeammatePaneInfo struct {
	Role    string
	PaneID  terminal.PaneID
	LogPath string
}

// @AX:NOTE [AUTO] @AX:REASON: security — shell escaping prevents injection when constructing commands for terminal panes; do not simplify without security review
// teamShellEscape wraps a string in single quotes for safe shell interpolation.
// Embedded single quotes use the standard POSIX quote-break pattern.
func teamShellEscape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// sanitizeRole returns a safe role name for use in file paths.
func sanitizeRole(name string) string {
	var sb strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			sb.WriteRune(r)
		}
	}
	if sb.Len() == 0 {
		return "unknown"
	}
	return sb.String()
}

// createTeammatePanes creates log files and starts tail -f in each pane.
func createTeammatePanes(ctx context.Context, term terminal.Terminal, specID string, paneIDs []terminal.PaneID, roles []string) ([]TeammatePaneInfo, error) {
	if err := ValidateSpecID(specID); err != nil {
		return nil, err
	}

	panes := make([]TeammatePaneInfo, 0, len(roles))
	for i, role := range roles {
		safeRole := sanitizeRole(role)
		prefix := fmt.Sprintf("autopus-team-%s-%s-", specID, safeRole)
		tmpFile, err := os.CreateTemp("", prefix)
		if err != nil {
			cleanupTeammatePanes(panes)
			return nil, fmt.Errorf("create log file for %s: %w", role, err)
		}
		tmpFile.Close()

		panes = append(panes, TeammatePaneInfo{
			Role:    role,
			PaneID:  paneIDs[i],
			LogPath: tmpFile.Name(),
		})

		streamToPane(ctx, term, paneIDs[i], tmpFile.Name())
	}
	return panes, nil
}

// streamToPane sends a tail -f command to stream a log file in a pane.
func streamToPane(ctx context.Context, term terminal.Terminal, paneID terminal.PaneID, logPath string) {
	cmd := fmt.Sprintf("tail -f %s", teamShellEscape(logPath))
	// Best-effort: pane commands must not block pipeline execution.
	_ = term.SendCommand(ctx, paneID, cmd)
}

// cleanupTeammatePanes removes temporary log files for all teammate panes.
func cleanupTeammatePanes(panes []TeammatePaneInfo) {
	for _, p := range panes {
		if p.LogPath != "" {
			_ = os.Remove(p.LogPath)
		}
	}
}

// sendFailureMessage sends a failure indicator to a teammate's pane.
func sendFailureMessage(ctx context.Context, term terminal.Terminal, paneID terminal.PaneID, role string, errMsg string) {
	cmd := fmt.Sprintf("echo %s", teamShellEscape(fmt.Sprintf("[FAILED] %s: %s", role, errMsg)))
	_ = term.SendCommand(ctx, paneID, cmd)
}
