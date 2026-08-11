# SPEC-EXECPLANE-002: orca 감독 실행을 PhaseBackend로 구현

**Status**: draft
**Created**: 2026-08-11
**Domain**: EXECPLANE

---
id: SPEC-EXECPLANE-002
title: orca 감독 실행을 PhaseBackend로 구현
version: 0.1.0
status: draft
priority: HIGH
---

## Purpose

`--execution-owner orca`가 handoff 스텁에서 멈추는 상태를 끝내고, orca가 감독하는 워커에서 파이프라인이 실제로 완주하도록 실행 계약을 정의한다. 정책 평면은 순서·게이트·재시도를 계속 소유하고, orca는 격리·감독·durability만 맡는다.

이 SPEC은 설계 고정 문서다. 구현은 포함하지 않는다.

## Background

SPEC-EXECPLANE-001이 세 평면 경계를 고정하고 티어 정합 게이트를 handoff 직전에 배치했다. 게이트는 동작하지만 그 뒤가 비어 있다 — `internal/cli/pipeline_run.go:162`가 `status=handoff_required`를 반환하고 사람이 손으로 이어받는다.

접합점은 이미 있다. `pkg/pipeline/engine.go:29-31`의 `PhaseBackend`는 단일 메서드다.

```go
type PhaseBackend interface {
	Execute(ctx context.Context, req PhaseRequest) (*PhaseResponse, error)
}
```

파이프라인은 5-phase이고 게이트와 재시도가 이미 정의되어 있다(`pkg/pipeline/phase.go:34-42`): plan → test_scaffold → implement → validate(3회) → review(2회). 따라서 orca 실행은 **파이프라인 재구현이 아니라 `PhaseBackend` 구현 하나**다.

여기서 유일한 설계 갈림길은 DAG 소유권이다. orca에는 `task-create --deps`가 있어 자기 DAG를 가질 수 있고, 단일 DAG 소유자 불변식(`pkg/adapter/omp/omp_workflow_render.go:77-78`)이 두 DAG를 금지한다.

| 접근 | DAG 소유자 | 결과 |
| --- | --- | --- |
| orca가 5-phase를 `--deps`로 재현 | orca | 게이트·재시도·체크포인트를 프로세스 평면에 복제 — SPEC-EXECPLANE-001 INV-001 위반 |
| **autopus 엔진이 순서를 소유, orca는 워커만 감독** | autopus | 정책은 정책 평면에, 격리·감독은 프로세스 평면에 |

후자를 택한다. orca task는 `--deps` 없이 만든다. orca는 DAG를 만들지 않고 워커 하나씩만 감독한다.

## Outcome Boundary

- **Outcome Lock**: `auto pipeline run --platform omp --execution-owner orca`가 orca 감독 워커에서 5-phase를 완주하고, 정책 평면의 게이트·재시도·체크포인트가 omp 경로와 동일하게 적용되며, 실행이 끝나거나 실패해도 orca에 살아있는 Dispatch가 남지 않는다.
- **Mandatory requirements**: PhaseBackend 구현으로 접합(REQ-101), orca task는 deps 없이 생성(REQ-102), phase 하나당 Dispatch 하나(REQ-103), 워커 출력과 대기의 바운드(REQ-104), 실패 경로 리소스 정리(REQ-105), 티어 정합 게이트를 Run 이전에 유지(REQ-106), 프로세스 평면 경계에 티어 어휘 무증식(REQ-107), handoff 스텁 대체와 관측 가능한 실행 결과(REQ-108).
- **Explicit non-goals**: 이 SPEC의 구현 코드(별도 작업), phase 내부 executor 병렬 fan-out, 사람 결정 게이트(`gate-create`) 도입, 원격 환경(`worker-start --on`) 지원, 5-phase 집합 변경, 게이트·재시도 semantics 변경, omp 경로 동작 변경(regression-0), 실행 원장 통합.
- **Completion evidence**: 4개 SPEC 문서가 `auto spec validate`를 통과하고, DAG 소유권 선택에 대한 리뷰 합의가 기록되며, Completion Debt가 해소되거나 착수 중 해소 가능한 항목으로 분류된다. 구현 증거는 요구하지 않는다.

## Requirements

### REQ-101 — orca 실행은 PhaseBackend 구현으로 접합한다
THE SYSTEM SHALL execute orca-supervised phases through an implementation of the existing `PhaseBackend` interface rather than a parallel pipeline runner, so phase ordering, gate evaluation, retry accounting, and checkpointing keep their single implementation in the policy plane.
- EARS type: Ubiquitous
- Priority: Must
- Trigger/Condition: `--execution-owner orca`로 파이프라인이 실행될 때.
- Observability: orca 경로가 `pipeline.SubprocessEngine`을 omp 경로와 같은 `EngineConfig`로 구동하고, 다른 점은 주입된 `Backend` 하나뿐임을 S101로 확인한다.

### REQ-102 — orca task는 의존성 없이 생성한다
THE SYSTEM SHALL create every orca task without dependency edges, so the process plane never holds a second task graph while the policy plane owns phase ordering.
- EARS type: Ubiquitous
- Priority: Must
- Trigger/Condition: 백엔드가 phase 실행을 위해 task를 만들 때.
- Observability: 생성된 task의 deps가 빈 상태이고, 순서 결정이 전적으로 엔진의 `DefaultPhases()`에서 나옴을 S102로 확인한다.

### REQ-103 — phase 하나당 Dispatch 하나를 시작하고 회수한다
WHEN the backend executes one phase attempt, THEN THE SYSTEM SHALL start exactly one supervised worker for it and settle that Dispatch exactly once, so worker accounting maps one-to-one onto phase attempts.
- EARS type: Event-driven
- Priority: Must
- Trigger/Condition: `Execute` 한 번의 호출.
- Observability: 한 attempt에 대해 시작된 Dispatch가 정확히 1건이고 종결도 1건임을 S103으로 확인한다. 재시도는 새 attempt이므로 새 Dispatch를 갖는다.

