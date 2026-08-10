# SPEC-EXECPLANE-001 Plan: 실행 평면 분리와 티어·계정·카탈로그 정합

**Created**: 2026-08-10
**Domain**: EXECPLANE
**Kind**: 설계 고정(design-only). 구현 코드는 이 SPEC의 산출물이 아니다.

## Implementation Strategy

### 게이트는 정책 평면(autopus-adk)에 둔다

정합 게이트가 알아야 하는 값은 셋이다 — **요청 티어**(정책 평면 소유), **실행 계정**(프로세스 평면 소유), **그 계정의 카탈로그**(모델 평면 어휘). 세 값이 한자리에 모여야 판정이 성립하므로, 게이트를 어디에 두느냐는 "어느 평면이 남의 어휘를 읽어도 되는가"의 문제로 환원된다.

정책 평면만 이 조건을 위반 없이 만족한다. 티어 어휘의 단일 출처가 이미 정책 평면(`pkg/config/quality_tier.go`, research.md Feature Coverage Map)이고, J1(티어 -> omp `modelRoles`)의 투영 지점도 정책 평면(`pkg/adapter/omp/omp_model_projection.go:59-107`)이다. 정책 평면이 프로세스 평면에서 계정 식별자를, 모델 평면에서 카탈로그를 **읽어 오는** 방향은 이미 존재하는 의존 방향(정책 -> 모델, 정책 -> 프로세스)과 같다. 새 역방향 의존이 생기지 않는다.

### 기각한 대안 (a) — orca(프로세스 평면)에 둔다

orca가 게이트를 소유하면 orca가 "요청 티어"를 이해해야 한다. 그런데 F4의 전수 검색 결과 `orca agent-context --json`(223 command, schema v1)에 `quality` 0건, `tier` 0건, `opus`/`sonnet`/`gpt-5`/`balanced`/`ultra` 0건이고, 모델·effort를 받는 명령은 `orchestration worker-start` 하나뿐이며 그 계약은 다음과 같다.

```
--model supports Claude, Codex, and Cursor opaque provider model ids;
--effort requires --model. Neither can combine with --terminal.
```

즉 orca는 설계상 opaque provider model id 소비자다. 여기에 티어 사다리를 심으면 티어 어휘가 두 평면에 동시에 존재하게 되어 **INV-001(어휘 무증식)을 직접 위반**하고, REQ-002가 금지하는 바로 그 상태가 된다. 게이트를 orca에 두는 선택은 이 SPEC의 요구 두 건과 정면으로 충돌하므로 기각한다.

### 기각한 대안 (b) — omp(모델 평면)에 둔다

omp가 게이트를 소유하면 omp가 "이 워크로드를 실제로 실행할 provider 계정"을 알아야 한다. 계정 소유와 활성 계정 선택은 프로세스 평면의 독점 표면(`orca account list`, research.md Feature Coverage Map)이다. omp의 어휘는 role -> `provider/model:thinking` 10종(`default/plan/slow/smol/designer/vision/commit/tiny/task/advisor`, F5)이며 계정 축이 아예 없다. 계정 축을 omp에 도입하는 것은 프로세스 평면 표면의 **평면 침범**이고, 동시에 정책 평면이 이미 소유한 티어 판정 책임을 모델 평면으로 옮기는 이중 이관이 된다. 기각한다.

### 배치 지점 — handoff 직전, 부작용 이전

`--execution-owner orca` 경로는 현재 실행이 없다. `internal/cli/pipeline_run_owner.go:160-164`은 소유자를 기록하고 반환하며 끝난다.

```go
result := pipelineExecutionOwnerResult{
    Schema: pipelineExecutionOwnerResultSchema, Status: "handoff_required",
    ...
    RequiredAction: "orca skills get orchestration --full",
}
```

