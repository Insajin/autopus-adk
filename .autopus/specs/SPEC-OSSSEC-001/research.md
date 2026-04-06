# SPEC-OSSSEC-001 리서치

## 기존 코드 분석

### 1. Credential 저장 현황

**현재 구현: `pkg/worker/setup/auth.go:270-292`**
- `SaveCredentials()`: `~/.config/autopus/credentials.json`에 평문 JSON 저장
- 디렉토리 0700, 파일 0600 권한 설정 (기본 보안은 있음)
- `map[string]any` 형태로 유연하게 저장

**`pkg/worker/setup/apikey.go:12-18`**
- `SaveAPIKeyCredentials()`: 주석에 "plaintext — treated like a long-lived password" 명시
- API Key + backend URL + created_at 저장

**`pkg/worker/setup/status.go:46-56`**
- `loadRawCredentials()`: `DefaultCredentialsPath()`에서 직접 파일 읽기
- `rawCredentials` struct: AccessToken, RefreshToken, APIKey, AuthType 등

**위험 평가:**
- 평문 파일이므로 같은 사용자 권한의 다른 프로세스가 읽을 수 있음
- macOS에서 Time Machine 백업에 포함될 수 있음
- 파일 시스템 접근 권한만으로 전체 인증 토큰 탈취 가능

### 2. ValidateCommand 분석

**현재 구현: `pkg/worker/security/types.go:24-68`**

취약점 식별:
1. **Prefix match 문제 (line 43-48)**: `strings.HasPrefix(command, allowed)` — `"go"` allowed가 `"gobuster"` 허용
2. **입력 정규화 없음**: null byte, unicode confusable 문자에 대한 처리 없음
3. **DeniedPatterns regex 무제한 (line 31-38)**: `regexp.Compile`에 timeout 없음 → ReDoS 가능
4. **AllowedDirs prefix match (line 54-59)**: `strings.HasPrefix(workDir, dir)` — `/home/user` allowed가 `/home/user-evil` 허용

**fail-closed 원칙은 올바르게 적용됨 (line 26-28)**

### 3. PolicyCache 분석

**현재 구현: `pkg/worker/security/policy_cache.go`**
- `/tmp/autopus-{uid}/` 디렉토리 사용 (line 18)
- atomic write: temp file → rename (line 24-62)
- 파일 권한 0600 (line 41)

**위험:**
- symlink 체크 없음: 공격자가 policy 파일 경로에 symlink 생성 가능
- `/tmp` 디렉토리는 다른 사용자도 접근 가능 (sticky bit이지만 race condition 가능)

### 4. SecretScanner 분석

**현재 구현: `pkg/worker/security/secret_scanner.go:17-39`**

현재 패턴 커버리지:
- OpenAI/Stripe API keys (`sk-`)
- AWS access keys (`AKIA`)
- GitHub tokens (`ghp_`, `gho_`)
- Bearer tokens
- Generic secret assignments
- AWS secret keys

**미커버 패턴:**
- GCP service account JSON keys
- Azure client secrets
- SSH private key headers
- Autopus 자체 JWT (`eyJ` prefix)
- Private key PEM 블록

### 5. .gitignore 현황

**현재 파일: `autopus-adk/.gitignore`**

포함됨: `bin/`, `dist/`, `.autopus/cache/docs/`, `.claude/settings.json`
미포함: `credentials.json`, `*.pem`, `*.key`, `.env`, `.env.*`

### 6. A2A 인증 경로

**`pkg/worker/poll/poller.go:76`**: `req.Header.Set("Authorization", "Bearer "+p.authToken)`
**`pkg/worker/mcpserver/tools.go:126`**: 동일 패턴
**`pkg/worker/scheduler/dispatcher.go:118`**: 동일 패턴

authToken은 `LoadAuthToken()` (apikey.go:40) → `loadRawCredentials()` → 평문 파일에서 로드.

## 설계 결정

### D1: Credential Store — OS Keychain 우선

**결정**: go-keyring 라이브러리로 OS-native keychain 사용, fallback으로 암호화 파일

**근거:**
- OS keychain은 프로세스 격리, 메모리 보호, 잠금 화면 연동 등 OS-level 보안 제공
- macOS Keychain은 Secure Enclave 연동 가능
- Docker/CI 환경에서는 keychain이 없으므로 fallback 필수

**대안 검토:**
- (A) Hashicorp Vault: 과도한 인프라 의존성, CLI 도구에 부적합
- (B) age 암호화: 좋은 도구이나 키 관리 문제가 credential 저장과 동일
- (C) 환경 변수만 사용: CI에서는 적합하나, 로컬 개발에서 UX 저하

### D2: ValidateCommand 강화 — 입력 정규화 레이어

**결정**: ValidateCommand 진입점에 정규화 전처리 레이어 추가

**근거:**
- 기존 메서드 시그니처 유지 (하위 호환성)
- 정규화는 검증 로직과 분리 가능 (단일 책임)
- NFC normalization은 Go 표준 확장 라이브러리(`golang.org/x/text`)로 안정적

**대안 검토:**
- (A) 별도 sanitize 함수 호출을 caller에 강제: 호출자 실수 위험
- (B) 정규식만으로 처리: unicode normalization은 regex로 불가

### D3: ReDoS 방어 — context 기반 timeout

**결정**: `regexp.Compile` + goroutine timeout 래퍼

**근거:**
- Go의 regexp는 RE2 기반으로 catastrophic backtracking이 이론적으로 불가하지만, 복잡 패턴에서 선형 시간이라도 100ms 이상 소요 가능
- DeniedPatterns는 서버에서 전달되므로 신뢰할 수 없는 입력으로 취급해야 함
- 패턴 길이 제한 + timeout 이중 방어

**참고**: Go의 `regexp`는 PCRE와 달리 RE2 엔진이므로 exponential backtracking은 발생하지 않음. 그러나 서버가 compromise된 경우를 고려하면, 패턴 복잡도 자체를 제한하는 것이 defense-in-depth.

### D4: prefix match → word-boundary match

**결정**: `strings.HasPrefix(command, allowed+" ")` 또는 `command == allowed` 패턴

**근거:**
- 현재 `strings.HasPrefix(command, allowed)` (types.go:44)에서 `"go"` allowed가 `"gobuster"` 매칭
- space separator 또는 exact match로 변경하면 최소 변경으로 해결
- AllowedDirs도 동일 패턴 (`/home/user` → `/home/user/` trailing slash 강제)

## 의존성 라이브러리

| 라이브러리 | 용도 | 상태 |
|-----------|------|------|
| `github.com/zalando/go-keyring` | OS Keychain 추상화 | 신규 추가 필요 |
| `golang.org/x/text/unicode/norm` | Unicode NFC normalization | 신규 추가 필요 |
| `github.com/zricethezav/gitleaks` | Secret 스캔 (CI) | CI action으로만 사용 |
| `sigstore/cosign` | 바이너리 서명 (CI) | CI action으로만 사용 |

## 관련 SPEC

| SPEC | 관계 |
|------|------|
| SPEC-AXSEC-001 | AX annotation security — 실험 회로 차단 (보안 도메인 참고) |
| SPEC-SECBRIDGE-001 | Bridge 보안 강화 (sunset, 패턴 참고) |
| SPEC-ADKW-001 | ADK Worker 전체 아키텍처 (보안층 정의) |
| SPEC-ADKWIRE-003 | Worker wiring (인증 경로 참고) |
| SPEC-OSSUX-001 | 오픈소스 UX (usage profile 연동) |
