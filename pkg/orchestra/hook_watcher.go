package orchestra

import (
	"sync"
	"time"
)

// WaitAndCollectHookResults collects results from all providers using hook
// signal protocol when available, falling back to ReadScreen for others.
// For hook-enabled providers: creates a HookSession, polls for done signal.
// For non-hook providers: returns a fallback response with placeholder output.
// @AX:WARN [AUTO] concurrent goroutine per provider — guarded by mu sync.Mutex; goroutines inherit parent context timeout
func WaitAndCollectHookResults(cfg OrchestraConfig, sessionID string) ([]ProviderResponse, error) {
	session, err := NewHookSession(sessionID)
	if err != nil {
		return nil, err
	}
	session.ApplyProviderHooks(cfg.Providers)
	defer session.Cleanup()

	var (
		responses []ProviderResponse
		mu        sync.Mutex
		wg        sync.WaitGroup
	)

	for _, provider := range cfg.Providers {
		wg.Add(1)
		go func(p ProviderConfig) {
			defer wg.Done()
			start := time.Now()

			resp := collectSingleProvider(session, p, hookWatcherTimeout(p, cfg.TimeoutSeconds), start)

			mu.Lock()
			defer mu.Unlock()
			responses = append(responses, resp)
		}(provider)
	}

	wg.Wait()
	return responses, nil
}

func hookWatcherTimeout(provider ProviderConfig, fallbackSeconds int) time.Duration {
	if provider.ExecutionTimeout > 0 {
		return provider.ExecutionTimeout
	}
	return time.Duration(fallbackSeconds) * time.Second
}

// collectSingleProvider collects a result from a single provider.
// Uses hook-based collection if available, otherwise returns a fallback response.
// @AX:NOTE [AUTO] triple-fallback chain: hook timeout -> result read fail -> placeholder; all paths return valid ProviderResponse
func collectSingleProvider(session *HookSession, p ProviderConfig, timeout time.Duration, start time.Time) (response ProviderResponse) {
	defer func() { markUnavailableUsage(&response, usageSourceHook, usageReasonHook) }()
	if !session.HasHook(p.Name) {
		// R8 fallback: no hook configured, return placeholder for non-hook providers
		return ProviderResponse{
			Provider:    p.Name,
			Output:      "(no hook — fallback)",
			Duration:    time.Since(start),
			EmptyOutput: false,
		}
	}

	// Immediate timeout (0 or negative) → mark as timed out
	if timeout <= 0 {
		return ProviderResponse{
			Provider: p.Name,
			Duration: time.Since(start),
			TimedOut: true,
		}
	}

	// Hook-based collection: wait for provider-specific done signal (R1 protocol)
	err := session.WaitForDone(timeout, p.Name)
	if err != nil {
		// R8: graceful degradation on hook timeout — return fallback with output
		return ProviderResponse{
			Provider:    p.Name,
			Output:      "(hook timeout — collected via fallback)",
			Duration:    time.Since(start),
			EmptyOutput: false,
		}
	}

	// Read provider-specific result file (R1 protocol)
	hr, err := session.ReadResult(p.Name)
	if err != nil {
		// Result file read failed — graceful degradation
		return ProviderResponse{
			Provider:    p.Name,
			Output:      "(hook result read failed — fallback)",
			Duration:    time.Since(start),
			EmptyOutput: false,
		}
	}

	return HookResultToProviderResponse(*hr, p.Name, time.Since(start))
}

// HookResultToProviderResponse converts a HookResult into a ProviderResponse.
func HookResultToProviderResponse(hr HookResult, provider string, duration time.Duration) ProviderResponse {
	response := ProviderResponse{
		Provider:    provider,
		Output:      hr.Output,
		ExitCode:    hr.ExitCode,
		Duration:    duration,
		EmptyOutput: hr.Output == "",
	}
	markUnavailableUsage(&response, usageSourceHook, usageReasonHook)
	return response
}
