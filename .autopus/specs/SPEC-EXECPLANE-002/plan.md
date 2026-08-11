# SPEC-EXECPLANE-002 Plan: orca 감독 실행을 PhaseBackend로 구현

**Created**: 2026-08-11
**Domain**: EXECPLANE
**Kind**: 설계 고정(design-only). 구현 코드는 이 SPEC의 산출물이 아니다.

## Implementation Strategy

### 접합점이 이미 있다 — 단일 메서드가 범위를 어디까지 줄이는가

`pkg/pipeline/engine.go:29-31`의 `PhaseBackend`는 메서드가 하나다 — `Execute(ctx, PhaseRequest) (*PhaseResponse, error)`. 이 사실이 범위 결정을 대신 해 준다. 엔진이 백엔드에게 요구하는 것은 "이 프롬프트를 실행하고 출력을 돌려달라" 하나뿐이고, 그 밖의 전부 — phase 순서, 게이트 판정, 재시도 카운팅, 체크포인트, 프롬프트 조립 — 는 백엔드 바깥에 있다(F1, Feature Coverage Map).

줄어든 크기는 셀 수 있다. 5-phase와 게이트·재시도는 `pkg/pipeline/phase.go:34-42`에 이미 **데이터로** 박혀 있다 — plan → test_scaffold → implement → validate(`GateValidation`, `MaxRetries: 3`) → review(`GateReview`, `MaxRetries: 2`). 따라서 orca 경로가 새로 정의하는 표면은 Feature Coverage Map 7개 중 **1개**(phase 실행 프로세스)이고 나머지 6개는 불변이다. 백엔드가 갖는 메서드는 `Execute` 하나와 run-scoped 해제 훅 `PhaseBackendCloser.Close()` 하나, 합쳐 둘이다(`engine.go:34-36`). 구조 선례도 있다 — `internal/cli/pipeline_backend_omp.go:16-46`이 `config + mu + run-scoped 리소스 + Close()` 모양을 이미 쓴다(F6). 새 관례를 발명하지 않는다.

여기서 계약 하나가 곧바로 파생된다. orca 경로는 omp 경로와 **같은 `EngineConfig`**로 `pipeline.SubprocessEngine`을 구동하고, 갈리는 지점은 주입되는 `Backend` 하나뿐이다(REQ-101). 별도 runner를 세우면 순서·게이트·재시도가 두 벌이 되고, 그 순간 REQ-101의 관측(S101)이 성립할 수 없다. "파이프라인을 orca로 다시 짠다"가 아니라 "백엔드를 하나 더 꽂는다"가 이 SPEC의 형태인 이유다.

### DAG 소유권 — `--deps`를 안 쓰는 것은 기능 낭비가 아니라 평면 분리다

orca에는 `task-create --deps`가 있다(F3). 5-phase를 orca DAG로 그리는 것이 **가능하다**. 가능한 것과 옳은 것은 다르다.

기각한 안 (a)(orca가 5-phase를 `--deps`로 재현)가 프로세스 평면에 복제하게 되는 것을 열거한다. 넷이다.

1. **게이트 판정.** validate는 `GateValidation`을, review는 `GateReview`를 통과해야 다음 간선이 열린다. orca가 순서를 소유하면 orca가 "이 phase가 통과했는가"를 판정해야 하고, 그 판정 기준은 정책 평면의 어휘다.
2. **재시도 카운팅.** `MaxRetries: 3`(validate)와 `2`(review)는 phase 정의에 붙어 있는 값이다. orca DAG가 재시도를 돌리려면 이 숫자와 소진 규칙이 프로세스 평면에도 있어야 한다. 카운터가 둘이 되는 순간 "몇 번째 시도인가"에 답이 둘 생긴다.
3. **체크포인트.** 재개는 autopus `pipelineStateDir`가 담당한다. orca task 상태와 autopus 체크포인트가 둘 다 "어디까지 갔는가"를 답하면, 재개 시점에 두 답이 갈릴 수 있고 어느 쪽이 진실인지 정할 규칙이 새로 필요해진다.
4. **프롬프트 조립.** `PhasePromptBuilder`가 phase별 프롬프트를 만든다. task를 DAG로 미리 만들어 두려면 실행 전에 프롬프트가 확정돼야 하는데, 실제 프롬프트는 앞 phase의 산출물에 의존한다. 미리 조립하면 조립기가 둘이 되고, 미리 조립하지 않으면 DAG를 먼저 그릴 이유 자체가 사라진다.

