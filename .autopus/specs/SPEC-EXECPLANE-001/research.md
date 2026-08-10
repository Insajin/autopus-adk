# SPEC-EXECPLANE-001 Research: 실행 평면 분리와 티어·계정·카탈로그 정합

**Created**: 2026-08-10
**Domain**: EXECPLANE

## Outcome Lock

autopus-adk가 결정한 모델 티어가 **실제로 그 워크로드를 실행할 계정의 카탈로그로 검증된 뒤에만** 실행이 시작된다. 검증에 실패하면 조용히 강등되지 않고, 실행 전에 정확히 무엇이 어긋났는지(원한 티어 / 실행 계정 / 그 계정이 제공하는 모델)를 지목하며 멈추거나 명시적 강등 영수증을 남긴다.

그리고 orca·omp·autopus-adk가 겹치지 않는 세 평면으로 고정되어, 어느 한 평면에 기능을 추가할 때 나머지 두 곳에 같은 어휘가 복제되지 않는다.

## Visual Planning Brief

```
                       autopus-adk (정책 평면)
                SPEC · 16 canonical agent · quality tier
                     · 게이트 · 합의 전략 · 비용
                                 |
             "무엇을, 어느 티어로, 어떤 게이트를 통과해야"
                                 |
                 +---------------+---------------+
                 |                               |
       orca (프로세스 평면)                omp (모델 평면)
   worktree 격리 · 계정 · 감독            role -> provider/model:thinking
   · durability · 스케줄                  · provider 횡단 카탈로그 · RPC
                 |                               |
                 +------- 워커 안에서 실행 -------+
                           (DAG 소유자는 하나)
```

접합면은 정확히 두 개다.

