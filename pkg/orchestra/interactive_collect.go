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
			var baseline string
			if baselines != nil {
				baseline = baselines[pi.provider.Name]
			}
			timedOut := !waitForCompletion(ctx, cfg, pi, patterns, baseline, hookSession, round)
			// Fresh context for final read — original ctx may be cancelled after timeout.
			readCtx, readCancel := context.WithTimeout(context.Background(), 5*time.Second)
			screen, _ := cfg.Terminal.ReadScreen(readCtx, pi.paneID, terminal.ReadScreenOpts{
				Scrollback:      true,
				ScrollbackLines: scrollbackDepth(cfg.ScrollbackLines),
			})
			readCancel()
			output := cleanScreenOutput(screen)

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
				receipt := collectionReceipt(cfg.RunID, pi.provider.Name, "poll", "poll", status, errMsg, output, round, partial)
				receiptPath = cfg.ReliabilityStore.recordCollection(receipt)
			}

			mu.Lock()
			defer mu.Unlock()
			responses = append(responses, ProviderResponse{
				Provider: pi.provider.Name,
				Output:   output,
				Duration: time.Since(start),
				TimedOut: timedOut,
				Receipt:  receiptPath,
			})
		}(pi)
	}
	wg.Wait()
	return responses
}
