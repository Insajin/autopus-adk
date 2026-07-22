package orchestra

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

const (
	hookResponseSettleBudget       = 200 * time.Millisecond
	hookResponseSettlePollInterval = 10 * time.Millisecond
	skippedHookCollectionError     = "provider was skipped before hook completion collection"
)

// collectRoundHookResults isolates each hook provider behind its execution
// timeout while preserving an unavailable result for panes skipped before send.
func collectRoundHookResults(ctx context.Context, cfg OrchestraConfig, session *HookSession, round int, paneGroups ...[]paneInfo) []ProviderResponse {
	paneByProvider := hookPanesByProvider(paneGroups)
	reliabilityStoreResolved := cfg.ReliabilityStore != nil || cfg.RunID == ""
	var mu sync.Mutex
	var responses []ProviderResponse
	var wg sync.WaitGroup
	for _, provider := range cfg.Providers {
		pi, found := paneByProvider[provider.Name]
		if found && pi.skipWait {
			mu.Lock()
			responses = append(responses, skippedHookResponse(provider))
			mu.Unlock()
			continue
		}
		if !session.HasHook(provider.Name) {
			continue
		}
		if !reliabilityStoreResolved {
			if store, err := newReliabilityStore(cfg.RunID); err == nil {
				cfg.ReliabilityStore = store
			}
			reliabilityStoreResolved = true
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			response := hookTimeoutResponse(cfg, provider, round, time.Now(), ctxErr)
			mu.Lock()
			responses = append(responses, response)
			mu.Unlock()
			continue
		}
		if !found {
			pi.provider = provider
		}
		wg.Add(1)
		go func(provider ProviderConfig, pi paneInfo) {
			defer wg.Done()
			response := collectRoundHookProvider(ctx, cfg, session, provider, pi, round)
			mu.Lock()
			responses = append(responses, response)
			mu.Unlock()
		}(provider, pi)
	}
	wg.Wait()
	return responses
}

func hookPanesByProvider(paneGroups [][]paneInfo) map[string]paneInfo {
	panes := make(map[string]paneInfo)
	if len(paneGroups) == 0 {
		return panes
	}
	for _, pi := range paneGroups[0] {
		panes[pi.provider.Name] = pi
	}
	return panes
}

func skippedHookResponse(provider ProviderConfig) ProviderResponse {
	return unavailableResponse(ProviderResponse{
		Provider:        provider.Name,
		ModelFamily:     provider.ModelFamily,
		ExecutedBackend: paneBackendName,
		Role:            "participant",
		TimedOut:        true,
		EmptyOutput:     true,
		Error:           skippedHookCollectionError,
	}, usageSourcePane, usageReasonPane)
}

func collectRoundHookProvider(ctx context.Context, cfg OrchestraConfig, session *HookSession, provider ProviderConfig, pi paneInfo, round int) ProviderResponse {
	start := time.Now()
	providerCtx, cancel := context.WithTimeout(ctx, providerExecutionTimeout(provider, cfg.TimeoutSeconds))
	defer cancel()

	completed, detectorErr := (&FileIPCDetector{session: session}).WaitForCompletion(providerCtx, pi, nil, "", round)
	if !completed {
		return hookTimeoutResponse(cfg, provider, round, start, completionWaitError(providerCtx, detectorErr))
	}

	output, responseFileOK := waitForMarkedHookResponse(providerCtx, pi.responseFile)
	if responseFileOK {
		provenance, provenanceErr := resolveHookCompletionHandoff(
			providerCtx, cfg, session, provider, pi, round,
		)
		if provenanceErr != nil {
			log.Printf("[Round %d] %s completion handoff failed closed; retaining hook: %v", round, provider.Name, provenanceErr)
			return hookTimeoutResponse(
				cfg, provider, round, start, fmt.Errorf("completion handoff: %w", provenanceErr),
			)
		} else if provenance == hookCompletionResponseFileOnly {
			log.Printf("[Round %d] %s completion hook inactive after stable response-only handoff", round, provider.Name)
		}
	}
	status, errMsg := "pass", ""
	partial := false
	if !responseFileOK {
		result, readErr := session.ReadResultRound(provider.Name, round)
		if readErr == nil && result != nil {
			output = result.Output
		} else if readErr != nil {
			status = "read_failed"
			errMsg = readErr.Error()
			partial = true
		}
	}

	receiptPath := ""
	if cfg.ReliabilityStore != nil {
		mode := "hook"
		if responseFileOK {
			mode = "response_file"
		}
		receipt := collectionReceipt(cfg.RunID, provider.Name, mode, mode, status, errMsg, output, round, partial)
		receiptPath = cfg.ReliabilityStore.recordCollection(receipt)
	}
	return unavailableResponse(ProviderResponse{
		Provider:        provider.Name,
		ModelFamily:     provider.ModelFamily,
		ExecutedBackend: paneBackendName,
		Output:          output,
		Duration:        time.Since(start),
		Receipt:         receiptPath,
	}, usageSourcePane, usageReasonPane)
}

func completionWaitError(ctx context.Context, detectorErr error) error {
	if detectorErr != nil {
		return detectorErr
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return context.DeadlineExceeded
}

func hookTimeoutResponse(cfg OrchestraConfig, provider ProviderConfig, round int, start time.Time, waitErr error) ProviderResponse {
	errMsg := "hook completion timeout before a done signal was collected"
	if waitErr != nil {
		errMsg = fmt.Sprintf("%s: %v", errMsg, waitErr)
	}
	receiptPath := ""
	if cfg.ReliabilityStore != nil {
		receipt := collectionReceipt(cfg.RunID, provider.Name, "hook", "hook", "timeout", errMsg, "", round, true)
		receiptPath = cfg.ReliabilityStore.recordCollection(receipt)
		event := timeoutEvent(cfg.RunID, provider.Name, round, "retry with subprocess fallback")
		_ = cfg.ReliabilityStore.recordEvent(event)
		_ = cfg.ReliabilityStore.writeFailureBundle("hook collection timed out", "retry with subprocess fallback", true)
	}
	return unavailableResponse(ProviderResponse{
		Provider:        provider.Name,
		ModelFamily:     provider.ModelFamily,
		ExecutedBackend: paneBackendName,
		Role:            "participant",
		Duration:        time.Since(start),
		TimedOut:        true,
		EmptyOutput:     true,
		Error:           errMsg,
		Receipt:         receiptPath,
	}, usageSourcePane, usageReasonPane)
}

// waitForMarkedHookResponse gives a response file that follows done a small,
// explicit settle window. The provider/parent context always remains the outer
// bound, and a final read at either boundary closes the write-vs-timeout race.
func waitForMarkedHookResponse(ctx context.Context, path string) (string, bool) {
	if strings.TrimSpace(path) == "" {
		return "", false
	}
	if output, ok := readResponseFile(path); ok {
		return output, true
	}
	settle := time.NewTimer(hookResponseSettleBudget)
	defer settle.Stop()
	ticker := time.NewTicker(hookResponseSettlePollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return readResponseFile(path)
		case <-settle.C:
			return readResponseFile(path)
		case <-ticker.C:
			if output, ok := readResponseFile(path); ok {
				return output, true
			}
		}
	}
}
