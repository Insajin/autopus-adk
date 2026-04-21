# SPEC-ORCH-020 Plan: Orchestra Reliability Kit

## Implementation Strategy

기존 orchestra 기능을 교체하기보다 reliability contract를 한 층 덧씌우는 방식으로 진행한다.

1. **Preflight layer**
   - provider launch 전 `cwd`, binary, transport mode, capability, hook readiness를 검사
   - 결과를 structured receipt로 저장
2. **Receipt layer**
   - prompt transport receipt와 collection receipt를 추가
   - hook timeout, transport mismatch, degraded execution을 구조화 이벤트로 표면화
3. **Core separation**
   - round state transition, timeout budget, retry/degrade policy를 transport-specific 코드에서 분리
4. **Failure bundle**
   - review/debug에 필요한 최소 failure bundle과 correlation ID를 표준화

초기 구현은 pane-mode와 hook-mode 안정화에 집중하고, subprocess default 전환은 후속 SPEC로 남긴다.

## Sync Outcome

2026-04-21 sync 기준 구현 결과:

- reliability schema/types/store/runtime artifact contract를 추가했다.
- pane/hook/deferred job 경로에 `run_id`, degraded 상태, receipt/bundle summary를 연결했다.
- prompt/collection receipt와 hook timeout structured event를 실제 실행 경로에 붙였다.
- full package runtime을 줄여 `go test -timeout 120s ./pkg/orchestra -count=1`가 안정적으로 통과하도록 테스트 fixture와 polling/backoff를 정리했다.

이번 sync에서 남긴 follow-up:

- effective `cwd` mismatch를 round 시작 전에 실제 shell 상태로 검증하고 차단하는 경로
- prompt transport mutation을 completion wait 전에 mismatch로 fail시키는 경로
- reliability metrics / replay ledger

## File Impact Analysis

| 파일 | 작업 (생성/수정/삭제) | 설명 |
|------|---------------------|------|
| `pkg/orchestra/runner.go` | 수정 | preflight/receipt orchestration 진입점 |
| `pkg/orchestra/types.go` | 수정 | reliability config, receipt metadata, correlation fields |
| `pkg/orchestra/interactive_launch.go` | 수정 | pane launch 전 cwd/transport preflight |
| `pkg/orchestra/interactive_debate_round.go` | 수정 | timeout/receipt/degrade handling |
| `pkg/orchestra/hook_watcher.go` | 수정 | collection receipt와 timeout event 보강 |
| `pkg/orchestra/job.go` | 수정 | run/job snapshot에 correlation, degraded state 추가 |
| `pkg/orchestra/completion_*` | 수정 | timeout provenance와 fallback path 명시 |
| `[NEW] pkg/orchestra/reliability_preflight.go` | 생성 | provider preflight core |
| `[NEW] pkg/orchestra/reliability_receipt.go` | 생성 | launch/prompt/collection receipt 타입과 기록 |
| `[NEW] pkg/orchestra/reliability_bundle.go` | 생성 | compact failure bundle 생성 |
| `[NEW] pkg/orchestra/reliability_*_test.go` | 생성 | cwd mismatch, prompt truncation, hook timeout 회귀 |

## Architecture Considerations

- `pkg/orchestra` 내부에서 pure core와 transport driver를 분리하되 public CLI surface는 유지한다.
- 기존 `SPEC-SURFCOMP-001`, `SPEC-ORCH-019`와 충돌하지 않도록 lifecycle/completion/subprocess backend 위에 reliability contract를 얹는다.
- `auto arch enforce` 기준 현재 아키텍처 규칙 위반은 없다.
- pipeline 영역과 직접 결합하지 말고, correlation ID와 bundle format으로만 연결한다.

## Tasks

- [x] 현재 pane/hook launch path에서 필요한 preflight 체크 포인트를 식별한다.
- [x] preflight receipt와 collection receipt의 공통 스키마를 정의한다.
- [ ] pane launch 경로에 deterministic cwd verification을 추가한다.
- [ ] prompt transport integrity 검증을 추가한다.
- [x] hook timeout/collection failure를 structured event와 degraded policy로 연결한다.
- [x] failure bundle과 correlation ID를 job snapshot/logs와 정렬한다.
- [x] hook-mode regression test와 pane-mode reliability regression test를 추가한다.

참고:
- pane-mode 회귀는 reliability artifact와 stale/recreate/hook timeout 경로 중심으로 추가되었고, explicit wrong-`cwd` block regression은 후속 작업이다.

## Risks & Mitigations

| 리스크 | 영향도 | 대응 |
|--------|--------|------|
| pane/transport 로직 수정이 기존 happy path를 깨뜨릴 수 있음 | 높음 | preflight layer를 additive하게 추가하고 기존 tests + 신규 regression tests를 함께 유지 |
| receipt/bundle이 과도하게 verbose해질 수 있음 | 중간 | compact bundle과 full debug bundle을 분리 |
| provider별 capability 차이가 커서 공통 contract가 약해질 수 있음 | 높음 | required/optional capability를 분리한 descriptor 사용 |
| timeout guard가 너무 공격적으로 동작할 수 있음 | 중간 | round/transport별 threshold를 config로 분리 |

## Dependencies

- 내부:
  - `pkg/orchestra/*`
  - `pkg/terminal/*`
  - `internal/cli/orchestra*.go`
- 참조 SPEC:
  - `SPEC-SURFCOMP-001`
  - `SPEC-ORCH-019`
- 외부 개념 참조:
  - Temporal durable execution
  - LangGraph persistence/checkpointing
  - OpenTelemetry trace/log correlation

## Exit Criteria

- [ ] P0 requirements 구현 완료
- [ ] hook-mode timeout 재현 테스트가 안정적으로 통과
- [ ] pane-mode cwd mismatch regression test 통과
- [ ] degraded run summary와 failure bundle이 생성됨
- [ ] 관련 orchestra package tests 통과
