# SPEC-EXECPLANE-002 Research: orca 감독 실행을 PhaseBackend로 구현

**Created**: 2026-08-11
**Domain**: EXECPLANE

## Outcome Lock

`auto pipeline run --platform omp --execution-owner orca`가 handoff 스텁을 반환하고 멈추는 대신, 실제로 orca가 감독하는 워커에서 파이프라인을 끝까지 실행한다. 정책 평면(SPEC·게이트·재시도·체크포인트)은 autopus가 그대로 소유하고, orca는 격리·감독·durability만 제공한다. 어느 평면도 상대의 어휘를 복제하지 않는다.

## Visual Planning Brief

```
  autopus 엔진 (정책 평면)                      orca (프로세스 평면)
  DefaultPhases() 5-phase DAG
  게이트 · 재시도 · 체크포인트
        |
        |  phase 하나당 Execute 1회
        v
  +-------------------------+
  | PhaseBackend.Execute    |            run-create        (백엔드 생성 시 1회)
  |   req.Prompt            |  -------->  task-create      (deps 없음)
  |   req.PhaseID           |  -------->  worker-start     (--task, --agent, --worktree)
  |                         |  <-------   worker-show/read (settled까지)
  |   PhaseResponse.Output  |  -------->  worker-release
  +-------------------------+
        |
        v  (게이트·재시도는 autopus가 판정)
  다음 phase
```

`--deps`를 쓰지 않는 것이 핵심이다. 순서는 autopus 엔진이 소유하고, orca는 워커 하나씩만 감독한다.

## Semantic Invariant Inventory

| ID | 불변식 | 유형 | 관측 지점 |
| --- | --- | --- | --- |
| INV-101 | orca task는 `--deps` 없이 생성된다. 순서와 게이트는 정책 평면이 소유하므로 프로세스 평면에 두 번째 DAG가 생기지 않는다 | 배타 | 생성된 task의 deps가 빈 배열 |
| INV-102 | phase 하나당 Dispatch 하나. 워커는 정확히 한 번 시작되고 한 번 회수된다 | 1:1 | `worker-list`의 잔여 terminal 0건 |
| INV-103 | 워커 출력은 바운드된다. 무한 대기도 무한 버퍼도 없다 | 바운드 | `worker-read --limit`, 백엔드 deadline |
| INV-104 | 실행이 실패해도 orca 리소스가 남지 않는다. 모든 Dispatch는 release 또는 abandon으로 종결된다 | 정리 | 종료 후 `worker-list --terminal-state active` 0건 |
| INV-105 | 티어 정합 영수증이 Run 시작 이전에 존재한다. 검증 없이 워커를 띄우지 않는다 | 순서 | `<SPEC-ID>.tier-integrity.json`의 `checked_at` < Run 생성 시각 |

## Feature Coverage Map

| 표면 | 현재 소유자 | 이 SPEC 이후 |
| --- | --- | --- |
| 5-phase DAG 정의와 순서 | autopus `pipeline.DefaultPhases()` | 불변 |
| 게이트 판정과 재시도 | autopus 엔진 | 불변 |
| 체크포인트·재개 | autopus `pipelineStateDir` | 불변 |
| 프롬프트 조립 | autopus `PhasePromptBuilder` | 불변 |
| **phase 실행 프로세스** | omp 서브프로세스 / provider CLI | **orca 감독 워커 추가** |
| worktree 격리·계정·durability | orca | 불변 |
| 티어 정합 게이트 | autopus (SPEC-EXECPLANE-001) | 불변, Run 이전에 실행 |

## 측정된 근거

모두 2026-08-11 이 워크스테이션에서 직접 확인했다.

### F1 — `PhaseBackend`는 단일 메서드 인터페이스다

`pkg/pipeline/engine.go:29-31`
```go
type PhaseBackend interface {
	Execute(ctx context.Context, req PhaseRequest) (*PhaseResponse, error)
}
```

`PhaseRequest`는 `Prompt`, `PhaseID`, `Attempt`, `OMPExecutionView`(omp 전용)를 담고, `PhaseResponse`는 `Output`, `Provider`, `Backend`, `Role`, `ExitCode`, `TimedOut`, `FailureClass`, `Artifact`를 돌려준다(`engine.go:38-56`). `PhaseBackendCloser`가 run-scoped 리소스 해제 훅을 제공한다(`engine.go:34-36`).

**이것이 접합점이다.** orca 실행은 파이프라인 재구현이 아니라 `PhaseBackend` 구현 하나다.

### F2 — 파이프라인은 5-phase이고 게이트·재시도가 이미 정의돼 있다

`pkg/pipeline/phase.go:34-42`
```go
{ID: PhasePlan}
{ID: PhaseTestScaffold, DependsOn: [plan]}
{ID: PhaseImplement,    DependsOn: [test_scaffold]}
{ID: PhaseValidate,     DependsOn: [implement], Gate: GateValidation, MaxRetries: 3}
{ID: PhaseReview,       DependsOn: [validate],  Gate: GateReview,     MaxRetries: 2}
```

