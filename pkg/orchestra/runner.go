package orchestra

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/insajin/autopus-adk/pkg/detect"
)

// RunOrchestra는 설정에 따라 오케스트레이션을 실행한다.
// @AX:NOTE [AUTO] [downgraded from ANCHOR — fan_in < 3] 오케스트레이션의 주 진입점 — 2개 호출자 (spec_review, orchestra)
func RunOrchestra(ctx context.Context, cfg OrchestraConfig) (*OrchestraResult, error) {
	if len(cfg.Providers) == 0 {
		return nil, fmt.Errorf("providers 목록이 비어있습니다")
	}
	if !cfg.Strategy.IsValid() {
		return nil, fmt.Errorf("유효하지 않은 전략: %q", cfg.Strategy)
	}

	// 타임아웃 설정
	timeout := cfg.TimeoutSeconds
	if timeout <= 0 {
		timeout = 120
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	// 바이너리 설치 여부 사전 검증
	for _, p := range cfg.Providers {
		if !detect.IsInstalled(p.Binary) {
			return nil, fmt.Errorf("프로바이더 바이너리를 찾을 수 없습니다: %q", p.Binary)
		}
	}

	start := time.Now()
	var responses []ProviderResponse
	var failed []FailedProvider
	var err error

	switch cfg.Strategy {
	case StrategyPipeline:
		responses, err = runPipeline(timeoutCtx, cfg)
	case StrategyFastest:
		responses, err = runFastest(timeoutCtx, cfg)
	case StrategyDebate:
		responses, err = runDebate(timeoutCtx, cfg)
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

	// 전략별 병합 처리
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
		Merged:          merged,
		Duration:        total,
		Summary:         summary,
		FailedProviders: failed,
	}, nil
}

// providerResult holds the result of a single provider execution.
type providerResult struct {
	resp ProviderResponse
	err  error
	idx  int
}

// runParallel executes all providers in parallel with graceful degradation.
// It collects all successful responses and failed provider info.
// Returns (responses, failed, error). Error is non-nil only when ALL providers fail.
func runParallel(ctx context.Context, cfg OrchestraConfig) ([]ProviderResponse, []FailedProvider, error) {
	results := make([]providerResult, len(cfg.Providers))
	var wg sync.WaitGroup

	for i, p := range cfg.Providers {
		wg.Add(1)
		go func(idx int, provider ProviderConfig) {
			defer wg.Done()
			resp, err := runProvider(ctx, provider, cfg.Prompt)
			if err != nil {
				results[idx] = providerResult{err: err, idx: idx}
				return
			}
			results[idx] = providerResult{resp: *resp, idx: idx}
		}(i, p)
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
		} else if r.resp.EmptyOutput {
			// Treat empty stdout (exit 0 but no content) as a failed provider.
			// This catches silent failures such as wrong CLI flags or interactive mode.
			failed = append(failed, FailedProvider{
				Name:  r.resp.Provider,
				Error: "empty output: provider returned no content (check binary args or prompt_via_args setting)",
			})
		} else {
			responses = append(responses, r.resp)
		}
	}

	if len(responses) == 0 {
		// All failed: return first error
		return nil, failed, results[0].err
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

// runProvider는 단일 프로바이더를 실행하고 결과를 반환한다.
func runProvider(ctx context.Context, provider ProviderConfig, prompt string) (*ProviderResponse, error) {
	start := time.Now()

	// 실행 명령 구성
	args := append([]string{}, provider.Args...)

	// PromptViaArgs=true인 경우 프롬프트를 마지막 인자로 추가 (예: gemini -p "prompt")
	if provider.PromptViaArgs {
		args = append(args, prompt)
	}

	// @AX:WARN: exec.CommandContext는 컨텍스트 취소 시 프로세스를 강제 종료한다.
	// @AX:REASON: 타임아웃/취소 후 좀비 프로세스 방지를 위해 의도적 설계
	cmd := newCommand(ctx, provider.Binary, args...)

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.SetStdout(&stdoutBuf)
	cmd.SetStderr(&stderrBuf)

	// PromptViaArgs=false인 경우 stdin으로 프롬프트 전달 (claude, codex)
	if !provider.PromptViaArgs {
		stdinPipe, err := cmd.StdinPipe()
		if err != nil {
			return nil, fmt.Errorf("%s stdin 파이프 생성 실패: %w", provider.Name, err)
		}

		if err := cmd.Start(); err != nil {
			return nil, fmt.Errorf("%s 시작 실패: %w", provider.Name, err)
		}

		// 프롬프트 전송 후 stdin 닫기
		if _, err := io.WriteString(stdinPipe, prompt); err != nil {
			_ = cmd.Wait()
			return nil, fmt.Errorf("%s stdin 쓰기 실패: %w", provider.Name, err)
		}
		_ = stdinPipe.Close()
	} else {
		// Close stdin explicitly to prevent CLIs (e.g. claude -p) from waiting for input
		cmd.SetStdin(nil)
		if err := cmd.Start(); err != nil {
			return nil, fmt.Errorf("%s 시작 실패: %w", provider.Name, err)
		}
	}

	waitErr := cmd.Wait()
	duration := time.Since(start)

	output := stdoutBuf.String()
	resp := &ProviderResponse{
		Provider:    provider.Name,
		Output:      output,
		Error:       stderrBuf.String(),
		Duration:    duration,
		ExitCode:    cmd.ExitCode(),
		EmptyOutput: strings.TrimSpace(output) == "",
	}

	// Check for context cancellation (includes timeout).
	if ctx.Err() != nil {
		resp.TimedOut = true
	}

	// Return error only when process failed and was not timed out.
	if waitErr != nil && !resp.TimedOut && resp.ExitCode != 0 {
		return resp, fmt.Errorf("%s 실행 오류 (exit %d): %w", provider.Name, resp.ExitCode, waitErr)
	}

	return resp, nil
}
