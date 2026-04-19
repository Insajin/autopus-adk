package orchestra

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// HookResult represents a structured result from a provider hook.
type HookResult struct {
	Output   string `json:"output"`
	ExitCode int    `json:"exit_code"`
}

// HookSession manages the file-based signal protocol for hook result collection.
type HookSession struct {
	sessionID     string
	sessionDir    string
	hookProviders map[string]bool
}

// defaultHookProviders lists providers that have hooks by default.
// @AX:NOTE [AUTO] hardcoded provider set — update when adding new hook-capable providers
var defaultHookProviders = map[string]bool{
	"claude": true,
	"gemini": true,
	"codex":  true,
}

// NewHookSession creates a new hook session with the given session ID.
// Creates /tmp/autopus/{session-id}/ directory with 0o700 permissions.
// @AX:ANCHOR [AUTO] fan_in=4 — called by interactive.go, interactive_debate.go, relay_pane.go, hook_watcher.go; do not change session dir layout
func NewHookSession(sessionID string) (*HookSession, error) {
	dir := filepath.Join(os.TempDir(), "autopus", sanitizeProviderName(sessionID))

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create session dir: %w", err)
	}

	return &HookSession{
		sessionID:     sessionID,
		sessionDir:    dir,
		hookProviders: defaultHookProviders,
	}, nil
}

// WaitForDone polls for the provider-specific "{provider}-done" file at 200ms intervals.
// Returns nil when the done file is detected, or error on timeout.
// @AX:NOTE [AUTO] magic constant 200ms polling interval — balances responsiveness vs CPU; adjust with care
func (s *HookSession) WaitForDone(timeout time.Duration, providers ...string) error {
	deadline := time.After(timeout)
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	// Use provider-specific done file if provider name is given (R1 protocol)
	doneName := "done"
	if len(providers) > 0 && providers[0] != "" {
		doneName = sanitizeProviderName(providers[0]) + "-done"
	}
	donePath := filepath.Join(s.sessionDir, doneName)

	for {
		select {
		case <-deadline:
			return fmt.Errorf("timeout waiting for done signal in session %s", s.sessionID)
		case <-ticker.C:
			if _, err := os.Stat(donePath); err == nil {
				return nil
			}
		}
	}
}

// WaitForDoneRound polls for the round-scoped done signal file.
// When round > 0, uses RoundSignalName to generate the filename;
// otherwise falls back to the standard provider-done format.
func (s *HookSession) WaitForDoneRound(timeout time.Duration, provider string, round int) error {
	if round > 0 {
		doneName := RoundSignalName(provider, round, "done")
		return s.waitForFileCtx(context.Background(), timeout, doneName)
	}
	return s.WaitForDone(timeout, provider)
}

// WaitForDoneRoundCtx polls for the round-scoped done signal file, respecting context cancellation.
func (s *HookSession) WaitForDoneRoundCtx(ctx context.Context, timeout time.Duration, provider string, round int) error {
	if round > 0 {
		doneName := RoundSignalName(provider, round, "done")
		return s.waitForFileCtx(ctx, timeout, doneName)
	}
	return s.WaitForDone(timeout, provider)
}

// waitForFileCtx polls for a specific file at 200ms intervals, respecting context cancellation.
func (s *HookSession) waitForFileCtx(ctx context.Context, timeout time.Duration, filename string) error {
	deadline := time.After(timeout)
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	path := filepath.Join(s.sessionDir, filename)
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled waiting for %s in session %s: %w", filename, s.sessionID, ctx.Err())
		case <-deadline:
			return fmt.Errorf("timeout waiting for %s in session %s", filename, s.sessionID)
		case <-ticker.C:
			if _, err := os.Stat(path); err == nil {
				return nil
			}
		}
	}
}

// ReadResult reads and parses the provider-specific "{provider}-result.json" from the session directory.
func (s *HookSession) ReadResult(providers ...string) (*HookResult, error) {
	// Use provider-specific result file if provider name is given (R1 protocol)
	resultName := "result.json"
	if len(providers) > 0 && providers[0] != "" {
		resultName = sanitizeProviderName(providers[0]) + "-result.json"
	}
	data, err := os.ReadFile(filepath.Join(s.sessionDir, resultName))
	if err != nil {
		return nil, fmt.Errorf("read result file: %w", err)
	}

	var result HookResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse result json: %w", err)
	}

	return &result, nil
}

// ReadResultRound reads the round-scoped result file for a provider.
// When round > 0, uses RoundSignalName to generate the filename;
// otherwise falls back to the standard provider-result.json format.
func (s *HookSession) ReadResultRound(provider string, round int) (*HookResult, error) {
	if round > 0 {
		resultName := RoundSignalName(provider, round, "result.json")
		return s.readResultFile(resultName)
	}
	return s.ReadResult(provider)
}

// readResultFile reads and parses a named result file from the session directory.
func (s *HookSession) readResultFile(filename string) (*HookResult, error) {
	data, err := os.ReadFile(filepath.Join(s.sessionDir, filename))
	if err != nil {
		return nil, fmt.Errorf("read result file: %w", err)
	}

	var result HookResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse result json: %w", err)
	}

	return &result, nil
}

// Cleanup removes the session directory and all its contents.
func (s *HookSession) Cleanup() {
	_ = os.RemoveAll(s.sessionDir)
}

// HasHook checks if a provider has hook configuration available.
func (s *HookSession) HasHook(provider string) bool {
	return s.hookProviders[provider]
}

// SetHookProviders overrides which providers have hooks configured.
func (s *HookSession) SetHookProviders(providers map[string]bool) {
	s.hookProviders = providers
}

// Dir returns the session directory path.
func (s *HookSession) Dir() string {
	return s.sessionDir
}

// SessionID returns the session's unique identifier.
func (s *HookSession) SessionID() string {
	return s.sessionID
}

// WriteInput writes a prompt to the provider's input file (convenience for round 0).
func (s *HookSession) WriteInput(provider, prompt string) error {
	return s.WriteInputRound(provider, 0, prompt)
}

// WriteInputRound writes a round-scoped input prompt file using atomic write.
// Creates {provider}-round{N}-input.json with HookInput JSON.
func (s *HookSession) WriteInputRound(provider string, round int, prompt string) error {
	filename := RoundSignalName(provider, round, "input.json")
	path := filepath.Join(s.sessionDir, filename)
	input := HookInput{Provider: provider, Round: round, Prompt: prompt}
	return atomicWriteJSON(path, input)
}

// WaitForReady polls for the provider's ready signal file (convenience wrapper).
func (s *HookSession) WaitForReady(timeout time.Duration, provider string, round int) error {
	return s.WaitForReadyCtx(context.Background(), timeout, provider, round)
}

// WaitForReadyCtx polls for the round-scoped ready signal file, respecting context.
// Ready file format: {provider}-round{N}-ready
func (s *HookSession) WaitForReadyCtx(ctx context.Context, timeout time.Duration, provider string, round int) error {
	readyName := RoundSignalName(provider, round, "ready")
	return s.waitForFileCtx(ctx, timeout, readyName)
}

// WriteAbortSignal creates an abort signal file to unblock hook input watchers.
// R5-SAFETY: Prevents deadlock when Orchestra falls back to SendLongText.
func (s *HookSession) WriteAbortSignal(provider string, round int) error {
	abortName := RoundSignalName(provider, round, "abort")
	path := filepath.Join(s.sessionDir, abortName)
	return os.WriteFile(path, []byte{}, 0o600)
}
