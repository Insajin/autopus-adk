# SPEC-ADK-WORKER-TRUST-DEFAULTS-001: 워커 신뢰 기본값 강화 (PATH 순서 + a2a HMAC enforce-by-default)

**Status**: completed
**Created**: 2026-07-17
**Domain**: ADK

## 목적

autopus-adk 워커(`pkg/worker/**`)는 두 개의 신뢰 기본값이 안전하지 않은 쪽으로 설정되어 있다.

1. **PATH prepend (P1-6)**: `pkg/worker/adapter/resolve.go::EnvironWithToolPath`가 사용자-쓰기가능한 well-known 디렉토리(`/opt/homebrew/bin`, `~/.local/bin`, `~/go/bin`, `~/.npm-global/bin`, `~/.cargo/bin` 등)를 상속 PATH **앞에** 붙인다. 로컬 공격자나 악성 패키지가 이 디렉토리에 `node`·`git`·`sh`를 심으면 모든 프로바이더 서브프로세스(claude/gemini/codex)가 시스템 바이너리 대신 그것을 실행한다(binary shadowing).
2. **HMAC fail-open (P2-9)**: `pkg/worker/controlplane/controlplane.go`가 `AUTOPUS_A2A_POLICY_SIGNING_SECRET` 미설정 시 control-plane/policy 서명을 검증 없이 통과시킨다(fail-open by design). 백엔드 a2a broker에 연결된 워커가 서명되지 않은 메타데이터(모델, 파이프라인 단계·지시·프롬프트, iteration budget, `SecurityPolicy`)를 그대로 신뢰한다. 진단 명령 `auto worker validate`(`internal/cli/worker_validate.go`)도 같은 fail-open 경로를 거쳐 unsigned 정책을 조용히 유효로 통과시킨다.

이 SPEC은 두 기본값을 안전한 쪽(system PATH 우선, 서명 enforce-by-default)으로 바꾸되, 정당한 사용(GUI/launchd 제한 PATH 보완, dev/self-host의 unsigned 모드)은 명시적 경로로 계속 지원한다.

## Outcome Boundary

- Outcome Lock: (1) 서브프로세스 PATH에서 well-known 디렉토리가 상속 PATH **뒤로** 이동(shadowing 불가), (2) a2a broker 모드 워커가 signing secret 미설정 + opt-out 미설정이면 명확한 에러로 기동 거부, (3) `AUTOPUS_A2A_ALLOW_UNSIGNED` truthy일 때만 종전 warn-once fail-open 유지, (4) enforce-by-default가 워커 신뢰 표면(워커 기동·a2a task-intake·`auto worker validate` 진단)에 적용되고, **워커-계열(`auto worker *`)이 아닌 CLI 명령**(예: `auto spec validate`)은 UX 불변.
- Mandatory requirements: REQ-001 ~ REQ-011 (전부 Primary SPEC).
- Explicit non-goals: 대칭 HMAC→비대칭 서명 전환, well-known 디렉토리 목록 자체 변경, backend 측 서명 로직, 프로바이더별 도구 우선순위 설정 UI.
- Completion evidence: `auto spec validate --strict` 무오류 + acceptance S1~S9 oracle green + 신규/수정 소스 각 ≤300줄.

## Requirements

### REQ-001 — PATH append 순서 (Event-Driven, Priority: Must)
WHEN EnvironWithToolPath가 서브프로세스 PATH를 구성할 때 THEN THE SYSTEM SHALL 상속 PATH 항목을 well-known 도구 디렉토리보다 앞에 배치하여 상속 PATH가 먼저 해석되게 한다.
- 관측: `resolve_test.go`의 PATH 순서 비교(S1).

### REQ-002 — well-known 디렉토리 searchability 유지 (State-Driven, Priority: Must)
WHERE well-known 도구 디렉토리가 상속 PATH에 없을 때 THEN THE SYSTEM SHALL 그 디렉토리를 PATH 뒤에 덧붙여 서브프로세스가 계속 검색 가능하게 한다.
- 관측: `resolve_test.go`의 미포함 디렉토리 존재 확인(S3).

### REQ-003 — PATH 중복 제거 (Event-Driven, Priority: Must)
WHEN 한 디렉토리가 상속 PATH와 well-known 목록 양쪽에 나타날 때 THEN THE SYSTEM SHALL 상속 PATH 위치에 단일 항목만 유지한다.
- 관측: 중복 카운트 == 1(S2).

### REQ-004 — 기동 fail-fast (Event-Driven, Priority: Must)
WHEN 워커가 a2a broker 모드로 기동하고 signing secret과 opt-out 환경변수가 모두 미설정일 때 THEN THE SYSTEM SHALL 기동을 거부하고 signing-secret 변수명을 담은 에러를 반환한다.
- 관측: `EnforceSignedControlPlane` 반환값과 `WorkerLoop.Start` 에러(S4).

