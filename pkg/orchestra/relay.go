package orchestra

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// relayStageResult holds the output of a single relay stage.
type relayStageResult struct {
	provider string
	output   string
}

// runRelay executes providers sequentially in agentic mode.
// Each provider's result is saved to a temp file and injected into the next prompt.
// On failure, remaining providers are skipped and partial results are returned.
// @AX:NOTE: [AUTO] temp dir written under os.TempDir() with 0o700 — not cleaned on abnormal exit (deferred only)
// @AX:NOTE: [AUTO] skip-continue: failed providers are skipped; relay continues with next provider (REQ-3a)
func runRelay(ctx context.Context, cfg *OrchestraConfig) ([]ProviderResponse, error) {
	jobID := randomHex() + randomHex()
	relayDir := filepath.Join(os.TempDir(), fmt.Sprintf("autopus-relay-%s", jobID))

	if err := os.MkdirAll(relayDir, 0o700); err != nil {
		return nil, fmt.Errorf("relay: failed to create temp dir: %w", err)
	}

	defer cleanupRelayDir(relayDir, cfg.KeepRelayOutput)

	var responses []ProviderResponse
	var previous []relayStageResult

	for _, p := range cfg.Providers {
		// Build agentic provider config (copy args, do not mutate original)
		agenticP := p
		agenticP.Args = append(append([]string{}, p.Args...), agenticArgs(p.Name)...)

		prompt := buildRelayPrompt(cfg.Prompt, previous)

		resp, err := runProvider(ctx, agenticP, prompt)
		if err != nil {
			// Skip failed provider and continue with next (REQ-3a)
			failed := ProviderResponse{
				Provider: p.Name,
				Output:   fmt.Sprintf("[SKIPPED: %s — %s]", p.Name, err.Error()),
				ExitCode: -1,
			}
			if resp != nil {
				failed.ExitCode = resp.ExitCode
				failed.Error = resp.Error
				failed.Duration = resp.Duration
			}
			responses = append(responses, failed)
			continue
		}

		responses = append(responses, *resp)

		// Save result to temp file (filepath.Base prevents path traversal via provider name)
		safeName := filepath.Base(p.Name)
		outputFile := filepath.Join(relayDir, fmt.Sprintf("%s.md", safeName))
		_ = os.WriteFile(outputFile, []byte(resp.Output), 0o600)

		previous = append(previous, relayStageResult{
			provider: p.Name,
			output:   resp.Output,
		})
	}

	// Return error only if all providers failed (REQ-3a)
	allFailed := true
	for _, r := range responses {
		if r.ExitCode != -1 {
			allFailed = false
			break
		}
	}
	if allFailed && len(responses) > 0 {
		return responses, fmt.Errorf("relay: all providers failed")
	}

	return responses, nil
}

// buildRelayPrompt constructs a prompt that includes previous provider analyses.
func buildRelayPrompt(original string, previousResults []relayStageResult) string {
	if len(previousResults) == 0 {
		return original
	}

	var sb strings.Builder
	sb.WriteString(original)

	for _, r := range previousResults {
		fmt.Fprintf(&sb, "\n\n## Previous Analysis by %s\n\n%s", r.provider, r.output)
	}

	return sb.String()
}

// agenticArgs returns extra CLI flags to enable agentic (tool-access) mode per provider.
// @AX:NOTE: [AUTO] hardcoded provider flag lists — update when adding new providers or changing CLI flags
func agenticArgs(providerName string) []string {
	switch providerName {
	case "claude":
		return []string{"--allowedTools", "Read,Grep,Bash,Glob"}
	case "codex":
		return []string{"--approval-mode", "full-auto", "--quiet"}
	default:
		// gemini and unknown providers: no extra flags
		return nil
	}
}

// FormatRelay formats relay responses as staged output sections.
// @AX:NOTE: [AUTO] public API — called by strategy.go handleRelay and runner.go merge block (fan_in=2, downgraded from ANCHOR)
func FormatRelay(responses []ProviderResponse) string {
	var sb strings.Builder
	for i, r := range responses {
		fmt.Fprintf(&sb, "## Relay Stage %d: (by %s)\n\n%s\n", i+1, r.Provider, r.Output)
	}
	return sb.String()
}

// cleanupRelayDir removes the relay temp directory unless keep is true.
func cleanupRelayDir(dir string, keep bool) {
	if !keep {
		_ = os.RemoveAll(dir)
	}
}
