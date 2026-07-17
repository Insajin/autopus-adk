# SPEC-ADK-WORKER-TRUST-DEFAULTS-001 리서치

## 기존 코드 분석

- `pkg/worker/adapter/resolve.go:17-37` — `wellKnownDirs`: `/opt/homebrew/bin`, `/usr/local/bin`, (darwin) `/Applications/cmux.app/Contents/Resources/bin`, `~/.local/bin`, `~/go/bin`, `~/.npm-global/bin`, `~/.cargo/bin`. 대부분 사용자-쓰기가능.
- `resolve.go:55-85` — `EnvironWithToolPath`: `parts = wellKnownDirs ++ inheritedPATH` 후 first-wins dedup → wellKnownDirs가 상속 PATH를 이긴다(prepend, 취약). `envValue`(87-95)는 마지막 `PATH=` 항목을 선택.
- `resolve.go:41` — `ResolveBinary`는 `exec.LookPath` 우선이라 PATH 순서에 정합(선반영). 이 SPEC은 서브프로세스 상속 PATH만 고친다.
- 호출부: `adapter/claude.go:65`, `gemini.go:55`, `codex.go:76` 3곳 모두 `cmd.Env = EnvironWithToolPath(env)`.
- `pkg/worker/controlplane/controlplane.go:16` — `PolicySigningSecretEnv`. `22-31` warn-once. `33-37` `@AX:ANCHOR` + `SignedControlPlaneEnforced()` (secret != ""). `39-49` `ValidateSecurityPolicySignature`: secret "" → warn + `return nil`(fail-open). `72-87` `VerifyCachedPolicyFile`, `89-105` `ValidateControlPlaneSignature` 동일 fail-open.
- request-intake: `a2a/server_task_runtime.go:42-58` `prepareTaskDispatch`가 매 task마다 policy/control-plane 서명을 검증한 뒤 `applyControlPlaneCapabilities`로 서버 메타데이터 반영.
- 워커 기동 경로: `internal/cli/worker_start.go:35` / `host/sidecar.go:147` → `host/runtime.go:39` `r.loop.Start` → `loop_runtime.go:35` `WorkerLoop.Start`. WorkerLoop 생성은 `host/runtime.go:23` 유일.
- **controlplane 외부 importer (수정된 실측)**: `pkg/worker` 밖에서 `pkg/worker/controlplane`를 import하는 파일은 정확히 `internal/cli/worker_validate.go`(`:10` import, `:72` `VerifyCachedPolicyFile` 호출 — `auto worker validate`)와 `internal/cli/worker_validate_test.go`(`:14`). 초판의 "외부 import 0" 주장은 오류였다: import 경로 문자열이 `pkg/worker`를 포함하여 `grep -v pkg/worker` 필터가 importer를 함께 제외했다. path 기준 재grep으로 정정. (나머지 importer는 `pkg/worker/{a2a,pipeline*}`로 워커 내부.)
- `worker` 패키지는 `controlplane`를 직접 import(`pipeline_parse.go:75` 등). `loop_task.go:139`는 `a2a.SignedControlPlaneEnforced()`(재수출) 사용.
- tamper 표적 타입: `pkg/worker/security/types.go:11` `SecurityPolicy{AllowNetwork, AllowFS, AllowedPaths, AllowedCommands, DeniedPatterns, AllowedDirs, TimeoutSec}` — 서명 미검증 시 broker가 네트워크 허용·명령 allowlist를 임의 주입 가능.
- `worker_validate.go:57-86` `runWorkerValidate`는 fail-closed(정책 파일 없음→DENY exit 1) 원칙이나, `:72` `VerifyCachedPolicyFile`가 secret 미설정 시 nil 반환이라 **unsigned 정책을 조용히 PASS로 통과**시킬 수 있다(기존 결함의 진단-표면 축소판).

## Outcome Lock

