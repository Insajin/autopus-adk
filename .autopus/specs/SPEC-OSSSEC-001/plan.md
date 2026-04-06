# SPEC-OSSSEC-001 구현 계획

## 태스크 목록

### P0 태스크 (공개 전 필수)

- [ ] T1: Credential Store 추상화 인터페이스 정의
  - `pkg/worker/setup/credstore.go` 신규: `CredentialStore` interface (`Save`, `Load`, `Delete`)
  - 기존 `SaveCredentials()` / `loadRawCredentials()` 시그니처 유지하며 내부를 interface로 위임

- [ ] T2: OS Keychain 백엔드 구현
  - `pkg/worker/setup/credstore_keychain.go`: macOS Keychain (via go-keyring)
  - `pkg/worker/setup/credstore_keychain_test.go`: 통합 테스트 (CI에서는 skip)
  - `go-keyring` 라이브러리 사용 (macOS/Linux/Windows 지원)

- [ ] T3: Encrypted-file fallback 백엔드 구현
  - `pkg/worker/setup/credstore_file.go`: AES-256-GCM + user-derived key (PBKDF2)
  - Keychain 불가 환경 자동 감지 후 fallback
  - startup 시 warning 로그 출력

- [ ] T4: 기존 평문 credentials.json 마이그레이션 경로
  - `auto worker setup` 실행 시 기존 평문 파일 감지 → Keychain으로 자동 마이그레이션
  - 마이그레이션 후 평문 파일 secure-delete (zero-fill + unlink)

- [ ] T5: ValidateCommand() 입력 정규화
  - null byte 제거: `strings.ReplaceAll(cmd, "\x00", "")`
  - unicode NFC 정규화: `golang.org/x/text/unicode/norm`
  - path canonicalization: `filepath.Clean` + symlink resolution
  - prefix match → word-boundary match 변경

- [ ] T6: DeniedPatterns ReDoS 방어
  - `regexp.CompileTimeout` 래퍼 (100ms context deadline)
  - 패턴 길이 1024자 제한
  - 컴파일 실패 시 해당 패턴을 deny-all로 처리 (fail-closed)

- [ ] T7: .gitignore 완전성 보강
  - `credentials.json`, `*.pem`, `*.key`, `.env`, `.env.*` 패턴 추가
  - 기존 `.autopus/cache/` 패턴 확인 (이미 존재)

- [ ] T8: 하드코딩 Secret 스캔 CI 추가
  - `.github/workflows/security.yml`: gitleaks action (push + PR trigger)
  - `.gitleaks.toml` 설정 파일 (allowlist for test fixtures)
  - 1회성 full history 스캔 실행 및 결과 확인

### P1 태스크 (공개 후)

- [ ] T9: goreleaser cosign 서명 설정
  - `.goreleaser.yml`에 cosign sign 단계 추가
  - GitHub Actions에서 cosign keyless signing (OIDC)

- [ ] T10: SECURITY.md 작성
  - 취약점 보고 이메일, PGP 키 (선택)
  - 지원 버전 정책 (latest release only)
  - 응답 SLA: 확인 48시간, 패치 7일

- [ ] T11: govulncheck CI 자동화
  - `.github/workflows/security.yml`에 govulncheck step 추가
  - weekly cron + PR trigger

- [ ] T12: SecretScanner 패턴 확장
  - GCP service account JSON key pattern
  - Azure client secret pattern
  - SSH private key header (`-----BEGIN.*PRIVATE KEY-----`)
  - Autopus JWT pattern (`eyJ` prefix + specific claims)

- [ ] T13: PolicyCache symlink 방어
  - `os.Lstat` → symlink 감지 → 거부
  - `O_NOFOLLOW` 플래그로 파일 열기 (unix)
  - 테스트: symlink 공격 시나리오

## 구현 전략

### 기존 코드 활용
- `SaveCredentials()` (auth.go:270) / `loadRawCredentials()` (status.go:46) 의 내부 구현만 교체, 외부 API 유지
- `ValidateCommand()` (types.go:24) 메서드 시그니처 유지, 내부 로직 강화
- `SecretScanner.defaultPatterns()` (secret_scanner.go:17) 배열 확장

### 변경 범위 최소화
- Credential Store는 interface로 추상화하여 기존 코드 변경 최소화
- ValidateCommand 강화는 입력 정규화 레이어를 앞에 추가하는 방식
- 기존 테스트는 모두 통과해야 함 (regression 없음)

### 의존성
- T1 → T2, T3, T4 (interface 먼저)
- T5, T6, T7, T8은 독립 실행 가능
- T9~T13은 P0 완료 후 순서 무관
