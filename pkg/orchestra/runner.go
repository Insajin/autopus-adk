package orchestra

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/insajin/autopus-adk/pkg/detect"
)

// RunOrchestra executes orchestration according to the given config.
// @AX:ANCHOR: [AUTO] public API — 4 callers; do not change signature
func RunOrchestra(ctx context.Context, cfg OrchestraConfig) (*OrchestraResult, error) {
	if len(cfg.Providers) == 0 {
		return nil, fmt.Errorf("providers 목록이 비어있습니다")
	}
	if !cfg.Strategy.IsValid() {
		return nil, fmt.Errorf("유효하지 않은 전략: %q", cfg.Strategy)
	}

	// Delegate to pane runner for non-plain terminals
	if !cfg.SubprocessMode && cfg.Terminal != nil && cfg.Terminal.Name() != "plain" {
		return RunPaneOrchestra(ctx, cfg)
	}

	timeout := cfg.TimeoutSeconds
	if timeout <= 0 {
		timeout = 120
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	for _, p := range cfg.Providers {
		if !detect.IsInstalled(p.Binary) {
			return nil, fmt.Errorf("프로바이더 바이너리를 찾을 수 없습니다: %q", p.Binary)
		}
	}

	start := time.Now()
	var responses []ProviderResponse
	var roundHistory [][]ProviderResponse
	var failed []FailedProvider
	var err error

	switch cfg.Strategy {
	case StrategyPipeline:
		responses, err = runPipeline(timeoutCtx, cfg)
	case StrategyFastest:
		responses, err = runFastest(timeoutCtx, cfg)
	case StrategyDebate:
		responses, roundHistory, failed, err = runDebate(timeoutCtx, cfg)
	case StrategyRelay:
		responses, err = runRelay(timeoutCtx, &cfg)
	default:
		// consensus: prepend structured prompt prefix, then run parallel with graceful degradation
		consensusCfg := cfg
		consensusCfg.Prompt = buildStructuredPromptPrefix() + cfg.Prompt
		responses, failed, err = runParallel(timeoutCtx, consensusCfg)
	}
	if err != nil {
		return nil, err
	}

	total := time.Since(start)

	var merged, summary string
	switch cfg.Strategy {
	case StrategyConsensus:
		merged, summary = MergeConsensus(responses, 0.66)
	case StrategyPipeline:
		merged = FormatPipeline(responses)
		summary = fmt.Sprintf("파이프라인: %d단계 완료", len(responses))
	case StrategyDebate:
		merged, summary = buildDebateMerged(responses, cfg)
	case StrategyFastest:
		if len(responses) > 0 {
			merged = responses[0].Output
			summary = fmt.Sprintf("최속 응답: %s (%.1fs)", responses[0].Provider, responses[0].Duration.Seconds())
		}
	case StrategyRelay:
		merged = FormatRelay(responses)
		summary = fmt.Sprintf("릴레이: %d단계 완료", len(responses))
	}

	// Append failed provider info to summary if any
	if len(failed) > 0 {
		var names []string
		for _, f := range failed {
			names = append(names, f.Name)
		}
		summary = fmt.Sprintf("%s (실패: %s)", summary, strings.Join(names, ", "))
	}

	return &OrchestraResult{
		Strategy:        cfg.Strategy,
		Responses:       responses,
		RoundHistory:    roundHistory,
		Merged:          merged,
		Duration:        total,
		Summary:         summary,
		FailedProviders: failed,
	}, nil
}

// runParallel executes all providers in parallel with per-goroutine context (R1)
// and per-provider timeout (R2). Error is non-nil only when ALL providers fail.
func runParallel(ctx context.Context, cfg OrchestraConfig) ([]ProviderResponse, []FailedProvider, error) {
	results := make([]providerResult, len(cfg.Providers))
	var wg sync.WaitGroup

	// R2: per-provider timeout from config (default 120s)
	perTimeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	if perTimeout <= 0 {
		perTimeout = 120 * time.Second
	}

	for i, p := range cfg.Providers {
		wg.Add(1)
		// R1: derive per-goroutine context for independent cancellation
		childCtx, childCancel := context.WithTimeout(ctx, perTimeout)
		go func(idx int, provider ProviderConfig, cancel context.CancelFunc) {
			defer wg.Done()
			defer cancel()
			resp, err := runProvider(childCtx, provider, cfg.Prompt)
			if err != nil {
				results[idx] = providerResult{err: err, idx: idx}
				return
			}
			results[idx] = providerResult{resp: *resp, idx: idx}
		}(i, p, childCancel)
	}
	wg.Wait()

	var responses []ProviderResponse
	var failed []FailedProvider

	for _, r := range results {
		if r.err != nil {
			failed = append(failed, FailedProvider{
				Name:  cfg.Providers[r.idx].Name,
				Error: r.err.Error(),
			})
		} else if r.resp.TimedOut {
			// R2: provider exceeded per-provider timeout — record as failed
			failed = append(failed, FailedProvider{
				Name:  r.resp.Provider,
				Error: fmt.Sprintf("timeout: provider exceeded %v deadline", perTimeout),
			})
		} else if r.resp.EmptyOutput {
			// Treat empty stdout (exit 0 but no content) as a failed provider.
			failed = append(failed, FailedProvider{
				Name:  r.resp.Provider,
				Error: "empty output: provider returned no content (check binary args or prompt_via_args setting)",
			})
		} else {
			responses = append(responses, r.resp)
		}
	}

	if len(responses) == 0 {
		return nil, failed, buildAllProvidersFailedError(failed, results[0].err)
	}
	return responses, failed, nil
}

// runPipeline은 프로바이더를 순차적으로 실행하며 이전 출력을 다음 입력에 추가한다.
func runPipeline(ctx context.Context, cfg OrchestraConfig) ([]ProviderResponse, error) {
	responses := make([]ProviderResponse, 0, len(cfg.Providers))
	prompt := cfg.Prompt

	for _, p := range cfg.Providers {
		resp, err := runProvider(ctx, p, prompt)
		if err != nil {
			return responses, err
		}
		responses = append(responses, *resp)
		// 다음 단계 프롬프트에 이전 출력 추가
		if resp.Output != "" {
			prompt = fmt.Sprintf("%s\n\n이전 단계 결과:\n%s", cfg.Prompt, resp.Output)
		}
	}
	return responses, nil
}

// runFastest는 모든 프로바이더를 병렬로 실행하고 첫 번째 성공 응답을 반환한다.
func runFastest(ctx context.Context, cfg OrchestraConfig) ([]ProviderResponse, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	resultCh := make(chan ProviderResponse, len(cfg.Providers))
	var wg sync.WaitGroup

	for _, p := range cfg.Providers {
		wg.Add(1)
		go func(provider ProviderConfig) {
			defer wg.Done()
			resp, err := runProvider(ctx, provider, cfg.Prompt)
			if err != nil || (resp != nil && resp.TimedOut) {
				return
			}
			if resp == nil {
				return
			}
			select {
			case resultCh <- *resp:
				cancel() // 첫 번째 응답이 도착하면 나머지 취소
			default:
			}
		}(p)
	}

	// 고루틴 완료 후 채널 닫기
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	resp, ok := <-resultCh
	if !ok {
		return nil, fmt.Errorf("모든 프로바이더가 응답하지 않았습니다")
	}
	return []ProviderResponse{resp}, nil
}
