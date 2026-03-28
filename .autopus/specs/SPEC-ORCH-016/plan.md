# SPEC-ORCH-016 구현 계획

## 태스크 목록

- [ ] T1: `pkg/orchestra/interactive_surface.go` — surface 유효성 검증 함수 작성
  - `validateSurface(ctx, term, paneID) bool` — ReadScreen 호출로 surface 활성 여부 판단
  - `needsSurfaceCheck(provider) bool` — 프로바이더별 세션 유지 특성 판단 (claude → skip)

- [ ] T2: `pkg/orchestra/interactive_surface.go` — pane 재생성 함수 작성
  - `recreatePane(ctx, cfg, pi) (paneInfo, error)` — stale pane 정리 후 새 pane 생성, CLI 재실행, pipe 재개

- [ ] T3: `pkg/orchestra/interactive_debate.go` — `executeRound` 수정
  - Round > 1 진입 시 `validateSurface` 호출
  - 실패 시 `recreatePane` 호출, 성공하면 panes 슬라이스 업데이트
  - `SendLongText` 실패 시 기존 retry 대신 recreatePane 1회 시도

- [ ] T4: `pkg/orchestra/interactive_surface_test.go` — 단위 테스트
  - MockTerminal로 ReadScreen 실패 시나리오 검증
  - 재생성 성공/실패 경로 검증
  - Claude(세션 유지) 프로바이더 skip 검증

## 구현 전략

### 접근 방법

Terminal 인터페이스를 변경하지 않고, 기존 `ReadScreen`의 에러 반환을 surface 유효성 프록시로 활용한다. cmux에서 stale surface에 대한 `read-screen`은 에러를 반환하므로 별도의 `IsSurfaceAlive` 메서드 없이도 검증이 가능하다.

### 기존 코드 활용

- `splitProviderPanes` — 새 pane 생성 시 재사용
- `startPipeCapture` — 단일 pane에 대해 pipe 재개
- `launchInteractiveSessions` — 프로바이더 CLI 재실행
- `waitForSessionReady` — 세션 준비 대기
- `cleanupInteractivePanes` — stale pane 정리 (Close + PipePaneStop)

### 변경 범위

- **신규**: `pkg/orchestra/interactive_surface.go` (~80줄)
- **수정**: `pkg/orchestra/interactive_debate.go` — executeRound 내 ~30줄 변경
- **신규**: `pkg/orchestra/interactive_surface_test.go` (~120줄)
- Terminal 인터페이스 변경 없음 → 어댑터(cmux, tmux, plain) 수정 불필요
