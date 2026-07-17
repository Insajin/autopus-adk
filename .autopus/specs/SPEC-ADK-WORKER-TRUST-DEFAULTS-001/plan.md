# SPEC-ADK-WORKER-TRUST-DEFAULTS-001 구현 계획

## Tasks

- [x] **T1** — PATH append 순서 전환 (REQ-001·002·003, INV-001·002)
  - `pkg/worker/adapter/resolve.go::EnvironWithToolPath`에서 `parts`를 [상속 PATH..., wellKnownDirs...] 순으로 조립(현재는 [wellKnownDirs..., 상속 PATH...]). 기존 dedup 루프는 first-occurrence를 유지하므로 순서만 바꾸면 상속 PATH 위치가 승리한다.
  - doc comment "prepends" → "appends"로 정정하고, 트레이드오프(동일 이름 바이너리가 양쪽에 있으면 이제 시스템 PATH가 이김)를 코드 주석 1줄로 명시.
  - `resolve_test.go::TestEnvironWithToolPathPrependsWellKnownDirs`를 append 의미로 갱신(이름 포함)하고, `TestEnvironWithToolPathUsesLastInputPathAndDedupes`(last-PATH-wins + dedup)는 불변 유지 확인.

- [x] **T2** — a2a 서명 request-intake fail-closed + opt-out 헬퍼 (REQ-005·006·007, INV-003)
  - `[NEW] pkg/worker/controlplane/enforce.go` 신설: `const AllowUnsignedControlPlaneEnv = "AUTOPUS_A2A_ALLOW_UNSIGNED"`; `UnsignedControlPlaneAllowed() bool`(strconv.ParseBool truthy); `unsignedResult(context string) error`(opt-out truthy면 warn-once + nil, 아니면 secret 변수명 담은 에러); `EnforceSignedControlPlane() error`(T3에서 사용).
  - `controlplane.go`의 `ValidateSecurityPolicySignature`·`ValidateControlPlaneSignature`·`VerifyCachedPolicyFile`에서 `if secret == "" { warnUnsignedControlPlane(); return nil }`을 `if secret == "" { return unsignedResult("...") }`로 교체.
  - 실측 호출자 분리: `ValidateSecurityPolicySignature`/`ValidateControlPlaneSignature`는 a2a task-intake(`a2a/server_task_runtime.go:42-58` prepareTaskDispatch)의 라이브 검증기다. `VerifyCachedPolicyFile`의 유일한 프로덕션 호출자는 `internal/cli/worker_validate.go:72`(`auto worker validate` 진단; a2a intake 아님). 세 함수 모두 함수 단위 fail-closed 전환 대상.
  - secret 값은 에러/로그에 절대 포함하지 않는다(변수명만).

- [x] **T3** — 워커 기동 fail-fast 게이트 (REQ-004·007, INV-003)
  - `pkg/worker/loop_runtime.go::WorkerLoop.Start` 상단(서버 연결·PID lock 취득 전)에서 `controlplane.EnforceSignedControlPlane()` 호출. 에러면 즉시 반환하여 broker 연결·task 수신 이전에 종료.
  - `worker` 패키지는 이미 `controlplane`를 직접 import(`pipeline_parse.go` 등)하므로 a2a 재수출 불필요.
  - `[NEW] loop_runtime_enforce_test.go`: unsafe env에서 `Start`가 secret 변수명 담은 에러를 반환하고 broker에 연결하지 않음을 확인.

- [x] **T4** — 앵커 재협상 + 라우팅 불변 (REQ-008·010, INV-004)
  - `controlplane.go`의 `@AX:ANCHOR`(33행) 문구를 "env-driven on/off contract"에서 "enforce-by-default; unsigned은 AUTOPUS_A2A_ALLOW_UNSIGNED opt-out에서만 허용"으로 갱신(의도적 재협상).
  - `SignedControlPlaneEnforced()`(secret set일 때만 true)와 호출부(`pipeline_phase.go:27`, `pipeline.go:233`, `pipeline_parse.go:75`, `loop_task.go:139`, `a2a/control_plane.go:39`)는 시그니처·의미 불변 유지. `control_plane_cov_test.go`는 편집 없이 green.

