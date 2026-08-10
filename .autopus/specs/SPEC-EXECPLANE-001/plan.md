# SPEC-EXECPLANE-001 Plan: 실행 평면 분리와 티어·계정·카탈로그 정합

**Created**: 2026-08-10
**Domain**: EXECPLANE
**Kind**: 설계 고정(design-only). 구현 코드는 이 SPEC의 산출물이 아니다.

## Implementation Strategy

### 게이트는 정책 평면(autopus-adk)에 둔다

정합 게이트가 알아야 하는 값은 셋이다 — **요청 티어**(정책 평면 소유), **실행 계정과 그 자격 등급**(프로세스 평면 소유), **그 자격으로 얻은 카탈로그**(모델 평면 어휘). 세 값이 한자리에 모여야 판정이 성립하므로, 게이트를 어디에 두느냐는 "어느 평면이 남의 어휘를 읽어도 되는가"의 문제로 환원된다.

정책 평면만 이 조건을 위반 없이 만족한다. 티어 어휘의 단일 출처가 이미 정책 평면(`pkg/config/quality_tier.go`, research.md Feature Coverage Map)이고, J1(티어 -> omp `modelRoles`)의 투영 지점도 정책 평면(`pkg/adapter/omp/omp_model_projection.go:59-107`)이다. 정책 평면이 프로세스 평면에서 계정 식별자를, 모델 평면에서 카탈로그를 **읽어 오는** 방향은 이미 존재하는 의존 방향(정책 -> 모델, 정책 -> 프로세스)과 같다. 새 역방향 의존이 생기지 않는다.

### 위험 조건은 자격 등급 차이다 — 기본 경로에 네트워크 호출이 없다

착수 시점의 서술은 "검증 계정과 실행 계정이 다르면 판정이 성립하지 않는다"였다. 계정 분기 자체를 위험으로 본 것이다. F9의 실측이 이 조건을 좁혔다. `codex debug models`를 세 번 프로브했다 — 임시 `CODEX_HOME`에 각 계정의 `auth.json`만 복사해 orca 관리 홈을 건드리지 않았다. 결과는 두 갈래다.

- **자격 종속성은 참이다.** 인증을 빼면 슬러그 집합이 달라진다. 인증 프로브에만 `gpt-5.6-sol-wm`과 `gpt-5.3-codex-spark`가 있고, 미인증 프로브에는 대신 `gpt-5.2`가 있다. 서버가 호출자의 자격을 보고 목록을 바꾼다.
- **그러나 계정 신원 자체는 판정을 가르지 않았다.** `bitgapnam@gmail.com`과 `gnkong@alipeople.kr`은 슬러그 집합도 `supported_reasoning_levels`도 완전히 동일했다. 354131 bytes 페이로드에서 실제로 갈리는 키는 `base_instructions`와 `model_messages` 둘뿐이고, 둘 다 `ResolveCodexProfile`이 읽지 않는 시스템 프롬프트 텍스트다. 두 계정이 같은 `chatgpt_plan_type: "pro"`이고 다른 것은 조직뿐이기 때문이다.

따라서 게이트가 막아야 하는 것은 **자격 등급 차이**이고, 조직만 다른 계정 쌍은 무해하다. 이 좁힘이 설계를 바꾼다. `chatgpt_plan_type`은 `auth.json` id_token의 `https://api.openai.com/auth` 클레임이므로 파일 읽기로 얻어진다. 게이트의 기본 경로가 카탈로그 재프로브에서 **자격 등급 비교**로 내려가고, 그 경로의 **네트워크 호출은 0회**다. 재프로브는 상시 비용이 아니라 등급이 갈릴 때만 치르는 예외 비용이 된다(REQ-004).

이것이 기각한 대안 (c)와 충돌하지 않는 이유는 하는 일이 매핑이 아니라 비교이기 때문이다. 두 등급이 같은지만 보고 등급에서 모델을 유추하지 않으므로 하드코딩 매핑 표가 생기지 않는다. (c)를 기각한 근거 — 플랜 -> 모델 표는 증거가 아니라 우리 쪽 추정이다 — 는 그대로 유효하다.

남은 미검증은 하나다. 자격 등급이 **다른** 계정 쌍이 이 워크스테이션에 없어, 등급이 갈릴 때 판정 필드가 실제로 어떻게 달라지는지는 확인하지 못했다. 미인증 프로브가 자격 종속성 자체를 증명하므로 계약은 성립하지만, 등급 불일치의 구체적 형태는 관측이 아니라 추론이다.

### 기각한 대안 (a) — orca(프로세스 평면)에 둔다