이 지점은 워크트리·Run·워커·provider 세션이 아직 하나도 만들어지지 않은 상태다. 따라서 게이트를 이 반환 직전에 두면 REQ-007(부작용 이전)과 REQ-008(handoff 직전)이 같은 한 지점에서 동시에 충족되며, INV-005(부작용 없음)는 배치의 부산물로 보장된다. 별도의 롤백 경로를 설계할 필요가 없다.

세 평면 합성이 기존 불변식과 충돌하지 않는다는 점도 확인됐다. `pkg/adapter/omp/omp_workflow_render.go:99`의 금지 대상은 OMP **task DAG**이지 omp 실행 자체가 아니다(F7). orca Run이 소유한 워커 안에서 omp가 모델 평면 역할만 수행하고 자기 DAG를 만들지 않으면 INV-002를 준수한다.

### 왜 이 SPEC은 설계만 고정하고 구현을 분리하는가

research.md Completion Debt 3건이 **미해결인 채로는 구현 선택이 정해지지 않는다.** 세 건 모두 계약이 아니라 구현 형태를 바꾸는 질문이다.

| Completion Debt | 미해결이면 바뀌는 구현 선택 |
| --- | --- |
| 계정 정보의 정확한 소스 (`orca account list`는 사람이 읽는 텍스트, `--json` 유무 미확인) | 계정 조회를 구조화 출력 파싱으로 할지 `orca agent-context --json` 스키마 경유로 할지 |
| Claude 쪽 카탈로그 프로브 부재 (Codex는 `codex debug models`, Claude 등가물 미확인) | Claude 티어를 어떤 신호로 검증할지, 아니면 REQ-009의 `unverified` 경로로 보낼지 |
| 원격 orca 환경(`--on <saved-environment>`)의 계정 조회 | 계정 조회가 로컬 단일 호출인지 원격 host 축을 갖는지 |

계약(요구 9건 · 불변식 5건 · 영수증 6필드)은 세 질문의 답과 무관하게 고정된다. 반대로 구현은 답에 따라 달라진다. 그래서 계약을 먼저 얼리고 구현을 분리했다. 이 분리는 spec.md `## Out of Scope` 첫 항목("정합 게이트의 구현 코드")과 일치한다.

## File Impact Analysis

이 SPEC은 설계 고정 문서이므로 소스 파일을 변경하지 않는다. 산출물은 SPEC 디렉터리의 4개 문서뿐이다.

| 파일 | 작업 | 설명 |
|------|------|------|
| `.autopus/specs/SPEC-EXECPLANE-001/spec.md` | 생성 | 요구 9건, Outcome Boundary, Traceability Matrix, Out of Scope |
| `.autopus/specs/SPEC-EXECPLANE-001/research.md` | 생성 | 실측 근거 F1~F7, 불변식 5건, Completion Debt 3건, Reference Discipline |
| `.autopus/specs/SPEC-EXECPLANE-001/plan.md` | 생성 | 이 문서. 게이트 배치 논증과 T1~T4 설계 태스크 |
| `.autopus/specs/SPEC-EXECPLANE-001/acceptance.md` | 생성 | S1~S8 시나리오와 Oracle Acceptance Notes |

아래 코드는 **참조 대상**이며 이 SPEC에서 수정하지 않는다: `pkg/codexruntime/probe.go`, `internal/cli/codex_catalog_runtime.go`, `internal/cli/pipeline_run_owner.go`, `pkg/adapter/omp/omp_workflow_render.go`, `pkg/config/role_model_policy_matrix.go`.

## Architecture Considerations

- **의존 방향**: 정책 -> 모델, 정책 -> 프로세스 단방향만 허용한다. 모델 -> 프로세스, 프로세스 -> 정책 방향의 참조를 만들면 INV-001이 깨진다.
- **경계 통과 값의 형태**: 정책 -> 프로세스 경계를 넘는 것은 opaque provider model id와 effort뿐이다(F4의 `worker-start` 계약과 동일한 형태). 티어 문자열은 경계에서 소멸한다.
- **J1 불변**: PR #154가 닫은 `quality.default` -> 내장 role-model 프로파일 -> omp `modelRoles` 경로는 이 SPEC에서 손대지 않는다. J2는 J1 결과를 입력으로 받는 후단 검증이다.
- **기존 패턴 재사용**: 영수증은 이미 존재하는 `pipeline_execution_owner_receipt.v1`(INV-002 관측 지점)과 같은 스키마-버전 방식을 따른다. 새 관측 메커니즘을 발명하지 않는다.

