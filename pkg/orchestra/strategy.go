package orchestra

import (
	"context"
	"fmt"
)

// StrategyFunc는 프로바이더 응답을 전략에 따라 처리하는 함수 타입이다.
// 반환값: (병합된 결과, 요약, 에러)
type StrategyFunc func(ctx context.Context, responses []ProviderResponse, cfg OrchestraConfig) (string, string, error)

// strategyHandlers는 전략별 후처리 핸들러 맵이다.
// @AX:NOTE: [AUTO] global strategy registry — add new strategy here and in runner.go switch when extending strategies
var strategyHandlers = map[Strategy]StrategyFunc{
	StrategyConsensus: handleConsensus,
	StrategyPipeline:  handlePipeline,
	StrategyDebate:    handleDebate,
	StrategyFastest:   handleFastest,
	StrategyRelay:     handleRelay,
}

// GetStrategyFunc는 전략에 맞는 StrategyFunc를 반환한다.
func GetStrategyFunc(s Strategy) (StrategyFunc, error) {
	fn, ok := strategyHandlers[s]
	if !ok {
		return nil, fmt.Errorf("알 수 없는 전략: %q", s)
	}
	return fn, nil
}

// handleConsensus는 합의 전략 후처리이다.
func handleConsensus(_ context.Context, responses []ProviderResponse, _ OrchestraConfig) (string, string, error) {
	merged, summary := MergeConsensus(responses, 0.66)
	return merged, summary, nil
}

// handlePipeline은 파이프라인 전략 후처리이다.
func handlePipeline(_ context.Context, responses []ProviderResponse, _ OrchestraConfig) (string, string, error) {
	merged := FormatPipeline(responses)
	summary := fmt.Sprintf("파이프라인: %d단계 완료", len(responses))
	return merged, summary, nil
}

// handleDebate는 토론 전략 후처리이다.
func handleDebate(_ context.Context, responses []ProviderResponse, cfg OrchestraConfig) (string, string, error) {
	merged, summary := buildDebateMerged(responses, cfg)
	return merged, summary, nil
}

// handleRelay is the relay strategy post-processor.
func handleRelay(_ context.Context, responses []ProviderResponse, _ OrchestraConfig) (string, string, error) {
	merged := FormatRelay(responses)
	summary := fmt.Sprintf("릴레이: %d단계 완료", len(responses))
	return merged, summary, nil
}

// handleFastest는 최속 전략 후처리이다.
func handleFastest(_ context.Context, responses []ProviderResponse, _ OrchestraConfig) (string, string, error) {
	if len(responses) == 0 {
		return "", "", fmt.Errorf("응답이 없습니다")
	}
	r := responses[0]
	summary := fmt.Sprintf("최속 응답: %s (%.1fs)", r.Provider, r.Duration.Seconds())
	return r.Output, summary, nil
}