넷 모두 SPEC-EXECPLANE-001 INV-001(어휘 무증식) 위반이며, 4번은 그에 더해 자기모순이다 — DAG를 선행 생성해 얻으려던 이익이 조건을 충족하는 순간 소멸한다. 그리고 `pkg/adapter/omp/omp_workflow_render.go:77-78`의 단일 DAG 소유자 불변식이 두 DAG의 공존을 애초에 금지하므로, 이 선택은 취향이 아니라 배타적 분기다(F5).

그래서 (b)를 택한다. orca task는 `--deps` 없이 만들고, orca는 DAG를 만들지 않고 워커 하나씩만 감독한다(REQ-102 / INV-101). 쓸 수 있는 기능을 놀리는 것으로 보일 수 있으나, `--deps`는 orca가 **자기 워크플로**를 소유할 때 쓰라고 있는 것이지 남의 DAG를 대신 그리라고 있는 것이 아니다. 이 접합면에서 orca가 실제로 파는 값은 격리(워크트리)·감독(워커 수명)·durability 셋이고, 그 셋은 `--deps` 없이 전부 얻어진다. 반대로 `--deps`를 쓰면 위 네 가지를 얻는 대가 없이 복제한다.

배치는 이미 준비돼 있다. `internal/cli/pipeline_run.go:147-163`에서 티어 정합 게이트가 `handoff_required` 반환보다 **위**에 있다. 이 SPEC은 그 `return`을 orca 백엔드 선택으로 바꾸는 것이고, 게이트가 이미 그 위에 있으므로 INV-105(게이트 선행)는 새 순서 제약이 아니라 기존 배치의 부산물로 충족된다(F4).

### 왜 계약만 고정하고 구현을 분리하는가

research.md `## Completion Debt` 4건은 전부 orca 런타임을 실제로 구동해야 답이 나온다 — `--agent` 값 집합, 워커 settled 판정 방법, 워커 출력에서 phase 결과를 추출하는 규약, 재개 시 Run 재바인딩 정책. 네 답은 구현 선택을 실제로 가른다. settled를 읽을 상태 필드가 없으면 폴링 루프 대신 다른 대기 수단을 써야 하고, `worker-read`의 터미널 텍스트에서 결과를 안정적으로 뽑을 수 없으면 워커 쪽이 `orchestration send --outcome --report-path`로 결과를 밀어 올리는 반대 방향 설계가 된다. 같은 계약에 대해 코드가 두 갈래로 갈린다.

그러나 **계약 자체는 네 답과 무관하게 고정된다.** 이 문서가 정하는 것은 "무엇을 지켜야 하는가"이지 "어떤 호출로 지키는가"가 아니다. INV-102(1:1 Dispatch)는 settled를 어떤 필드로 읽든 성립해야 하고, INV-103(바운드)은 대기가 폴링이든 블로킹이든 deadline과 읽기 상한을 요구하며, INV-104(정리)는 결과 추출 규약이 무엇이든 모든 Dispatch의 종결을 요구한다. 네 답이 바꾸는 것은 계약을 만족시키는 **수단**이지 계약의 **내용**이 아니다.

분리에는 방향성이 하나 더 있다. Debt 4건은 문서 작업으로 닫히지 않고 런타임 프로브로만 닫힌다. 계약을 먼저 고정해 두면 그 프로브가 "orca로 무엇을 할 수 있나"라는 열린 탐색이 아니라 "이 계약을 어느 호출로 만족시키나"라는 닫힌 질문이 된다. T102가 정확히 그 자리이며, 이 분리는 spec.md `## Out of Scope` 첫 항목("이 SPEC의 구현 코드")과 일치한다.

## File Impact Analysis

