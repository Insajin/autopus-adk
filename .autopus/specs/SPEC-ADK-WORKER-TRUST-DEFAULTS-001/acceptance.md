# SPEC-ADK-WORKER-TRUST-DEFAULTS-001 수락 기준

Oracle 표기: PATH는 `os.PathListSeparator`로 split한 슬라이스 인덱스로 비교한다. `/opt/homebrew/bin`과 `/usr/local/bin`은 wellKnownDirs의 OS-불변 상수 항목이다.

## Test Scenarios

### S1: PATH append 순서 — 상속 PATH가 well-known보다 앞 (REQ-001, INV-001)
Given 상속 env가 `PATH=/usr/bin:/bin` 이다.
When `EnvironWithToolPath`를 호출한다.
Then 결과 PATH를 split한 슬라이스의 index 0은 `/usr/bin` 이고 index 1은 `/bin` 이다.
And `/opt/homebrew/bin`의 index는 2 이상이다.
And `/usr/bin`의 index가 `/opt/homebrew/bin`의 index보다 작다.

### S2: dedup — 양쪽에 있는 디렉토리는 상속 위치 1개만 (REQ-003, INV-002)
Given 상속 env가 `PATH=/usr/local/bin:/usr/bin` 이고 `/usr/local/bin`은 wellKnownDirs에도 있다.
When `EnvironWithToolPath`를 호출한다.
Then 결과 PATH에서 `/usr/local/bin`은 정확히 1회 나타난다.
And `/usr/local/bin`의 index는 0 이다.

### S3: searchability — 상속에 없는 well-known 디렉토리는 뒤에 덧붙는다 (REQ-002, INV-001)
Given 상속 env가 `PATH=/usr/bin:/bin` 이고 여기에 `/opt/homebrew/bin`은 포함되지 않는다.
When `EnvironWithToolPath`를 호출한다.
Then 결과 PATH는 `/opt/homebrew/bin`을 포함한다.
And `/usr/bin`과 `/bin`의 index는 모두 `/opt/homebrew/bin`의 index보다 작다.

### S4: 기동 fail-fast — secret·opt-out 미설정이면 거부 (REQ-004, REQ-007, INV-003)
Given `AUTOPUS_A2A_POLICY_SIGNING_SECRET`가 미설정이고 `AUTOPUS_A2A_ALLOW_UNSIGNED`도 미설정이다.
When `EnforceSignedControlPlane`를 호출한다.
Then non-nil 에러를 반환한다.
And 에러 메시지는 `AUTOPUS_A2A_POLICY_SIGNING_SECRET` 문자열을 포함한다.
And 에러 메시지는 어떤 signing-secret 값도 포함하지 않는다.
And 같은 env에서 `WorkerLoop.Start`를 호출하면 broker에 연결하기 전에 그 에러를 반환한다.

### S5: opt-out warn-once fail-open (REQ-005, REQ-007, INV-003)
Given `AUTOPUS_A2A_POLICY_SIGNING_SECRET`가 미설정이고 `AUTOPUS_A2A_ALLOW_UNSIGNED`가 `1` 이다.
When `ValidateSecurityPolicySignature("task-1", policy, "")`를 두 번 호출한다.
Then 두 호출 모두 nil을 반환한다.
And `[controlplane]`와 `fail-open`을 포함한 경고가 정확히 1회 방출된다.
And 그 경고는 어떤 signing-secret 값도 포함하지 않는다.

### S6: request-intake fail-closed (REQ-006, INV-003)
Given `AUTOPUS_A2A_POLICY_SIGNING_SECRET`가 미설정이고 `AUTOPUS_A2A_ALLOW_UNSIGNED`도 미설정이다.
When `ValidateSecurityPolicySignature("task-1", policy, "")`, `ValidateControlPlaneSignature("task-1", "", nil, nil, nil, nil, nil, "")`, `VerifyCachedPolicyFile("autopus-policy-task-1.json", policy)`를 각각 호출한다.
Then 세 호출 모두 non-nil 에러를 반환한다.
And 어떤 호출도 fail-open nil을 반환하지 않는다.