- [x] **T5** — worker-family 경계 재정의 + worker validate 진단 UX (REQ-009·011·007, INV-005)
  - `internal/cli/worker_validate.go`(현재 `:72`에서 `VerifyCachedPolicyFile` 호출): T2로 secret 미설정+opt-out 미설정 시 이 호출이 에러를 반환하게 되므로, `:73` DENY 출력을 "서명 검증 비활성(unsigned)" 진단 메시지 + `AUTOPUS_A2A_POLICY_SIGNING_SECRET` 설정 안내로 명확화하고 non-zero(os.Exit(1)) 유지. secret 값 미노출. opt-out 설정 시 warn-once 후 종전 PASS/DENY 동작.
  - `worker_validate_test.go`(현재 secret 설정 케이스): 유지하고, secret 미설정+opt-out 미설정 → non-zero+env 변수명 포함 케이스와 opt-out=1 → 종전 동작 케이스를 신규 추가.
  - 경계 증거: `pkg/worker` 밖에서 `pkg/worker/controlplane`를 import하는 파일 집합이 정확히 `{internal/cli/worker_validate.go, internal/cli/worker_validate_test.go}`임을 grep으로 고정(추가 유입 시 실패). `auto spec validate` 등 워커-계열 밖 명령은 게이트 무영향.

- [x] **T6** — 계약 테스트 갱신 + 신규 oracle (REQ-005·006)
  - `controlplane_unsigned_test.go` 2개 테스트를 opt-out(`t.Setenv(AllowUnsignedControlPlaneEnv, "1")`) 전제로 갱신(warn-once nil 유지).
  - `[NEW] enforce_test.go`: secret/opt-out 상태 3분기(enforce 에러 / opt-out nil / secret 설정 nil)와 request-intake 3함수 fail-closed oracle.

## Implementation Strategy

- **최소 변경**: 결함1은 두 append 순서 교환(신규 로직 없음). 결함2는 기존 `signingSecret()`·`warnUnsignedControlPlane()`·`unsignedWarnOnce`를 재사용하고, opt-out 판정만 `[NEW]` 헬퍼로 추가. 새 third-party 의존성 없음(stdlib `os`/`strconv`).
- **게이트 위치로 경계 확보**: 게이트를 `WorkerLoop.Start`(워커 데몬 진입) + worker-only validator에 배치하고, 그 validator를 호출하는 유일한 `pkg/worker` 밖 명령 `auto worker validate`는 워커 신뢰 표면의 진단으로 의도적 포함. `auto worker *` 계열이 아닌 CLI는 controlplane를 import하지 않아 구조적으로 무영향.
- **파일 크기**: `controlplane.go`는 239줄 → 신규 헬퍼를 별도 `enforce.go`로 분리해 ≤300 유지. 모든 신규/수정 소스 ≤300, 목표 ≤200.

## Visual Planning Brief

PATH 조립 데이터-플로우 (결함1):

```
inherited env ─▶ envValue(PATH) = last PATH= entry
                        │
   BEFORE: parts = [wellKnownDirs...] ++ [inherited...]  ─▶ dedup(first-wins) ─▶ wellKnownDirs win  (shadowing)
   AFTER : parts = [inherited...] ++ [wellKnownDirs...]   ─▶ dedup(first-wins) ─▶ inherited win     (safe)
                        │
                        ▼
             PATH=<merged> ─▶ claude.go:65 / gemini.go:55 / codex.go:76
```

a2a 기동·검증 결정 게이트 (결함2):

```
worker trust surface ─┬─ WorkerLoop.Start ─▶ EnforceSignedControlPlane()
                      ├─ a2a task-intake  ─▶ Validate{SecurityPolicy,ControlPlane}Signature
                      └─ auto worker validate ─▶ VerifyCachedPolicyFile  (internal/cli/worker_validate.go:72)
   각 경로 공통 결정:
     secret set  ─▶ 통상 검증 (enforced=true)
     secret ""  + opt-out truthy ─▶ warn-once + fail-open (dev/self-host)
     secret ""  + opt-out no     ─▶ ERROR: refuse start / reject task / non-zero 진단
   워커-계열 밖 CLI (auto spec validate 등) ─▶ controlplane 미참조, 게이트 무영향
```

## Feature Completion Scope

- Primary SPEC이 두 결함 모두를 닫는다: PATH append(T1) + HMAC enforce-by-default 기동/검증/진단 게이트(T2·T3·T5) + 앵커 재협상·라우팅 불변(T4) + 계약 테스트(T6).
- 승인된 sibling 의존성: 없음.
- 남은 Completion Debt: 없음. 리뷰에서 드러난 `auto worker validate` 경계 충돌은 REQ-009 재정의 + REQ-011 신설 + S7/S9 oracle로 이 SPEC 내에서 해소된다.