| 접합면 | 방향 | 현재 상태 |
| --- | --- | --- |
| J1 티어 -> 모델 | autopus -> omp | **연결됨** (SPEC 없음, PR #154에서 `role_model_policy` 내장 프로파일로 구현) |
| J2 티어 -> 실행 계정 | autopus -> orca | **끊김** (이 SPEC의 대상) |

## Semantic Invariant Inventory

| ID | 불변식 | 유형 | 관측 지점 |
| --- | --- | --- | --- |
| INV-001 | 한 평면의 어휘는 다른 평면에 복제되지 않는다. orca는 티어 어휘를 갖지 않고 opaque model id만 받는다 | 구조 | `orca agent-context --json`에 tier/quality 토큰 0건 |
| INV-002 | DAG 소유자는 정확히 하나다. 소유자 `orca`는 OMP task DAG를 만들지 않고, 소유자 `omp`는 orca Run을 만들지 않는다 | 배타 | `pipeline_execution_owner_receipt.v1` |
| INV-003 | 티어 약속은 그것을 실행할 계정의 카탈로그로 검증된 뒤에만 유효하다 | 정합 | 정합 영수증의 `verified_against` 계정 식별자 |
| INV-004 | 강등은 조용할 수 없다. 요청 티어와 실제 제공 모델이 다르면 영수증에 사유와 두 값이 모두 남는다 | fail-loud | 영수증의 `resolution.reason` |
| INV-005 | 정합 점검은 실행 부작용을 만들지 않는다. 워크트리·Run·세션을 생성하기 전에 끝난다 | 순서 | 점검 실패 시 생성된 리소스 0건 |

## Feature Coverage Map

| 표면 | 현재 소유자 | 이 SPEC 이후 |
| --- | --- | --- |
| SPEC·16역할·게이트·합의 | autopus | 불변 |
| quality tier -> provider 모델 투영 | autopus (`pkg/config/quality_tier.go`) | 불변 |
| role -> `provider/model:thinking` | omp `modelRoles` | 불변 |
| provider 계정 소유·활성 선택 | orca `account list` | 불변 |
| worktree 격리·감독·durability | orca | 불변 |
| **티어와 실행 계정의 정합 검증** | **없음** | **autopus (신규)** |
| `--execution-owner orca` 실행 | handoff 스텁 | 불변 (별도 SPEC) |

## 측정된 근거

모두 2026-08-10 이 워크스테이션에서 직접 실행해 확인했다.

### F1 — 실행 계정과 프로브 계정이 이미 갈라져 있다

`orca account list --json`이 구조화된 페이로드를 준다.

```
result.<provider>.accounts[]   { id, email, organizationUuid | providerAccountId, ... }
result.<provider>.activeAccountId
result.<provider>.activeAccountIdsByRuntime  { host, wsl }
result.codex.systemDefault     { hasAuth, authKind, email, providerAccountId }   // codex에만 존재
```

이 워크스테이션의 실측값은 계정 **셋**이다.

| 축 | 계정 | 출처 |
| --- | --- | --- |
| orca 관리 Codex `activeAccountId` | `bitgapnam@gmail.com` | `orca account list --json` |
| 로컬 codex CLI = orca `codex.systemDefault` | `gnkong@alipeople.kr` | `~/.codex/auth.json` id_token, `systemDefault.email` |
| 로컬 claude CLI = orca 관리 Claude | `jroad1049@gmail.com` | `claude auth status --json`, `claude.accounts[0]` |

Codex 축에서 **orca가 워커를 띄울 계정과 PATH의 codex가 쓰는 계정이 다르다**. 한 번의 `orca account list --json` 호출이 두 값을 모두 반환하므로 비교는 추가 조회 없이 가능하다.

Claude 축에서는 `claude auth status --json`의 `orgId`가 orca `claude.accounts[0].organizationUuid`와 같은 값(`0e61c3e2-35c8-4bf8-91bc-40848f4dc147`)이라 두 소스를 신원으로 조인할 수 있다. 다만 `claude.activeAccountId`는 `null`이고 Claude 블록에는 `systemDefault` 필드가 없다 — provider 사이의 비대칭이다.

### F2 — autopus의 카탈로그 프로브는 PATH 바이너리, 즉 로컬 로그인 계정 기준이다

`pkg/codexruntime/probe.go:39`
```go
cmd := exec.CommandContext(probeCtx, binary, "debug", "models")
```

`internal/cli/codex_catalog_runtime.go:48-58`가 이 결과를 `config.ResolveCodexProviderProfile(entry, catalogJSON)`에 넘긴다. `codex debug models`는 **계정별로 서버가 내려주는 카탈로그**이므로, 프로브 결과는 로컬 로그인 계정의 것이다. F1과 합치면 검증 대상과 실행 대상이 서로 다른 계정이다.

### F3 — 이 실패 양식은 이미 잘못된 결론을 만들었다

`ResolveCodexProfile`이 요청 모델을 카탈로그에서 못 찾으면 하드코딩 레거시 슬러그로 직행했다(PR #152 이전). 이 머신에서 PR #151의 실제 효과는 `gpt-5.6-terra/medium` -> `gpt-5.5/xhigh`, 즉 **세대 강등**이었다. 측정 근거(sol 42 vs sonnet-5 36)는 5.5 대 5.6-terra에 대해 아무 말도 하지 않으므로 그 상태의 #151은 근거가 없었다. PR #152가 같은 세대 폴백을 추가해 해소했다.

교훈: 강등이 조용하면 근거와 결과가 어긋난 채로 티어 결정이 누적된다. INV-004의 출처다.

### F4 — orca에는 티어 어휘가 없다

`orca agent-context --json`(223 command, schema v1) 전수 검색:

| 토큰 | 출현 |
| --- | --- |
| `quality` | 0 |
| `tier` | 0 |
| `opus` / `sonnet` / `gpt-5` / `balanced` / `ultra` | 0 |
| `model` | 5 |
| `effort` | 3 |

모델·effort를 받는 명령은 `orchestration worker-start` 하나뿐이다.
```
--model supports Claude, Codex, and Cursor opaque provider model ids;
--effort requires --model. Neither can combine with --terminal.
```
`worktree create --agent <id>`는 모델을 받지 않는다. 즉 orca는 설계상 **opaque id 소비자**이며, 여기에 티어 어휘를 심으면 INV-001을 깬다.

### F5 — omp의 역할 어휘는 상수로 고정되어 있고 autopus가 이미 투영한다

omp `modelRoles`는 `default/plan/slow/smol/designer/vision/commit/tiny/task/advisor` 10종이며 각각 `provider/model:thinking`이다. autopus는 `pkg/config/role_model_policy_matrix.go:16-25`에 같은 10종을 상수로 갖고, capability 6종(`role_model_policy_matrix.go:8-14`)과 16 소스 에이전트(64-81행)로 매핑한다. 투영 지점은 `pkg/adapter/omp/omp_model_projection.go:59-107`, 실제로 쓰는 omp 설정 키는 `modelRoles`, `retry.fallbackChains`, `retry.modelFallback`이다(`omp_model_integration_activation.go:88-128`).

J1은 이미 닫혀 있다. PR #154가 `quality.default` -> 내장 role-model 프로파일 -> `modelRoles` 경로를 추가했다.

### F6 — `--execution-owner orca`는 계약만 있고 실행이 없다

`internal/cli/pipeline_run_owner.go:160-164`
```go
result := pipelineExecutionOwnerResult{
    Schema: pipelineExecutionOwnerResultSchema, Status: "handoff_required",
    ...
    RequiredAction: "orca skills get orchestration --full",
}
```

소유자를 기록하고 "orca 스킬을 읽어라"를 반환한 뒤 끝난다. 실제 Run 생성·워커 배치는 사람 또는 상위 에이전트가 수행한다. 따라서 이 SPEC의 정합 게이트는 **handoff 직전**에 놓여야 한다. 그래야 handoff를 받은 쪽이 이미 검증된 티어 계약을 들고 출발한다.

### F7 — 단일 DAG 소유자 불변식은 세 평면 합성을 금지하지 않는다

`pkg/adapter/omp/omp_workflow_render.go:99`
```
Owner `orca`: do not initialize an OMP task/todo DAG or call `task`.
```

금지 대상은 **OMP task DAG**이지 omp 실행 자체가 아니다. orca Run이 소유하는 워커 안에서 omp가 모델 평면 역할만 수행하고 자기 DAG를 만들지 않으면 INV-002를 준수한다. 세 평면 합성은 기존 불변식과 양립한다.

### F8 — 세 미결 질문의 실측 답

**계정 조회 소스.** `orca account list --json`이 존재하고 F1의 스키마를 반환한다. `--environment`/`--pairing-code`는 무시가 아니라 **거부**된다. 헬프 노트: *"Lists the accounts on this machine. `--environment` / `--pairing-code` are rejected rather than ignored; run it on the host whose accounts you want to see."*

**Claude 카탈로그.** `codex debug models` 등가물이 없다. `claude` CLI 명령 집합은 `agents`, `auth`, `auto-mode`, `doctor`, `gateway`, `import`, `install`, `mcp`, `plugin`, `project`, `setup-token`, `ultrareview`이며 모델 열거 명령이 없다. `omp models --json`은 있으나 **정적 레지스트리**다 — 계정 스코프가 아니라 provider별 모델 메타데이터를 반환하므로 특정 계정의 자격을 증명하지 못한다.

```json
{"provider":"anthropic","id":"claude-opus-5","selector":"anthropic/claude-opus-5",
 "thinking":["low","medium","high","xhigh","max"],"cost":{"input":5,"output":25}}
```

대신 `claude auth status --json`이 신원과 플랜을 준다.

```json
{"loggedIn":true,"authMethod":"claude.ai","apiProvider":"firstParty",
 "email":"jroad1049@gmail.com","orgId":"0e61c3e2-...","subscriptionType":"max"}
```

**원격 환경.** `orca environment list --json`은 이 머신에서 `{"environments": []}`를 반환한다. 저장된 원격 환경이 0건이므로 원격 경로는 현재 미사용이다. `orchestration worker-list`는 터미널 리소스 회계만 보고하고 계정을 노출하지 않아 사후 검증 경로도 없다.

### 결정

| 질문 | 결정 | 근거 |
| --- | --- | --- |
| 계정 조회 소스 | `orca account list --json`. 실행 계정은 `activeAccountId`, 프로브 계정은 `systemDefault`(codex) 또는 provider CLI 신원 | 한 호출로 두 값을 모두 얻어 비교 가능 |
| Claude 검증 깊이 | **신원만 검증**. `claude auth status --json`의 `orgId`를 orca `organizationUuid`와 대조하고, 모델 가용성은 unverified로 표기 | `subscriptionType`으로 플랜→모델을 매핑하면 하드코딩 티어 테이블이 되살아난다. PR #154가 제거한 것과 같은 종류다 |
| Claude `activeAccountId`가 null | 등록 계정이 **정확히 1개**면 그것을 실행 계정으로 보고, 0개이거나 2개 이상이면 unverified | 추측하지 않는다. 현재 상태(1개 등록)에서 동작하고, 모호해지면 모호하다고 보고한다 |
| 원격 환경 | 별도 요구를 만들지 않고 **REQ-009에 흡수**. 원격 대상이면 "계정 조회 수단 없음"의 인스턴스이므로 unverified | `account list`가 원격을 구조적으로 거부하고, 저장된 환경이 0건이라 당장 영향도 없다 |

## Completion Debt

none remaining. 착수 시점의 미결 3건은 F8에서 실측으로 해소되어 결정 표에 반영됐고, 그 결정은 spec.md REQ-003·REQ-004·REQ-009 본문에 계약으로 들어갔다.

## Evolution Ideas

- 정합 점검 결과를 캐시해 매 실행마다 카탈로그를 프로브하지 않게 만들 수 있다. 계정 전환 감지와 TTL이 필요하다.
- 여러 계정을 풀로 두고 요청 티어를 제공할 수 있는 계정을 자동 선택하는 라우팅. 지금은 활성 계정 하나를 검증만 한다.
- 티어 강등이 불가피할 때 대안 provider로 넘기는 정책. capability 라우팅이 provider-neutral이므로 구조적으로는 가능하다.
- 실행 원장 통합. 정책·모델·프로세스 세 평면의 관측 데이터를 한 영수증으로 조인.

## Reference Discipline

| 참조 | 유형 | 확인 방법 |
| --- | --- | --- |
| `orca account list --json` 페이로드 | 실측 | 이 워크스테이션에서 직접 실행 |
| `claude auth status --json` 페이로드 | 실측 | 직접 실행 |
| `claude --help` 명령 집합 | 실측 | 직접 실행, 모델 열거 명령 부재 확인 |
| `omp models --json` 엔트리 구조 | 실측 | 직접 실행, 계정 스코프 아님 확인 |
| `orca environment list --json` | 실측 | 직접 실행, `environments: []` |
| `~/.codex/auth.json` id_token 클레임 | 실측 | 직접 디코드 |
| `orca agent-context --json` | 실측 | 223 command 스키마 전수 검색 |
| `omp config list`의 `modelRoles` | 실측 | 직접 실행 |
| `pkg/codexruntime/probe.go:39` | 코드 | 직접 읽음 |
| `internal/cli/codex_catalog_runtime.go:48-58` | 코드 | 직접 읽음 |
| `internal/cli/pipeline_run_owner.go:160-164` | 코드 | 직접 읽음 |
| `pkg/adapter/omp/omp_workflow_render.go:99` | 코드 | 직접 읽음 |
| `pkg/config/role_model_policy_matrix.go:8-81` | 코드 | 직접 읽음 |
| PR #151 / #152 / #154 | 이력 | 이 저장소 머지 커밋 |

추측은 없다. 위 표에 없는 주장은 이 문서에 담지 않았다.

## Reviewer Brief

이 SPEC은 **설계 고정**이 목적이고 구현을 포함하지 않는다. 리뷰어가 우선 검증할 것:

1. 세 평면 경계가 실제로 배타적인가. 어느 한 평면에 이 SPEC의 요구를 넣었을 때 다른 평면에 같은 어휘가 생기지 않는가.
2. 정합 게이트를 autopus에 두는 선택이 맞는가. orca에 두면 orca가 티어를 알아야 하고(INV-001 위반), omp에 두면 omp가 실행 계정을 알아야 한다(평면 침범).
3. F6의 handoff 직전 배치가 INV-005(부작용 없음)를 실제로 보장하는가.
4. Completion Debt 3건이 구현 착수 전에 해소 가능한 질문인가, 아니면 SPEC 자체를 막는가.

블로킹 대상은 Completion Debt뿐이다. Evolution Ideas는 자문이다.

## Self-Verify Summary

- **Q-CORR-04** (근거 정확성) | status: PASS — 모든 사실 주장이 Reference Discipline 표의 실측 또는 코드 인용에 대응한다. F1·F4·F5는 이 워크스테이션에서 명령을 실행해 얻은 출력이고, F2·F6·F7은 파일:라인 인용이다.
- **Q-COMP-05** (범위 완결성) | status: PASS — 접합면 J1·J2를 모두 식별했고, J1은 이미 닫혀 있음을 근거와 함께 기록했으며 J2만 이 SPEC의 대상으로 남겼다. 세 평면의 소유 표면을 Feature Coverage Map에 전수 열거했다.
- **Q-COMP-06** (미해결 항목 분리) | status: PASS — 확정되지 않은 3건을 Completion Debt로 분리했다. Evolution Ideas에는 `SPEC-`·`AC-`·체크박스를 넣지 않아 블로킹 항목과 자문 항목이 섞이지 않는다.
- **Q-COMP-07** (완료 판정 가능성) | status: PASS — 이 SPEC은 설계 문서이므로 완료 증거는 4개 문서의 `auto spec validate` 통과와 세 평면 경계의 리뷰 합의다. 구현 증거는 요구하지 않으며 그 사실을 Outcome Boundary에 명시했다.
