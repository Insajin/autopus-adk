# SPEC-ONBOARD-001 구현 계획

## 태스크 목록

- [ ] T1: `pkg/worker/setup/messages.go` 신규 생성 — 에러 메시지 매핑 테이블
- [ ] T2: `install.sh` 에러 메시지 개선 (P0: R1-R3, P1: R10)
- [ ] T3: `internal/cli/worker_setup_wizard.go` Device Auth UX 개선 (P0: R4-R6, P2: R13)
- [ ] T4: `pkg/worker/setup/provider_auth.go` 프로바이더 안내 개선 (P0: R7-R8)
- [ ] T5: `internal/cli/init.go` post-init next steps 수정 (P1: R9)
- [ ] T6: `internal/cli/worker_commands.go` Worker 개념 설명 추가 (P2: R12)
- [ ] T7: 에러 체인 래핑 적용 — wizard/commands에서 messages.go 활용 (P1: R11)

## 구현 전략

### T1: messages.go (독립, 선행 태스크)

`pkg/worker/setup/messages.go`에 에러 패턴 → 사용자 메시지 매핑 함수를 구현한다.

```go
// HumanError maps a technical error to a user-friendly message.
func HumanError(err error) string
```

- 패턴 매칭 기반: `strings.Contains(err.Error(), "HTTP 500")` → 서버 연결 안내
- PKCE/OAuth 용어 필터링: "PKCE", "code_verifier", "device_code" 등은 숨김
- fallback: 매칭되지 않는 에러는 원본 그대로 반환

### T2: install.sh 개선

- `detect_os()`: Windows/WSL 감지 추가, 지원 OS 목록 표시
- `detect_arch()`: 지원 아키텍처 목록 표시
- `verify_checksum()`: sha256sum 미설치 시 OS별 안내
- sudo 실행 전 설명 메시지 추가 (line 122 부근)
- `main()` 종료 메시지에 `auto worker setup` 안내 추가

### T3: Device Auth UX

- `stepDeviceAuth()`에 카운트다운 타이머 goroutine 추가
- `\r` 캐리지 리턴으로 같은 줄에 남은 시간 갱신
- 브라우저 실패 시 URL 수동 복사 안내 강화
- `fmt.Errorf("generate PKCE: %w", err)` → `messages.HumanError()` 래핑

### T4: 프로바이더 안내

- `CheckProviderAuth()` 반환 guide를 단계별 안내로 변경
- 각 프로바이더별 공식 URL 포함
- "환경변수 설정" → 실제 터미널 명령어 형태로 변환

### T5: post-init next steps

- line 189 `tui.Info(out, "Next: run 'auto doctor'...")` 수정
- worker config 존재 여부에 따라 분기: worker setup 안내 또는 /auto plan 안내

### T6: Worker 개념 설명

- `newWorkerSetupCmd()` Long 필드에 Worker 설명 추가
- `runWorkerSetup()` 시작 시 한 줄 개념 설명 출력

### T7: 에러 체인 래핑

- `worker_setup_wizard.go`의 `fmt.Errorf(...: %w)` 패턴에 `messages.HumanError()` 적용
- 사용자에게 직접 노출되는 에러만 래핑 (내부 로깅용 에러는 유지)

## 의존성 그래프

```
T1 (messages.go)
  ├── T3 (wizard에서 사용)
  ├── T7 (에러 래핑에서 사용)
T2 (install.sh) — 독립
T4 (provider_auth) — 독립
T5 (init.go) — 독립
T6 (worker_commands) — 독립
```

T1을 먼저 구현하고, T2/T4/T5/T6은 병렬 실행 가능. T3/T7은 T1 완료 후 진행.

## 파일 크기 준수

모든 수정 대상 파일이 현재 300줄 미만이며, 변경 후에도 300줄을 초과하지 않는다.
- `messages.go` (신규): ~80줄 목표
- `install.sh`: 207 → ~240줄 (셸 스크립트이므로 소스코드 제한 제외 대상은 아니지만, 간결하게 유지)
- `worker_setup_wizard.go`: 206 → ~240줄
- `worker_commands.go`: 241 → ~250줄
- `provider_auth.go`: 77 → ~120줄
- `init.go`: 203 → ~210줄
