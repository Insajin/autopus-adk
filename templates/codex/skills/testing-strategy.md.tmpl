# auto-testing-strategy

# Testing Strategy Skill

단위 테스트를 넘어 통합/E2E/계약 테스트를 설계하는 스킬입니다.

## 테스트 피라미드

```
        /  E2E  \        ← 적게, 핵심 플로우만
       /  통합   \       ← 중간, 컴포넌트 간 연동
      /   단위    \      ← 많이, 빠르고 격리
```

| 계층 | 비중 | 속도 | 범위 |
|------|------|------|------|
| 단위 | 70% | 빠름 (ms) | 함수/메서드 |
| 통합 | 20% | 중간 (s) | 컴포넌트 간 |
| E2E | 10% | 느림 (min) | 전체 시스템 |

## 통합 테스트

### 데이터베이스 통합
```go
func TestUserRepo_Integration(t *testing.T) {
    if testing.Short() {
        t.Skip("통합 테스트 건너뜀")
    }
    db := setupTestDB(t)
    defer db.Close()

    repo := NewUserRepo(db)
    user, err := repo.Create(ctx, &User{Name: "test"})
    require.NoError(t, err)

    found, err := repo.FindByID(ctx, user.ID)
    require.NoError(t, err)
    assert.Equal(t, "test", found.Name)
}
```

### HTTP API 통합
```go
func TestAPI_CreateUser(t *testing.T) {
    srv := setupTestServer(t)
    defer srv.Close()

    resp, err := http.Post(srv.URL+"/api/v1/users",
        "application/json",
        strings.NewReader(`{"name":"test"}`))
    require.NoError(t, err)
    assert.Equal(t, http.StatusCreated, resp.StatusCode)
}
```

## E2E 테스트

### 핵심 플로우만 테스트
```
사용자 등록 → 로그인 → 리소스 생성 → 조회 → 삭제
```

### 원칙
- 핵심 비즈니스 플로우만 (5-10개)
- 깨지기 쉬운 UI 셀렉터 피하기
- 테스트 데이터 격리 (각 테스트 독립)
- CI에서 실행 시 타임아웃 설정

## 계약 테스트 (Contract Testing)

서비스 간 API 계약을 검증:
```go
// Provider 측: 계약 준수 검증
func TestProvider_FulfillsContract(t *testing.T) {
    // 컨슈머가 정의한 계약 로드
    // Provider API가 계약을 충족하는지 검증
}

// Consumer 측: 기대하는 응답 정의
func TestConsumer_ExpectedResponse(t *testing.T) {
    // Mock provider 응답 설정
    // Consumer가 올바르게 처리하는지 검증
}
```

## 테스트 격리 전략

| 전략 | 용도 | 예시 |
|------|------|------|
| `t.Parallel()` | 독립적 단위 테스트 | CPU 바운드 로직 |
| `TestMain` | DB 셋업/티어다운 | 통합 테스트 스위트 |
| `t.Cleanup` | 리소스 정리 | 임시 파일, 포트 |
| Build tags | 테스트 유형 분리 | `//go:build integration` |

## 커버리지 전략

```bash
# 전체 커버리지
go test -coverprofile=coverage.out ./...

# 패키지별 커버리지 확인
go tool cover -func=coverage.out

# 미커버 라인 시각화
go tool cover -html=coverage.out
```

목표:
- 전체: 85%+
- 핵심 비즈니스 로직: 95%+
- 유틸리티/헬퍼: 80%+
- 생성된 코드: 제외

## 테스트 전략 체크리스트

- [ ] 테스트 피라미드 비율 준수 (70/20/10)
- [ ] 통합 테스트에 `testing.Short()` 가드
- [ ] E2E는 핵심 플로우만 (10개 이하)
- [ ] 테스트 데이터 격리
- [ ] CI에서 전체 스위트 실행
- [ ] 커버리지 85%+ 달성
