package orchestra

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/insajin/autopus-adk/pkg/detect"
)

// RunOrchestra는 설정에 따라 오케스트레이션을 실행한다.
// @AX:ANCHOR: 오케스트레이션의 주 진입점 — 모든 전략 분기가 여기서 시작된다.
// @AX:REASON: StrategyFunc 맵과 파이프라인/fastest 특수 경로 모두 이 함수를 통해 접근된다.
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
	var err error

	switch cfg.Strategy {
	case StrategyPipeline:
		responses, err = runPipeline(timeoutCtx, cfg)
	case StrategyFastest:
		responses, err = runFastest(timeoutCtx, cfg)
	default:
		// consensus, debate: 병렬 실행
		responses, err = runParallel(timeoutCtx, cfg)
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
		merged = FormatDebate(responses)
		judgeLabel := cfg.JudgeProvider
		if judgeLabel == "" {
			judgeLabel = "없음"
		}
		summary = fmt.Sprintf("토론 완료, 판정: %s", judgeLabel)
	case StrategyFastest:
		if len(responses) > 0 {
			merged = responses[0].Output
			summary = fmt.Sprintf("최속 응답: %s (%.1fs)", responses[0].Provider, responses[0].Duration.Seconds())
		}
	}

	return &OrchestraResult{
		Strategy:  cfg.Strategy,
		Responses: responses,
		Merged:    merged,
		Duration:  total,
		Summary:   summary,
	}, nil
}

// runParallel은 모든 프로바이더를 병렬로 실행한다.
func runParallel(ctx context.Context, cfg OrchestraConfig) ([]ProviderResponse, error) {
	responses := make([]ProviderResponse, len(cfg.Providers))
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error

	for i, p := range cfg.Providers {
		wg.Add(1)
		go func(idx int, provider ProviderConfig) {
			defer wg.Done()
			resp, err := runProvider(ctx, provider, cfg.Prompt)
			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
				return
			}
			responses[idx] = *resp
		}(i, p)
	}
	wg.Wait()

	if firstErr != nil {
		return nil, firstErr
	}
	return responses, nil
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
		stdinPipe.Close()
	} else {
		if err := cmd.Start(); err != nil {
			return nil, fmt.Errorf("%s 시작 실패: %w", provider.Name, err)
		}
	}

	waitErr := cmd.Wait()
	duration := time.Since(start)

	resp := &ProviderResponse{
		Provider: provider.Name,
		Output:   stdoutBuf.String(),
		Error:    stderrBuf.String(),
		Duration: duration,
		ExitCode: cmd.ExitCode(),
	}

	// 컨텍스트 취소(타임아웃 포함) 여부 확인
	if ctx.Err() != nil {
		resp.TimedOut = true
	}

	// 프로세스 오류는 TimedOut이 아닌 경우에만 반환
	if waitErr != nil && !resp.TimedOut && resp.ExitCode != 0 {
		return resp, fmt.Errorf("%s 실행 오류 (exit %d): %w", provider.Name, resp.ExitCode, waitErr)
	}

	return resp, nil
}