이 SPEC은 설계 고정 문서이므로 소스 파일을 변경하지 않는다. 산출물은 SPEC 디렉터리의 4개 문서뿐이다.

| 파일 | 작업 | 설명 |
|------|------|------|
| `.autopus/specs/SPEC-EXECPLANE-002/spec.md` | 생성 | REQ-101~108, Outcome Boundary, Traceability Matrix, Out of Scope |
| `.autopus/specs/SPEC-EXECPLANE-002/research.md` | 생성 | 실측 근거 F1~F6, 불변식 INV-101~105, Completion Debt 4건 |
| `.autopus/specs/SPEC-EXECPLANE-002/plan.md` | 생성 | 이 문서. DAG 소유권 논증과 T101~T104 설계 태스크 |
| `.autopus/specs/SPEC-EXECPLANE-002/acceptance.md` | 생성 | S101~S108 시나리오와 Oracle Acceptance Notes |

아래 코드는 **참조 대상**이며 이 SPEC에서 수정하지 않는다: `pkg/pipeline/engine.go`, `pkg/pipeline/phase.go`, `internal/cli/pipeline_run.go`, `internal/cli/pipeline_backend_omp.go`, `pkg/adapter/omp/omp_workflow_render.go`.

## Architecture Considerations

- **의존 방향**: 정책 -> 프로세스 단방향만 허용한다. 백엔드는 orca 명령을 호출하지만 orca는 autopus의 phase·게이트·재시도 어휘를 참조하지 않는다. 역방향 참조가 생기면 INV-101이 깨진다.
- **경계 통과 값의 형태**: `worker-start`에 넘기는 것은 opaque provider model id와 effort뿐이다(REQ-107 / INV-101). `worker-start`가 이미 opaque id 소비자로 정의돼 있으므로(F3 노트) 새 계약이 아니라 기존 인터페이스를 그대로 지키는 것이다. 티어 문자열은 경계에서 소멸한다.
- **리소스 스코프 두 층**: Run은 run-scoped(백엔드 생성 시 `run-create` 1회, `Close()`에서 정리)이고 task·워커는 attempt-scoped(`Execute` 1회당 1세트)다. 두 층을 한 뮤텍스 아래 두는 `pipelineOMPBackend` 형태를 따르면 정리 책임의 소재가 한 곳에 남는다(F6).
- **omp 경로 회귀 0**: 갈리는 것은 주입되는 `Backend` 하나뿐이므로 omp 경로의 코드 경로는 손대지 않는다. spec.md `## Out of Scope`의 "omp 경로 동작 변경" 항목과 같은 제약이다.
- **기존 관측 표면 재사용**: 실행 결과는 omp 경로와 같은 `PhaseResponse`·체크포인트 표면으로 남긴다(REQ-108). 새 결과 스키마를 만들지 않는다.

## Visual Planning Brief

`Execute` 한 번의 **런타임 생애**다. research.md의 정적 평면 다이어그램과 달리 여기서는 시간이 위에서 아래로 흐르고, 세로 경계선이 두 평면의 소유권을 가른다.

