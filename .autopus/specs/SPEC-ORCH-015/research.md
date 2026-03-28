# SPEC-ORCH-015 리서치

## 기존 코드 분석

### 프롬프트 감지 체계

핵심 함수 체인:
- `isPromptVisible()` (`interactive_detect.go:105-123`): ANSI strip 후 provider 패턴 → default 패턴 순서로 매칭
- `pollUntilPrompt()` (`interactive.go:155-176`): 500ms 간격 ReadScreen 폴링
- `waitForCompletion()` (`interactive_completion.go:15-46`): 2초 간격 2-phase consecutive match
- `waitForSessionReady()` (`interactive.go:144-152`): `DefaultCompletionPatterns()`으로 `pollUntilPrompt` 호출 (30초 고정)

### 패턴 정의 위치

| 패턴셋 | 위치 | 용도 |
|--------|------|------|
| `defaultPromptPatterns` | `interactive_detect.go:20-27` | `isPromptVisible` fallback, `isPromptLine` 필터 |
| `DefaultCompletionPatterns()` | `types.go:95-102` | `waitForCompletion`, `waitForSessionReady` 전달 |
| `cliNoisePatterns` | `interactive_detect.go:30-59` | output 필터링 (프롬프트 감지 무관) |

**문제점**: `defaultPromptPatterns`에 `^\$\s*$` (shell `$`)와 `^#\s*$` (root `#`)가 포함되어 있어, `waitForSessionReady`에서 CLI 미시작 상태의 shell 프롬프트에 false match 발생.

### opencode stdin 모드

설정 체인:
- `defaults.go:68`: `PromptViaArgs: false` (기본값)
- `migrate.go` `MigrateOpencodeToTUI()`: 기존 `PromptViaArgs: true` → `false` 마이그레이션
- `orchestra_helpers.go:99`: CLI 실행 시 `PromptViaArgs: false` 설정

opencode가 CLI arg를 파일명으로 사용하기 때문에 긴 프롬프트 시 ENAMETOOLONG 발생. stdin 모드로 전환하여 해결.

### waitForCompletion 구조

```
interactive_completion.go:15-46
  ├─ 2초 ticker loop
  ├─ ReadScreen(ctx, paneID, opts)
  ├─ baseline 비교 (debate R2 false positive 방지)
  └─ 2-phase consecutive match (candidateDetected)
```

idle fallback (R7) 추가 위치: `waitForCompletion` 루프 내부에서 `isOutputIdle` 병렬 체크.

### debate executeRound 에러 핸들링

현재 (`interactive_debate.go:204-206`):
```go
_ = cfg.Terminal.SendLongText(ctx, pi.paneID, prompt)
time.Sleep(500 * time.Millisecond)
_ = cfg.Terminal.SendCommand(ctx, pi.paneID, "\n")
```

에러가 `_ =`로 무시됨. 재시도 로직 없음. 빈 응답 핸들링도 없음.

## PoC 검증 결과 (scripts/test-pane-providers.sh)

| Provider | 패턴 감지 | 시간 | 프롬프트 주입 | 응답 수집 | 비고 |
|----------|----------|------|-------------|----------|------|
| claude   | ✓ `❯`   | ~2초 | ✓ buffer mode | ✓ 6초 | MCP 로딩 시 startup 지연 |
| gemini   | ✓ `Type your` | ~6초 | ✓ buffer mode | ✓ 처리중 감지 | 인증 확인 시간 포함 |
| opencode | ✓ `Ask anything` | ~2초 | ✓ buffer mode | △ 완료감지 타이밍 | TUI 렌더링 지연으로 `Ask anything` 재표시 불안정 |

### ReadScreen 캡처 데이터 (실측)

**claude pane**:
```
\x1b[32m❯\x1b[0m
```
→ ANSI strip 후 `❯` 매칭 성공

**gemini pane**:
```
> Type your message or @mention a tool (e.g. @shell)
```
→ `^\s*>\s+(Type your|@)` 매칭 성공

**opencode pane (응답 전)**:
```
Ask anything... (or type /help)
```
→ `Ask anything` 매칭 성공

**opencode pane (응답 후 — 불안정)**:
```
[AI 응답 텍스트]
⬝⬝⬝ esc                                    ctrl+k new session
```
→ `Ask anything` 미표시 상태에서 2-phase 매치 실패. TUI 렌더링 완료까지 추가 대기 또는 idle fallback 필요.

## 설계 결정

### D1: waitForSessionReady 전용 패턴셋

**결정**: `SessionReadyPatterns()` 함수로 분리, CLI 프롬프트만 포함.

**대안 검토**:
- (A) `defaultPromptPatterns`에서 shell 패턴 제거 → `isPromptLine` 필터에서 shell 프롬프트 제거 부작용
- (B) `waitForSessionReady`에 명시적 sleep 추가 → 비결정적, 프로바이더마다 다름
- **(C) 전용 패턴셋** → 기존 로직 영향 없이 session ready에만 적용. 채택.

### D2: idle fallback for opencode

**결정**: `waitForCompletion` 내에서 `isOutputIdle(outputFile, 15s)` 보조 체크.

**대안 검토**:
- (A) timeout 연장 (60초) → 다른 프로바이더 대기 시간 증가
- (B) pipe-pane 파일 기반 단독 감지 → prompt 기반 감지의 정확도 포기
- **(C) 2-phase + idle 하이브리드** → prompt 감지 우선, 실패 시 idle fallback. 채택.

### D3: debate 재시도 정책

**결정**: 1회 재시도, 500ms 대기 후 재시도, 2회 실패 시 skip.

**대안 검토**:
- (A) 무제한 재시도 → debate round timeout 소진 위험
- (B) 즉시 skip → 일시적 네트워크/TTY 이슈에 취약
- **(C) 1회 재시도** → 간단하고 timeout 예산 영향 최소. 채택.
