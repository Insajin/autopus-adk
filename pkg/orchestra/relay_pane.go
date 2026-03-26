package orchestra

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/insajin/autopus-adk/pkg/detect"
	"github.com/insajin/autopus-adk/pkg/terminal"
)

// runRelayPaneOrchestra executes relay strategy using sequential terminal panes.
// Each provider runs in its own pane and its output is injected into the next prompt.
// Falls back to standard relay when terminal is nil or "plain".
// @AX:NOTE [AUTO] temp dir written under os.TempDir() with 0o700 — not cleaned on abnormal exit (deferred only)
// @AX:NOTE [AUTO] skip-continue: failed providers are skipped; relay continues with next provider (REQ-3a)
func runRelayPaneOrchestra(ctx context.Context, cfg OrchestraConfig) (*OrchestraResult, error) {
	// Fallback: nil or plain terminal delegates to standard relay
	if cfg.Terminal == nil || cfg.Terminal.Name() == "plain" {
		responses, err := runRelay(ctx, &cfg)
		if err != nil {
			return nil, err
		}
		return &OrchestraResult{
			Strategy:  cfg.Strategy,
			Responses: responses,
			Merged:    FormatRelay(responses),
			Summary:   fmt.Sprintf("relay: %d stages completed", len(responses)),
		}, nil
	}

	start := time.Now()
	timeout := cfg.TimeoutSeconds
	if timeout <= 0 {
		timeout = 120
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	// Create relay temp directory
	jobID := randomHex() + randomHex()
	relayDir := filepath.Join(os.TempDir(), fmt.Sprintf("autopus-relay-%s", jobID))
	if err := os.MkdirAll(relayDir, 0o700); err != nil {
		return nil, fmt.Errorf("relay pane: failed to create temp dir: %w", err)
	}
	defer cleanupRelayDir(relayDir, cfg.KeepRelayOutput)

	var (
		responses []ProviderResponse
		previous  []relayStageResult
		panes     []paneInfo
	)
	defer func() { cleanupPanes(cfg.Terminal, panes) }()

	for _, p := range cfg.Providers {
		resp := executeRelayPaneProvider(timeoutCtx, cfg.Terminal, p, cfg.Prompt, relayDir, previous, &panes)
		responses = append(responses, resp)

		// Accumulate outputs for context injection (skip truly failed providers)
		if resp.ExitCode != -1 {
			previous = append(previous, relayStageResult{
				provider: p.Name,
				output:   resp.Output,
			})
		}
	}

	// Return error only if all providers were SKIPPED (ExitCode == -1) (REQ-3a)
	allFailed := true
	for _, r := range responses {
		if r.ExitCode != -1 {
			allFailed = false
			break
		}
	}
	if allFailed && len(responses) > 0 {
		return nil, fmt.Errorf("relay pane: all providers failed")
	}

	total := time.Since(start)
	return &OrchestraResult{
		Strategy:  cfg.Strategy,
		Responses: responses,
		Merged:    FormatRelay(responses),
		Duration:  total,
		Summary:   fmt.Sprintf("relay pane: %d stages completed", len(responses)),
	}, nil
}

// executeRelayPaneProvider runs a single provider in a pane and collects its output.
// On any failure (split, send, sentinel wait), it returns a SKIPPED response.
func executeRelayPaneProvider(
	ctx context.Context,
	term terminal.Terminal,
	provider ProviderConfig,
	prompt, relayDir string,
	previous []relayStageResult,
	panes *[]paneInfo,
) ProviderResponse {
	// Pre-check: verify binary exists before allocating a pane
	if !detect.IsInstalled(provider.Binary) {
		return skippedResponse(provider.Name, fmt.Sprintf("binary not found: %s", provider.Binary))
	}

	safeName := sanitizeProviderName(provider.Name)
	outputFile := filepath.Join(relayDir, fmt.Sprintf("%s.md", safeName))

	// Create pane
	paneID, err := term.SplitPane(ctx, terminal.Horizontal)
	if err != nil {
		return skippedResponse(provider.Name, fmt.Sprintf("SplitPane failed: %v", err))
	}
	*panes = append(*panes, paneInfo{paneID: paneID, outputFile: outputFile, provider: provider})

	// Build and send command
	cmd := buildRelayPaneCommand(provider, prompt, outputFile, previous)
	if err := term.SendCommand(ctx, paneID, cmd); err != nil {
		return skippedResponse(provider.Name, fmt.Sprintf("SendCommand failed: %v", err))
	}

	// Wait for sentinel completion; timeout is non-fatal (provider may still have produced output)
	timedOut := waitForSentinel(ctx, outputFile) != nil
	output := readOutputFile(outputFile)

	// If no sentinel and no output, the command was sent but produced nothing
	if timedOut && output == "" {
		output = fmt.Sprintf("[pane: %s completed]", provider.Name)
	}

	return ProviderResponse{
		Provider: provider.Name,
		Output:   output,
		TimedOut: timedOut,
	}
}

// skippedResponse creates a SKIPPED response for a failed provider (REQ-3a pattern).
// @AX:NOTE [AUTO] ExitCode -1 is the magic sentinel for "provider never ran" — checked by allFailed logic
func skippedResponse(name, reason string) ProviderResponse {
	return ProviderResponse{
		Provider: name,
		Output:   fmt.Sprintf("[SKIPPED: %s -- %s]", name, reason),
		ExitCode: -1,
	}
}

// buildRelayPaneCommand constructs a heredoc shell command for relay pane execution.
// Uses interactive mode (no -p flag) with prompt injected via heredoc (REQ-4).
// SEC-001: unique heredoc delimiter. SEC-006: escaped binary path.
func buildRelayPaneCommand(provider ProviderConfig, prompt, outputFile string, previous []relayStageResult) string {
	fullPrompt := buildRelayPrompt(prompt, previous)

	// SEC-006: escape binary path
	binary := shellEscapeArg(provider.Binary)

	// Build args without -p flag (REQ-4: interactive mode)
	args := filterMinusPFlag(paneArgs(provider))
	escapedArgs := shellEscapeArgs(args)

	// SEC-001: collision-free heredoc delimiter
	delim := uniqueHeredocDelimiter("RELAY_EOF", fullPrompt, randomHex())

	var sb strings.Builder
	fmt.Fprintf(&sb, "%s %s <<'%s'\n%s\n%s\n | tee %s; echo %s >> %s",
		binary, escapedArgs, delim, fullPrompt, delim, outputFile, sentinel, outputFile)
	return sb.String()
}

// filterMinusPFlag removes -p flag from args to enforce interactive mode (REQ-4).
func filterMinusPFlag(args []string) []string {
	filtered := make([]string, 0, len(args))
	for _, a := range args {
		if a != "-p" {
			filtered = append(filtered, a)
		}
	}
	return filtered
}