### REQ-104 — 워커 대기와 출력은 바운드된다
THE SYSTEM SHALL bound both the wait for a worker to settle and the amount of output it reads, so a stalled or noisy worker cannot hang the pipeline or exhaust memory.
- EARS type: Ubiquitous
- Priority: Must
- Trigger/Condition: 백엔드가 워커의 종결을 기다리거나 출력을 읽을 때.
- Observability: 대기에 deadline이 있고 읽기에 상한이 있으며, 두 한계 모두 초과 시 `PhaseResponse`가 `TimedOut` 또는 `FailureClass`로 사유를 드러냄을 S104로 확인한다.

### REQ-105 — 실패해도 살아있는 Dispatch를 남기지 않는다
IF a phase attempt fails, is cancelled, or the backend is closed, THEN THE SYSTEM SHALL settle every Dispatch it started via release, stop, or abandon, and SHALL NOT leave an active supervised terminal behind.
- EARS type: Unwanted
- Priority: Must
- Trigger/Condition: 실행 실패·취소·백엔드 종료.
- Observability: 실행 종료 후 이 Run에 속한 `active` 상태 terminal이 0건임을 S105로 확인한다. 프로세스 정지를 주장할 수 없는 경우에는 release 대신 abandon으로 펜싱한다.

### REQ-106 — 티어 정합 게이트는 Run 생성 이전에 끝난다
WHERE the execution owner is orca, THE SYSTEM SHALL complete the tier integrity gate of SPEC-EXECPLANE-001 before creating the orca Run, so no worker starts under an unverified tier contract.
- EARS type: State-driven
- Priority: Must
- Trigger/Condition: orca 경로의 준비 단계.
- Observability: 정합 영수증의 `checked_at`이 Run 생성 시각보다 앞서고, 게이트가 실행되지 않은 채 Run이 만들어지는 경로가 없음을 S106으로 확인한다.

### REQ-107 — 프로세스 평면 경계에 티어 어휘를 넘기지 않는다
THE SYSTEM SHALL pass only opaque provider model identifiers and effort levels to orca, never the policy plane's tier names, so the tier ladder stays single-sourced.
- EARS type: Ubiquitous
- Priority: Must
- Trigger/Condition: 백엔드가 `worker-start`에 모델과 effort를 넘길 때.
- Observability: 경계를 넘는 인자에 `balanced`/`ultra`/`opus`/`sonnet`/`haiku` 토큰이 없음을 S107로 확인한다. `worker-start`는 이미 opaque provider model id를 받는 인터페이스다.

### REQ-108 — handoff 스텁을 실행으로 대체하고 결과를 관측 가능하게 남긴다
WHEN the orca path completes or fails, THEN THE SYSTEM SHALL report the outcome through the same pipeline result surface the omp path uses, and SHALL NOT return `handoff_required` as a terminal state for a supported configuration.
- EARS type: Event-driven
- Priority: Must
- Trigger/Condition: orca 경로의 실행 종료.
- Observability: 지원되는 구성에서 `status=handoff_required`가 최종 결과로 반환되지 않고, phase 결과와 체크포인트가 omp 경로와 같은 형태로 남음을 S108로 확인한다.

## Acceptance Criteria

- [ ] orca 실행이 PhaseBackend 구현 하나로 접합된다
- [ ] orca task에 의존성 간선이 없다
- [ ] phase attempt 하나당 Dispatch 하나가 시작되고 종결된다
- [ ] 워커 대기와 출력이 모두 바운드된다
- [ ] 실패 경로가 살아있는 Dispatch를 남기지 않는다
- [ ] 티어 정합 게이트가 Run 생성보다 앞선다
- [ ] 프로세스 평면 경계에 티어 어휘가 없다
- [ ] 지원 구성에서 `handoff_required`가 최종 상태로 남지 않는다

## Traceability Matrix

| Requirement | Plan Task | Acceptance Scenario | Semantic Invariant |
|-------------|-----------|---------------------|--------------------|
| REQ-101 | T101 | S101 | INV-101 |
| REQ-102 | T101 | S102 | INV-101 |
| REQ-103 | T102 | S103 | INV-102 |
| REQ-104 | T102 | S104 | INV-103 |
| REQ-105 | T103 | S105 | INV-104 |
| REQ-106 | T104 | S106 | INV-105 |
| REQ-107 | T104 | S107 | INV-101 |
| REQ-108 | T103 | S108 | INV-102, INV-104 |

## Out of Scope

- 이 SPEC의 구현 코드. 계약만 고정한다.
- phase 내부에서 executor를 여러 워커로 병렬 fan-out하는 일.
- `gate-create`/`gate-resolve`로 사람 결정을 파이프라인에 끼우는 일.
- 원격 환경(`worker-start --on <saved-environment>`) 지원. 계정 조회가 원격을 지원하지 않아 티어 검증이 unverified로 떨어진다.
- 5-phase 집합 변경, 게이트·재시도 semantics 변경.
- omp 경로 동작 변경. 회귀 0을 유지한다.
- 실행 원장 통합.

## Traceability

| Requirement | Test | Status |
|-------------|------|--------|
| REQ-101 | S101 | pending |
| REQ-102 | S102 | pending |
| REQ-103 | S103 | pending |
| REQ-104 | S104 | pending |
| REQ-105 | S105 | pending |
| REQ-106 | S106 | pending |
| REQ-107 | S107 | pending |
| REQ-108 | S108 | pending |
