# SPEC-OSSSEC-001 수락 기준

## P0 시나리오

### S1: Keychain credential 저장 및 로드
- Given: macOS 또는 Linux with libsecret이 설치된 환경
- When: `auto worker setup`으로 OAuth 인증 완료
- Then: credentials가 OS Keychain에 저장되고, `~/.config/autopus/credentials.json`에는 평문 토큰이 없음
- And: `LoadAuthToken()`이 Keychain에서 토큰을 정상 반환

### S2: Keychain 불가 환경 fallback
- Given: headless server 또는 container에서 Keychain 접근 불가
- When: `auto worker setup`으로 인증 완료
- Then: AES-256-GCM 암호화된 파일로 저장되고, startup 시 "Using encrypted file storage (keychain unavailable)" 경고 출력

### S3: 기존 평문 credentials 마이그레이션
- Given: `~/.config/autopus/credentials.json`에 평문 토큰이 존재
- When: `auto worker setup` 또는 `auto worker start` 실행
- Then: 평문 토큰이 Keychain으로 이동하고, 원본 파일이 zero-fill 후 삭제됨

### S4: ValidateCommand null byte injection 차단
- Given: SecurityPolicy에 `AllowedCommands: ["git "]` 설정
- When: `ValidateCommand("git\x00 rm -rf /", "/work")` 호출
- Then: null byte가 제거된 "git rm -rf /"로 평가되어 denied pattern 또는 allowlist 검증 적용

### S5: ValidateCommand unicode normalization
- Given: SecurityPolicy에 `DeniedPatterns: ["rm -rf"]` 설정
- When: 유니코드 confusable 문자로 `rm` 변형된 명령어 입력
- Then: NFC 정규화 후 패턴 매칭되어 거부

### S6: ValidateCommand prefix collision 방지
- Given: SecurityPolicy에 `AllowedCommands: ["go "]` 설정
- When: `ValidateCommand("gobuster scan", "/work")` 호출
- Then: "go " prefix가 아닌 word-boundary 검증으로 거부 (false, "command not in allowed list")

### S7: DeniedPatterns ReDoS 방어
- Given: 악의적 서버가 ReDoS 패턴 `(a+)+$`을 DeniedPatterns에 주입
- When: `ValidateCommand("aaaaaaaaaaaaaaaaX", "/work")` 호출
- Then: 100ms timeout 초과로 해당 패턴이 deny-all 처리되어 명령 거부

### S8: .gitignore 완전성
- Given: `.gitignore` 파일
- When: `credentials.json`, `*.pem`, `*.key` 파일이 작업 디렉토리에 존재
- Then: `git status`에 untracked로 표시되지 않음

### S9: 하드코딩 Secret 전수 스캔
- Given: autopus-adk 전체 git history
- When: gitleaks 스캔 실행
- Then: 0건의 secret 발견 (기존 발견 건은 allowlist 처리 또는 history rewrite)

## P1 시나리오

### S10: 바이너리 서명 검증
- Given: goreleaser로 빌드된 릴리즈 바이너리
- When: `cosign verify-blob` 실행
- Then: 서명 검증 성공

### S11: SECURITY.md 존재
- Given: 오픈소스 공개된 저장소
- When: 보안 연구자가 SECURITY.md 확인
- Then: 취약점 보고 이메일, 지원 버전, 응답 SLA 정보가 명시

### S12: govulncheck CI
- Given: 취약한 의존성이 포함된 PR
- When: CI가 실행
- Then: govulncheck가 취약점을 감지하고 PR check 실패

### S13: SecretScanner 확장 패턴
- Given: subprocess 출력에 GCP service account key JSON 포함
- When: `SecretScanner.Scan()` 실행
- Then: 해당 부분이 `***REDACTED***`로 대체

### S14: PolicyCache symlink 공격 방어
- Given: `/tmp/autopus-{uid}/autopus-policy-task1.json`이 `/etc/passwd`로의 symlink
- When: `PolicyCache.Write("task1", policy)` 호출
- Then: symlink 감지로 쓰기 거부, 에러 반환
