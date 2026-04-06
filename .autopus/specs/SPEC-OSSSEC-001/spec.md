# SPEC-OSSSEC-001: 오픈소스 공개 전 보안 강화

**Status**: completed
**Created**: 2026-04-06
**Domain**: OSSSEC
**Priority**: P0 (Must) / P1 (Should) — MoSCoW

## 목적

autopus-adk를 오픈소스로 공개할 때, ADK Worker가 Autopus 서버와 로컬 환경 사이의 "브릿지/창구" 역할을 하는 구조에서 발생할 수 있는 보안 위험을 체계적으로 제거한다. 현재 "security through obscurity"에 의존하는 부분을 식별하고, 소스 코드가 완전히 공개된 상태에서도 안전한 구조로 전환한다.

## 현재 보안 아키텍처

```
Autopus Server → A2A WebSocket (Bearer Token) → ADK Worker → SecurityPolicy 검증 → Subprocess → 결과 반환
```

5층 보안 구조:
1. Bearer Token 인증 (A2A WebSocket)
2. SecurityPolicy 캐싱 (per-task, /tmp/autopus-{uid}/, 0600)
3. 명령어 allowlist (fail-closed ValidateCommand())
4. Worktree 격리 + 세마포어
5. EmergencyStop (SIGTERM → SIGKILL 프로세스 그룹)

## 요구사항

### P0 — Must (공개 전 필수)

**REQ-01: Credential 저장소 마이그레이션**
WHEN the ADK stores authentication credentials (OAuth tokens, API keys),
THE SYSTEM SHALL use OS-native secure storage (macOS Keychain, Linux libsecret, Windows Credential Manager) instead of plaintext `~/.config/autopus/credentials.json`,
SO THAT credential theft via file system access is prevented.

**REQ-02: 평문 fallback 경고**
WHERE OS keychain is unavailable (headless server, container environment),
THE SYSTEM SHALL fall back to encrypted-at-rest file storage (AES-256-GCM with user-derived key) and emit a warning at startup,
SO THAT users are aware of the reduced security posture.

**REQ-03: 하드코딩 Secret 전수 스캔**
WHEN preparing for open-source release,
THE SYSTEM SHALL pass a comprehensive secret scan (gitleaks/trufflehog) on both current code and full git history with zero findings,
SO THAT no internal API URLs, test tokens, or server secrets are leaked.

**REQ-04: ValidateCommand() 강화**
WHEN validating commands against SecurityPolicy,
THE SYSTEM SHALL normalize input (null byte stripping, unicode NFC normalization, path canonicalization) before pattern matching,
SO THAT bypass via encoding tricks (null byte injection, unicode confusables, symlink traversal) is prevented.

**REQ-05: ValidateCommand() prefix match 강화**
WHEN checking commands against AllowedCommands,
THE SYSTEM SHALL use word-boundary-aware matching (command + space or command + end-of-string) instead of simple `strings.HasPrefix`,
SO THAT prefix collision attacks (e.g., `go` matching `gobuster`) are prevented.

**REQ-06: .gitignore 완전성 검증**
WHEN the repository is published,
THE SYSTEM SHALL include `credentials.json`, `.autopus/cache/`, `*.pem`, `*.key`, `/tmp/autopus-*/` patterns in `.gitignore`,
SO THAT sensitive files are never accidentally committed.

**REQ-07: DeniedPatterns regex 안전성**
WHEN compiling DeniedPatterns regex,
THE SYSTEM SHALL enforce a compilation timeout (100ms) and pattern length limit (1024 chars),
SO THAT ReDoS (Regular Expression Denial of Service) attacks via malicious policy injection are prevented.

### P1 — Should (공개 후 빠르게)

**REQ-08: 바이너리 서명 및 체크섬**
WHEN releasing ADK binaries via goreleaser,
THE SYSTEM SHALL sign releases with cosign and publish SHA256 checksums,
SO THAT users can verify binary integrity and provenance.

**REQ-09: SECURITY.md 작성**
WHEN the repository is published,
THE SYSTEM SHALL include a SECURITY.md file specifying vulnerability reporting process, supported versions, and responsible disclosure timeline,
SO THAT security researchers have a clear reporting channel.

**REQ-10: Dependency audit 자동화**
WHEN CI runs on pull requests and weekly schedule,
THE SYSTEM SHALL execute `govulncheck` and `go.sum` verification,
SO THAT known vulnerable dependencies are detected before merge.

**REQ-11: SecretScanner 패턴 확장**
WHEN scanning subprocess output for secrets,
THE SYSTEM SHALL detect additional patterns: GCP service account keys, Azure client secrets, SSH private key headers, and Autopus-specific JWT format,
SO THAT output redaction coverage matches the expanded attack surface of open-source usage.

**REQ-12: PolicyCache 경로 race condition 방지**
WHEN writing SecurityPolicy to the cache directory,
THE SYSTEM SHALL verify the target path is not a symlink before write (TOCTOU mitigation via O_NOFOLLOW),
SO THAT symlink attacks replacing the policy file are prevented.

## 영향 범위

| 패키지 | 변경 유형 | 관련 REQ |
|--------|----------|---------|
| `pkg/worker/setup/auth.go` | Keychain 추상화 추가 | REQ-01, REQ-02 |
| `pkg/worker/setup/apikey.go` | Keychain 백엔드 전환 | REQ-01 |
| `pkg/worker/setup/status.go` | Keychain 읽기 경로 | REQ-01 |
| `pkg/worker/security/types.go` | ValidateCommand 강화 | REQ-04, REQ-05, REQ-07 |
| `pkg/worker/security/policy_cache.go` | symlink 방어 | REQ-12 |
| `pkg/worker/security/secret_scanner.go` | 패턴 확장 | REQ-11 |
| `.gitignore` | 패턴 추가 | REQ-06 |
| `SECURITY.md` (신규) | 보안 정책 문서 | REQ-09 |
| `.github/workflows/` | govulncheck, gitleaks CI | REQ-03, REQ-10 |
| `.goreleaser.yml` | cosign 서명 | REQ-08 |
