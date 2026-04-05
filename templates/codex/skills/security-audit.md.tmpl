# auto-security-audit

# Security Audit Skill

OWASP Top 10 기준으로 보안 취약점을 탐지하고 수정하는 스킬입니다.

## OWASP Top 10 체크리스트

### A01: 접근 제어 실패
```go
// ❌ 위험: 권한 확인 없음
func GetUserData(userID string) (*User, error) {
    return db.FindUser(userID)
}

// ✅ 안전: 권한 확인 포함
func GetUserData(ctx context.Context, requestorID, targetUserID string) (*User, error) {
    if !hasPermission(ctx, requestorID, "read:user", targetUserID) {
        return nil, ErrUnauthorized
    }
    return db.FindUser(targetUserID)
}
```

### A02: 암호화 실패
- [ ] 민감 데이터 전송 시 TLS 사용
- [ ] 비밀번호 해싱 (bcrypt, argon2)
- [ ] 하드코딩된 시크릿 없음
- [ ] 최신 암호화 알고리즘 사용

### A03: 인젝션
```go
// ❌ SQL 인젝션 취약점
query := fmt.Sprintf("SELECT * FROM users WHERE name = '%s'", userInput)

// ✅ 파라미터화 쿼리
query := "SELECT * FROM users WHERE name = $1"
db.QueryRow(query, userInput)
```

### A04: 불안전한 설계
- [ ] 위협 모델링 완료
- [ ] 최소 권한 원칙 적용
- [ ] 심층 방어 전략

### A05: 보안 설정 오류
- [ ] 기본 비밀번호 변경
- [ ] 불필요한 포트/서비스 비활성화
- [ ] 에러 메시지에 민감 정보 미포함

### A06: 취약하고 오래된 컴포넌트
```bash
# Go 의존성 취약점 스캔
govulncheck ./...

# 최신 버전 확인
go list -u -m all
```

### A07: 식별 및 인증 실패
- [ ] 강력한 패스워드 정책
- [ ] 세션 만료 처리
- [ ] 브루트포스 방지 (레이트 리밋)

### A08: 소프트웨어 및 데이터 무결성 실패
- [ ] 의존성 검증 (go.sum)
- [ ] 코드 서명
- [ ] CI/CD 파이프라인 보안

### A09: 보안 로깅 및 모니터링 실패
```go
// 보안 이벤트 로깅
log.WithFields(log.Fields{
    "event":     "auth_failed",
    "user_ip":   clientIP,
    "user_id":   userID,
    "timestamp": time.Now(),
}).Warn("인증 실패")
```

### A10: 서버 사이드 요청 위조 (SSRF)
- [ ] 외부 URL 요청 시 화이트리스트 적용
- [ ] 내부 네트워크 접근 차단

## 감사 출력

```markdown
## 보안 감사 결과

### 위험도 요약
- 심각(Critical): N
- 높음(High): N
- 중간(Medium): N
- 낮음(Low): N

### 발견된 취약점
| ID | 파일:라인 | 유형 | 위험도 | 설명 |

### 권장 수정 사항
1. [CVE/CWE] [파일:라인] — [수정 방법]
```