## Visual Planning Brief

J2 접합면의 런타임 흐름. research.md의 세 평면 정적 다이어그램과 달리 여기서는 **한 워크로드가 준비되는 동안의 시간 순서**를 그린다.

```
  [정책 평면: autopus-adk]                            부작용 경계 (INV-005)
                                          =====================================
  요청 티어 (quality.default)                 이 선 위: 리소스 생성 0건
        |                                     이 선 아래: 워크트리·Run·워커·세션
        | J1 (PR #154, 불변)
        v
  role -> provider/model:thinking  <----[모델 평면: omp modelRoles]
        |
        |  (여기부터 J2 — 이 SPEC의 대상)
        v
  +-------------------------------------------------+
  | (1) 실행 계정 조회            REQ-003 / INV-003 |
  |     프로세스 평면에 질의                        |
  |     "이 워크로드를 실제로 실행할 계정은?"       |
  |     ! 로컬 CLI 로그인 계정으로 가정하지 않는다  |
  |       F1: orca active = bitgapnam@gmail.com     |
  |           local codex = jroad1049@gmail.com     |
  +-------------------------------------------------+
        | account_id
        v
  +-------------------------------------------------+
  | (2) 그 계정의 카탈로그 확보    REQ-004 / INV-003|
  |     catalog_source := account_id 기준           |
  |     ! F2: PATH 바이너리 프로브는 로컬 로그인    |
  |       계정 기준 -> 실행 계정과 다르면 증거 아님 |
  +--------------------+----------------------------+
                       |
          +------------+------------+
          | 프로브 있음            | 프로브 없음
          v                        v
  (3a) 해석 / 검증          (3b) unverified 표기
       요청 티어 in 카탈로그?      REQ-009 / INV-004
       INV-003                     사유 기록, verified 금지
          |                        |
          +------------+-----------+
                       v
  +-------------------------------------------------+
  | (4) 정합 영수증 방출          REQ-005 / INV-004 |
  |   요청 티어 | 해석된 provider 모델              |
  |   실행 계정 식별자 | 카탈로그 출처              |
  |   resolution reason | verification status       |
  |   (+ schema version)                            |
  +--------------------+----------------------------+
                       |
        +--------------+---------------+
        | 통과 / 명시적 강등            | 불일치·검증 실패
        v                              v
  (5) 실행 위임                  (6) fail-loud
      REQ-008: handoff 직전          REQ-006 / INV-004
      pipeline_run_owner.go:160-164  요청 티어 + 실제 제공 모델
      handoff 결과가 영수증 참조     두 값 + 사유를 모두 지목
        |                              |
        | REQ-007: 여기까지 부작용 0건  | handoff 결과 대신
  ======v=============================v==== 부작용 경계 ====
        |                             (X) 종료. 리소스 0건
        v                                  INV-005 충족
  [프로세스 평면: orca]
   worktree 생성 -> Run -> 워커 -> provider 세션
   받는 값: opaque model id + effort 만 (REQ-002 / INV-001)
   티어 어휘는 경계에서 소멸
```

읽는 법 세 가지.