- **User-visible outcome**: (1) 서브프로세스 PATH에서 wellKnownDirs가 상속 PATH 뒤로 이동, (2) a2a broker 모드 워커가 secret 미설정 시 명확한 에러로 기동 거부, (3) `AUTOPUS_A2A_ALLOW_UNSIGNED=1` 명시 시에만 warn-once fail-open, (4) enforce-by-default가 워커 신뢰 표면(기동·a2a intake·`auto worker validate`)에 적용되고, `auto worker *` 계열이 아닌 CLI(예: `auto spec validate`)는 불변.
- **Mandatory requirements**: REQ-001~REQ-011 (Primary SPEC).
- **Explicit non-goals**: 대칭→비대칭 서명 전환, wellKnownDirs 목록 변경, backend 서명 로직, 도구 우선순위 설정 UI.
- **Completion evidence**: `auto spec validate --strict` 무오류 + S1~S9 oracle green + 소스 각 ≤300줄.

## Visual Planning Brief

plan.md의 `## Visual Planning Brief`에 PATH 조립 데이터-플로우와 기동/검증/진단 결정 게이트 다이어그램을 둔다. 요지: BEFORE는 dedup 전 `wellKnownDirs ++ inherited`, AFTER는 `inherited ++ wellKnownDirs`; 게이트는 세 워커-표면(Start·intake·worker validate)에서 `secret set → 검증 / opt-out truthy → warn+통과 / else → error(refuse/reject/non-zero)`.

## 설계 결정

- **결함1은 순서 교환만**: dedup(first-wins) 유지, `parts` 조립 순서만 뒤집으면 상속 PATH 우선. 트레이드오프: 동일 이름 바이너리가 양쪽에 있으면 system이 이김(예: 구버전 system node가 homebrew node를 이김) — 보안 기본값 우선. 특정 도구 우선은 Evolution Ideas.
- **결함2 게이트 위치**: `WorkerLoop.Start`(fail-fast) + `Validate*` 3함수(request-intake fail-closed)에 배치. 이 validator를 호출하는 유일한 `pkg/worker` 밖 명령 `auto worker validate`는 워커 신뢰 표면의 진단이므로 enforce **적용 대상**으로 의도적 포함(제외 아님). 진단이 unsigned를 조용히 통과시키는 것 자체가 기존 결함의 축소판이므로 이 강화는 회귀가 아니다.
- **opt-out 판정**: `strconv.ParseBool` truthy만 unsigned 허용. dev/self-host 전용.
- **`SignedControlPlaneEnforced()` 불변**: 의미는 "secret set". opt-out은 "미검증 허용"이지 "서버 메타데이터 신뢰"가 아니므로 라우팅은 계속 로컬 fallback(불변). 앵커 문구만 재협상.

## Minimality Decision Matrix

| Ladder step | Evidence | Decision | Receipt item |
|-------------|----------|----------|--------------|
| actual need | Outcome Lock (1)(2)(3)(4): PATH shadowing 차단 + unsigned 메타데이터·진단 기본 거부 | proceed | 2개 신뢰 기본값 전환 |
| existing code/helper/pattern | `EnvironWithToolPath` dedup 루프, `signingSecret()`, `warnUnsignedControlPlane()`, `unsignedWarnOnce`, `worker_validate.go` DENY/exit 패턴 | reuse | 순서 교환 + 기존 warn-once/DENY 재사용 |
| stdlib/native | `os.Getenv`, `strconv.ParseBool` | use | opt-out 판정에 stdlib만 |
| existing dependency | 새 의존성 불필요 | reuse | 신규 third-party 0 |
| new dependency or new abstraction | 새 env `[NEW] AUTOPUS_A2A_ALLOW_UNSIGNED` + `[NEW] enforce.go` 헬퍼 2종 | accepted | "explicitly allowed unsigned"와 "unsafe default" 구분 위해 최소 추가 |
| minimum sufficient verification | S1~S3(PATH), S4(기동), S5(opt-out), S6(intake), S7(경계 집합), S8(라우팅 불변), S9(worker validate 진단) | required checks | oracle 9종 + `auto spec validate --strict` |

## Semantic Invariant Inventory

