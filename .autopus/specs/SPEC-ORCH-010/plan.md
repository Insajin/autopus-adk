# SPEC-ORCH-010 구현 계획

## 태스크 목록

### Phase 1: P0 버그 수정 (Must Have)

- [ ] T1: Permission bypass — `buildInteractiveLaunchCmd()`에 `--dangerously-skip-permissions` 추가
  - 파일: `pkg/orchestra/interactive.go` (L279-289)
  - Claude binary 감지 로직 추가 (`p.Binary == "claude"` 또는 strings.Contains)
  - 기존 PaneArgs에 이미 포함되어 있으면 스킵
  - 테스트: `TestBuildInteractiveLaunchCmd_PermissionBypass`

- [ ] T2: Topic isolation — 프롬프트에 isolation instruction prefix 삽입
  - 파일: `pkg/orchestra/interactive_debate.go` (L159-198, `executeRound()`)
  - 파일: `pkg/orchestra/debate.go` (L95-104, `buildRebuttalPrompt()`)
  - Round 1 prompt과 rebuttal prompt 양쪽에 prefix 적용
  - Isolation prefix는 상수로 정의 (`topicIsolationInstruction`)
  - 테스트: `TestBuildRebuttalPrompt_TopicIsolation`

- [ ] T3: Completion detection 2-phase — `waitForCompletion()` 연속 match 로직 구현
  - 파일: `pkg/orchestra/interactive.go` (L256-272, `waitForCompletion()`)
  - 첫 번째 prompt match 후 2초 대기, 두 번째 match 확인 시 completion 확정
  - Initial delay 15s → configurable (기본 20s)
  - `executeRound()`의 initial delay 5s → 10s
  - 테스트: `TestWaitForCompletion_TwoPhase`

### Phase 2: P1 개선사항 (Should Have)

- [ ] T4: Rebuttal context summarization
  - 파일: `pkg/orchestra/debate.go` (`buildRebuttalPrompt()` 시그니처 변경)
  - Round number 파라미터 추가, round >= 3일 때 500자 truncation
  - 호출 측 수정: `interactive_debate.go` `executeRound()`, `debate.go` `runRebuttalRound()`
  - 테스트: `TestBuildRebuttalPrompt_Summarization`

- [ ] T5: Per-round minimum timeout
  - 파일: `pkg/orchestra/interactive_debate_helpers.go` (L108-116, `perRoundTimeout()`)
  - `max(totalSeconds/rounds, 45)` 적용
  - 테스트: `TestPerRoundTimeout_MinimumFloor`

### Phase 3: P2 추가 기능 (Could Have)

- [ ] T6: Consensus threshold 파라미터화
  - 파일: `pkg/orchestra/interactive_debate_helpers.go` (`consensusReached()`)
  - 파일: `pkg/orchestra/config.go` 또는 types 파일에 `ConsensusThreshold float64` 필드 추가
  - 기존 하드코딩 0.66을 config에서 읽도록 변경
  - 테스트: `TestConsensusReached_ConfigurableThreshold`

- [ ] T7: --rounds 동작 검증 (SPEC-ORCH-009에서 이미 구현)
  - brainstorm 커맨드에서 `--rounds` 플래그 전달이 debate loop까지 도달하는지 e2e 확인
  - 기존 코드 변경 불필요 — 테스트만 추가

## 구현 전략

### 접근 방법
- T1, T2, T3는 독립적이므로 병렬 실행 가능
- T4는 `buildRebuttalPrompt()` 시그니처를 변경하므로 T2 이후에 진행
- T5, T6은 독립적이며 Phase 1 이후 순서 무관

### 기존 코드 활용
- `buildInteractiveLaunchCmd()` (interactive.go:279) — 기존 arg 필터링 로직에 permission 플래그 추가
- `buildRebuttalPrompt()` (debate.go:95) — 기존 StringBuilder 패턴 유지, prefix 삽입
- `waitForCompletion()` (interactive.go:256) — 기존 polling loop 구조 위에 consecutive match counter 추가
- `perRoundTimeout()` (interactive_debate_helpers.go:108) — 기존 계산식에 max() 추가

### 변경 범위
- 수정 파일 5개 (모두 `pkg/orchestra/` 내)
- 신규 테스트 코드 ~70줄
- 기존 파일 수정량 ~75줄
- 총 변경량 ~145줄 — 단일 에이전트로 처리 가능