1. **부작용 경계**는 (5)와 프로세스 평면 사이에 정확히 한 번 그어진다. (1)~(4)와 (6)은 전부 경계 위에 있으므로 실패 경로에 롤백이 필요 없다 — REQ-007이 배치로 충족된다.
2. **handoff 지점**은 (5)다. `internal/cli/pipeline_run_owner.go:160-164`이 `handoff_required`를 반환하기 직전이며, 게이트가 그 앞에 있으므로 handoff를 받는 쪽은 이미 검증된 티어 계약을 들고 출발한다(REQ-008).
3. **(1)과 (2)의 계정이 반드시 같아야 한다.** 두 값이 갈라지면 (3a)의 판정은 성립하지 않은 것으로 처리된다(REQ-004). F1·F2가 이 갈라짐이 이 워크스테이션에 이미 존재함을 보여준다.

## Feature Completion Scope

이 SPEC은 **설계 고정 문서**다. 완료 판정에 실행 가능한 코드나 통과하는 테스트를 요구하지 않는다. Outcome Lock을 닫는 것은 "정합 게이트가 동작한다"가 아니라 "정합 게이트의 계약과 배치가 더 이상 해석의 여지 없이 고정됐다"다.

완료 조건은 spec.md `## Outcome Boundary`의 Completion evidence와 동일하게 셋이다.

1. 4개 SPEC 문서(`spec.md`·`plan.md`·`acceptance.md`·`research.md`)가 `auto spec validate`를 통과한다.
2. research.md Completion Debt 3건이 해소되거나 후속 SPEC으로 이관된다. 3건 중 2건(계정 조회 소스, Claude 카탈로그 부재)은 T2가 해소 대상으로 안고 간다.
3. 세 평면 경계에 대한 리뷰 합의가 기록된다 — research.md `## Reviewer Brief`의 4개 질문에 대한 판단이 남아야 한다.

| 구분 | 항목 |
| --- | --- |
| **범위 안** | 세 평면 소유 표면의 배타 선언 (REQ-001) |
| | 정책 -> 프로세스 경계의 티어 어휘 금지 목록 (REQ-002) |
| | 실행 계정 조회의 인터페이스 계약 (REQ-003) |
| | 실행 계정 기준 카탈로그 검증의 인터페이스 계약 (REQ-004) |
| | 정합 영수증 6필드 스키마와 스키마 버전 (REQ-005) |
| | fail-loud 판정 규칙과 강등 기록 규칙 (REQ-006) |
| | 게이트 배치 지점과 부작용 경계의 확정 (REQ-007, REQ-008) |
| | 검증 불가 provider의 `unverified` 표기 규칙 (REQ-009) |
| | S1~S8 시나리오와 그 관측 지점 |
| **범위 밖** | 정합 게이트의 구현 코드. 이 SPEC은 계약만 고정한다 |
| | `--execution-owner orca`의 handoff 스텁을 실제 orca Run 생성으로 대체하는 작업 |
| | 요청 티어를 제공 가능한 계정을 자동 선택하는 다계정 라우팅 |
| | 티어 강등이 불가피할 때 다른 provider로 넘기는 폴백 정책 |
| | autopus·omp·orca 관측 데이터를 한 영수증으로 합치는 실행 원장 |
| | J1(티어 -> omp `modelRoles`) 재설계. PR #154 결과를 불변으로 둔다 |
| | orca CLI 표면 변경. orca는 소비자로만 취급한다 |

범위 밖 목록은 spec.md `## Out of Scope`와 항목 대응이 1:1이다. 새 항목을 추가하지 않았다.

## Tasks

T1~T4는 모두 **설계·문서·합의 산출물**이다. 코드 작성 태스크는 없다 — 구현은 위 Feature Completion Scope의 범위 밖이다.

- [ ] **T1 — 세 평면 소유 표면 배타 선언과 경계 어휘 금지 목록 확정** (REQ-001, REQ-002 / INV-001 / S1, S2)
  - 산출물: research.md Feature Coverage Map의 각 표면에 소유 평면을 정확히 하나 지정한 확정판, 그리고 정책 -> 프로세스 경계에서 금지되는 티어 토큰 목록(`balanced`/`ultra`/`opus`/`sonnet`/`haiku`)과 허용되는 통과 값(opaque provider model id, effort)의 명시.
  - 완료 판정: 같은 어휘가 둘 이상의 평면에 나타나지 않고(S1), 경계 통과 값 정의에 금지 토큰이 하나도 포함되지 않음이 문서로 확인된다(S2). F4의 orca 티어 어휘 0건 실측이 기준선이다.