| ID | source clause | invariant type | affected outputs | acceptance IDs |
|----|---------------|----------------|------------------|----------------|
| INV-001 | "prepend → append … 섀도잉 불가" | ordering | 서브프로세스 PATH env 문자열 | S1, S3 |
| INV-002 | "중복 제거 유지" | deduplication | PATH env 문자열 | S2 |
| INV-003 | "secret 미설정이면 기동 실패 … AUTOPUS_A2A_ALLOW_UNSIGNED … 때만 warn-once fail-open" | comparative policy gate (secret × opt-out → 결정) | 기동 에러, validate 에러/nil, warn 로그 | S4, S5, S6 |
| INV-004 | "SignedControlPlaneEnforced()의 의미(true iff secret set) … 불변 유지" | invariant preservation | bool 반환 + 라우팅/fallback 분기 | S8 |
| INV-005 | "worker validate는 워커-계열 명령 … enforce 적용 대상 … 워커-계열이 아닌 CLI 무영향" | scope boundary (worker-family 포함/제외) | worker validate exit·메시지, 비워커 명령 종료, 외부 importer 집합 | S7, S9 |

## Feature Coverage Map

| Outcome slice | Covered by | Status |
|---------------|------------|--------|
| PATH shadowing 차단 (append+dedup) | Primary SPEC T1 | covered |
| 기동 fail-fast + intake fail-closed | Primary SPEC T2·T3 | covered |
| worker validate 진단 fail-closed | Primary SPEC T5 | covered |
| opt-out warn-once 유지 | Primary SPEC T2·T6 | covered |
| 라우팅/앵커 불변·재협상 | Primary SPEC T4 | covered |
| worker-family 경계 고정 | Primary SPEC T5 | covered |

## Completion Debt

| Item | Blocks | Required resolution |
|------|--------|---------------------|
| None | - | - |

리뷰에서 드러난 `auto worker validate` 경계 충돌은 이 SPEC 내에서 해소된다(REQ-009 재정의 + REQ-011 신설 + T5 + S7/S9 oracle). 따라서 미해결 Completion Debt 없음.

## Evolution Ideas

Optional improvements. sync completion을 막지 않는다.

| Idea | Why not required now | Promotion trigger |
|------|----------------------|-------------------|
| 프로바이더별 도구 우선순위 명시 설정 | Outcome Lock 밖; 보안 기본값이 우선 | 사용자가 명시 요청 |
| 대칭 HMAC → 비대칭(공개키) 서명 전환 | non-goal; backend 협조 필요 | 사용자가 명시 요청 |

## Sibling SPEC Decision

| Decision | Reason | Sibling SPEC IDs |
|----------|--------|------------------|
| none | Primary SPEC이 Outcome Lock을 단독으로 닫는다 | None |

## Trust Boundary Analysis (Q-SEC)

| Boundary | Current trust | Threat | Mitigation |
|----------|---------------|--------|------------|
| 로컬 사용자-쓰기가능 디렉토리 (wellKnownDirs) | 상속 PATH보다 앞(prepend) | 심어진 `node`/`git`/`sh`로 프로바이더 서브프로세스 binary shadowing → 로컬 코드 실행 | append로 이동 → system PATH 우선(REQ-001~003) |
| a2a control-plane/policy 메타데이터 (broker, WebSocket) | secret 미설정 시 fail-open | unsigned broker/MITM가 `SecurityPolicy.AllowNetwork`·`AllowedCommands`·모델·파이프라인 프롬프트 주입 | enforce-by-default: 기동 거부 + intake 거부 + worker validate non-zero, opt-out에서만 허용(REQ-004~006, REQ-011) |
| signing-secret env (`AUTOPUS_A2A_POLICY_SIGNING_SECRET`) | 대칭 HMAC 키 | 로그/에러/진단 출력 유출 | 변수명만 기록, 값은 에러·warn·진단에서 제외(REQ-007) |

## Technology Stack Decision

| Mode | Selected stack | Resolved versions | Source refs | Checked at | Rejected alternatives |
|------|----------------|-------------------|-------------|------------|-----------------------|
| brownfield | Go stdlib(`os`, `strconv`, `crypto/hmac`) 재사용, 신규 의존성 없음 | 기존 `go.mod` major 유지 | repo `go.mod` | 2026-07-17 | 신규 third-party env/HMAC 라이브러리 — 불필요 |

## Reference Discipline

