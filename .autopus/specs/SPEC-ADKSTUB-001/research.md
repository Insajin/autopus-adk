# SPEC-ADKSTUB-001 리서치

## 기존 코드 분석

### 1. agent_run.go — Placeholder 위치

**파일**: `internal/cli/agent_run.go`
- L67-68: `_ = ctx` — context 파싱 후 discarded
- L71-76: 하드코딩된 `status: "success"` 결과 기록
- `taskContext` struct (L18-21): `TaskID`, `Description` 필드
- `taskResult` struct (L24-28): `TaskID`, `Status`, `Timestamp` — cost/duration/output 필드 없음

**문제**: `taskResult`에 실행 결과 필드가 부족하다. `adapter.TaskResult`와 매핑 필요.

### 2. Provider Adapter 인프라

**파일**: `pkg/worker/adapter/interface.go`
- `ProviderAdapter` 인터페이스: `Name()`, `BuildCommand()`, `ParseEvent()`, `ExtractResult()`
- `TaskConfig`: subprocess 실행에 필요한 모든 파라미터 보유
- `TaskResult`: `CostUSD`, `DurationMS`, `SessionID`, `Output`, `Artifacts`

**파일**: `pkg/worker/adapter/claude.go`
- `ClaudeAdapter.BuildCommand()`: `--print --output-format stream-json --verbose --resume` 플래그 조합
- stdin을 통해 prompt 전달 (cmd에 stdin 연결 필요)
- `ParseEvent()`: `stream.ParseLine()` 호출 후 `tool_use` → `tool_call` 매핑

**파일**: `pkg/worker/adapter/registry.go`
- adapter registry 패턴 확인 필요 (등록/조회 API)

**파일**: `pkg/worker/stream/parser.go`
- `Parser.Next()`: io.Reader에서 JSON 라인 파싱, `io.EOF`까지 반복
- `ParseLine()`: 단일 라인 파싱, `type` 필드 추출

**파일**: `pkg/worker/stream/events.go`
- `EventResult = "result"`: 최종 결과 이벤트 타입
- `EventError = "error"`: 에러 이벤트 타입
- `ResultData`: cost_usd, duration_ms, session_id, output

### 3. Worker TUI — 기존 콜백 패턴

**파일**: `pkg/worker/tui/model.go`
- L78-79: `OnApprovalDecision func(taskID, decision string)`, `OnViewDiff func(taskID string)` — 기존 콜백 패턴
- L129-152: approval 키 핸들러에서 콜백 호출 → nil 체크 후 실행
- L159-163: `p`, `c` 키는 placeholder — 같은 콜백 패턴으로 연결 가능
- L192: help line에 이미 `[p]ause  [c]ancel` 표시 → UI 안내는 이미 존재

### 4. Config Schema — 미사용 필드 분석

#### TelemetryConf (제거 대상)
- `pkg/config/schema.go` L42-47: `TelemetryConf` struct 정의
- `pkg/config/schema.go` L79: `HarnessConfig.Telemetry` 필드
- `pkg/config/defaults.go` L124: default 값 설정
- **실제 사용**: `pkg/telemetry/` 패키지는 자체 파일 기반 reader 사용. `HarnessConfig.Telemetry.Enabled` 등을 읽는 코드 = 0건
- **CLI**: `internal/cli/telemetry.go`의 `auto telemetry` 커맨드 그룹은 `pkg/telemetry/reader`를 직접 사용, config 참조 없음
- **결론**: config struct에서 제거해도 안전. telemetry 기능 자체는 파일 기반으로 동작 유지.

#### ConstraintConf (구현 대상)
- `pkg/config/schema.go` L226-229: `ConstraintConf` struct
- `pkg/constraint/checker.go`: `Check(dir, constraints, opts)` — 완성된 checker 존재
- `pkg/constraint/registry.go`: constraint 로딩 로직 존재
- `internal/cli/init_constraints.go`: `generateDefaultConstraints()` — default 파일 생성만 담당
- **갭**: config의 `Constraints.Path`를 읽어 로딩하는 연결 코드 부재

