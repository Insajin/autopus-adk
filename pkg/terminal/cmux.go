// Package terminal provides the cmux terminal adapter.
package terminal

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// CmuxAdapter implements Terminal using the cmux terminal multiplexer.
type CmuxAdapter struct {
	workspaceRef string // e.g. "workspace:1" or a canonical UUID
}

// Name returns the adapter name.
func (a *CmuxAdapter) Name() string { return "cmux" }

// CreateWorkspace creates a new cmux workspace and renames it to the given name.
// It stores the workspace ref internally for later workspace-scoped commands.
func (a *CmuxAdapter) CreateWorkspace(_ context.Context, name string) error {
	if err := validateWorkspaceName(name); err != nil {
		return fmt.Errorf("cmux: %w", err)
	}
	cmd := execCommand("cmux", "new-workspace")
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("cmux: create workspace: %w", err)
	}
	workspaceRef := parseCmuxRef(string(out), "workspace")
	if workspaceRef == "" {
		return fmt.Errorf("cmux: create workspace: failed to parse workspace ref from output %q", string(out))
	}
	if err := validateCmuxWorkspaceRef(workspaceRef); err != nil {
		return fmt.Errorf("cmux: create workspace: %w", err)
	}
	a.workspaceRef = workspaceRef
	renameCmd := execCommand("cmux", "rename-workspace", "--workspace", a.workspaceRef, name)
	if err := renameCmd.Run(); err != nil {
		return fmt.Errorf("cmux: rename workspace %q: %w", name, err)
	}
	return nil
}

// CreateSurface creates a new independent surface (tab) via cmux new-surface.
// Each surface gets the full terminal width, avoiding pane width starvation.
func (a *CmuxAdapter) CreateSurface(_ context.Context) (PaneID, error) {
	args, err := a.workspaceCommandArgs("new-surface")
	if err != nil {
		return "", fmt.Errorf("cmux: new-surface: %w", err)
	}
	cmd := execCommand("cmux", args...)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("cmux: new-surface: %w", err)
	}
	ref := parseCmuxRef(string(out), "surface")
	if ref == "" {
		return "", fmt.Errorf("cmux: new-surface: failed to parse surface ref from output %q", string(out))
	}
	return PaneID(ref), nil
}

// SplitPane creates a new split pane in the given direction and returns its surface ref.
func (a *CmuxAdapter) SplitPane(ctx context.Context, dir Direction) (PaneID, error) {
	direction := "right"
	if dir == Vertical {
		direction = "down"
	}
	args, err := a.workspaceCommandArgs("new-split", direction)
	if err != nil {
		return "", fmt.Errorf("cmux: split pane: %w", err)
	}
	cmd := execCommandContext(ctx, "cmux", args...)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("cmux: split pane: %w", err)
	}
	ref := parseCmuxRef(string(out), "surface")
	if ref == "" {
		return "", fmt.Errorf("cmux: split pane: failed to parse surface ref from output %q", string(out))
	}
	return PaneID(ref), nil
}

// FocusPane brings an existing cmux surface to the foreground. cmux exposes
// focus through move-surface --focus even when no destination is supplied.
func (a *CmuxAdapter) FocusPane(_ context.Context, paneID PaneID) error {
	if err := validatePaneID(paneID); err != nil {
		return fmt.Errorf("cmux: %w", err)
	}
	args, err := a.surfaceCommandArgs("move-surface", paneID, "--focus", "true")
	if err != nil {
		return fmt.Errorf("cmux: focus pane %s: %w", paneID, err)
	}
	cmd := execCommand("cmux", args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("cmux: focus pane %s: %w", paneID, err)
	}
	return nil
}

// SendCommand sends a command string to the specified pane via --surface flag.
func (a *CmuxAdapter) SendCommand(ctx context.Context, paneID PaneID, command string) error {
	if err := validatePaneID(paneID); err != nil {
		return fmt.Errorf("cmux: %w", err)
	}
	if isEnterCommand(command) {
		args, err := a.surfaceCommandArgs("send-key", paneID, "Enter")
		if err != nil {
			return fmt.Errorf("cmux: send enter to pane %s: %w", paneID, err)
		}
		cmd := execCommandContext(ctx, "cmux", args...)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("cmux: send enter to pane %s: %w", paneID, err)
		}
		return nil
	}
	args, err := a.surfaceCommandArgs("send", paneID, "--", command)
	if err != nil {
		return fmt.Errorf("cmux: send command to pane %s: %w", paneID, err)
	}
	cmd := execCommandContext(ctx, "cmux", args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("cmux: send command to pane %s: %w", paneID, err)
	}
	return nil
}