orca가 게이트를 소유하면 orca가 "요청 티어"를 이해해야 한다. 그런데 F4의 전수 검색 결과 `orca agent-context --json`(223 command, schema v1)에 `quality` 0건, `tier` 0건, `opus`/`sonnet`/`gpt-5`/`balanced`/`ultra` 0건이고, 모델·effort를 받는 명령은 `orchestration worker-start` 하나뿐이며 그 계약은 다음과 같다.

```
--model supports Claude, Codex, and Cursor opaque provider model ids;
--effort requires --model. Neither can combine with --terminal.
```

즉 orca는 설계상 opaque provider model id 소비자다. 여기에 티어 사다리를 심으면 티어 어휘가 두 평면에 동시에 존재하게 되어 **INV-001(어휘 무증식)을 직접 위반**하고, REQ-002가 금지하는 바로 그 상태가 된다. 게이트를 orca에 두는 선택은 이 SPEC의 요구 두 건과 정면으로 충돌하므로 기각한다.

### 기각한 대안 (b) — omp(모델 평면)에 둔다

omp가 게이트를 소유하면 omp가 "이 워크로드를 실제로 실행할 provider 계정"을 알아야 한다. 계정 소유와 활성 계정 선택은 프로세스 평면의 독점 표면(`orca account list`, research.md Feature Coverage Map)이다. omp의 어휘는 role -> `provider/model:thinking` 10종(`default/plan/slow/smol/designer/vision/commit/tiny/task/advisor`, F5)이며 계정 축이 아예 없다. 계정 축을 omp에 도입하는 것은 프로세스 평면 표면의 **평면 침범**이고, 동시에 정책 평면이 이미 소유한 티어 판정 책임을 모델 평면으로 옮기는 이중 이관이 된다. 기각한다.

### 기각한 대안 (c) — Claude 검증을 `subscriptionType` 플랜 게이트로 한다

Claude에는 `codex debug models` 등가물이 없다. F8의 확인 결과 `claude` CLI 명령 집합은 `agents`/`auth`/`auto-mode`/`doctor`/`gateway`/`import`/`install`/`mcp`/`plugin`/`project`/`setup-token`/`ultrareview`이고 모델 열거 명령이 없으며, `omp models --json`은 계정 스코프가 아닌 **정적 레지스트리**라 특정 계정의 자격을 증명하지 못한다. 대신 `claude auth status --json`이 신원과 플랜을 준다.

```json
{"loggedIn":true,"authMethod":"claude.ai","apiProvider":"firstParty",
 "email":"jroad1049@gmail.com","orgId":"0e61c3e2-...","subscriptionType":"max"}
```

여기서 `subscriptionType`(`max`/`pro`/`free`)을 게이트 입력으로 삼아 "이 플랜이면 이 티어까지 허용"으로 판정하자는 안이 있었다. 기각한다. 판정이 성립하려면 플랜 -> 모델 매핑 표를 정책 평면에 하드코딩해야 하는데, 그 표는 계정이 실제로 무엇을 제공받는지에 대한 증거가 아니라 **우리 쪽의 추정**이다. provider가 플랜 정책을 바꾸면 표는 조용히 틀리고, 게이트는 틀린 표를 근거로 `verified`를 찍는다 — REQ-009가 금지하는 상태이자 INV-004가 막으려는 조용한 어긋남이다.

형태로 보면 이 안은 PR #154가 제거한 하드코딩 티어 테이블을 provider 축에 되살리는 일이다. 같은 것을 다른 이름으로 되돌리지 않는다. Claude는 `orgId` 신원 대조까지만 검증하고 모델 가용성은 REQ-009의 `unverified`로 남긴다. 모르는 것을 모른다고 적는 편이 짐작해서 맞히는 것보다 싸다.

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

착수 시점의 이유는 "미결 3건이 열려 있어 구현 선택이 정해지지 않는다"였다. 그 이유는 더 이상 유효하지 않다. research.md F8이 3건을 모두 실측으로 닫았고, 결정은 `### 결정` 표에, 계약은 spec.md REQ-003·REQ-004·REQ-009 본문에 들어갔다. `## Completion Debt`는 `none remaining`이다.