```
[정책 평면: autopus 엔진]                                         | [프로세스 평면: orca]
소유: 순서 · 게이트 · 재시도 · 체크포인트 · 프롬프트              | 소유: 격리 · 감독 · durability · 회수
                                                                  |
=== Run 수명 시작 — 백엔드 1개당 1회 =============                | ===================================
  T0  티어 정합 게이트 (SPEC-EXECPLANE-001)                       | 리소스 0건. 워크트리·Run·워커 없음
        |  실패 -> 종료. Run을 만들지 않는다                      |
        v  통과: receipt.checked_at = T0                          |
  T1  run-create                          ===>                    | Run 생성 — T1 > T0 (INV-105)
                                                                  |
- - - attempt 수명 시작 — Execute 1회 = attempt 1회 - - - - - - - |
  Execute(ctx, req{PhaseID, Attempt, Prompt})                     | |
        |                                                         | |
  (1)  task-create --spec                  ===>                   | task 생성 · deps 빈 배열 (INV-101)
        |   --deps를 넘기지 않는다                                | orca 쪽 DAG 없음
        v                                                         | |
  (2)  worker-start --task --agent --worktree                     | |
         --model --effort --timeout-ms     ===>                   | Dispatch #N 시작 — 정확히 1건
        |   경계를 넘는 값: opaque model id + effort              | (INV-102)
        |   티어 이름은 넘기지 않는다 (REQ-107)                   | 워커가 워크트리 안에서 실행
        v                                                         | |
  (3)  worker-show 폴링 루프               <==>                   | 워커 상태
        |   조건: settled 아님 AND now < deadline                 | |
        |   deadline = ctx 마감 & 대기 상한 (INV-103)             | |
        +-- 마감 초과 ------------+                               | |
        v settled                 |                               | |
  (4)  worker-read --cursor --limit        <===                   | 출력 — 누적 상한까지만 (INV-103)
        |                         |                               | |
        v                         v                               | |
  (5a) PhaseResponse{Output,   (5b) PhaseResponse                 | |
        ExitCode, FailureClass}   {TimedOut: true}                | |
        +------------+------------+                               | |
                     v                                            | |
  (6)  종결 — 어느 경로로 오든 정확히 1회                         | |
        정상           -> worker-release   ===>                   | Dispatch settled
        마감 초과·취소 -> worker-stop      ===>                   | Dispatch settled
        정지 주장 불가 -> worker-abandon   ===>                   | Dispatch 펜싱 (INV-104)
- - - attempt 수명 끝 — 살아있는 Dispatch 0건 - - - - - - - - - - |
        |                                                         | |
        v                                                         | |
  게이트 판정 · 재시도 카운팅 · 체크포인트 기록                   | 관여 없음
        |   정책 평면 단독. orca는 이 판정을 보지 않는다          | |
        +-- 통과        -> 다음 phase, Attempt=1  --+             | |
        +-- 실패·여유   -> 같은 phase, Attempt+1  --+             | |
        |     어느 쪽이든 새 attempt = 새 task +    |             | |
        |     새 worker-start = 새 Dispatch.        |             | |
        |     워커 재사용도 재바인딩도 없다(INV-102)|             | |
        |   +---------------------------------------+             | |
        |   v  (1)로 되돌아간다                                   | |
        |                                                         | |
        +-- 재시도 소진 또는 5-phase 완주                         | |
             v                                                    | |
  Close()  잔여 Dispatch 전부 종결          ===>                  | worker-list --terminal-state
           결과는 omp 경로와 같은 표면으로 반환                   |   active == 0건 (INV-104)
           handoff_required가 아니다 (REQ-108)                    | |
=== Run 수명 끝 ==================================                | ===================================
```

읽는 법 넷.

1. **두 수명이 다른 띠로 그려져 있다.** `===` 띠는 Run 수명(백엔드 1개당 1회, `run-create`에서 열려 `Close()`에서 닫힘)이고 `- - -` 띠는 attempt 수명(`Execute` 호출마다 열리고 닫힘)이다. 5-phase에 validate 3회·review 2회의 재시도 여유가 붙으므로 하나의 `===` 띠 안에서 `- - -` 띠가 여러 번 열린다. Run을 attempt마다 새로 만들지 않는 이유는 워크트리와 계정 바인딩이 실행 전체의 속성이지 phase의 속성이 아니기 때문이다.
2. **세로선이 소유권 선언이고, 경계를 넘는 것은 명령 호출뿐이다.** 판정은 한 번도 경계를 넘지 않는다 — 그래서 "게이트 판정 · 재시도 카운팅 · 체크포인트" 블록의 오른쪽이 "관여 없음"이다. 같은 이유로 T0의 게이트도 왼쪽에만 있고, 실패하면 화살표가 아예 오른쪽으로 건너가지 않는다. 검증되지 않은 티어 계약 아래에서 워커가 뜨는 경로가 그림에 존재하지 않는다(INV-105).
3. **재시도는 attempt 띠를 다시 여는 것이지 워커를 되살리는 것이 아니다.** 루프 화살표가 (6) 종결 **뒤**에서 (1)로 돌아가므로, attempt N의 Dispatch는 attempt N+1이 시작되기 전에 이미 settled다. 두 Dispatch가 동시에 살아 있는 구간이 그림에 없다(INV-102). `worker-start`에 `--retry-of`가 있으나(F3) 이 계약은 그 플래그의 의미에 기대지 않는다 — 쓰든 쓰지 않든 새 attempt는 새 Dispatch다.
4. **성공·실패·타임아웃이 종결 블록을 공유한다.** (3)의 마감 초과 화살표는 별도 출구로 빠지지 않고 (5b)를 거쳐 (6)의 같은 블록으로 합류한다. 따라서 "실패해서 정리를 못 한 경로"는 그림상 그릴 수 없다(INV-104). `Close()`는 그 위의 안전망이지 유일한 정리 수단이 아니며, 정지를 주장할 수 없는 경우에는 release 대신 abandon으로 펜싱한다.

