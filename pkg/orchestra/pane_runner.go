package orchestra

import (
	"bufio"
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/insajin/autopus-adk/pkg/terminal"
)

// @AX:NOTE [AUTO] sentinel marker written to output files to signal provider completion
const sentinel = "__AUTOPUS_DONE__"

// paneInfo tracks a provider's pane and output file.
type paneInfo struct {
	paneID     terminal.PaneID
	outputFile string
	provider   ProviderConfig
	skipWait   bool // true when SendCommand failed — skip sentinel wait
}

// RunPaneOrchestra runs orchestration using terminal panes when available.
// Falls back to RunOrchestra for plain terminals or when pane creation fails.
// @AX:NOTE [AUTO] pane-based orchestration entry point — 2 callers (runner.go, tests)
func RunPaneOrchestra(ctx context.Context, cfg OrchestraConfig) (*OrchestraResult, error) {
	// Fallback: nil terminal or plain terminal
	if cfg.Terminal == nil || cfg.Terminal.Name() == "plain" {
		return RunOrchestra(ctx, cfg)
	}

	// Interactive mode: use interactive CLI sessions instead of sentinel-based (SPEC-ORCH-006)
	if cfg.Interactive {
		return RunInteractivePaneOrchestra(ctx, cfg)
	}

	// Relay strategy uses sequential pane execution (SPEC-ORCH-005)
	if cfg.Strategy == StrategyRelay {
		return runRelayPaneOrchestra(ctx, cfg)
	}

	start := time.Now()
	timeout := cfg.TimeoutSeconds
	if timeout <= 0 {
		timeout = 120
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	// Split panes for each provider
	panes, failed, err := splitProviderPanes(timeoutCtx, cfg)
	if err != nil {
		return runFallback(ctx, cfg)
	}

	// Ensure cleanup runs on exit
	defer cleanupPanes(cfg.Terminal, panes)

	// Send commands to each pane (REV-001: track SendCommand failures)
	sendFailed := sendPaneCommands(timeoutCtx, cfg, panes)
	failed = append(failed, sendFailed...)

	// Wait for all providers and collect results
	responses, waitFailed := collectPaneResults(timeoutCtx, panes, start)
	failed = append(failed, waitFailed...)

	total := time.Since(start)
	merged, summary := mergeByStrategy(cfg.Strategy, responses, cfg)
	if merged == "" {
		merged = fmt.Sprintf("[pane mode] %d providers executed", len(responses))
	}

	return &OrchestraResult{
		Strategy:        cfg.Strategy,
		Responses:       responses,
		Merged:          merged,
		Duration:        total,
		Summary:         summary,
		FailedProviders: failed,
	}, nil
}

// splitProviderPanes creates a pane and temp file for each provider.
// Uses independent surfaces (tabs) when the terminal supports SurfaceCreator,
// falling back to horizontal splits otherwise. Independent surfaces prevent
// pane width starvation when 3+ providers run in parallel.
// Returns early with error if pane creation fails (caller should fallback).
func splitProviderPanes(ctx context.Context, cfg OrchestraConfig) ([]paneInfo, []FailedProvider, error) {
	surfCreator, hasSurface := cfg.Terminal.(terminal.SurfaceCreator)
	panes := make([]paneInfo, 0, len(cfg.Providers))
	for _, p := range cfg.Providers {
		var paneID terminal.PaneID
		var err error
		if hasSurface {
			paneID, err = surfCreator.CreateSurface(ctx)
		} else {
			paneID, err = cfg.Terminal.SplitPane(ctx, terminal.Horizontal)
		}
		if err != nil {
			cleanupPanes(cfg.Terminal, panes)
			return nil, nil, err
		}
		// SEC-002: sanitize provider name to prevent path traversal
		safeName := sanitizeProviderName(p.Name)
		// SEC-003: use os.CreateTemp to avoid symlink race
		tmpFile, err := os.CreateTemp("", "autopus-orch-"+safeName+"-")
		if err != nil {
			cleanupPanes(cfg.Terminal, panes)
			return nil, nil, err
		}
		tmpFile.Close()
		panes = append(panes, paneInfo{paneID: paneID, outputFile: tmpFile.Name(), provider: p})
	}
	return panes, nil, nil
}

// sendPaneCommands sends the interactive command to each pane.
// Returns FailedProvider entries for any SendCommand errors.
// Appends \n to execute the command (cmux send does not auto-press Enter).
func sendPaneCommands(ctx context.Context, cfg OrchestraConfig, panes []paneInfo) []FailedProvider {
	var failed []FailedProvider
	for i, pi := range panes {
		cmd := buildPaneCommand(pi.provider, cfg.Prompt, pi.outputFile) + "\n"
		if err := cfg.Terminal.SendCommand(ctx, pi.paneID, cmd); err != nil {
			failed = append(failed, FailedProvider{
				Name:  pi.provider.Name,
				Error: fmt.Sprintf("SendCommand failed: %v", err),
			})
			panes[i].skipWait = true
		}
	}
	return failed
}

// collectPaneResults waits for each pane's sentinel and collects output.
// Every provider produces a response. Timed-out providers are also recorded as failed.
func collectPaneResults(ctx context.Context, panes []paneInfo, start time.Time) ([]ProviderResponse, []FailedProvider) {
	var (
		responses []ProviderResponse
		failed    []FailedProvider
		mu        sync.Mutex
		wg        sync.WaitGroup
	)

	for _, pi := range panes {
		if pi.skipWait {
			// SendCommand already failed — record as response with no output
			responses = append(responses, ProviderResponse{
				Provider: pi.provider.Name,
				Duration: time.Since(start),
				TimedOut: true,
			})
			continue
		}
		wg.Add(1)
		go func(pi paneInfo) {
			defer wg.Done()
			err := waitForSentinel(ctx, pi.outputFile)
			output := readOutputFile(pi.outputFile)

			mu.Lock()
			defer mu.Unlock()

			responses = append(responses, ProviderResponse{
				Provider: pi.provider.Name,
				Output:   output,
				Duration: time.Since(start),
				TimedOut: err != nil,
			})
			if err != nil {
				failed = append(failed, FailedProvider{
					Name:  pi.provider.Name,
					Error: err.Error(),
				})
			}
		}(pi)
	}
	wg.Wait()
	return responses, failed
}

// paneArgs returns the args to use in pane mode for the given provider.
// Uses PaneArgs if set; otherwise falls back to Args unchanged.
func paneArgs(p ProviderConfig) []string {
	if len(p.PaneArgs) > 0 {
		return p.PaneArgs
	}
	return p.Args
}

// buildPaneCommand constructs the shell command to execute in a pane.
// SEC-001/SEC-004: all arguments are shell-escaped to prevent injection.
func buildPaneCommand(provider ProviderConfig, prompt, outputFile string) string {
	// SEC-004: escape each arg individually
	args := shellEscapeArgs(paneArgs(provider))

	// SEC-006: escape binary path to prevent shell metacharacter injection
	binary := shellEscapeArg(provider.Binary)

	// SEC-007: escape outputFile to prevent command injection
	safeOutput := shellEscapeArg(outputFile)

	if provider.PromptViaArgs {
		// SEC-001: use shell-escaped prompt instead of raw double quotes
		return fmt.Sprintf("%s %s %s | tee %s; echo %s >> %s",
			binary, args, shellEscapeArg(prompt), safeOutput, sentinel, safeOutput)
	}
	// SEC-001: use unique heredoc delimiter to prevent prompt content from terminating it
	delim := uniqueHeredocDelimiter("PROMPT_EOF", prompt, randomHex())
	return fmt.Sprintf("%s %s <<'%s'\n%s\n%s\n | tee %s; echo %s >> %s",
		binary, args, delim, prompt, delim, safeOutput, sentinel, safeOutput)
}

// waitForSentinel polls the output file until the sentinel marker is found.
func waitForSentinel(ctx context.Context, outputFile string) error {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if hasSentinel(outputFile) {
				return nil
			}
		}
	}
}