| 닫힌 질문 | 확정된 결정 | 계약이 앉는 자리 |
| --- | --- | --- |
| 계정 조회 소스 | `orca account list --json` 한 호출. 실행 계정은 `result.<provider>.activeAccountId`, 비교 대상 프로브 계정은 `result.codex.systemDefault`(codex) 또는 provider CLI 신원 | REQ-003 / T2 |
| Claude 카탈로그 부재 | 신원만 검증. `claude auth status --json`의 `orgId`를 orca `claude.accounts[].organizationUuid`와 대조하고, 모델 가용성은 `unverified`로 남긴다 | REQ-004 / T2, REQ-009 / T3 |
| 원격 orca 환경 계정 조회 | 새 요구를 만들지 않고 REQ-009에 흡수. `orca account list`가 `--environment`를 구조적으로 거부하므로 "계정 조회 수단 없음"의 인스턴스다 | REQ-009 / T3 |

그렇다면 왜 여전히 구현을 분리하는가. 이유가 바뀌었기 때문이다 — "질문이 열려 있어서"가 아니라 **계약이 먼저 합의되어야 구현이 바로잡힐 수 있어서**다.

먼저 이 자리에서 F3을 사례로 쓰지 않는다는 점을 분명히 해 둔다. PR #151의 세대 강등(`gpt-5.6-terra/medium` -> `gpt-5.5/xhigh`)은 **한 계정 안에서** 카탈로그 폴백 로직이 만든 버그였고, PR #152가 같은 세대 폴백을 추가해 고쳤다. 이 SPEC이 다루는 계정·자격 불일치로 인한 손해는 **아직 관측된 적이 없다**(F9). 두 실패 양식을 인과로 엮으면 근거가 없는 곳에 근거가 있는 척하게 된다.

F3이 주는 것은 사례가 아니라 **비용의 크기**다. #151은 측정 근거(sol 42 vs sonnet-5 36)가 5.5 대 5.6-terra에 대해 아무 말도 하지 않는데도 조용히 통과했다. 티어 판정이 틀리면 조용히 틀리고, 조용하면 근거와 결과가 어긋난 채로 결론까지 간다. 그 비용이 INV-004(fail-loud)의 출처다.

계약을 먼저 고정하는 이유는 따로 있다. 암묵 가정은 코드가 공백을 만나는 자리마다 새로 생기고, 이 접합면에서는 후보가 이미 셋 나왔다 — "로컬 CLI 로그인이 곧 실행 계정"(F1·F2가 반증), "플랜을 알면 모델을 안다"(기각한 대안 (c)), 그리고 새로 좁혀진 축에서 "계정이 다르면 무조건 재프로브해야 한다"(F9가 반증). 셋 다 실제로 검토됐고, 실측이 없었다면 셋 다 그럴듯했다. 계약 없이 구현부터 손대면 같은 종류의 가정이 다른 자리에 다시 자리잡는다.

분리가 남기는 부채도 작아졌다. 요구 9건·불변식 5건·영수증 6필드는 이제 열린 질문에 기대지 않고 단독으로 판정 가능하므로, 후속 구현 SPEC이 발명할 것은 없고 이 문서를 참조 구현하면 된다. 이 분리는 spec.md `## Out of Scope` 첫 항목("정합 게이트의 구현 코드")과 일치한다.

## File Impact Analysis

이 SPEC은 설계 고정 문서이므로 소스 파일을 변경하지 않는다. 산출물은 SPEC 디렉터리의 4개 문서뿐이다.

| 파일 | 작업 | 설명 |
|------|------|------|
| `.autopus/specs/SPEC-EXECPLANE-001/spec.md` | 생성 | 요구 9건, Outcome Boundary, Traceability Matrix, Out of Scope |
| `.autopus/specs/SPEC-EXECPLANE-001/research.md` | 생성 | 실측 근거 F1~F9, 불변식 5건, 해소된 결정 표(Completion Debt `none remaining`), Reference Discipline |
| `.autopus/specs/SPEC-EXECPLANE-001/plan.md` | 생성 | 이 문서. 게이트 배치 논증과 T1~T4 설계 태스크 |
| `.autopus/specs/SPEC-EXECPLANE-001/acceptance.md` | 생성 | S1~S10 시나리오와 Oracle Acceptance Notes |

아래 코드는 **참조 대상**이며 이 SPEC에서 수정하지 않는다: `pkg/codexruntime/probe.go`, `internal/cli/codex_catalog_runtime.go`, `internal/cli/pipeline_run_owner.go`, `pkg/adapter/omp/omp_workflow_render.go`, `pkg/config/role_model_policy_matrix.go`.

## Architecture Considerations