## Feature Completion Scope

이 SPEC은 **설계 고정 문서**다. 완료 판정에 동작하는 코드나 통과하는 테스트를 요구하지 않는다. Outcome Lock을 닫는 것은 "orca 경로가 5-phase를 완주한다"가 아니라 "완주하기 위한 접합 계약과 소유권 경계가 더 이상 해석의 여지 없이 고정됐다"다.

완료 조건은 spec.md `## Outcome Boundary`의 Completion evidence와 동일하게 셋이다.

1. 4개 SPEC 문서(`spec.md`·`plan.md`·`acceptance.md`·`research.md`)가 `auto spec validate`를 통과한다.
2. DAG 소유권 선택((b) — autopus가 순서를 소유하고 orca는 워커만 감독)에 대한 리뷰 합의가 기록된다. research.md `## Reviewer Brief`의 1번 질문이 그 자리다.
3. research.md Completion Debt 4건이 해소되거나 **착수 중 해소 가능한 항목으로 분류**된다. 네 건 모두 런타임 프로브로만 닫히므로 이 SPEC 안에서 해소되지 않는다. 분류가 완료 조건이다.

| 구분 | 항목 |
| --- | --- |
| **범위 안** | `PhaseBackend` 접합 계약과 omp 경로와 동일한 `EngineConfig` 요구 (REQ-101 / S101) |
| | orca task를 `--deps` 없이 만든다는 DAG 소유권 선언 (REQ-102 / S102) |
| | phase attempt와 Dispatch의 1:1 대응 규칙 (REQ-103 / S103) |
| | 워커 대기 deadline과 출력 읽기 상한의 계약, 초과 시 `TimedOut`/`FailureClass` 표기 (REQ-104 / S104) |
| | 실패·취소·`Close()` 경로의 종결 계약과 release / stop / abandon 선택 규칙 (REQ-105 / S105) |
| | 티어 정합 게이트가 Run 생성보다 앞선다는 순서 계약 (REQ-106 / S106) |
| | 경계를 넘는 인자를 opaque model id와 effort로 한정하는 규칙 (REQ-107 / S107) |
| | handoff 스텁 대체와 결과 표면의 omp 동형성 (REQ-108 / S108) |
| | S101~S108 시나리오와 그 관측 지점 |
| **범위 밖** | 이 SPEC의 구현 코드. 계약만 고정한다 |
| | phase 내부에서 executor를 여러 워커로 병렬 fan-out하는 일 |
| | `gate-create`/`gate-resolve`로 사람 결정을 파이프라인에 끼우는 일 |
| | 원격 환경(`worker-start --on <saved-environment>`) 지원 |
| | 5-phase 집합 변경, 게이트·재시도 semantics 변경 |
| | omp 경로 동작 변경. 회귀 0을 유지한다 |
| | 실행 원장 통합 |

범위 밖 목록은 spec.md `## Out of Scope`의 7항목과 1:1 대응이다. 새 항목을 추가하지 않았다.