- [ ] **T2 — 실행 계정 조회와 계정 기준 카탈로그 검증의 인터페이스 계약 확정** (REQ-003, REQ-004 / INV-003 / S3)
  - 산출물: (i) 실행 계정 식별자를 프로세스 평면에서 얻는 조회의 입력·출력·실패 모드 계약, (ii) 그 계정 식별자를 기준으로 카탈로그를 확보하는 계약과 "카탈로그 출처 계정 ≠ 실행 계정이면 검증 미성립"이라는 판정 규칙.
  - **이 태스크가 research.md Completion Debt 2건을 해소해야 한다**: ① orca account 조회 소스 — `orca account list`는 사람이 읽는 텍스트이므로 `--json` 유무를 확인하고, 없으면 `orca agent-context --json` 스키마로 대체 가능한지 확정한다. ② Claude 카탈로그 부재 — Codex는 `codex debug models`가 있으나 Claude 등가물이 확인되지 않았으므로, Claude 티어를 어떤 신호로 검증할지 또는 REQ-009의 `unverified` 경로로 보낼지 결정한다. 남은 1건(원격 `--on <saved-environment>` 계정 조회)은 해소하거나 후속 SPEC으로 이관 기록한다.
  - 완료 판정: 두 계약이 파라미터 수준으로 적혀 있고, S3의 분기 계정 픽스처(orca 활성 `bitgapnam@gmail.com` vs 로컬 codex `jroad1049@gmail.com`, F1)에 대해 판정 결과가 유일하게 결정된다. Completion Debt 2건이 체크 해제 상태로 남아 있지 않다.

- [ ] **T3 — 정합 영수증 스키마 확정** (REQ-005, REQ-006, REQ-009 / INV-004 / S4, S5, S8)
  - 산출물: 6필드(요청 티어 / 해석된 provider 모델 / 실행 계정 식별자 / 카탈로그 출처 / resolution reason / verification status) 각각의 타입·필수 여부·허용값을 담은 스키마 정의와 스키마 버전 이름. `verification status`의 허용값에 `unverified`가 포함되고 그 경우 사유 필드가 필수임을 규정한다. 명시적 강등 시 요청 값과 실제 값이 **둘 다** 남아야 한다는 제약도 스키마에 표현한다.
  - 완료 판정: S4에서 6필드와 스키마 버전이 모두 존재하고, S5에서 강등 경로의 `reason`이 비어 있을 수 없으며 요청·실제 두 값이 모두 요구되고, S8에서 프로브 없는 provider가 `verified`로 표기될 수 없음이 스키마만 보고 판정된다. 기존 `pipeline_execution_owner_receipt.v1`과 같은 스키마-버전 방식을 따른다.

- [ ] **T4 — 게이트 배치 지점과 부작용 경계 확정** (REQ-007, REQ-008 / INV-002, INV-005 / S6, S7)
  - 산출물: 게이트가 실행되는 지점을 `internal/cli/pipeline_run_owner.go:160-164`의 `handoff_required` 반환 직전으로 확정한 배치 결정문, 그 지점 이전에 생성되지 않아야 하는 리소스 목록(워크트리·Run·워커·provider 세션), 그리고 점검 실패 시 handoff 결과 대신 정합 실패를 반환한다는 반환값 규칙. INV-002와의 양립 근거(F7 — 금지 대상은 OMP task DAG이지 omp 실행 자체가 아님, `pkg/adapter/omp/omp_workflow_render.go:99`)를 함께 기록한다.
  - 완료 판정: S6에서 점검 실패 시 생성된 워크트리·Run·세션이 0건이라는 관측이 배치만으로 도출되고, S7에서 handoff 결과가 영수증을 참조하며 실패 시 정합 실패로 대체됨이 확정된다. 롤백 경로를 추가로 설계할 필요가 없음이 논증돼 있다.

