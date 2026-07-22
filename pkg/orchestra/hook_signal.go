package orchestra

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// HookResult represents a structured result from a provider hook.
type HookResult struct {
	Output   string `json:"output"`
	ExitCode int    `json:"exit_code"`
}

// HookSession manages the file-based signal protocol for hook result collection.
type HookSession struct {
	sessionID            string
	sessionDir           string
	hookProviders        map[string]bool
	startupHookProviders map[string]bool
	hookHealth           hookCompletionHealth
	startupHealth        hookStartupHealth
	storage              *hookSessionStorage
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
	if err := validateHookSessionID(sessionID); err != nil {
		return nil, fmt.Errorf("create hook session: %w", err)
	}
	storage, dir, err := newHookSessionStorage(sessionID)
	if err != nil {
		return nil, fmt.Errorf("create hook session: %w", err)
	}

	return &HookSession{
		sessionID:            sessionID,
		sessionDir:           dir,
		hookProviders:        DefaultHookProviders(),
		startupHookProviders: DefaultStartupHookProviders(),
		storage:              storage,
	}, nil
}

// ApplyProviderHooks resolves independent completion and startup overrides.
func (s *HookSession) ApplyProviderHooks(providers []ProviderConfig) {
	s.SetHookProviders(resolveHookProviders(providers))
	s.SetStartupHookProviders(resolveStartupHookProviders(providers))
}

// WaitForDone polls for the provider-specific "{provider}-done" file at 200ms intervals.
// Returns nil when the done file is detected, or error on timeout.
// @AX:NOTE [AUTO] magic constant 200ms polling interval — balances responsiveness vs CPU; adjust with care
func (s *HookSession) WaitForDone(timeout time.Duration, providers ...string) error {
	if len(providers) > 0 {
		if err := validateHookArtifactProvider(providers[0]); err != nil {
			return err
		}
	}
	deadline := time.After(timeout)
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	// Use provider-specific done file if provider name is given (R1 protocol)
	doneName := "done"
	if len(providers) > 0 {
		doneName = providerArtifactIdentity(providers[0]) + "-done"
	}

	for {
		select {
		case <-deadline:
			return fmt.Errorf("timeout waiting for done signal in session %s", s.sessionID)
		case <-ticker.C:
			if _, err := s.statArtifact(doneName); err == nil {
				return nil
			}
		}
	}
}

// WaitForDoneRound polls for the round-scoped done signal file.
// When round > 0, uses RoundSignalName to generate the filename;
// otherwise falls back to the standard provider-done format.
func (s *HookSession) WaitForDoneRound(timeout time.Duration, provider string, round int) error {
	if err := validateHookArtifactProvider(provider); err != nil {
		return err
	}
	if round > 0 {
		doneName := RoundSignalName(provider, round, "done")
		return s.waitForFileCtx(context.Background(), timeout, doneName)
	}
	return s.WaitForDone(timeout, provider)
}

// WaitForDoneRoundCtx polls for the round-scoped done signal file, respecting context cancellation.
func (s *HookSession) WaitForDoneRoundCtx(ctx context.Context, timeout time.Duration, provider string, round int) error {
	if err := validateHookArtifactProvider(provider); err != nil {
		return err
	}
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

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled waiting for %s in session %s: %w", filename, s.sessionID, ctx.Err())
		case <-deadline:
			return fmt.Errorf("timeout waiting for %s in session %s", filename, s.sessionID)
		case <-ticker.C:
			if _, err := s.statArtifact(filename); err == nil {
				return nil
			}
		}
	}
}

// ReadResult reads and parses the provider-specific "{provider}-result.json" from the session directory.
func (s *HookSession) ReadResult(providers ...string) (*HookResult, error) {
	// Use provider-specific result file if provider name is given (R1 protocol)
	resultName := "result.json"
	if len(providers) > 0 {
		if err := validateHookArtifactProvider(providers[0]); err != nil {
			return nil, err
		}
		resultName = providerArtifactIdentity(providers[0]) + "-result.json"
	}
	data, err := s.readArtifact(resultName)
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
	if err := validateHookArtifactProvider(provider); err != nil {
		return nil, err
	}
	if round > 0 {
		resultName := RoundSignalName(provider, round, "result.json")
		return s.readResultFile(resultName)
	}
	return s.ReadResult(provider)
}

// ResetAttempt removes stale IPC artifacts for exactly one provider attempt.
// Round zero keeps the legacy unscoped done/result names while readiness and
// bidirectional-input signals remain explicitly round-scoped. Sibling providers
// and other rounds are never touched, so parallel executions may safely share a
// HookSession directory.
func (s *HookSession) ResetAttempt(provider string, round int) error {
	if err := validateHookArtifactProvider(provider); err != nil {
		return err
	}
	safeProvider := providerArtifactIdentity(provider)
	doneName := safeProvider + "-done"
	resultName := safeProvider + "-result.json"
	if round > 0 {
		doneName = RoundSignalName(safeProvider, round, "done")
		resultName = RoundSignalName(safeProvider, round, "result.json")
	}

	names := []string{
		doneName,
		resultName,
		RoundSignalName(safeProvider, round, "ready"),
		RoundSignalName(safeProvider, round, "input.json"),
		RoundSignalName(safeProvider, round, "abort"),
	}
	for _, name := range names {
		if err := s.removeArtifact(name); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("reset hook attempt artifact %s: %w", name, err)
		}
	}
	return nil
}

func validateHookArtifactProvider(provider string) error {
	return validateSafeArtifactName("hook provider name", provider)
}

// readResultFile reads and parses a named result file from the session directory.
func (s *HookSession) readResultFile(filename string) (*HookResult, error) {
	data, err := s.readArtifact(filename)
	if err != nil {
		return nil, fmt.Errorf("read result file: %w", err)
	}

	var result HookResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse result json: %w", err)
	}

	return &result, nil
}

// Cleanup removes an owned session directory. Reusers only close their handles.
func (s *HookSession) Cleanup() {
	if s != nil && s.storage != nil {
		s.storage.cleanup()
	}
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
	if err := validateHookArtifactProvider(provider); err != nil {
		return err
	}
	filename := RoundSignalName(provider, round, "input.json")
	input := HookInput{Provider: providerArtifactIdentity(provider), Round: round, Prompt: prompt}
	return s.writeJSONArtifact(filename, input)
}

// WaitForReady polls for the provider's ready signal file (convenience wrapper).
func (s *HookSession) WaitForReady(timeout time.Duration, provider string, round int) error {
	return s.WaitForReadyCtx(context.Background(), timeout, provider, round)
}

// WaitForReadyCtx polls for the round-scoped ready signal file, respecting context.
// Ready file format: {provider}-round{N}-ready
func (s *HookSession) WaitForReadyCtx(ctx context.Context, timeout time.Duration, provider string, round int) error {
	if err := validateHookArtifactProvider(provider); err != nil {
		return err
	}
	readyName := RoundSignalName(provider, round, "ready")
	return s.waitForFileCtx(ctx, timeout, readyName)
}

// WriteAbortSignal creates an abort signal file to unblock hook input watchers.
// R5-SAFETY: Prevents deadlock when Orchestra falls back to SendLongText.
func (s *HookSession) WriteAbortSignal(provider string, round int) error {
	if err := validateHookArtifactProvider(provider); err != nil {
		return err
	}
	abortName := RoundSignalName(provider, round, "abort")
	return s.writeArtifact(abortName, []byte{}, 0o600)
}
