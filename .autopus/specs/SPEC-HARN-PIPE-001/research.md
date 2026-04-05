# SPEC-HARN-PIPE-001 리서치

## 기존 코드 분석

### 핵심 재활용 대상

#### 1. `pkg/orchestra/backend.go` — ExecutionBackend 인터페이스

```go
type ExecutionBackend interface {
    Execute(ctx context.Context, req ProviderRequest) (*ProviderResponse, error)
    Name() string
}
```

- `subprocessBackend` (subprocess_runner.go)이 이미 CLI subprocess 실행을 구현
- `ProviderRequest`에 `Provider`, `Prompt`, `Config`, `Timeout` 등 모든 필요 필드 존재
- `ProviderResponse`에 `Output`, `Error`, `ExitCode`, `TimedOut`, `Duration` 포함
- `SelectBackend()` 함수가 `SubprocessMode` 플래그 기반으로 백엔드 선택

#### 2. `pkg/orchestra/subprocess_runner.go` — Subprocess 실행

- `buildSubprocessArgs()` — 프로바이더별 CLI 인자 구성
- `setupStdin()` — 프롬프트 전달 (stdin pipe / file / args)
- `validateJSONOutput()` — 구조화된 출력 검증
- `maxStdinLen = 64KB` — 대형 프롬프트는 임시 파일로 전달
- `command` 인터페이스 (`command.go`) — 테스트 모킹 가능

#### 3. `pkg/orchestra/pipeline.go` — Subprocess Pipeline (debate 용)

- `RunSubprocessPipeline()` — 병렬 실행 + 라운드 기반 파이프라인
- `executeParallel()` — goroutine 기반 병렬 프로바이더 실행
- 이 코드는 "debate" 전용이지만, 병렬 실행 패턴을 파이프라인에 재적용 가능

#### 4. `pkg/config/schema.go` — 프로바이더 설정

- `OrchestraConf.Providers` — `map[string]ProviderEntry` (binary, args, subprocess 설정)
- `SubprocessConf` — 전역 subprocess 설정 (max_concurrent, rounds)
- `SubprocessProvConf` — 프로바이더별 오버라이드 (schema_flag, stdin_mode, output_format, timeout)

#### 5. `pkg/pipeline/` — 기존 파이프라인 인프라

- `Checkpoint` (types.go) — Phase별 상태 저장 (phase, git_commit_hash, task_status)
- `Event` (events.go) — JSONL 이벤트 (phase_start, phase_end, agent_spawn, agent_done)
- `MonitorSession` (monitor.go) — cmux/tmux pane 기반 모니터링
- `TeamMonitorSession` (team_monitor.go) — 다중 에이전트 모니터링
- `PipelineMonitor` 인터페이스 — Start/UpdateAgent/Close/LogPath

#### 6. `pkg/adapter/registry.go` — 플랫폼 감지

- `Registry.DetectAll()` — 설치된 CLI 자동 감지
- 각 어댑터의 `CLIBinary()` — 실행 바이너리명 반환

#### 7. `internal/cli/pipeline.go` — 기존 CLI

- `LoadCheckpointIfContinue()` — `--continue` 플래그 처리
- `specCheckpointPath()` — `.autopus/pipeline-state/{specID}.yaml`
- `getCurrentGitHash()` — stale checkpoint 감지

### Phase 구조 (agent-pipeline.md 기준)

| Phase | 역할 | 입력 | 출력 |
|-------|------|------|------|
| Phase 1 | Planning | SPEC 파일 (spec.md, plan.md, acceptance.md, research.md) | 구현 계획, 태스크 분해 |
| Phase 1.5 | Test Scaffold | Phase 1 결과 + acceptance.md | 테스트 스켈레톤 코드 |
| Phase 2 | Implementation | Phase 1 결과 + Phase 1.5 테스트 | 구현 코드 |
| Gate 2 | Validation | Phase 2 결과 | PASS/FAIL (재시도 최대 3회) |
| Phase 3 | Testing | Phase 2 코드 + Phase 1.5 테스트 | 테스트 실행 결과 |
| Phase 4 | Review | 전체 결과 | APPROVE/REQUEST_CHANGES (재시도 최대 2회) |

