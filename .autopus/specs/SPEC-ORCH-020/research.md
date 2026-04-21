# SPEC-ORCH-020 Research: Orchestra Reliability Kit

## Codebase Analysis

현재 orchestra는 이미 completion detector, surface manager, warm pool, hook watcher, subprocess backend, detach job을 보유한 상태다. 이번 SPEC은 새 엔진을 만드는 것이 아니라, 실행 contract를 reliability-first로 고정하는 작업이다.

### Target Files

| 파일 | 역할 | 변경 필요 |
|------|------|-----------|
| `pkg/orchestra/runner.go` | orchestration 진입점, backend 선택 | 높음 |
| `pkg/orchestra/interactive_launch.go` | pane launch 및 prompt 주입 | 높음 |
| `pkg/orchestra/interactive_debate_round.go` | round timeout/수집/복구 | 높음 |
| `pkg/orchestra/hook_watcher.go` | hook collection 및 fallback | 높음 |
| `pkg/orchestra/job.go` | detach/job snapshot | 중간 |
| `pkg/orchestra/types.go` | config, runtime metadata | 중간 |
| `pkg/orchestra/completion_*` | completion/fallback path | 중간 |
| `internal/cli/orchestra*.go` | CLI surface 및 summary | 중간 |

### Dependencies

- 내부 의존:
  - `pkg/orchestra` ↔ `pkg/terminal`
  - `internal/cli/orchestra*.go` → `pkg/orchestra`
- 관련 기존 SPEC:
  - `SPEC-SURFCOMP-001` - surface lifecycle/completion overhaul
  - `SPEC-ORCH-019` - subprocess orchestration engine

## Lore Decisions

`auto lore context`로 별도 출력된 lore는 없었다. changelog 기준 최근 orchestra 방향은 completion/hook/cc21/subprocess 안정화였고, reliability 보강은 자연스러운 다음 단계다.

## Architecture Compliance

`auto arch enforce` 결과 현재 아키텍처 규칙 위반은 없다.

## Key Findings

1. 현재 orchestra는 기능 폭보다 실행 contract가 더 취약하다.
2. pane-mode와 hook-mode 모두 "실행 전에 이것이 안전한지"를 명시적으로 증명하지 않는다.
3. job snapshot, logs, timeout, collection provenance가 하나의 correlation space로 묶여 있지 않다.
4. 기존 SPEC가 completion과 subprocess를 강화했지만 reliability preflight/receipt 층은 비어 있다.

## Reproduced Evidence

- 실제 pane-mode brainstorm에서 Claude pane가 `autopus-adk/` 대신 `~/Documents/github/bitgapnam`에서 시작되어 context 파일 로드가 실패했다.
- Codex/Gemini pane에서는 prompt transport가 절단되어 shell command로 일부가 흘러들어갔다.
- `go test -timeout 120s ./pkg/orchestra`는 초기에는 `TestRunPaneDebate_HookMode` 근처에서 timeout panic으로 실패했지만, reliability/runtime fixture sync 이후 `60.424s`에 통과하도록 수렴했다.
- `go test -timeout 120s -cover ./pkg/orchestra`는 낮은 coverage에서 FAIL 했다.

## Implementation Outcome

### Delivered in Sync

- `pkg/orchestra/reliability_receipt.go`
  - preflight, prompt transport, collection, event, failure bundle schema를 정의했다.
- `pkg/orchestra/reliability_preflight.go`
  - `run_id` 생성, working dir resolution, provider capability receipt, timeout event builder를 추가했다.
- `pkg/orchestra/reliability_bundle.go`
  - runtime artifact root, retention(20 runs / 7일), redaction, safe preview, bundle persistence를 추가했다.
- `pkg/orchestra/interactive_debate*.go`, `interactive_collect.go`, `detach.go`, `job.go`
  - hook timeout/partial collection receipt, degraded result, remediation summary, detached job artifact pointer를 연결했다.
- `internal/cli/orchestra*.go`
  - run id / degraded / artifact dir를 사용자 결과물에 표면화했다.
- 테스트
  - `reliability_core_test.go`, `reliability_collection_test.go`로 redaction, retention, preflight receipt, hook timeout evidence를 검증했다.
  - `go test -timeout 120s ./pkg/orchestra -count=1` 기준 전체 패키지가 통과한다.

### Remaining Gaps

- requested `cwd`를 launch command와 receipt에 바인딩하는 수준까지는 반영됐지만, provider shell에서 관측한 effective `cwd` mismatch를 차단하는 path는 아직 없다.
- prompt transport는 receipt/hash를 남기지만 mutation mismatch를 검출해 deterministic fail로 전환하지는 않는다.
- reliability metrics와 replay ledger는 이번 sync에 포함하지 않았다.

## Recommendations

- pane/subprocess 선택을 implicit fallback이 아니라 explicit capability/preflight decision으로 바꾼다.
- prompt injection 자체를 검증 가능한 transport로 간주하고 receipt를 남긴다.
- failure bundle은 screen scraping이 아니라 run metadata 중심으로 재구성 가능해야 한다.
- pure core와 transport driver를 분리하여 timeout/retry/degrade policy를 테스트 가능한 구조로 만든다.

후속 sync 권장 순서:

1. shell-observed `cwd` verification + preflight fail short-circuit
2. prompt transport mismatch detection
3. metrics / replay ledger
