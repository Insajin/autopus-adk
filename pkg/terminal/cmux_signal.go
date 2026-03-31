// Package terminal provides signal-based communication for the cmux adapter.
package terminal

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// @AX:NOTE [AUTO] signal name validation — prevents shell injection in cmux wait-for commands
// validSignalName matches safe signal names: alphanumeric and hyphens only.
var validSignalName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9-]*$`)

// validateSignalName checks that name is safe for use as a cmux signal name.
func validateSignalName(name string) error {
	if name == "" {
		return fmt.Errorf("signal name must not be empty")
	}
	if !validSignalName.MatchString(name) {
		return fmt.Errorf("invalid signal name %q: must be alphanumeric with hyphens", name)
	}
	return nil
}

// SurfaceHealth checks surface health via `cmux surface-health`.
// Output format: "surface:7 type=terminal in_window=true"
func (a *CmuxAdapter) SurfaceHealth(_ context.Context, paneID PaneID) (SurfaceStatus, error) {
	if err := validatePaneID(paneID); err != nil {
		return SurfaceStatus{}, fmt.Errorf("cmux: %w", err)
	}
	cmd := execCommand("cmux", "surface-health", "--surface", string(paneID))
	out, err := cmd.Output()
	if err != nil {
		return SurfaceStatus{}, fmt.Errorf("cmux: surface-health pane %s: %w", paneID, err)
	}
	return parseSurfaceHealth(string(out))
}

// parseSurfaceHealth parses cmux surface-health output.
// Expected format: "surface:7 type=terminal in_window=true"
func parseSurfaceHealth(output string) (SurfaceStatus, error) {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return SurfaceStatus{}, fmt.Errorf("cmux: empty surface-health output")
	}
	status := SurfaceStatus{Valid: true}
	for field := range strings.FieldsSeq(trimmed) {
		switch {
		case strings.HasPrefix(field, "surface:") || strings.HasPrefix(field, "pane:"):
			status.SurfaceRef = field
		case field == "in_window=true":
			status.InWindow = true
		case field == "in_window=false":
			status.InWindow = false
		}
	}
	if status.SurfaceRef == "" {
		return SurfaceStatus{}, fmt.Errorf("cmux: no surface ref in output %q", trimmed)
	}
	return status, nil
}

// WaitForSignal blocks until the named signal is received via `cmux wait-for`.
// Uses exec.CommandContext to respect the provided timeout.
func (a *CmuxAdapter) WaitForSignal(ctx context.Context, name string, timeout time.Duration) error {
	if err := validateSignalName(name); err != nil {
		return fmt.Errorf("cmux: %w", err)
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	// Pass --timeout to cmux wait-for so its internal timeout matches ours.
	// cmux defaults to 30s which is too short for AI provider responses.
	timeoutSec := fmt.Sprintf("%d", max(int(timeout.Seconds()), 30))
	cmd := execCommandContext(timeoutCtx, "cmux", "wait-for", name, "--timeout", timeoutSec)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail != "" {
			return fmt.Errorf("cmux: wait-for signal %q: %w (%s)", name, err, detail)
		}
		return fmt.Errorf("cmux: wait-for signal %q: %w", name, err)
	}
	return nil
}

// SendSignal sends a named signal via `cmux wait-for -S`.
func (a *CmuxAdapter) SendSignal(_ context.Context, name string) error {
	if err := validateSignalName(name); err != nil {
		return fmt.Errorf("cmux: %w", err)
	}
	cmd := execCommand("cmux", "wait-for", "-S", name)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("cmux: send signal %q: %w", name, err)
	}
	return nil
}
