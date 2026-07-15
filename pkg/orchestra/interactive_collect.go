package orchestra

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/insajin/autopus-adk/pkg/terminal"
)

const emptyScreenRetryDelay = 250 * time.Millisecond

// waitAndCollectResults waits for completion and collects cleaned results.
// Round is forwarded to waitForCompletion; pass 0 for non-debate strategies.
// @AX:WARN [AUTO] concurrent goroutine writes to shared responses slice — guarded by mu sync.Mutex
func waitAndCollectResults(ctx context.Context, cfg OrchestraConfig, panes []paneInfo, patterns []CompletionPattern, start time.Time, baselines map[string]string, hookSession *HookSession, round int) []ProviderResponse {
	var (
		responses []ProviderResponse
		mu        sync.Mutex
		wg        sync.WaitGroup
	)

	for _, pi := range panes {
		if pi.skipWait {
			responses = append(responses, unavailableResponse(ProviderResponse{
				Provider:    pi.provider.Name,
				Duration:    time.Since(start),
				TimedOut:    true,
				EmptyOutput: true,
				Error:       "provider was skipped before completion collection",
			}, usageSourcePane, usageReasonPane))
			continue
		}
		wg.Add(1)
		go func(pi paneInfo) {
			defer wg.Done()
			var baseline string
			if baselines != nil {
				baseline = baselines[pi.provider.Name]
			}
			timedOut := !waitForCompletion(ctx, cfg, pi, patterns, baseline, hookSession, round)
			output, responseFileOK := readResponseFile(pi.responseFile)
			if responseFileOK {
				timedOut = false
			}

			if !responseFileOK {
				// Fresh context for final read — original ctx may be cancelled after timeout.
				readCtx, readCancel := context.WithTimeout(context.Background(), 5*time.Second)
				screen, _ := cfg.Terminal.ReadScreen(readCtx, pi.paneID, terminal.ReadScreenOpts{
					Scrollback:      true,
					ScrollbackLines: scrollbackDepth(cfg.ScrollbackLines),
				})
				readCancel()
				output = cleanScreenOutput(screen)

				// Retry once if output is empty — pane may still be rendering
				// or completion detection may have fired slightly early.
				if output == "" {
					time.Sleep(emptyScreenRetryDelay)
					retryCtx, retryCancel := context.WithTimeout(context.Background(), 5*time.Second)
					screen2, _ := cfg.Terminal.ReadScreen(retryCtx, pi.paneID, terminal.ReadScreenOpts{
						Scrollback:      true,
						ScrollbackLines: scrollbackDepth(cfg.ScrollbackLines),
					})
					retryCancel()
					if retried := cleanScreenOutput(screen2); retried != "" {
						output = retried
						log.Printf("[ReadScreen] retry succeeded for %s (pane %s, timedOut=%v)", pi.provider.Name, pi.paneID, timedOut)
					}
				}
			}

			status := "pass"
			errMsg := ""
			partial := false
			if timedOut {
				status = "timeout"
				errMsg = "completion detector timed out before terminal output stabilized"
			} else if output == "" {
				status = "partial"
				errMsg = "screen collection returned empty output"
				partial = true
			}
			receiptPath := ""
			if cfg.ReliabilityStore != nil {
				collectionMode := "poll"
				if responseFileOK {
					collectionMode = "response_file"
				}
				receipt := collectionReceipt(cfg.RunID, pi.provider.Name, collectionMode, collectionMode, status, errMsg, output, round, partial)
				receiptPath = cfg.ReliabilityStore.recordCollection(receipt)
			}

			mu.Lock()
			defer mu.Unlock()
			responses = append(responses, unavailableResponse(ProviderResponse{
				Provider:    pi.provider.Name,
				Output:      output,
				Duration:    time.Since(start),
				TimedOut:    timedOut,
				EmptyOutput: output == "",
				Error:       errMsg,
				Receipt:     receiptPath,
			}, usageSourcePane, usageReasonPane))
		}(pi)
	}
	wg.Wait()
	return responses
}

func responseFailuresFromInteractivePanes(panes []paneInfo, responses []ProviderResponse, fallbackSeconds int, existing []FailedProvider) []FailedProvider {
	providers := make(map[string]ProviderConfig, len(panes))
	for _, pi := range panes {
		providers[pi.provider.Name] = pi.provider
	}
	seen := make(map[string]struct{}, len(existing))
	for _, failure := range existing {
		seen[failure.Name] = struct{}{}
	}
	failures := make([]FailedProvider, 0)
	for _, resp := range responses {
		if !resp.TimedOut && !resp.EmptyOutput {
			continue
		}
		if _, ok := seen[resp.Provider]; ok {
			continue
		}
		provider := providers[resp.Provider]
		if provider.Name == "" {
			provider = ProviderConfig{Name: resp.Provider}
		}
		respCopy := resp
		failures = append(failures, buildFailedProvider(provider, &respCopy, nil, fallbackSeconds))
		seen[resp.Provider] = struct{}{}
	}
	return failures
}
