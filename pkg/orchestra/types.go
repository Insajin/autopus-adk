// Package orchestra provides the multi-coding CLI orchestration engine.
package orchestra

import (
	"regexp"
	"slices"
	"time"

	"github.com/insajin/autopus-adk/pkg/terminal"
)

// Strategy는 오케스트레이션 전략이다.
type Strategy string

const (
	StrategyConsensus Strategy = "consensus"
	StrategyPipeline  Strategy = "pipeline"
	StrategyDebate    Strategy = "debate"
	StrategyFastest   Strategy = "fastest"
	StrategyRelay     Strategy = "relay"
)

// ValidStrategies는 유효한 전략 목록이다.
var ValidStrategies = []Strategy{StrategyConsensus, StrategyPipeline, StrategyDebate, StrategyFastest, StrategyRelay}

// IsValid는 전략의 유효성을 검증한다.
func (s Strategy) IsValid() bool {
	return slices.Contains(ValidStrategies, s)
}

// ProviderConfig는 프로바이더 실행 설정이다.
type ProviderConfig struct {
	Name          string   // provider name (claude, codex, gemini)
	Binary        string   // executable binary path
	Args          []string // args for non-interactive mode
	PaneArgs      []string // args for pane mode (overrides Args when set)
	PromptViaArgs bool     // true: pass prompt as last arg (gemini), false: pass via stdin (claude, codex)
}

// ProviderResponse는 프로바이더 실행 결과이다.
type ProviderResponse struct {
	Provider    string        // 프로바이더 이름
	Output      string        // stdout 출력
	Error       string        // stderr 출력
	Duration    time.Duration // 실행 시간
	ExitCode    int           // 종료 코드
	TimedOut    bool          // 타임아웃 여부
	EmptyOutput bool          // true when stdout is empty (exit 0 but no content)
}

// FailedProvider records a provider that failed during execution.
type FailedProvider struct {
	Name  string // Provider name
	Error string // Error message
}

// OrchestraResult는 오케스트레이션 최종 결과이다.
type OrchestraResult struct {
	Strategy        Strategy           // 사용된 전략
	Responses       []ProviderResponse // 개별 프로바이더 응답
	Merged          string             // 병합된 최종 결과
	Duration        time.Duration      // 전체 실행 시간
	Summary         string               // 전략별 요약 (합의율, 파이프라인 단계 등)
	FailedProviders []FailedProvider     // Providers that failed during execution
	RoundHistory    [][]ProviderResponse // Per-round provider responses for debate strategy
}

// OrchestraConfig는 오케스트레이션 실행 설정이다.
type OrchestraConfig struct {
	Providers      []ProviderConfig  // 참여 프로바이더 목록
	Strategy       Strategy          // 실행 전략
	Prompt         string            // 전달할 프롬프트
	TimeoutSeconds int               // 타임아웃 (초)
	JudgeProvider  string            // debate 전략에서 최종 판정 프로바이더
	DebateRounds   int               // Number of debate rounds (1=no rebuttal, 2=with rebuttal). 0 defaults to 1.
	Terminal       terminal.Terminal // Optional terminal for pane-based execution. Nil means non-interactive mode.
	NoDetach           bool             // @AX:NOTE [AUTO] REQ-1 — when true, disable auto-detach even on pane terminals; maps to CLI --no-detach flag
	KeepRelayOutput    bool             // when true, preserve temp relay output files after execution
	Interactive        bool             // when true, use interactive pane mode instead of sentinel-based
	HookMode           bool             // when true, use hook file signals instead of ReadScreen for result collection
	SessionID          string           // unique session ID for hook file signal directory
	ConsensusThreshold float64          // consensus threshold (0 uses default 0.66)
	InitialDelay       time.Duration    // delay before completion polling starts (0 uses default 20s)
}

// CompletionPattern defines a provider-specific prompt detection pattern.
type CompletionPattern struct {
	Provider string         // provider name (claude, codex, gemini)
	Pattern  *regexp.Regexp // compiled regex for prompt detection
}

// DefaultCompletionPatterns returns the built-in prompt patterns for known providers.
// @AX:NOTE [AUTO] hardcoded provider prompt patterns — update when adding new providers
func DefaultCompletionPatterns() []CompletionPattern {
	return []CompletionPattern{
		{Provider: "claude", Pattern: regexp.MustCompile(`(?m)^>\s*$`)},
		{Provider: "codex", Pattern: regexp.MustCompile(`(?m)^codex>\s*$`)},
		{Provider: "gemini", Pattern: regexp.MustCompile(`(?m)^>\s*$`)},
		{Provider: "opencode", Pattern: regexp.MustCompile(`(?m)^>\s*$`)},
	}
}

// IdleThreshold is the default duration for idle detection (no new output).
// Set to 30s to allow for AI model thinking time before triggering completion.
const IdleThreshold = 30 * time.Second
