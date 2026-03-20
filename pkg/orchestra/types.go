// Package orchestra는 다중 코딩 CLI 오케스트레이션 엔진을 제공한다.
package orchestra

import "time"

// Strategy는 오케스트레이션 전략이다.
type Strategy string

const (
	StrategyConsensus Strategy = "consensus"
	StrategyPipeline  Strategy = "pipeline"
	StrategyDebate    Strategy = "debate"
	StrategyFastest   Strategy = "fastest"
)

// ValidStrategies는 유효한 전략 목록이다.
var ValidStrategies = []Strategy{StrategyConsensus, StrategyPipeline, StrategyDebate, StrategyFastest}

// IsValid는 전략의 유효성을 검증한다.
func (s Strategy) IsValid() bool {
	for _, v := range ValidStrategies {
		if s == v {
			return true
		}
	}
	return false
}

// ProviderConfig는 프로바이더 실행 설정이다.
type ProviderConfig struct {
	Name          string   // 프로바이더 이름 (claude, codex, gemini)
	Binary        string   // 실행 바이너리 경로
	Args          []string // 추가 인자 (-p, -q 등)
	PromptViaArgs bool     // true: 프롬프트를 마지막 인자로 전달 (gemini), false: stdin으로 전달 (claude, codex)
}

// ProviderResponse는 프로바이더 실행 결과이다.
type ProviderResponse struct {
	Provider string        // 프로바이더 이름
	Output   string        // stdout 출력
	Error    string        // stderr 출력
	Duration time.Duration // 실행 시간
	ExitCode int           // 종료 코드
	TimedOut bool          // 타임아웃 여부
}

// OrchestraResult는 오케스트레이션 최종 결과이다.
type OrchestraResult struct {
	Strategy  Strategy           // 사용된 전략
	Responses []ProviderResponse // 개별 프로바이더 응답
	Merged    string             // 병합된 최종 결과
	Duration  time.Duration      // 전체 실행 시간
	Summary   string             // 전략별 요약 (합의율, 파이프라인 단계 등)
}

// OrchestraConfig는 오케스트레이션 실행 설정이다.
type OrchestraConfig struct {
	Providers      []ProviderConfig // 참여 프로바이더 목록
	Strategy       Strategy         // 실행 전략
	Prompt         string           // 전달할 프롬프트
	TimeoutSeconds int              // 타임아웃 (초)
	JudgeProvider  string           // debate 전략에서 최종 판정 프로바이더
}