// SendLongText sends text to a pane via a bounded set-buffer/paste-buffer pair.
// Buffer paste is used for short and long payloads because cmux send goes
// through the active keyboard input path and can be affected by IME state.
// @AX:ANCHOR: [AUTO] public API contract — Terminal interface method; fan_in=3 (interactive.go x2, interactive_debate.go)
func (a *CmuxAdapter) SendLongText(ctx context.Context, paneID PaneID, text string) error {
	if err := validatePaneID(paneID); err != nil {
		return fmt.Errorf("cmux: %w", err)
	}
	if text == "" {
		return nil
	}
	pasteArgs, err := a.surfaceCommandArgs("paste-buffer", paneID)
	if err != nil {
		return fmt.Errorf("cmux: paste text to pane %s: %w", paneID, err)
	}
	releaseBuffer, err := acquireCmuxInputBuffer(ctx)
	if err != nil {
		return fmt.Errorf("cmux: paste text to pane %s: %w", paneID, err)
	}
	release := func() {
		if releaseBuffer != nil {
			releaseBuffer()
			releaseBuffer = nil
		}
	}
	defer release()

	setCmd := execCommandContext(ctx, "cmux", "set-buffer", "--name", cmuxInputBufferName, "--", text)
	if err := setCmd.Run(); err != nil {
		// FR-10: fallback to chunked send on set-buffer failure
		release()
		return a.sendChunked(ctx, paneID, text)
	}
	// paste-buffer — fall back to chunked send if paste fails (e.g., on recreated surfaces)
	pasteArgs = append(pasteArgs, "--name", cmuxInputBufferName)
	pasteCmd := execCommandContext(ctx, "cmux", pasteArgs...)
	if err := pasteCmd.Run(); err != nil {
		clearCmuxInputBuffer()
		release()
		return a.sendChunked(ctx, paneID, text)
	}
	clearCmuxInputBuffer()
	return nil
}

// sendChunked sends long text in chunks to avoid PTY 4KB truncation.
// Each chunk is sent via SendCommand without a trailing newline, so the
// target TUI accumulates the text in its input buffer. The caller is
// responsible for sending Enter after SendLongText returns.
func (a *CmuxAdapter) sendChunked(ctx context.Context, paneID PaneID, text string) error {
	const chunkSize = 3500 // well under 4096 PTY limit
	for i := 0; i < len(text); i += chunkSize {
		end := min(i+chunkSize, len(text))
		if err := a.SendCommand(ctx, paneID, text[i:end]); err != nil {
			return fmt.Errorf("cmux: chunked send at offset %d: %w", i, err)
		}
		// Brief pause between chunks to let PTY flush.
		if end < len(text) {
			timer := time.NewTimer(150 * time.Millisecond)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}
	}
	return nil
}

// ReadScreen reads pane content via cmux read-screen.
func (a *CmuxAdapter) ReadScreen(ctx context.Context, paneID PaneID, opts ReadScreenOpts) (string, error) {
	if err := validatePaneID(paneID); err != nil {
		return "", fmt.Errorf("cmux: %w", err)
	}
	args, err := a.surfaceCommandArgs("read-screen", paneID)
	if err != nil {
		return "", fmt.Errorf("cmux: read-screen pane %s: %w", paneID, err)
	}
	if opts.Scrollback {
		args = append(args, "--scrollback")
	}
	if opts.ScrollbackLines > 0 {
		args = append(args, "--lines", fmt.Sprintf("%d", opts.ScrollbackLines))
	}
	if opts.Lines > 0 {
		args = append(args, "--lines", fmt.Sprintf("%d", opts.Lines))
	}
	cmd := execCommandContext(ctx, "cmux", args...)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("cmux: read-screen pane %s: %w", paneID, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// PipePaneStart starts streaming pane output to a file via cmux pipe-pane.
func (a *CmuxAdapter) PipePaneStart(_ context.Context, paneID PaneID, outputFile string) error {
	if err := validatePaneID(paneID); err != nil {
		return fmt.Errorf("cmux: %w", err)
	}
	// SEC-007: shell-escape outputFile to prevent command injection via malicious paths
	args, err := a.surfaceCommandArgs(
		"pipe-pane", paneID,
		"--command", "cat >> '"+strings.ReplaceAll(outputFile, "'", "'\\''")+"'",
	)
	if err != nil {
		return fmt.Errorf("cmux: pipe-pane start pane %s: %w", paneID, err)
	}
	cmd := execCommand("cmux", args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("cmux: pipe-pane start pane %s: %w", paneID, err)
	}
	return nil
}

// PipePaneStop stops pipe-pane output streaming via empty command.
func (a *CmuxAdapter) PipePaneStop(_ context.Context, paneID PaneID) error {
	if err := validatePaneID(paneID); err != nil {
		return fmt.Errorf("cmux: %w", err)
	}
	args, err := a.surfaceCommandArgs("pipe-pane", paneID, "--command", "")
	if err != nil {
		return fmt.Errorf("cmux: pipe-pane stop pane %s: %w", paneID, err)
	}
	cmd := execCommand("cmux", args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("cmux: pipe-pane stop pane %s: %w", paneID, err)
	}
	return nil
}