REQ 커버리지: REQ-001·002(T1), REQ-003·004(T2), REQ-005·006·009(T3), REQ-007·008(T4) — 9건 전부 덮이며 spec.md `## Traceability Matrix`와 일치한다.

## Risks & Mitigations

| 리스크 | 영향도 | 대응 |
|--------|--------|------|
| Completion Debt ①(계정 조회 소스)이 T2에서 해소되지 않으면 REQ-003의 계약이 추상 수준에 머문다 | 높음 | T2의 완료 판정에 명시적으로 묶었다. 해소 불가로 판단되면 후속 SPEC 이관을 기록하고 REQ-003 계약을 "조회 결과의 형태"까지만 고정한다 |
| Claude 카탈로그 등가물이 없어 Claude 티어가 전부 `unverified`로 떨어진다 | 중간 | 이는 결함이 아니라 REQ-009가 설계한 정직한 상태다. 은폐 대신 표기하는 것이 INV-004의 요지다. 대안 신호 탐색은 T2의 판단 사항 |
| 원격 orca 환경(`--on <saved-environment>`)에서 계정 축이 하나 더 늘어난다 | 중간 | Completion Debt ③으로 분리되어 있다. 계약을 계정 식별자 기준으로 쓰면 조회 경로가 로컬이든 원격이든 판정 규칙은 불변이다 |
| 설계만 고정하고 구현을 분리한 결과, 후속 SPEC이 나오지 않으면 J2가 계속 끊긴 채 남는다 | 중간 | 구현 분리는 spec.md `## Out of Scope`에 명시된 결정이다. 완료 판정 2번(Completion Debt 해소 또는 후속 SPEC 이관)이 이관 기록을 강제한다 |
| 게이트를 정책 평면에 두면 정책 평면이 프로세스 평면 상태를 읽는 결합이 생긴다 | 낮음 | 방향이 기존 의존 방향(정책 -> 프로세스)과 같고, 읽는 값은 계정 식별자 하나다. 역방향 참조는 만들지 않는다 |

## Dependencies

- **PR #154** — J1(`quality.default` -> 내장 role-model 프로파일 -> omp `modelRoles`)이 닫혀 있음. 이 SPEC은 J1 결과를 입력으로 받는다. 불변으로 취급한다.
- **PR #152** — 같은 세대 폴백 추가로 F3의 조용한 세대 강등이 해소됨. INV-004의 출처 사례.
- **orca CLI** — 계정 조회의 사실상 유일한 소스(`account list`). 표면 변경은 요구하지 않고 소비자로만 취급한다(spec.md `## Out of Scope`).
- **`codex debug models`** — Codex 계정별 카탈로그의 유일한 확인된 프로브(`pkg/codexruntime/probe.go:39`).
- **`auto spec validate`** — 완료 판정 1번의 검증 수단.

새 외부 라이브러리 의존은 없다. 설계 문서이므로 코드 의존도 추가하지 않는다.

## Exit Criteria

- [ ] `auto spec validate .autopus/specs/SPEC-EXECPLANE-001`이 4개 문서에 대해 통과한다
- [ ] T1~T4의 산출물이 모두 존재하고 REQ 9건을 빠짐없이 덮는다
- [ ] research.md Completion Debt 3건이 각각 해소 또는 후속 SPEC 이관으로 처리됐다
- [ ] research.md `## Reviewer Brief`의 4개 질문에 대한 리뷰 판단이 기록됐다
- [ ] 범위 밖 항목 목록이 spec.md `## Out of Scope`와 모순되지 않는다

구현 증거·테스트 통과·커버리지 수치는 이 SPEC의 종료 조건이 아니다. 설계 고정 문서이며, 구현은 별도 SPEC의 대상이다.