// hasSentinel checks if the output file contains the sentinel marker.
func hasSentinel(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), sentinel) {
			return true
		}
	}
	return false
}

// readOutputFile reads the output file and strips the sentinel marker.
func readOutputFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	output := strings.ReplaceAll(string(data), sentinel, "")
	return strings.TrimSpace(output)
}

// mergeByStrategy applies the configured merge strategy to responses.
func mergeByStrategy(s Strategy, responses []ProviderResponse, cfg OrchestraConfig) (string, string) {
	switch s {
	case StrategyPipeline:
		return FormatPipeline(responses), fmt.Sprintf("파이프라인: %d단계 완료", len(responses))
	case StrategyDebate:
		return buildDebateMerged(responses, cfg)
	case StrategyFastest:
		if len(responses) > 0 {
			return responses[0].Output, fmt.Sprintf("최속 응답: %s", responses[0].Provider)
		}
		return "", "응답 없음"
	default:
		return MergeConsensus(responses, 0.66)
	}
}

// runFallback runs the standard non-pane orchestration as fallback.
func runFallback(ctx context.Context, cfg OrchestraConfig) (*OrchestraResult, error) {
	fallbackCfg := cfg
	fallbackCfg.Terminal = nil
	return RunOrchestra(ctx, fallbackCfg)
}

// cleanupPanes closes panes and removes temporary output files.
func cleanupPanes(term terminal.Terminal, panes []paneInfo) {
	ctx := context.Background()
	for _, pi := range panes {
		_ = term.Close(ctx, string(pi.paneID))
		_ = os.Remove(pi.outputFile)
	}
}

// randomHex returns an 8-character random hex string.
// SEC-005: falls back to timestamp-based value on rand.Read failure.
func randomHex() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%08x", time.Now().UnixNano()&0xFFFFFFFF)
	}
	return fmt.Sprintf("%x", b)
}
