package orchestra

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// executeParallel runs all providers concurrently via the backend.
func executeParallel(
	ctx context.Context,
	backend ExecutionBackend,
	providers []ProviderConfig,
	prompt, schemaPath, role string,
	round int,
	timeoutSeconds int,
) ([]ProviderResult, []FailedProvider, error) {
	type result struct {
		pr   ProviderResult
		resp *ProviderResponse
		err  error
		idx  int
	}

	results := make([]result, len(providers))
	providerNames := make([]string, len(providers))
	for i, p := range providers {
		providerNames[i] = p.Name
	}
	progress := NewProgressTracker(providerNames)
	stopProgress := progress.StartHeartbeat(ctx, progressHeartbeatInterval)
	defer stopProgress()

	var wg sync.WaitGroup
	for i, p := range providers {
		wg.Add(1)
		go func(idx int, prov ProviderConfig) {
			defer wg.Done()
			req := ProviderRequest{
				Provider:   prov.Name,
				Prompt:     prompt,
				SchemaPath: schemaPath,
				Role:       role,
				Round:      round,
				Timeout:    providerExecutionTimeout(prov, timeoutSeconds),
				Config:     prov,
			}
			progress.MarkRunning(prov.Name)
			resp, err := backend.Execute(ctx, req)
			if err != nil {
				progress.MarkFailed(prov.Name)
				results[idx] = result{resp: resp, err: err, idx: idx}
				return
			}
			if resp == nil {
				progress.MarkFailed(prov.Name)
				results[idx] = result{err: fmt.Errorf("%s returned no response", prov.Name), idx: idx}
				return
			}
			if resp.TimedOut {
				progress.MarkFailed(prov.Name)
				results[idx] = result{resp: resp, err: fmt.Errorf("%s timed out", prov.Name), idx: idx}
				return
			}
			if resp.EmptyOutput {
				progress.MarkFailed(prov.Name)
				results[idx] = result{resp: resp, err: fmt.Errorf("%s returned empty output", prov.Name), idx: idx}
				return
			}
			progress.MarkDone(prov.Name)
			results[idx] = result{pr: ProviderResult{Provider: prov.Name, Output: resp.Output}, idx: idx}
		}(i, p)
	}
	wg.Wait()

	var successes []ProviderResult
	var failed []FailedProvider
	for _, r := range results {
		if r.err != nil {
			failed = append(failed, buildFailedProvider(providers[r.idx], r.resp, r.err, timeoutSeconds))
		} else {
			successes = append(successes, r.pr)
		}
	}
	if len(successes) == 0 {
		return nil, failed, fmt.Errorf("all %d providers failed", len(providers))
	}
	return successes, failed, nil
}

func providersSupportCLISchema(providers []ProviderConfig) bool {
	if len(providers) == 0 {
		return false
	}
	for _, provider := range providers {
		if strings.TrimSpace(provider.SchemaFlag) == "" {
			return false
		}
	}
	return true
}