- **의존 방향**: 정책 -> 모델, 정책 -> 프로세스 단방향만 허용한다. 모델 -> 프로세스, 프로세스 -> 정책 방향의 참조를 만들면 INV-001이 깨진다.
- **경계 통과 값의 형태**: 정책 -> 프로세스 경계를 넘는 것은 opaque provider model id와 effort뿐이다(F4의 `worker-start` 계약과 동일한 형태). 티어 문자열은 경계에서 소멸한다.
- **게이트가 읽어 오는 값의 형태**: 정책 평면이 프로세스 평면에서 가져오는 것은 계정 식별자와 그 계정의 자격 등급 둘뿐이고, 둘 다 로컬 상태 조회(`orca account list --json`, `auth.json` id_token 클레임)로 얻어진다. 기본 경로에 provider 네트워크 호출이 없고, 카탈로그 프로브는 두 등급이 갈릴 때만 발생하는 예외 비용이다(REQ-004).
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
  |     해석: active -> 단일 등록 -> unverified     |
  |     ! 로컬 CLI 로그인 계정으로 가정하지 않는다  |
  |       F1: orca active  = bitgapnam@gmail.com    |
  |           local codex  = gnkong@alipeople.kr    |
  |           local claude = jroad1049@gmail.com    |
  +-------------------------------------------------+
        | account_id + 자격 등급 (또는 미특정)
        v
  +-------------------------------------------------+
  | (2) 자격 등급 비교            REQ-004 / INV-003 |
  |     P_exec == P_probe ?                         |
  |     (chatgpt_plan_type, 네트워크 0회)           |
  |       같음 -> 기존 카탈로그 판정 신뢰           |
  |       다름 -> 실행 계정 기준 재프로브           |
  |       불가 -> (3b) unverified                   |
  |     P := auth.json id_token 클레임              |
  |          https://api.openai.com/auth            |
  |     Claude: 물려받을 카탈로그 없음              |
  |             -> orgId 신원 대조까지만            |
  |     ! F9: 조직만 다른 두 pro 계정은 판정 필드   |
  |       (슬러그·reasoning levels) 전부 동일       |
  +--------------------+----------------------------+
                       |
          +------------+------------+
          | 등급 동일 · 재프로브   | 등급 비교 불가 · 카탈로그 부재
          |   성공 (Codex 축)      |   (Claude 가용성 · 원격 · 미특정)
          v                        v
  (3a) 해석 / 검증          (3b) unverified 표기
       요청 티어 in 카탈로그?      REQ-009 / INV-004
       출처 자격 == 실행 자격      사유 기록, verified 금지
       INV-003                     신원만 대조, 가용성 미검증
          |                        |
          +------------+-----------+
                       v
  +-------------------------------------------------+
  | (4) 정합 영수증 방출          REQ-005 / INV-004 |
  |   요청 티어 | 해석된 provider 모델              |
  |   실행 계정 식별자 | 카탈로그 출처(+자격 등급)  |
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

읽는 법 다섯 가지.

1. **부작용 경계**는 (5)와 프로세스 평면 사이에 정확히 한 번 그어진다. (1)~(4)와 (6)은 전부 경계 위에 있으므로 실패 경로에 롤백이 필요 없다 — REQ-007이 배치로 충족된다.
2. **handoff 지점**은 (5)다. `internal/cli/pipeline_run_owner.go:160-164`이 `handoff_required`를 반환하기 직전이며, 게이트가 그 앞에 있으므로 handoff를 받는 쪽은 이미 검증된 티어 계약을 들고 출발한다(REQ-008).
3. **(2)의 기본 경로는 자격 등급 비교이고 네트워크 호출이 0회다.** 이 경로는 카탈로그를 다시 받지 않는다. `auth.json` id_token의 `chatgpt_plan_type` 두 값을 읽어 같은지 보는 것이 전부이며, 같으면 기존 카탈로그 판정을 그대로 증거로 인정한다. 카탈로그 재프로브는 등급이 갈릴 때만 발생하는 예외 경로다.
4. **판정을 무효화하는 것은 신원 차이가 아니라 자격 차이다.** (1)의 실행 계정과 (2)의 프로브 계정이 서로 다른 계정이어도 자격 등급이 같으면 (3a)의 판정은 성립한다. 이 워크스테이션이 정확히 그 경우다 — Codex 축에서 orca 활성 계정은 `bitgapnam@gmail.com`, PATH의 `codex`가 쓰는 계정은 `gnkong@alipeople.kr`로 신원은 갈라지지만(F1·F2), 둘 다 `pro`라 판정에 쓰이는 필드가 하나도 다르지 않다(F9). 등급이 다르거나 등급을 비교할 수 없을 때만 판정이 무너진다(REQ-004).
5. **(2)의 분기는 provider 속성이지 실행 시점의 운이 아니다.** Codex는 자격 클레임과 계정별 카탈로그 프로브를 둘 다 갖고 있으므로 등급 비교 -> (필요 시) 재프로브 -> (3a)로 간다. Claude는 물려받을 카탈로그 자체가 없어 등급이 같은지 따지는 것이 무의미하고, `orgId` 신원 대조를 통과하더라도 가용성 축에서는 항상 (3b)로 간다. (3b)에 모이는 세 인스턴스 — Claude 가용성, 원격 환경, 실행 계정 미특정 — 는 REQ-009 하나가 같은 규칙으로 다룬다.

