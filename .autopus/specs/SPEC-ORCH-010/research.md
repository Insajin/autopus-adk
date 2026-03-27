# SPEC-ORCH-010 리서치

## 기존 코드 분석

### Permission hang 근본 원인

**파일**: `pkg/orchestra/interactive.go` L279-289 (`buildInteractiveLaunchCmd()`)

현재 코드:
```go
func buildInteractiveLaunchCmd(p ProviderConfig) string {
    cmd := p.Binary
    for _, arg := range paneArgs(p) {
        if arg == "--print" || arg == "-p" || arg == "--quiet" || arg == "-q" || arg == "run" {
            continue
        }
        cmd += " " + arg
    }
    return cmd
}
```

이 함수는 `--print`, `--quiet`, `run` 플래그만 필터링하고, `--dangerously-skip-permissions`를 추가하지 않는다.
Claude가 interactive 모드에서 tool을 사용하려 할 때 (예: file read) permission 프롬프트가 표시되고,
pane에는 사용자 입력이 없으므로 무한 대기 상태에 빠진다.

**수정 위치**: 함수 끝에서 `p.Binary == "claude"` 체크 후 플래그 추가.
PaneArgs에 이미 포함되어 있는지 확인하여 중복 방지.

### Topic drift 근본 원인

**파일**: `pkg/orchestra/interactive_debate.go` L167-169 (`executeRound()`)

Round 1에서 `cfg.Prompt`가 그대로 전달된다. 프로바이더는 프로젝트 디렉토리에서 실행되므로
`--dangerously-skip-permissions`가 있으면 파일을 자유롭게 읽을 수 있다.
기존 BS 파일(`.autopus/brainstorms/BS-*.md`)을 발견하고 해당 내용을 토론 주제로 오해한다.

**수정 접근**: 프롬프트 앞에 topic isolation instruction을 삽입하여 프로바이더가 파일 읽기를 하지 않도록 유도.
이는 hard block이 아닌 instruction-level isolation이지만, permission bypass와 함께 작동하면
프로바이더가 지시에 따라 파일을 읽지 않을 가능성이 높다.

**대안 검토**:
1. CWD를 임시 디렉토리로 변경 — 프로바이더가 프로젝트 컨텍스트를 잃어 품질 저하
2. `.autopus/` 디렉토리를 .gitignore에 추가 — 이미 추가되어 있지만 file read는 무관
3. Instruction-level isolation (채택) — 가장 비침투적이며 대부분의 LLM이 준수

### Truncation 근본 원인

**파일**: `pkg/orchestra/interactive.go` L256-272 (`waitForCompletion()`)

현재 로직:
```go
func waitForCompletion(...) bool {
    ticker := time.NewTicker(2 * time.Second)
    for {
        select {
        case <-ctx.Done():
            return false
        case <-ticker.C:
            screen, err := term.ReadScreen(ctx, pi.paneID, terminal.ReadScreenOpts{})
            if err == nil && isPromptVisible(screen, patterns) {
                return true  // 단일 match로 즉시 completion
            }
        }
    }
}
```

문제: AI가 응답 중에 `>` 문자를 출력하거나 줄 바꿈 후 빈 줄이 생기면
`^>\s*$` 패턴이 매칭되어 false positive completion이 발생한다.

**수정 접근**: 2-phase detection.
1. 첫 번째 match 감지 시 `candidateTime` 기록
2. 2초 후 재확인 — 여전히 prompt visible이면 completion 확정
3. 그렇지 않으면 counter 리셋하고 polling 계속

**파일**: `pkg/orchestra/interactive.go` L81 및 `interactive_debate.go` L191

Initial delay 값:
- `interactive.go` L81: `time.Sleep(15 * time.Second)` — single-round 모드
- `interactive_debate.go` L191: `time.Sleep(5 * time.Second)` — debate per-round

5초는 debate 환경에서 너무 짧다. LLM이 아직 프롬프트를 파싱 중일 때 completion polling이
시작되어 기존 prompt 잔상을 감지할 수 있다.

### Rebuttal 토큰 폭발 분석

**파일**: `pkg/orchestra/debate.go` L95-104 (`buildRebuttalPrompt()`)

현재 구현은 `r.Output` 전체를 포함한다. Round 3에서 3개 프로바이더의 각 2000토큰 출력이
누적되면 rebuttal 프롬프트가 6000+ 토큰이 되어:
1. 프로바이더 컨텍스트 윈도우 압박
2. 프롬프트 전송 시간 증가 (SendCommand 기반 paste)
3. 프로바이더의 집중력 분산

**수정**: `buildRebuttalPrompt()`에 round 파라미터를 추가하고, round >= 3일 때 500자 제한 적용.

### perRoundTimeout 현재 동작

**파일**: `pkg/orchestra/interactive_debate_helpers.go` L108-116

```go
func perRoundTimeout(totalSeconds, rounds int) time.Duration {
    if totalSeconds <= 0 { totalSeconds = 120 }
    if rounds <= 0 { rounds = 1 }
    return time.Duration(totalSeconds/rounds) * time.Second
}
```

`totalSeconds=60, rounds=5`이면 12초/라운드가 되어 LLM이 응답을 완료할 수 없다.
최소 45초는 보장해야 한다 (Claude의 평균 첫 토큰 생성까지 5-8초 + 응답 생성 30-40초).

## 설계 결정

### D1: Permission bypass를 provider-specific으로 적용
- Claude만 `--dangerously-skip-permissions` 필요
- opencode, gemini는 해당 플래그가 없거나 다른 메커니즘 사용
- 향후 프로바이더 어댑터 인터페이스에서 `PermissionFlags()` 메서드로 추상화 가능

### D2: Topic isolation은 instruction-level로 구현
- Hard isolation(CWD 변경)은 프로바이더가 프로젝트 컨텍스트를 잃음
- Instruction-level은 100% 보장이 아니나, 현대 LLM에서 높은 준수율
- BS-009에서 이 접근이 ICE 3.92로 두 번째로 높은 점수

### D3: 2-phase completion detection 채택
- 단일 match: false positive 위험 (현재 버그)
- 3회 연속: too conservative, 불필요한 대기 시간
- 2회 연속 (2초 간격): 균형점 — 최대 4초 추가 지연으로 false positive 제거

### D4: Rebuttal summarization threshold를 round >= 3으로 설정
- Round 2: 상대방의 첫 응답을 온전히 보는 것이 중요 (논점 파악)
- Round 3+: 이미 논점이 형성되어 있으므로 요약으로 충분
- 500자 제한: 핵심 주장 1-2개 전달 가능한 최소 길이
