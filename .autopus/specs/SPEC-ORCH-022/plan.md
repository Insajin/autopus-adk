# SPEC-ORCH-022 Plan: Claude Code 안 hook-IPC headless 멀티프로바이더 실행 경로

## Implementation Strategy

재발명 금지가 핵심이다. 수신측(`HookSession`, `WaitAndCollectHookResults`, `FileIPCDetector`)은 SPEC-ORCH-007 구현을 그대로 재사용한다. 본 SPEC은 (a) 송신측 등록(T1/T2), (b) 가용성 인식(T3), (c) backend 배선(T6), (d) headless entry(T4), (e) 진입 경로(T5), (f) 에러 경계(T7)만 추가/수정한다.

floor 보존을 우선한다. 모든 변경은 hook/cmux 가용을 추가 조건으로만 사용하고, 미가용 경로는 기존 subprocess/plain 동작을 그대로 둔다. T5의 가드 완화는 `claudeCode!="" && !hookAvailable`이면 기존과 동일하게 plain을 반환해야 한다(회귀 금지).

파일 크기를 지킨다. `headless_hook_runner.go`는 launch/orchestrate만 담아 300줄 미만으로 유지한다. `hooks.go`는 현재 278줄이라 Stop/AfterAgent 분기 추가 시 한도에 근접하면 `generateSignalHooks` 헬퍼로 분리한다.

순서는 T1 → T2(등록) → T3(인식) → T6(backend) → T4(headless) → T5(진입) → T7(에러)다. T5는 T3·T4 완성 후에 진입을 열어야 회귀가 안전하다.

## File Impact Analysis

| 파일 | 작업 | 설명 |
|------|------|------|
| `pkg/content/hooks.go` | 수정 | `generateCLIHooks`에 Stop(claude)/AfterAgent(gemini)/Stop(codex) 이벤트 추가 |
| `internal/cli/orchestra_terminal.go` | 수정 | `paneInteractiveContext`에 hookAvailable·muxInstalled 입력 추가 |
| `internal/cli/orchestra_helpers.go` | 수정 | `isHookModeAvailable`가 project-local settings.json도 검사 |
| `pkg/orchestra/pane_backend.go` | 수정 | line 122 nil 하드코딩 제거, HookMode일 때 실 HookSession 생성 |
| `internal/cli/spec_review_loop.go` | 수정 | both-failed actionable 에러 경로 |
| `internal/cli/orchestra_run.go` | 수정 | both-failed actionable 에러 경로 |
| `[NEW] pkg/orchestra/headless_hook_runner.go` | 생성 | headless hook-IPC 수집 entry |
| `[NEW] pkg/orchestra/headless_hook_runner_test.go` | 생성 | 수집/타임아웃 oracle 테스트 |
| `[NEW] pkg/content/hooks_stop_test.go` | 생성 | Stop 이벤트 등록 oracle 테스트 |

## Architecture Considerations

기존 레이어 방향(`internal/cli` → `pkg/orchestra` → `pkg/terminal`)을 유지한다. `pkg/content`와 `pkg/adapter/claude`의 init 경로는 기존 `InstallHooks`(임의 이벤트 기록 지원, `claude_settings.go:79-95`)를 재사용하므로 새 스키마 코드를 추가하지 않는다. `content/`가 source of truth이고 `.claude/hooks/autopus/`는 generated install copy다 — 스크립트 자체는 `claude_prepare_files.go:56`에서 이미 복사되므로 T1은 settings.json 등록만 추가한다.

## Visual Planning Brief

작업 성격은 CLI/backend 실행 경로 분기 + 완료 신호 IPC다. command-flow + sequence가 적합하다(research.md에 상세 sequence 기재).

백엔드 선택 분기:
```
auto init --> generateCLIHooks(+Stop/AfterAgent) --> InstallHooks --> .claude/settings.json:hooks.Stop
orchestra/spec-review --> detectStructuredTerminal(isHookModeAvailable, cmuxInstalled)
  CLAUDECODE && hookAvail && cmux --> CmuxAdapter --> PaneBackend(HookSession!=nil) --> FileIPCDetector --> done-file
  else ----------------------------> PlainAdapter --> subprocess -p (floor)
  neither possible ----------------> actionable error
```