## Feature Completion Scope

이 SPEC은 **설계 고정 문서**다. 완료 판정에 실행 가능한 코드나 통과하는 테스트를 요구하지 않는다. Outcome Lock을 닫는 것은 "정합 게이트가 동작한다"가 아니라 "정합 게이트의 계약과 배치가 더 이상 해석의 여지 없이 고정됐다"다.

완료 조건은 spec.md `## Outcome Boundary`의 Completion evidence와 동일하게 셋이다.

1. 4개 SPEC 문서(`spec.md`·`plan.md`·`acceptance.md`·`research.md`)가 `auto spec validate`를 통과한다.
2. research.md Completion Debt 3건이 해소되거나 후속 SPEC으로 이관된다. **이미 충족됐다** — 3건 모두 F8에서 실측으로 해소되어 `### 결정` 표에 남았고 `## Completion Debt`는 `none remaining`이다. 후속 SPEC 이관은 한 건도 발생하지 않았다. 남은 일은 이관 기록이 아니라 그 결정을 인터페이스 계약으로 고정하는 것이며, T2와 T3가 그 자리다.
3. 세 평면 경계에 대한 리뷰 합의가 기록된다 — research.md `## Reviewer Brief`의 4개 질문에 대한 판단이 남아야 한다.

| 구분 | 항목 |
| --- | --- |
| **범위 안** | 세 평면 소유 표면의 배타 선언 (REQ-001) |
| | 정책 -> 프로세스 경계의 티어 어휘 금지 목록 (REQ-002) |
| | 실행 계정 조회의 인터페이스 계약 (REQ-003) |
| | 자격 등급 동일성 검증과 불일치 시 재프로브의 인터페이스 계약 (REQ-004) |
| | 정합 영수증 6필드 스키마와 스키마 버전 (REQ-005) |
| | fail-loud 판정 규칙과 강등 기록 규칙 (REQ-006) |
| | 게이트 배치 지점과 부작용 경계의 확정 (REQ-007, REQ-008) |
| | 검증 불가 provider의 `unverified` 표기 규칙 (REQ-009) |
| | S1~S10 시나리오와 그 관측 지점 |
| **범위 밖** | 정합 게이트의 구현 코드. 이 SPEC은 계약만 고정한다 |
| | `--execution-owner orca`의 handoff 스텁을 실제 orca Run 생성으로 대체하는 작업 |
| | 요청 티어를 제공 가능한 계정을 자동 선택하는 다계정 라우팅 |
| | 티어 강등이 불가피할 때 다른 provider로 넘기는 폴백 정책 |
| | autopus·omp·orca 관측 데이터를 한 영수증으로 합치는 실행 원장 |
| | J1(티어 -> omp `modelRoles`) 재설계. PR #154 결과를 불변으로 둔다 |
| | orca CLI 표면 변경. orca는 소비자로만 취급한다 |

범위 밖 목록은 spec.md `## Out of Scope`와 항목 대응이 1:1이다. 새 항목을 추가하지 않았다.

관측 한계 하나를 제약으로 명시해 둔다. 자격 등급이 **다른** Codex 계정 쌍이 이 워크스테이션에 없어, 등급이 갈릴 때 판정 필드가 실제로 어떻게 달라지는지는 실측되지 않았다. 이것은 완료를 막지 않는다 — F9의 미인증 프로브가 자격 종속성 자체를 이미 증명했고, 계약이 요구하는 것은 "등급이 다르면 재프로브하거나 unverified로 표기한다"는 규칙이지 등급별 카탈로그 내용의 목록이 아니기 때문이다. 따라서 Completion Debt로 되돌리지 않고 관측 한계로 기록하며, research.md `## Completion Debt`는 `none remaining`으로 유지된다.

## Tasks

T1~T4는 모두 **설계·문서·합의 산출물**이다. 코드 작성 태스크는 없다 — 구현은 위 Feature Completion Scope의 범위 밖이다.