**관측 한계.** 이 문서의 orca 쪽 주장은 전부 `orca agent-context --json`의 명령 스키마에서 나왔고, orca 런타임을 실제로 구동한 관측은 없다. 그 결과 Completion Debt 4건이 미해소로 남는다 — `--agent` 값 집합, settled 판정 필드와 종료 상태 집합, 워커 출력에서 `PhaseResponse.Output`을 얻는 규약, `--continue` 재개 시 Run 재바인딩 정책. 그럼에도 계약이 성립하는 이유는 넷 다 **계약의 내용이 아니라 만족 수단**을 정하기 때문이다. 1:1 Dispatch·바운드·정리·게이트 선행은 어떤 명령 조합으로 구현하든 동일하게 요구되고, S103~S105는 호출 시퀀스가 아니라 관측 가능한 결과(Dispatch 개수, deadline 초과 시 응답 필드, 잔여 `active` 0건)로 판정된다. 따라서 Debt는 계약을 미완으로 만들지 않고 T102·T103 착수의 첫 단계를 정의한다.

## Tasks

T101~T104는 모두 **설계·문서·합의 산출물**이다. 코드 작성 태스크는 없다 — 구현은 위 Feature Completion Scope의 범위 밖이다.

- [ ] **T101 — PhaseBackend 접합 계약과 DAG 소유권 선언 확정** (REQ-101, REQ-102 / INV-101 / S101, S102)
  - 산출물: (i) orca 경로가 omp 경로와 **같은 `EngineConfig`**로 `pipeline.SubprocessEngine`을 구동하고 차이가 주입 `Backend` 하나뿐임을 못박은 접합 선언, (ii) `task-create` 호출에서 `--deps`를 넘기지 않는다는 금지 규칙과 그 근거(위 네 가지 복제 목록), (iii) 기각한 안 (a)를 되살리려면 무엇을 먼저 뒤집어야 하는지의 기록.
  - 완료 판정: S101에서 두 경로의 `EngineConfig` 차이가 `Backend` 하나로 특정되고, S102에서 생성된 task의 deps가 빈 상태이며 순서 결정의 출처가 `DefaultPhases()` 단일임이 문서만 보고 판정된다. research.md `## Reviewer Brief` 1번 질문에 대한 판단이 기록된다.

- [ ] **T102 — 워커 수명과 바운드 계약 확정** (REQ-103, REQ-104 / INV-102, INV-103 / S103, S104)
  - 산출물: (i) `Execute` 1회 = task 1개 + `worker-start` 1회 + 종결 1회라는 1:1 규칙과, 재시도가 새 attempt이므로 새 Dispatch를 갖는다는 규칙, (ii) 대기 deadline의 출처(`ctx` 마감과 백엔드 대기 상한 중 먼저 오는 쪽)와 출력 읽기의 누적 상한, 그리고 두 한계 초과 시 `PhaseResponse`가 `TimedOut` 또는 `FailureClass`로 사유를 드러낸다는 표기 규칙.
  - **이 태스크가 Completion Debt 2건을 해소해야 한다.** ① **워커 settled 판정 방법** — `worker-show`가 노출하는 상태 필드 이름과 종료 상태 집합을 확정해야 폴링 종료 조건이 정의된다. 확정 전에는 "settled"가 계약 문장에만 있고 관측 가능한 술어가 되지 못한다. ② **워커 출력에서 phase 결과를 추출하는 규약** — `worker-read`가 주는 터미널/트랜스크립트 텍스트를 `PhaseResponse.Output`으로 삼을지, 아니면 워커 쪽이 `orchestration send --outcome --report-path`로 구조화된 결과를 밀어 올릴지를 정해야 한다. 두 선택은 읽기 상한의 의미도 바꾼다 — 전자는 텍스트 바이트 상한, 후자는 보고 파일 크기 상한이다. 둘 다 orca 런타임을 구동해야 답이 나오므로, 이 태스크의 첫 단계는 문서 작업이 아니라 프로브다.
  - 완료 판정: S103에서 한 attempt에 대한 Dispatch 시작 1건·종결 1건이 관측 가능한 술어로 적혀 있고, S104에서 대기 초과와 출력 초과 각각에 대해 반환되는 `PhaseResponse` 필드가 유일하게 결정된다. Debt 2건이 관측된 필드명·상태값으로 닫히거나, 닫히지 않으면 그 사실과 대안 설계가 명시된다.