순서·게이트·재시도가 전부 정책 평면에 있다. 프로세스 평면이 이걸 다시 가질 이유가 없다.

### F3 — orca orchestration 표면은 29개 명령이고 필요한 것이 다 있다

| 목적 | 명령 |
| --- | --- |
| Run 수명 | `run-create` `run-use` `run-current` `run-show` `run-list` |
| task | `task-create --spec --deps --parent` `task-list` `task-update` |
| 워커 | `worker-start --task --agent --worktree --model --effort --timeout-ms --retry-of` |
| 관측 | `worker-show` `worker-read --cursor --limit` `worker-list --terminal-state` |
| 종결 | `worker-release` `worker-stop` `worker-abandon` `worker-retain` |
| 결정 게이트 | `gate-create` `gate-resolve` `gate-list` |
| 메시징 | `send` `check` `reply` `inbox` `ask` |

`worker-start`는 `--model`과 `--effort`를 받고, 노트에 *"--model supports Claude, Codex, and Cursor opaque provider model ids; --effort requires --model"*이라 적혀 있다. SPEC-EXECPLANE-001 REQ-002(프로세스 평면에 티어 어휘 무증식)와 정확히 맞는 인터페이스다 — 티어가 아니라 opaque id를 넘긴다.

### F4 — 현재 handoff는 스텁이고 게이트는 이미 그 앞에 있다

`internal/cli/pipeline_run.go:147-163`
```go
if platform == "omp" {
    integrity := pipelineTierIntegritySkipped()
    if ownerDecision.Owner == pipelineExecutionOwnerOrca {
        integrity = runPipelineTierIntegrityGate(cmd.Context(), projectDir, specID)
    }
    ownerReceipt, ownerReceiptPath, receiptErr := persistPipelineExecutionOwnerReceipt(...)
    if ownerDecision.Owner == pipelineExecutionOwnerOrca {
        return emitPipelineExecutionOwnerHandoff(cmd, ownerReceipt, ownerReceiptPath, integrity)
    }
}
```

`return`이 체크포인트 로드·백엔드 생성·엔진 실행보다 앞에 있다. 이 SPEC은 그 `return`을 orca 백엔드 선택으로 바꾼다. 정합 게이트는 이미 그 위에 있으므로 INV-105가 배치로 충족된다.

### F5 — 단일 DAG 소유자 불변식이 설계를 강제한다

`pkg/adapter/omp/omp_workflow_render.go:77-78`
```
- The single DAG owner invariant is mandatory: when Orca owns the DAG, the OMP
  session does not dispatch a competing DAG; when OMP owns it, Orca does not
  dispatch one.
- owner `orca` creates no OMP task DAG, and owner `omp` creates no Orca Run.
```

orca에는 `task-create --deps`가 있어 **자기 DAG를 가질 수 있다**. 그래서 선택이 필요하다.

| 접근 | DAG 소유자 | 결과 |
| --- | --- | --- |
| (a) orca가 5-phase를 `--deps`로 재현 | orca | 게이트·재시도·체크포인트를 프로세스 평면에 복제해야 한다. SPEC-EXECPLANE-001 INV-001(어휘 무증식) 위반 |
| (b) autopus 엔진이 순서를 소유하고 orca는 워커만 감독 | autopus | 정책은 정책 평면에, 격리·감독은 프로세스 평면에. 세 평면 모델과 일치 |

**(b)를 택한다.** orca task는 `--deps` 없이 만든다 — orca는 DAG를 만들지 않고, 워커 하나씩만 감독한다. INV-002를 문자 그대로 지킨다.

### F6 — 기존 백엔드가 구조 선례를 제공한다

`internal/cli/pipeline_backend_omp.go:16-46`이 `pipelineOMPBackendConfig` + `pipelineOMPBackend{mu, config, process}` 형태로 run-scoped 리소스를 뮤텍스 아래 들고 `Close()`로 해제한다. orca 백엔드도 같은 모양을 따르면 새 관례를 만들지 않는다.

## Completion Debt

- [ ] `worker-start`의 `--agent` 값 집합. `worktree create --agent <id>`와 같은 어휘인지, 어떤 id가 유효한지 구현 전에 확정해야 한다.
- [ ] 워커 settled 판정 방법. `worker-show`의 상태 필드 이름과 종료 상태 집합을 확인해야 폴링 루프를 쓸 수 있다.
- [ ] 워커 출력에서 phase 결과를 추출하는 규약. `worker-read`는 터미널/트랜스크립트 텍스트를 주는데, 엔진은 `PhaseResponse.Output` 문자열을 기대한다. 구조화된 결과가 필요하면 `orchestration send --outcome --report-path` 경로를 워커 쪽에서 써야 할 수 있다.
- [ ] 재개(`--continue`) 시 기존 Run에 재바인딩할지 새 Run을 만들지. `run-use`가 있으나 체크포인트와의 결합 규칙이 미정이다.

