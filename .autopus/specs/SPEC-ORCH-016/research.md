# SPEC-ORCH-016 리서치

## 기존 코드 분석

### 문제 발생 지점

`pkg/orchestra/interactive_debate.go` 의 `executeRound` 함수 (L162-249):

```
L206: if err := cfg.Terminal.SendLongText(ctx, pi.paneID, prompt); err != nil {
L207:     log.Printf("[Round %d] %s SendLongText failed: %v — retrying", ...)
L208:     time.Sleep(1 * time.Second)
L209:     if retryErr := cfg.Terminal.SendLongText(ctx, pi.paneID, prompt); retryErr != nil {
L210:         log.Printf("[Round %d] %s SendLongText retry failed: %v — skipping", ...)
L211:         panes[i].skipWait = true
```

현재 R8 retry는 동일한 (stale) surface에 재시도하므로 반드시 실패한다.

### SendLongText 실패 경로

`pkg/terminal/cmux.go` L76-103의 `SendLongText`:
- 500바이트 미만 → `SendCommand` (cmux send --surface) 호출
- 500바이트 이상 → `set-buffer` → `paste-buffer --surface` → `delete-buffer`
- `paste-buffer`가 stale surface에 대해 `exit status 1` 반환

### 프로바이더 세션 유지 특성

| Provider | Binary | 세션 유지 | 비고 |
|----------|--------|-----------|------|
| claude | `claude` | O | `--dangerously-skip-permissions`로 대화 유지 |
| opencode | `opencode` | X | `InteractiveInput: "args"` — `opencode run` 실행 후 종료 |
| gemini | `gemini` | X | 응답 출력 후 CLI 종료 가능 |

### 재사용 가능한 기존 함수

- `splitProviderPanes` (`pkg/orchestra/pane_runner.go` L89-109): pane 생성 + temp file — 단일 프로바이더용으로 추출 필요
- `startPipeCapture` (`pkg/orchestra/interactive.go` L104-111): pipe-pane 시작
- `launchInteractiveSessions` (`pkg/orchestra/interactive.go` L114-141): CLI 실행
- `waitForSessionReady` (`pkg/orchestra/interactive.go` L145-154): 세션 준비 대기
- `cleanupInteractivePanes` (`pkg/orchestra/interactive_launch.go` L51-58): pane 정리
- `ReadScreen` (`pkg/terminal/cmux.go` L146-163): surface 상태 확인 프록시

### paneInfo 구조체

`pkg/orchestra/pane_runner.go` L20-25:
```go
type paneInfo struct {
    paneID     terminal.PaneID
    outputFile string
    provider   ProviderConfig
    skipWait   bool
}
```

paneID를 교체하면 되므로 구조체 변경 불필요.

## 설계 결정

### ReadScreen을 surface 유효성 프록시로 사용

**결정**: Terminal 인터페이스에 `IsSurfaceAlive` 메서드를 추가하지 않고, 기존 `ReadScreen`의 에러 반환으로 surface 상태를 판단한다.

**이유**:
1. Terminal 인터페이스는 `@AX:ANCHOR` — 변경 시 cmux/tmux/plain 3개 어댑터 모두 수정 필요
2. cmux `read-screen --surface surface:N`은 stale surface에서 에러를 반환 (검증됨)
3. ReadScreen은 이미 모든 어댑터에 구현되어 있음
4. 추가 오버헤드: ~5ms (무시 가능)

### 재생성 vs 재시도

**결정**: 동일 surface 재시도 대신 pane 재생성 (close → split → launch → ready-wait)

**이유**: stale surface는 프로세스가 종료된 상태이므로 동일 surface에 대한 재시도는 100% 실패. 새 surface를 만들어 CLI를 재실행해야 함.

### 프로바이더별 skip 판단

**결정**: `needsSurfaceCheck` 함수에서 `InteractiveInput` 필드와 Binary 이름으로 판단

**대안 검토**:
1. ProviderConfig에 `KeepAlive bool` 필드 추가 — 설정 파일 스키마 변경 필요, 과도함
2. 모든 프로바이더에 검증 수행 — ReadScreen 오버헤드 미미하므로 허용 가능하나, claude처럼 확실히 세션을 유지하는 경우 불필요한 로그 노이즈 방지
3. **채택**: Binary 이름 기반 화이트리스트 (`claude` → skip). 새 프로바이더 추가 시 리스트 갱신 필요하나, 현재 프로바이더 수가 적어 유지보수 부담 낮음
