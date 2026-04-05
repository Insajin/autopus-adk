# SPEC-HARN-PIPE-001 구현 계획

## 태스크 목록

- [ ] T1: `PipelineEngine` 인터페이스 및 `SubprocessEngine` 구현 (`pkg/pipeline/engine.go`)
- [ ] T2: Phase 정의 타입 및 5-Phase 레지스트리 (`pkg/pipeline/phase.go`)
- [ ] T3: Phase별 프롬프트 빌더 — 결과 파싱 및 다음 Phase 주입 (`pkg/pipeline/phase_prompt.go`)
- [ ] T4: Quality Gate 판정 로직 — PASS/FAIL/RETRY (`pkg/pipeline/phase_gate.go`)
- [ ] T5: 순차/병렬 Runner 구현 (`pkg/pipeline/runner.go`)
- [ ] T6: 병렬 실행용 git worktree 관리 (`pkg/pipeline/worktree.go`)
- [ ] T7: `auto pipeline run` CLI 커맨드 (`internal/cli/pipeline_run.go`)
- [ ] T8: 단위 테스트 작성 (각 파일별)
- [ ] T9: 통합 테스트 — 모킹된 subprocess로 전체 파이프라인 실행

## 구현 전략

### 핵심 원칙

1. **기존 인프라 최대 재활용**: `ExecutionBackend` 인터페이스, `subprocessBackend`, `Checkpoint`, `Event`, `MonitorSession`을 그대로 사용
2. **300줄 제한 준수**: Phase 정의, 프롬프트 빌더, 게이트 로직, 러너를 분리 파일로 분할
3. **테스트 가능 설계**: `command` 인터페이스 패턴 (orchestra/command.go)을 활용하여 subprocess 모킹

### 아키텍처

```
auto pipeline run SPEC-ID --platform codex --strategy parallel
         │
         ▼
    [CLI Layer: pipeline_run.go]
         │ config 로드, platform 감지, checkpoint 확인
         ▼
    [PipelineEngine: engine.go]
         │ Phase 순회, 결과 전달, checkpoint 저장
         ▼
    [Runner: runner.go]
         │ sequential: 1개씩 / parallel: worktree 기반 동시 실행
         ▼
    [ExecutionBackend: orchestra/backend.go]
         │ subprocess_runner.go 로 실제 CLI 호출
         ▼
    [Platform CLI: claude/codex/gemini]
```

### 태스크 의존성

```
T1 (engine) → T2 (phase) → T3 (prompt) → T4 (gate)
                                              │
T5 (runner) ←──────────────────────────────────┘
T6 (worktree) ← T5와 병행 가능
T7 (CLI) ← T1~T6 완료 후
T8~T9 (테스트) ← T1~T7 완료 후
```

### 주요 설계 결정

1. **`PipelineEngine` 인터페이스 도입 이유**: Agent() 기반과 subprocess 기반을 동일 인터페이스로 추상화. Claude Code에서는 기존 Agent() 엔진, 다른 플랫폼에서는 SubprocessEngine 사용.
2. **`pkg/pipeline/` 확장 이유**: 기존 `pkg/pipeline/`에 이미 `Checkpoint`, `Event`, `MonitorSession`이 있으므로, 같은 패키지에 엔진 로직을 추가하는 것이 일관성 있음.
3. **`pkg/orchestra/` 재활용 이유**: `ExecutionBackend`, `ProviderRequest`, `ProviderResponse`, `ProviderConfig` 타입이 이미 subprocess 실행에 필요한 모든 것을 제공.
4. **Phase 프롬프트 분리 이유**: 각 Phase의 프롬프트는 이전 Phase 결과를 파싱/주입해야 하므로, 프롬프트 빌드 로직이 복잡해 별도 파일이 필요.