| Reference | Type | Verification |
|-----------|------|--------------|
| `pkg/worker/adapter/resolve.go::EnvironWithToolPath` (55-85) | existing | Read 확인 |
| `pkg/worker/adapter/{claude,gemini,codex}.go` 호출부 | existing | grep 확인(65/55/76) |
| `pkg/worker/controlplane/controlplane.go` (16, 33-49, 72-105) | existing | Read 확인 |
| `pkg/worker/a2a/server_task_runtime.go::prepareTaskDispatch` (41-58) | existing | Read 확인 |
| `pkg/worker/loop_runtime.go::WorkerLoop.Start` (35) | existing | Read 확인 |
| `internal/cli/worker_validate.go` (10 import, 72 VerifyCachedPolicyFile) | existing | Read 확인 (초판 누락 정정) |
| `internal/cli/worker_validate_test.go` (14 import) | existing | grep 확인 |
| `pkg/worker/security/types.go::SecurityPolicy` (11) | existing | grep 확인 |
| 외부 importer 집합 = {worker_validate.go, worker_validate_test.go} | existing | path 기준 재grep(초판 필터 오류 정정) |
| `AUTOPUS_A2A_ALLOW_UNSIGNED` env | [NEW] planned addition | 전 repo grep 무충돌 |
| `pkg/worker/controlplane/enforce.go` + 헬퍼 3종 | [NEW] planned addition | 신규 파일/심볼 |
| `enforce_test.go`, `loop_runtime_enforce_test.go` | [NEW] planned addition | 신규 테스트 |

## Reviewer Brief

- Intended scope: 워커의 두 신뢰 기본값(PATH 순서, a2a 서명 enforce) 전환. 대상 코드는 `pkg/worker/**`와 그 진단 표면 `internal/cli/worker_validate.go`.
- Explicit non-goals: 비대칭 서명 전환, wellKnownDirs 목록 변경, backend 서명, 도구 우선순위 UI — 새 scope로 확장 금지.
- Self-verified: Traceability Matrix, Semantic Invariant Inventory, oracle acceptance S1~S9, existing/[NEW] reference 분리, 외부 importer 집합 실측(2파일), Trust Boundary 표.
- Reviewer should focus on: correctness(순서/dedup/게이트 결정표/경계 집합), convergence safety(라우팅·앵커 불변), regression risk(worker validate UX·기존 계약 테스트), Completion Debt only.

## Self-Verify Summary
- Q-CORR-01 | status: FAIL | attempt: 1 | files: research.md | reason: "controlplane 외부 import 0" 주장이 grep 필터 오류로 오측
- Q-CORR-01 | status: PASS | attempt: 2 | files: research.md, spec.md | reason: worker_validate.go:72 외부 importer 실측 정정, path 기준 재grep
- Q-CORR-04 | status: PASS | attempt: 2 | files: research.md | reason: worker_validate.go/test를 existing importer로 명기, [NEW]와 분리
- Q-COMP-02 | status: PASS | attempt: 2 | files: spec.md, plan.md, acceptance.md | reason: REQ-011 추가 후 REQ↔Task↔AC↔INV 재정합
- Q-COMP-04 | status: PASS | attempt: 2 | files: research.md, spec.md | reason: Outcome Lock item(4)를 worker-family 경계로 갱신
- Q-COMP-05 | status: PASS | attempt: 2 | files: research.md, acceptance.md | reason: INV-005 재정의 + S7/S9로 추적
- Q-COMP-06 | status: PASS | attempt: 2 | files: spec.md, research.md | reason: Traceability Matrix + Reviewer Brief에 worker validate 반영
- Q-COMP-07 | status: PASS | attempt: 2 | files: research.md | reason: 경계 충돌 SPEC 내 해소 근거 추가, Completion Debt None 정당화
- Q-SEC-01 | status: PASS | attempt: 2 | files: research.md | reason: 신뢰경계표에 진단 표면 유출 포함
- Q-SEC-02 | status: PASS | attempt: 2 | files: spec.md, acceptance.md | reason: 진단 출력에도 secret 값 비노출(REQ-007, S9)
- Q-STYLE-02 | status: PASS | attempt: 2 | files: spec.md | reason: Priority(Must)와 EARS type 분리 축 유지