## 설계 결정

### D1: `PipelineEngine` 인터페이스 도입

**결정**: `PipelineEngine` 인터페이스를 정의하여 Agent() 기반과 subprocess 기반을 통합

**이유**: Claude Code에서는 Agent() 기반이 더 효율적 (네이티브 서브에이전트), 다른 플랫폼에서는 subprocess가 유일한 옵션. 인터페이스로 추상화하면 `auto go`가 플랫폼에 따라 적절한 엔진을 선택할 수 있음.

**대안 검토**:
- (A) subprocess만 사용 → Claude Code의 Agent() 장점 (컨텍스트 공유, 도구 접근)을 포기해야 함
- (B) Agent()만 사용 → codex/gemini에서 동작 불가
- (C) 인터페이스 없이 플래그 분기 → 코드 중복, 테스트 복잡

### D2: `pkg/pipeline/` 패키지 확장

**결정**: 기존 `pkg/pipeline/`에 엔진 로직 추가

**이유**: Checkpoint, Event, MonitorSession 등 파이프라인 인프라가 이미 이 패키지에 있음. 새 패키지를 만들면 순환 의존성이 발생하거나 불필요한 래핑이 필요.

**대안 검토**:
- (A) `pkg/pipeline/engine/` 서브패키지 → Go에서 서브패키지는 별도 패키지이므로 Checkpoint 등에 접근하려면 import 필요. 양방향 참조 위험.
- (B) `pkg/orchestrator/` 새 패키지 → `pkg/orchestra/`와 이름 혼동, 기존 인프라 재활용 불가

### D3: `ExecutionBackend` 재활용

**결정**: 새 실행 레이어를 만들지 않고 `pkg/orchestra/backend.go`의 `ExecutionBackend`를 그대로 사용

**이유**: `subprocessBackend`가 이미 CLI 호출, stdin/stdout 관리, 타임아웃, JSON 검증을 모두 처리. Phase별 프롬프트만 적절히 구성하면 됨.

### D4: Phase간 결과 전달 방식

**결정**: subprocess stdout → Go 코드에서 파싱 → 다음 Phase 프롬프트에 텍스트로 주입

**이유**: 각 CLI (claude/codex/gemini)의 출력 형식이 다를 수 있으므로, Go 레이어에서 정규화하는 것이 안전. JSON 구조화 출력을 강제하면 일부 CLI에서 호환성 문제 발생 가능.

**대안 검토**:
- (A) 파일 기반 전달 (임시 파일에 쓰고 다음 Phase가 읽음) → 파일 관리 복잡성 증가, 하지만 대형 결과에는 필요할 수 있음
- (B) JSON 구조화 강제 → 모든 CLI가 `--output-format json`을 지원하지 않음
- **결론**: 텍스트 주입을 기본으로, 64KB 초과 시 파일 기반 폴백 (기존 `maxStdinLen` 패턴 재활용)

### D5: Worktree 관리

**결정**: `worktree-safety.md` 규칙을 Go 코드로 포팅 (최대 5 worktree, exponential backoff, GC 억제)

**이유**: 기존 규칙이 실전에서 검증된 안전 장치. 규칙을 코드로 하드코딩하면 플랫폼 무관하게 동작.

## 리스크

| 리스크 | 심각도 | 완화 |
|--------|--------|------|
| CLI 출력 형식 파싱 실패 | High | 정규화 레이어 + 폴백 (raw text) |
| 대형 프롬프트 stdin 한계 | Medium | 기존 `maxStdinLen` 파일 폴백 재사용 |
| Worktree lock contention | Medium | Exponential backoff (3s/6s/12s) |
| 플랫폼별 CLI 업데이트로 인자 변경 | Low | `autopus.yaml`에서 args 설정 가능 |