### S7: 워커-계열 밖 CLI 무영향 + importer 경계 고정 (REQ-009, INV-005)
Given `AUTOPUS_A2A_POLICY_SIGNING_SECRET`가 미설정이고 `AUTOPUS_A2A_ALLOW_UNSIGNED`도 미설정이다.
When 워커-계열 밖 명령 `auto spec validate .autopus/specs/SPEC-ADK-WORKER-TRUST-DEFAULTS-001`을 실행한다.
Then 기동 거부/진단 에러 없이 종료한다.
And `pkg/worker` 밖에서 `pkg/worker/controlplane`를 import하는 파일 집합은 정확히 `{internal/cli/worker_validate.go, internal/cli/worker_validate_test.go}` 이다.
And 그 집합에 새 importer가 추가되면 이 시나리오는 실패한다.

### S8: SignedControlPlaneEnforced 의미 불변 (REQ-008, REQ-010, INV-004)
Given enforcement 판정 함수 `SignedControlPlaneEnforced`가 있다.
When `AUTOPUS_A2A_POLICY_SIGNING_SECRET`를 non-empty 값으로 설정한다.
Then `SignedControlPlaneEnforced`는 true를 반환한다.
And `AUTOPUS_A2A_POLICY_SIGNING_SECRET`가 미설정이면 false를 반환한다.
And `loop_task.go`의 라우팅 분기(`Router != nil && !SignedControlPlaneEnforced()`)는 secret 미설정일 때만 로컬 라우팅을 선택하여 본 SPEC 이전과 동일하게 동작한다.

### S9: auto worker validate 진단 fail-closed (REQ-011, REQ-007, INV-005)
Given `AUTOPUS_A2A_POLICY_SIGNING_SECRET`가 미설정이고 `AUTOPUS_A2A_ALLOW_UNSIGNED`도 미설정이며, 유효한 policy JSON 파일이 있다.
When `auto worker validate --policy <file> --command <cmd>`를 실행한다.
Then 종료 코드는 non-zero(1) 이다.
And 출력은 `AUTOPUS_A2A_POLICY_SIGNING_SECRET` 문자열과 설정 안내를 포함한다.
And 출력은 어떤 signing-secret 값도 포함하지 않는다.
And `AUTOPUS_A2A_ALLOW_UNSIGNED`가 `1`이면 warn-once 후 policy 판정에 따른 종전 PASS/DENY 종료 코드로 동작한다.

## Oracle Acceptance Notes

각 Must 시나리오는 concrete expected output(구체 기대값) 또는 explicit tolerance를 포함한다. 구조 신호(파일 존재 여부, 섹션 제목, 종료 상태, 빈 출력 여부)만으로 닫지 않는다.

- S1 예상 값: `parts[0] == "/usr/bin"`, `parts[1] == "/bin"`, `indexOf("/opt/homebrew/bin") >= 2`.
- S2 예상 값: `count("/usr/local/bin") == 1`, `indexOf("/usr/local/bin") == 0`.
- S3 예상 출력: 결과 PATH에 `/opt/homebrew/bin` 포함, `indexOf("/usr/bin") < indexOf("/opt/homebrew/bin")`.
- S4 concrete expected output: 반환 에러 문자열이 `AUTOPUS_A2A_POLICY_SIGNING_SECRET`를 포함하고 signing-secret 값은 미포함.
- S5 예상 출력: `[controlplane]`+`fail-open` 경고가 정확히 1회, secret 값 미포함.
- S6 예상 값: 세 검증 함수 모두 non-nil error(nil fail-open 없음).
- S7 예상 값: 외부 importer 집합 == `{internal/cli/worker_validate.go, internal/cli/worker_validate_test.go}`(정확히 2, 초과 시 실패).
- S8 예상 값: secret set → `true`, unset → `false`; 라우팅 분기 결과가 본 SPEC 이전과 동일(exact match).
- S9 예상 값: exit==1 + stdout에 `AUTOPUS_A2A_POLICY_SIGNING_SECRET` 포함 + secret 값 미포함; opt-out=1이면 policy 판정 기반 종전 종료 코드.