### REQ-005 — 명시적 opt-out (State-Driven, Priority: Must)
WHERE `AUTOPUS_A2A_ALLOW_UNSIGNED`가 truthy이고 signing secret이 미설정일 때 THEN THE SYSTEM SHALL 기동과 검증을 허용하고 warn-once fail-open 경로를 유지한다.
- 관측: opt-out 설정 시 warn-once 로그(S5).

### REQ-006 — request-intake fail-closed (Event-Driven, Priority: Must)
WHEN signing secret과 opt-out이 모두 미설정인 상태에서 control-plane 또는 policy 서명을 검증할 때 THEN THE SYSTEM SHALL nil 대신 검증 에러를 반환한다.
- 관측: `ValidateSecurityPolicySignature`/`ValidateControlPlaneSignature`/`VerifyCachedPolicyFile` non-nil 반환(S6).

### REQ-007 — signing-secret 값 비노출 (Ubiquitous, Priority: Must)
THE SYSTEM SHALL 기동 에러, 진단 에러, 경고 로그에서 signing-secret 값을 제외한다.
- 관측: 에러/로그 문자열에 secret 값 부재(S4, S5, S9).

### REQ-008 — 라우팅 의미 불변 (Ubiquitous, Priority: Must)
THE SYSTEM SHALL SignedControlPlaneEnforced가 signing secret 설정 시에만 true를 반환하도록 유지하여 모델 라우팅과 프롬프트 fallback 호출부를 불변으로 둔다.
- 관측: `control_plane_cov_test.go` 유지 + 라우팅 분기 불변(S8).

### REQ-009 — 워커-계열 밖 CLI 무영향 (State-Driven, Priority: Must)
WHERE 명령이 `auto worker *` 계열이 아닌 CLI 명령일 때 THEN THE SYSTEM SHALL 기동 거부 게이트와 request-intake fail-closed 게이트를 적용하지 않는다.
- 관측: `auto spec validate` 등 워커-계열 밖 명령이 unsafe env에서 정상 종료 + controlplane 외부 importer 집합이 정확히 2파일로 고정(S7).

### REQ-010 — 앵커 재협상 (Event-Driven, Priority: Must)
WHEN signed control-plane enforcement 계약이 silent-disable에서 enforce-by-default로 바뀔 때 THEN THE SYSTEM SHALL `controlplane.go`의 `@AX:ANCHOR` 문구를 새 기본값과 일치하도록 갱신한다.
- 관측: 앵커 텍스트가 opt-out 계약을 명시(S8 문서 확인).

### REQ-011 — worker validate 진단 fail-closed (Event-Driven, Priority: Must)
WHEN `auto worker validate`가 signing secret 미설정 + opt-out 미설정 상태로 실행될 때 THEN THE SYSTEM SHALL 서명 검증 비활성 상태를 non-zero 종료로 보고하고 signing-secret 변수명과 설정 안내를 출력한다.
- 관측: `auto worker validate` 종료 코드·메시지(S9). opt-out 설정 시 warn-once 후 종전 PASS/DENY 동작은 REQ-005가 커버.

## 생성 파일 상세

- `pkg/worker/adapter/resolve.go` (수정): `EnvironWithToolPath` append 순서 + doc comment.
- `pkg/worker/controlplane/controlplane.go` (수정): 3개 validator의 unsigned 분기 + `@AX:ANCHOR` 문구.
- `[NEW] pkg/worker/controlplane/enforce.go`: `AllowUnsignedControlPlaneEnv` 상수, `UnsignedControlPlaneAllowed`, `EnforceSignedControlPlane`, `unsignedResult` 헬퍼.
- `pkg/worker/loop_runtime.go` (수정): `WorkerLoop.Start` 상단에 기동 게이트 배선.
- `internal/cli/worker_validate.go` (수정): `VerifyCachedPolicyFile` fail-closed 에러를 "서명 검증 비활성" 진단 메시지+안내로 명확화(non-zero 유지).
- 테스트: `resolve_test.go`(수정), `controlplane_unsigned_test.go`(수정), `worker_validate_test.go`(신규 케이스 추가), `[NEW] enforce_test.go`, `[NEW] loop_runtime_enforce_test.go`.

## Related SPECs

None. Primary SPEC이 Outcome Lock을 단독으로 닫는다. Sibling SPEC Decision = none.

## Traceability Matrix

| Requirement | Plan Task | Acceptance Scenario | Semantic Invariant |
|-------------|-----------|---------------------|--------------------|
| REQ-001 | T1 | S1, S3 | INV-001 |
| REQ-002 | T1 | S3 | INV-001 |
| REQ-003 | T1 | S2 | INV-002 |
| REQ-004 | T3 | S4 | INV-003 |
| REQ-005 | T2 | S5 | INV-003 |
| REQ-006 | T2 | S6 | INV-003 |
| REQ-007 | T2, T3, T5 | S4, S5, S9 | INV-003 |
| REQ-008 | T4 | S8 | INV-004 |
| REQ-009 | T5 | S7 | INV-005 |
| REQ-010 | T4 | S8 | INV-004 |
| REQ-011 | T5 | S9 | INV-005 |