- [ ] **T1 — 세 평면 소유 표면 배타 선언과 경계 어휘 금지 목록 확정** (REQ-001, REQ-002 / INV-001 / S1, S2)
  - 산출물: research.md Feature Coverage Map의 각 표면에 소유 평면을 정확히 하나 지정한 확정판, 그리고 정책 -> 프로세스 경계에서 금지되는 티어 토큰 목록(`balanced`/`ultra`/`opus`/`sonnet`/`haiku`)과 허용되는 통과 값(opaque provider model id, effort)의 명시.
  - 완료 판정: 같은 어휘가 둘 이상의 평면에 나타나지 않고(S1), 경계 통과 값 정의에 금지 토큰이 하나도 포함되지 않음이 문서로 확인된다(S2). F4의 orca 티어 어휘 0건 실측이 기준선이다.

- [ ] **T2 — 실행 계정 조회와 자격 등급 기준 카탈로그 검증의 인터페이스 계약 확정** (REQ-003, REQ-004 / INV-003 / S3, S9, S10)
  - 산출물: (i) 실행 계정 식별자를 프로세스 평면에서 얻는 조회의 입력·출력·실패 모드 계약, (ii) 그 실행 계정의 자격 등급을 프로브 계정의 자격 등급과 대조하는 계약과 "두 등급이 같음을 확인하지 못하면 그 카탈로그는 증거가 아니다"라는 판정 규칙, 그리고 등급이 다를 때의 재프로브 경로와 재프로브도 불가할 때의 REQ-009 분기.
  - **이 태스크는 미결을 해소하는 것이 아니라 이미 해소된 결정을 계약으로 고정한다**(research.md F8 `### 결정`). 계약에 반드시 들어갈 세 가지:
    - **조회 소스** — `orca account list --json` 단일 호출. 실행 계정은 `result.<provider>.activeAccountId`, 비교 대상 프로브 계정은 `result.codex.systemDefault`(codex) 또는 provider CLI 신원(`claude auth status --json`의 `orgId`를 `claude.accounts[].organizationUuid`에 조인). 한 호출로 두 값을 모두 얻으므로 비교에 추가 조회가 필요 없다. `--environment`는 구조적으로 거부되므로 이 계약은 로컬 호스트 축만 갖는다.
    - **실행 계정 해석 규칙** — `activeAccountId`가 있으면 그 값, 없고 등록 계정이 **정확히 1개**면 그 계정, 그 외(0개 또는 2개 이상)는 특정하지 않고 REQ-009의 `unverified`로 분기. 세 단계가 이 우선순위 그대로 계약에 적혀야 하고, 마지막 단계에서 추측으로 하나를 고르는 경로는 금지된다.
    - **provider별 검증 깊이** — Codex는 **자격 등급 비교가 기본 경로**이고, 카탈로그 재프로브는 등급이 다를 때만 수행하는 예외 경로다. 자격 등급은 `auth.json` id_token의 `https://api.openai.com/auth.chatgpt_plan_type` 클레임에서 읽으며 네트워크 호출을 요구하지 않는다. 세 분기가 계약에 그대로 적혀야 한다 — 등급이 같으면 기존 카탈로그 판정을 증거로 인정하고, 다르면 실행 계정 기준으로 재프로브하며, 둘 다 불가하면 REQ-009의 `unverified`로 떨어진다. 이 비교는 동일성 판정이며 등급에서 모델을 유추하지 않는다는 단서도 함께 박는다(하드코딩 매핑 표 금지). Claude는 계정별 카탈로그 프로브가 없어 물려받을 판정 자체가 없으므로 등급 비교 경로를 두지 않고 `orgId` 신원 대조까지만 하며, 모델 가용성은 `unverified`로 남긴다. 검증 깊이가 provider의 고정 속성이지 실행 시점의 재량이 아님을 계약이 못 박는다.
  - 완료 판정: 세 요소가 모두 파라미터 수준으로 적혀 있고, 세 픽스처 각각에 대해 판정 결과가 유일하게 결정된다 — S3의 신원 분기·등급 동일 픽스처(orca 활성 Codex `bitgapnam@gmail.com` vs 로컬 codex CLI `gnkong@alipeople.kr`, 둘 다 `chatgpt_plan_type: "pro"`, F1·F9)에서는 재프로브 없이 판정이 성립하고, S10의 자격 종속성 픽스처에서는 자격이 다른 카탈로그의 재사용이 위반으로 판정되며, S9의 계정 미특정 픽스처에서는 unverified로 분기한다. research.md `## Completion Debt`가 `none remaining` 상태로 유지된다.