headless hook 수집 sequence:
```
Orchestrator         cmux daemon         provider CLI(구독)        /tmp/autopus/{sid}/
  NewHookSession(sid) ----------------------------------------> mkdir 0o700
  setenv AUTOPUS_SESSION_ID, AUTOPUS_ROUND
  cmux new-split -----> surface
  SendLongText(launch+prompt) --------> 인터랙티브 세션 시작
                                         Stop hook 실행 -------> claude-result.json(0o600), claude-done
  FileIPCDetector.WaitForDone <폴링 200ms> ...detect <-------- claude-done
  [timeout 시] bounded deadline 경과 --> false (무한 hang 금지)
```

## Tasks

- [ ] T1: claude `Stop` hook 등록 (REQ-001, REQ-010). `generateCLIHooks`에 `Event:"Stop"` HookConfig 추가. 스크립트 복사는 이미 됨, settings.json 기록은 `InstallHooks` 자동 처리. 검증: `[NEW] pkg/content/hooks_stop_test.go::TestGenerateCLIHooks_StopEvent`.
- [ ] T2: gemini `AfterAgent` + codex `Stop` 등록 (REQ-002). `translateHookEvent`에 `Stop -> AfterAgent`(gemini) 매핑 추가, codex는 `Stop` 그대로.
- [ ] T3: `isHookModeAvailable` project-local 인식 (REQ-003). `os.Getwd()` 기준 project `.claude/settings.json`도 검사, user-global 또는 project-local 중 하나라도 `autopus`+`Stop`이면 true.
- [ ] T4: headless hook-IPC 수집 entry (REQ-004, REQ-007, REQ-010). `[NEW] headless_hook_runner.go` — cmux surface 위 launch + `WaitAndCollectHookResults` 재사용 + bounded timeout.
- [ ] T5: CLAUDECODE 가드 완화 (REQ-005, REQ-008). `paneInteractiveContext` 시그니처에 hookAvailable·muxInstalled 추가, hook-IPC 가능 시 CLAUDECODE에서도 true, 그 외 plain 유지.
- [ ] T6: PaneBackend 실 HookSession (REQ-006, REQ-007). `pane_backend.go:122` nil 제거, HookMode일 때 `NewHookSession` 생성·전달 + defer Cleanup.
- [ ] T7: both-failed actionable 에러 (REQ-009, REQ-008). hook도 subprocess도 불가할 때 복구 지침(`auto init`/cmux/API 키) 포함 에러 반환.

## Risks & Mitigations

| 리스크 | 영향도 | 대응 |
|--------|--------|------|
| 가드 완화가 plain/CI floor를 깸 | 높음 | T5에서 hookAvailable=false면 기존 plain 경로 그대로, truth-table oracle 테스트로 회귀 잠금 |
| cmux daemon 부재 시 hang | 중간 | `cmux new-split` 실패 시 즉시 subprocess degrade(REQ-008) |
| hooks.go 300줄 초과 | 낮음 | `generateSignalHooks` 헬퍼 분리 |

## Feature Completion Scope

Primary SPEC이 Outcome Lock의 모든 mandatory slice(T1~T7)를 닫는다. 승인된 sibling 의존성 없음. 남은 Completion Debt는 None(research.md `## Completion Debt`). 검증 범위 한계(Debt 아님)는 실제 cmux daemon + Claude Code Bash end-to-end 0/N 부재 확인으로, go 단계에서는 결정론적 oracle(detector 선택, settings.json 등록, bounded timeout, degrade routing)로 닫고 실 환경 e2e는 sync 단계 수동 caveat로 남긴다.

## Dependencies

내부: `pkg/orchestra`(HookSession, FileIPCDetector, WaitAndCollectHookResults), `pkg/terminal`(CmuxAdapter), `pkg/content`, `pkg/adapter/claude`. 외부 신규 라이브러리 없음. cmux는 외부 CLI 바이너리(설치 전제). SPEC-ORCH-007/021 산출물에 의존.

## Exit Criteria

- [ ] REQ-001 ~ REQ-010 구현 완료
- [ ] S1 ~ S10 oracle 테스트 통과
- [ ] 신규 소스 파일 300줄 이하
- [ ] hook 미설치/CI에서 subprocess floor 회귀 없음