- [ ] **T103 — 종결·정리 계약과 결과 표면 확정** (REQ-105, REQ-108 / INV-102, INV-104 / S105, S108)
  - 산출물: (i) 정상 완료·실패·취소·`Close()` 네 경로 각각에 대해 `worker-release` / `worker-stop` / `worker-abandon` 중 무엇을 쓰는지의 선택 규칙과, **프로세스 정지를 주장할 수 없는 경우에는 release 대신 abandon으로 펜싱한다**는 규정, (ii) run-scoped 리소스(Run)와 attempt-scoped 리소스(task·워커)의 정리 책임 분담 — attempt 정리는 `Execute` 안에서 끝나고 `Close()`는 잔여분에 대한 안전망이라는 선언, (iii) 실행 결과를 omp 경로와 같은 표면으로 반환하며 지원 구성에서 `handoff_required`를 최종 상태로 남기지 않는다는 규칙.
  - 완료 판정: S105에서 실패·취소·종료 어느 경로로 가도 이 Run의 `worker-list --terminal-state active`가 0건이 되는 근거가 경로별로 제시되고, S108에서 성공·실패 두 경우의 결과 형태가 omp 경로와 같음이 확정되며 `handoff_required`가 반환되는 남은 경우(비지원 구성)가 명시적으로 열거된다. Debt의 "재개 시 Run 재바인딩 정책"이 이 태스크의 정리 책임 분담과 충돌하지 않음이 확인된다.

- [ ] **T104 — 게이트 선행 순서와 경계 인자 확정** (REQ-106, REQ-107 / INV-101, INV-105 / S106, S107)
  - 산출물: (i) 티어 정합 게이트가 `run-create`보다 반드시 앞선다는 순서 계약과 그 관측 방법(정합 영수증의 `checked_at` < Run 생성 시각), 그리고 게이트를 우회해 Run이 만들어지는 경로가 존재하지 않음의 논증 — `internal/cli/pipeline_run.go:147-163`에서 게이트가 이미 반환점 위에 있다는 배치 근거(F4)를 함께 기록한다, (ii) `worker-start`에 넘기는 인자에서 금지되는 티어 토큰 목록(`balanced`/`ultra`/`opus`/`sonnet`/`haiku`)과 허용되는 통과 값(opaque provider model id, effort)의 명시.
  - 완료 판정: S106에서 게이트 실패 시 Run이 생성되지 않음이 배치만으로 도출되고, S107에서 경계를 넘는 인자 집합에 금지 토큰이 하나도 없음이 인자 목록만 보고 판정된다. 게이트 semantics는 SPEC-EXECPLANE-001의 것을 그대로 쓰고 이 SPEC에서 변경하지 않음이 명시된다.

REQ 커버리지: REQ-101·102(T101), REQ-103·104(T102), REQ-105·108(T103), REQ-106·107(T104) — 8건 전부 덮이며 spec.md `## Traceability Matrix`와 일치한다.

## Risks & Mitigations