- [ ] **T3 — 정합 영수증 스키마 확정** (REQ-005, REQ-006, REQ-009 / INV-004 / S4, S5, S8, S9)
  - 산출물: 6필드(요청 티어 / 해석된 provider 모델 / 실행 계정 식별자 / 카탈로그 출처 / resolution reason / verification status) 각각의 타입·필수 여부·허용값을 담은 스키마 정의와 스키마 버전 이름. **카탈로그 출처 필드는 프로브 계정 식별자와 그 계정의 자격 등급을 함께 담는다** — 계정 식별자만으로는 INV-003("실행 계정과 같은 자격으로 얻은 카탈로그인가")을 나중에 되짚을 수 없기 때문이며, 따라서 등급 값이 빠진 출처는 스키마 위반이다. `verification status`의 허용값에 `unverified`가 포함되고 그 경우 사유 필드가 필수임을 규정한다. 명시적 강등 시 요청 값과 실제 값이 **둘 다** 남아야 한다는 제약도 스키마에 표현한다. 실행 계정 식별자와 자격 등급은 각각 미특정일 수 있으므로 그 표현(빈 값이 아니라 명시적 미특정 상태)도 스키마가 정의한다.
  - **REQ-009가 흡수하는 세 인스턴스를 같은 fail-loud 규칙으로 다뤄야 한다**: ① Claude 모델 가용성 — 계정별 카탈로그 프로브가 없다, ② 원격 orca 환경 — `orca account list`가 `--environment`를 거부해 원격 호스트의 계정을 조회할 수 없다, ③ 실행 계정 미특정 — 활성 계정이 없고 등록 계정이 0개이거나 2개 이상이다(REQ-003). 셋은 사유 문자열만 다르고 상태값(`unverified`)·사유 필수·`verified` 금지는 동일하다. 셋을 구분하는 별도 상태값이나 예외 경로를 만들지 않는다.
  - 완료 판정: S4에서 6필드와 스키마 버전이 모두 존재하고 카탈로그 출처가 프로브 계정과 그 자격 등급을 함께 담으며, S5에서 강등 경로의 `reason`이 비어 있을 수 없고 요청·실제 두 값이 모두 요구되며, S8에서 프로브 없는 provider가, S9에서 실행 계정이 미특정인 워크로드가 각각 `verified`로 표기될 수 없음이 스키마만 보고 판정된다. 기존 `pipeline_execution_owner_receipt.v1`과 같은 스키마-버전 방식을 따른다.

- [ ] **T4 — 게이트 배치 지점과 부작용 경계 확정** (REQ-007, REQ-008 / INV-002, INV-005 / S6, S7)
  - 산출물: 게이트가 실행되는 지점을 `internal/cli/pipeline_run_owner.go:160-164`의 `handoff_required` 반환 직전으로 확정한 배치 결정문, 그 지점 이전에 생성되지 않아야 하는 리소스 목록(워크트리·Run·워커·provider 세션), 그리고 점검 실패 시 handoff 결과 대신 정합 실패를 반환한다는 반환값 규칙. INV-002와의 양립 근거(F7 — 금지 대상은 OMP task DAG이지 omp 실행 자체가 아님, `pkg/adapter/omp/omp_workflow_render.go:99`)를 함께 기록한다.
  - 완료 판정: S6에서 점검 실패 시 생성된 워크트리·Run·세션이 0건이라는 관측이 배치만으로 도출되고, S7에서 handoff 결과가 영수증을 참조하며 실패 시 정합 실패로 대체됨이 확정된다. 롤백 경로를 추가로 설계할 필요가 없음이 논증돼 있다.

REQ 커버리지: REQ-001·002(T1), REQ-003·004(T2), REQ-005·006·009(T3), REQ-007·008(T4) — 9건 전부 덮이며 spec.md `## Traceability Matrix`와 일치한다.

## Risks & Mitigations