## Evolution Ideas

- phase 안에서 executor를 여러 워커로 fan-out. `worker-start`를 병렬로 여러 번 부르고 결과를 합치면 route_team의 병렬 구현에 대응한다.
- `gate-create`/`gate-resolve`로 사람 결정을 파이프라인 중간에 끼우기. 지금은 게이트가 전부 자동 판정이다.
- 원격 실행. `worker-start --on <saved-environment>`가 이미 있으나 계정 조회가 원격을 지원하지 않아 티어 검증이 unverified로 떨어진다.
- 워커 출력을 실행 원장으로 흘려 정책·모델·프로세스 세 평면의 관측을 한곳에 모으기.

## Reference Discipline

| 참조 | 유형 | 확인 방법 |
| --- | --- | --- |
| `orca agent-context --json` orchestration 29개 명령 | 실측 | 스키마 전수 추출 |
| `orchestration worker-start` 노트(opaque model id, --effort 요구) | 실측 | 같은 스키마 |
| `pkg/pipeline/engine.go:29-56` | 코드 | 직접 읽음 |
| `pkg/pipeline/phase.go:34-42` | 코드 | 직접 읽음 |
| `internal/cli/pipeline_run.go:147-163` | 코드 | 직접 읽음 |
| `internal/cli/pipeline_backend_omp.go:16-46` | 코드 | 직접 읽음 |
| `pkg/adapter/omp/omp_workflow_render.go:77-78` | 코드 | 직접 읽음 |
| `pkg/pipeline/engine_run.go:102` — `for attempt := 1; attempt <= maxRetries+1` | 코드 | 직접 읽음. attempt 상한이 `MaxRetries+1`임을 고정 |
| `pkg/pipeline/engine_run.go:115` — `Receipt.DispatchCount++` | 코드 | 직접 읽음. attempt 하나가 Dispatch 하나로 계수됨 |
| `pkg/pipeline/engine_test.go:42-44` — 클린 런 `DispatchCount == 5` | 코드 | 직접 읽음. 5-phase 무재시도 실행의 기준선 |
| SPEC-EXECPLANE-001 | 문서 | 이 저장소, PR #155~#159 |

추측은 없다. 위 표에 없는 주장은 담지 않았다. Completion Debt 4건은 orca 런타임을 실제로 구동해야 답할 수 있어 미확인으로 분리했다.

## Reviewer Brief

이 SPEC도 설계 고정이고 구현을 포함하지 않는다. 우선 검증할 것:

1. F5의 (b) 선택이 옳은가. orca에 `--deps`가 있는데 안 쓰는 것이 낭비가 아니라 평면 분리인가.
2. `PhaseBackend` 한 개로 충분한가. 게이트·재시도·체크포인트가 정말 정책 평면에만 있는가, 아니면 orca 감독이 필요한 부분이 있는가.
3. Completion Debt 4건이 구현 착수를 막는가, 아니면 착수하며 답할 수 있는가.
4. INV-104(리소스 정리)가 실패 경로에서 실제로 보장되는가. `Close()`만으로 충분한지, `worker-abandon`이 필요한 경로가 있는지.

블로킹 대상은 Completion Debt뿐이다. Evolution Ideas는 자문이다.

## Self-Verify Summary

- **Q-CORR-04** (근거 정확성) | status: PASS — 모든 사실 주장이 Reference Discipline 표의 실측 또는 파일:라인 인용에 대응한다. orca 명령 표면은 스키마에서 직접 추출했고, 파이프라인 구조는 코드에서 직접 읽었다.
- **Q-COMP-05** (범위 완결성) | status: PASS — 접합점(`PhaseBackend`)을 특정하고, DAG 소유권이라는 유일한 설계 갈림길을 (a)/(b)로 열거해 근거와 함께 하나를 택했다. Feature Coverage Map이 이 SPEC이 바꾸는 표면 1개와 유지하는 표면 6개를 전수 구분한다.
- **Q-COMP-06** (미해결 항목 분리) | status: PASS — orca 런타임을 구동해야 답할 수 있는 4건을 Completion Debt로 분리했다. Evolution Ideas에는 `SPEC-`·`AC-`·체크박스를 넣지 않아 블로킹과 자문이 섞이지 않는다.
- **Q-COMP-07** (완료 판정 가능성) | status: PASS — 설계 문서이므로 완료 증거는 4개 문서의 `auto spec validate` 통과와 DAG 소유권 선택에 대한 리뷰 합의다. 구현 증거는 요구하지 않으며 그 사실을 Outcome Boundary에 명시했다.