| 리스크 | 영향도 | 대응 |
|--------|--------|------|
| Completion Debt 2건(settled 판정, 결과 추출)이 T102에서도 닫히지 않는다 | 중간 | 둘 다 런타임 프로브로만 닫히고 문서 작업으로는 닫히지 않는다. T102의 첫 단계를 프로브로 규정하고, 닫히지 않을 경우 대안 설계(`orchestration send --outcome --report-path` 경로)를 함께 남기도록 완료 판정에 요구했다. 계약 문장은 두 답 어느 쪽에서도 동일하다 |
| 구현 단계에서 편의를 위해 `--deps`가 슬그머니 들어온다 | 중간 | 위 네 가지 복제 목록을 T101 산출물에 못박고, S102가 "deps가 빈 상태"를 관측 지점으로 갖는다. 되살리려면 무엇을 먼저 뒤집어야 하는지도 T101에 기록하게 해 우회가 조용히 일어나지 않게 한다 |
| 재시도 경로에서 이전 워커를 재사용하려는 유혹이 INV-102를 깬다 | 중간 | `worker-start --retry-of`의 존재가 재사용처럼 읽힐 수 있다. 다이어그램이 종결 뒤에 루프를 두어 두 Dispatch의 동시 생존 구간을 없앴고, T102 산출물이 "새 attempt = 새 Dispatch"를 규칙으로 못박는다. 이 계약은 `--retry-of`의 의미에 기대지 않는다 |
| 실패 경로에서 `Close()`만 믿고 attempt 정리를 생략한다 | 중간 | 정리 책임을 두 층으로 나눠 attempt 정리는 `Execute` 안에서 끝내고 `Close()`를 안전망으로만 둔다(T103). S105가 경로별 근거를 요구하므로 "Close에서 다 치운다"는 한 줄로는 완료 판정이 서지 않는다 |
| 워커 출력이 커서 메모리를 소진하거나 정지한 워커가 파이프라인을 잡는다 | 중간 | REQ-104가 대기 deadline과 읽기 상한 둘 다를 요구한다. 한쪽만 두면 나머지 실패 양식이 남는다는 점을 T102 완료 판정이 두 시나리오(대기 초과·출력 초과)로 분리해 강제한다 |
| 재개(`--continue`) 시 새 Run을 만들지 기존 Run에 붙일지가 미정이라 체크포인트와 어긋난다 | 낮음 | Debt 4번째 항목이며 이 SPEC의 요구 8건 중 어느 것도 재개 정책에 의존하지 않는다. T103 완료 판정이 정리 책임 분담과의 무충돌만 확인하고, 정책 확정은 착수 중 해소 항목으로 분류한다 |
| 설계만 고정하고 구현을 분리한 결과 handoff 스텁이 계속 남는다 | 중간 | 분리는 spec.md `## Out of Scope`에 명시된 결정이다. 접합점이 단일 메서드라 후속 구현이 발명할 것이 없고, 남는 미결이 Debt 4건으로 좁혀져 있어 착수 장벽이 낮다 |

## Dependencies

- **SPEC-EXECPLANE-001** — 세 평면 모델, INV-001(어휘 무증식), 티어 정합 게이트. 이 SPEC은 그 결과를 입력으로 받고 게이트 semantics를 변경하지 않는다.
- **`pkg/pipeline` 엔진** — `PhaseBackend`/`PhaseBackendCloser`(`engine.go:29-36`), `DefaultPhases()`(`phase.go:34-42`). 접합면이자 불변 전제다. 인터페이스 변경을 요구하지 않는다.
- **`internal/cli/pipeline_backend_omp.go`** — 백엔드 구조의 선례(F6). 형태만 따르고 코드를 수정하지 않는다.
- **orca orchestration CLI** — `run-create` / `task-create` / `worker-start` / `worker-show` / `worker-read` / `worker-list` / `worker-release` `worker-stop` `worker-abandon`(F3). orca를 소비자로만 취급하며 CLI 표면 변경을 요구하지 않는다.
- **orca 런타임 구동** — Completion Debt 4건의 유일한 해소 수단. 문서 작업으로 대체할 수 없다.
- **`auto spec validate`** — 완료 조건 1번의 검증 수단.

새 외부 라이브러리 의존은 없다. 설계 문서이므로 코드 의존도 추가하지 않는다.

## Exit Criteria

- [ ] `auto spec validate .autopus/specs/SPEC-EXECPLANE-002`가 4개 문서에 대해 통과한다
- [ ] T101~T104의 산출물이 모두 존재하고 REQ-101~108 8건을 빠짐없이 덮는다
- [ ] DAG 소유권 선택 (b)에 대한 리뷰 합의가 기록됐다 — research.md `## Reviewer Brief` 1번
- [ ] Completion Debt 4건이 각각 해소 또는 착수 중 해소 가능 항목으로 분류됐다
- [ ] 범위 밖 목록이 spec.md `## Out of Scope` 7항목과 1:1로 대응하고 모순되지 않는다

구현 증거·테스트 통과·커버리지 수치는 이 SPEC의 종료 조건이 아니다. 설계 고정 문서이며, 구현은 별도 SPEC의 대상이다.