| 리스크 | 영향도 | 대응 |
|--------|--------|------|
| 해소된 결정 3건이 T2·T3의 계약 문장으로 옮겨지지 않고 research.md 결정 표에만 남는다 | 중간 | 리스크가 "미결"에서 "결정이 계약에 미반영"으로 이동했다. T2 완료 판정이 조회 소스·해석 규칙·검증 깊이 세 요소를 파라미터 수준으로 요구하고, T3 완료 판정이 REQ-009의 세 인스턴스를 요구한다 |
| Claude 카탈로그 등가물이 없어 Claude 티어의 모델 가용성이 전부 `unverified`로 떨어진다 | 중간 | 이는 결함이 아니라 REQ-009가 설계한 정직한 상태다. 은폐 대신 표기하는 것이 INV-004의 요지다. `subscriptionType` 플랜 게이트로 메우는 안은 기각한 대안 (c)로 기록됐다 |
| 원격 orca 환경(`--on <saved-environment>`)에서 계정 축이 하나 더 늘어난다 | 낮음 | REQ-009에 흡수됐다. `orca account list`가 `--environment`를 거부하므로 원격은 "조회 수단 없음"의 인스턴스이고, `orca environment list --json`이 `{"environments": []}`라 현재 영향도 0건이다 |
| 설계만 고정하고 구현을 분리한 결과, 후속 SPEC이 나오지 않으면 J2가 계속 끊긴 채 남는다 | 중간 | 구현 분리는 spec.md `## Out of Scope`에 명시된 결정이다. 결정이 모두 닫혔으므로 후속 SPEC은 발명이 아니라 이 문서의 참조 구현이며, 착수 장벽이 그만큼 낮다 |
| 게이트를 정책 평면에 두면 정책 평면이 프로세스 평면 상태를 읽는 결합이 생긴다 | 낮음 | 방향이 기존 의존 방향(정책 -> 프로세스)과 같고, 읽는 값은 계정 식별자와 그 자격 등급 둘뿐이다. 역방향 참조는 만들지 않는다 |
| 실행 계정과 프로브 계정이 갈라진 채로 티어 판정이 통과한다 | 낮음 (착수 시점 중간에서 하향) | F9가 위험 조건을 신원 차이에서 **자격 등급 차이**로 좁혔다. 이 워크스테이션의 분기 쌍(`bitgapnam@gmail.com` vs `gnkong@alipeople.kr`)은 둘 다 `chatgpt_plan_type: "pro"`라 판정 필드가 하나도 다르지 않다. REQ-004의 기본 경로가 등급 비교이므로 신원 분기 자체는 판정을 무효화하지 않고, 무효화 조건은 등급 불일치 하나로 남는다 |
| 자격 등급 클레임이 없거나 provider가 다른 형식으로 노출해 비교 자체가 불가능하다 | 중간 | 비교 불가는 "판정 성립"이 아니라 "판정 미성립"이다. REQ-009의 `unverified`로 떨어지고 사유에 비교 불가를 남긴다. Codex는 `auth.json` id_token 클레임이 실측으로 확인됐고(F9), Claude는 물려받을 카탈로그가 없어 이미 unverified 경로다. 새 provider가 붙을 때 클레임이 없으면 기본값은 `verified`가 아니라 `unverified`이며, 이 기본값을 계약이 명시한다 |

## Dependencies

- **PR #154** — J1(`quality.default` -> 내장 role-model 프로파일 -> omp `modelRoles`)이 닫혀 있음. 이 SPEC은 J1 결과를 입력으로 받는다. 불변으로 취급한다.
- **PR #152** — 한 계정 안에서 일어난 F3의 조용한 세대 강등을 같은 세대 폴백 추가로 해소했다. 계정·자격 불일치 사례가 아니라 **조용한 오판정의 비용**을 보여주는 근거이며, 그 비용이 INV-004의 출처다.
- **orca CLI** — 계정 조회의 사실상 유일한 소스(`account list`). 표면 변경은 요구하지 않고 소비자로만 취급한다(spec.md `## Out of Scope`).
- **`codex debug models`** — Codex 카탈로그의 유일한 확인된 프로브(`pkg/codexruntime/probe.go:39`). F9에서 이 카탈로그가 계정 신원이 아니라 **호출자의 자격**에 종속됨이 확인됐다.
- **`auto spec validate`** — 완료 판정 1번의 검증 수단.

새 외부 라이브러리 의존은 없다. 설계 문서이므로 코드 의존도 추가하지 않는다.

## Exit Criteria

- [ ] `auto spec validate .autopus/specs/SPEC-EXECPLANE-001`이 4개 문서에 대해 통과한다
- [ ] T1~T4의 산출물이 모두 존재하고 REQ 9건을 빠짐없이 덮는다
- [ ] research.md Completion Debt 3건이 각각 해소 또는 후속 SPEC 이관으로 처리됐다 — 3건 모두 F8에서 해소됐고 `## Completion Debt`는 `none remaining`이다. 남은 확인은 그 결정이 T2·T3 산출물에 계약으로 들어갔는지다
- [ ] research.md `## Reviewer Brief`의 4개 질문에 대한 리뷰 판단이 기록됐다
- [ ] 범위 밖 항목 목록이 spec.md `## Out of Scope`와 모순되지 않는다

구현 증거·테스트 통과·커버리지 수치는 이 SPEC의 종료 조건이 아니다. 설계 고정 문서이며, 구현은 별도 SPEC의 대상이다.
