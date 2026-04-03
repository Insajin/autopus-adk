# SPEC-ONBOARD-001 리서치

## 기존 코드 분석

### install.sh (207줄)

- **에러 함수**: `err()` (line 21) — `printf` 후 `exit 1`. 메시지만 받으며 복구 안내 없음
- **OS 감지**: `detect_os()` (line 24-30) — `uname -s` 기반. Windows/MINGW/MSYS 미감지, "지원하지 않는 OS" 한 줄 에러
- **아키텍처 감지**: `detect_arch()` (line 33-39) — 동일 패턴
- **체크섬**: `verify_checksum()` (line 64-79) — sha256sum/shasum 미발견 시 기술적 에러
- **sudo 사용**: line 118-124 — 권한 확인 후 조건부 sudo, 사전 설명 없음
- **post-install**: line 193-203 — `/auto setup`, `/auto plan`, `/auto fix`, `/auto review` 안내. `auto worker setup` 누락

### internal/cli/worker_setup_wizard.go (206줄)

- **stepDeviceAuth()** (line 57-119):
  - line 96: `"인증 대기 중..."` — 정적 메시지, 카운트다운 없음
  - line 91-93: 브라우저 실패 시 `"브라우저를 수동으로 열어주세요."` — URL 복사 방법 미안내
  - line 65: `fmt.Errorf("generate PKCE: %w", err)` — PKCE 용어 사용자 노출
  - line 69: `fmt.Errorf("request device code: %w", err)` — 기술 에러 체인 노출
  - line 98-99: `context.WithTimeout` 사용 — `dc.ExpiresIn` 초 후 타임아웃. 이 값으로 카운트다운 구현 가능
  - line 101: `setup.PollForToken(ctx, ...)` — `dc.Interval` 초마다 폴링. 이 간격으로 타이머 갱신

### internal/cli/worker_commands.go (241줄)

- **newWorkerSetupCmd()** (line 174-186):
  - `Long`: `"3-step setup: Autopus server auth → workspace selection → provider check"` — 기술적 설명, Worker 개념 없음
  - 241줄로 이미 300줄 경계에 여유 있음

### pkg/worker/setup/provider_auth.go (77줄)

- **checkClaude()** (line 30-36): `"Run \`claude login\` to authenticate"` — 설치 방법 없음, 가입 안내 없음
- **checkCodex()** (line 38-46): `"Set OPENAI_API_KEY or run \`codex login\`"` — 환경변수 개념 전제
- **checkGemini()** (line 49-58): 동일 패턴
- **checkOpencode()** (line 60-66): `"Set OPENAI_API_KEY to authenticate opencode"` — 동일 문제

### pkg/worker/setup/providers.go (106줄)

- `providerPackages` (line 18-23): npm 패키지명 매핑 존재 — 설치 안내에 활용 가능
- `DetectProviders()` (line 29-48): 설치 여부 + 버전 감지
- `InstallProvider()` (line 60-77): npm 자동 설치 함수 존재 — 안내에 이 함수 호출 제안 가능

### internal/cli/init.go (203줄)

- line 189: `tui.Info(out, "Next: run 'auto doctor' to verify installation")` — doctor는 진단 도구이지 워크플로우의 다음 단계가 아님
- `detect.DetectOrchestraProviders()` (line 90) — 프로바이더 감지 로직 이미 존재. worker setup 필요 여부 판단에 활용 가능

### pkg/worker/setup/auth.go (293줄)

- **PollForToken()** (line 124-153): `for` 루프에서 `time.Sleep(interval)` 후 폴링. 카운트다운 로직을 이 루프와 동기화하거나, 별도 goroutine에서 `dc.ExpiresIn`부터 감산하는 방식이 적절
- line 293줄 — 300줄 경계에 가까움. 이 파일에는 코드 추가를 최소화해야 함

## 설계 결정

### D1: 에러 메시지 매핑을 별도 파일로 분리

**결정**: `pkg/worker/setup/messages.go`에 에러 매핑 테이블 신규 생성

**근거**: 
- provider_auth.go(77줄)와 auth.go(293줄)에 인라인으로 넣으면 auth.go가 300줄을 초과
- 메시지 관리의 단일 책임 — 향후 i18n 확장 시 이 파일만 변경
- 테스트 용이성 — 에러 매핑 로직을 독립적으로 테스트 가능

**대안 검토**:
- 각 함수 내 인라인 매핑 → auth.go 300줄 초과 위험, 메시지 중복 가능
- 별도 `errors.go` → "errors"는 Go 표준 라이브러리와 혼동. "messages"가 의도를 더 잘 전달

### D2: 카운트다운 구현 방식

**결정**: `stepDeviceAuth()` 내에서 별도 goroutine으로 카운트다운 출력, `\r` 캐리지 리턴으로 같은 줄 갱신

**근거**:
- `PollForToken()`은 `pkg/worker/setup/auth.go`에 있으며 293줄로 수정 최소화 필요
- goroutine 방식은 폴링 로직을 변경하지 않고 UI만 추가
- `dc.ExpiresIn`(서버 응답)과 `dc.Interval`(폴링 주기)을 이미 활용 가능

**대안 검토**:
- PollForToken에 콜백 주입 → auth.go 300줄 초과 위험, 시그니처 변경으로 영향 범위 확대
- 별도 ticker 기반 → goroutine과 동일하나 코드량 증가

### D3: post-init next step 분기 로직

**결정**: worker config 존재 여부로 분기 — 있으면 `/auto plan`, 없으면 `auto worker setup` 안내

**근거**:
- `setup.DefaultWorkerConfigPath()`로 config 존재 확인 가능 (이미 setup 패키지에 구현됨)
- worker가 필수는 아니지만, 플랫폼 기능(원격 작업 실행)에는 필요
- 사용자가 worker 없이도 로컬에서 사용 가능하므로, 안내는 "선택적 권장" 톤

### D4: install.sh에서의 언어

**결정**: 한국어 유지 (기존 패턴 따름)

**근거**: install.sh는 이미 전체가 한국어 메시지. 프로젝트 language policy의 ai_responses=ko와 일관. 국제화가 필요하면 별도 SPEC으로 처리.