#### IssueReport.AutoSubmit (구현 대상)
- `pkg/config/schema.go` L53: `AutoSubmit bool`
- `internal/cli/issue.go` L42: `runIssueReport(cmd, dryRun, autoSubmit, errMsg, command, exitCode)` — 수동 CLI에서만 사용
- `pkg/issue/submitter.go` L136: `Submit(report, body)` — 실제 제출 로직 완성
- **갭**: pipeline 실패 시 자동 호출 트리거 부재

#### Hints.Platform (구현 대상)
- `pkg/config/schema_profile.go` L30-42: `HintsConf`, `IsPlatformHintEnabled()`
- `pkg/hint/hint.go` L47: `CheckAndShow(projectPath, profile, hintsEnabled, w)` — `hintsEnabled` 파라미터로 외부에서 전달
- `internal/cli/config_cmd.go` L64: `cfg.Hints.Platform = &b` — config set으로 저장은 가능
- **갭**: `CheckAndShow` 호출 지점에서 `cfg.Hints.IsPlatformHintEnabled()`를 직접 사용하지 않고 별도 bool을 전달

## 설계 결정

### D1: taskResult 확장 vs adapter.TaskResult 재사용

**결정**: `taskResult` struct를 확장하여 `CostUSD`, `DurationMS`, `Output` 필드를 추가한다.
**이유**: `adapter.TaskResult`는 `pkg/worker/adapter` 패키지에 속하며, `internal/cli`에서 YAML 직렬화용 struct를 별도로 유지하는 것이 계층 분리에 적합하다. 두 struct 간 매핑은 단순 필드 복사.
**대안**: `adapter.TaskResult`에 yaml 태그 추가 → 패키지 간 결합도 증가로 기각.

### D2: Provider 선택 전략

**결정**: `context.yaml`에 `provider` 필드를 추가하고, 없으면 "claude"를 default로 사용한다.
**이유**: 현재 `taskContext`에 provider 선택 필드가 없다. Worker daemon이 task를 생성할 때 provider를 지정하는 것이 자연스럽다.
**대안**: 환경변수 `AUTOPUS_PROVIDER`로 결정 → context.yaml과 불일치 가능성으로 기각.

### D3: TUI pause의 범위

**결정**: pause는 "새 태스크 수신 중단"으로 정의하고, 현재 실행 중인 태스크는 계속 진행한다.
**이유**: 이미 실행 중인 subprocess를 일시정지하는 것은 OS 레벨 SIGSTOP이 필요하며, 플랫폼 이식성과 복잡도가 과도하다. 대부분의 CI/CD 도구에서 pause는 "대기열 중단"을 의미한다.
**대안**: subprocess SIGSTOP/SIGCONT → 플랫폼 비이식성(Windows 미지원)으로 기각.

### D4: TelemetryConf 제거 시 하위 호환

**결정**: struct에서 제거하되, yaml.v3의 기본 동작(unknown field 무시)에 의존하여 기존 autopus.yaml의 `telemetry:` 섹션이 에러를 유발하지 않도록 한다.
**이유**: `yaml.v3.Decoder`는 `KnownFields(true)` 호출 없이 사용하면 unknown field를 무시한다. 현재 로더 코드가 strict mode를 사용하지 않는 것 확인 필요.
**위험**: 로더가 strict mode라면 breaking change. T8 구현 시 로더 코드 확인 필수.

### D5: IssueReport AutoSubmit의 rate limiting

**결정**: `IssueReportConf.RateLimitMinutes` 필드(이미 존재)를 활용하여 자동 제출 빈도를 제한한다.
**이유**: 자동 트리거는 반복 실패 시 대량 이슈 생성 위험이 있다. 기존 rate limit 필드를 실제로 작동시키면 된다.
