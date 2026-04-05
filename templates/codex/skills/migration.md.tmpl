# auto-migration

# Migration Skill

언어, 프레임워크, 라이브러리 버전 업그레이드를 안전하게 수행하는 스킬입니다.

## 마이그레이션 프로세스

### 1단계: 영향 분석
```bash
# Go 모듈 의존성 확인
go list -m -u all          # 업데이트 가능한 모듈 목록
go mod graph               # 의존성 그래프

# 브레이킹 체인지 확인
# 릴리스 노트, CHANGELOG, migration guide 확인
```

분석 항목:
- 제거된 API (removed/deprecated)
- 시그니처 변경 (parameter/return type)
- 동작 변경 (behavior change)
- 새 필수 설정 (required config)

### 2단계: 호환성 테스트
```bash
# 현재 테스트 전체 통과 확인 (기준선)
go test -race ./...

# 버전 업그레이드
go get -u github.com/pkg@v2.0.0
go mod tidy

# 컴파일 에러 확인
go build ./...

# 테스트 재실행
go test -race ./...
```

### 3단계: 점진적 마이그레이션

**소규모 변경 (1-2개 API 변경):**
- 직접 수정 후 커밋

**중규모 변경 (5-10개 파일):**
- 호환 레이어 작성 → 사용처 변경 → 호환 레이어 제거
- 각 단계별 커밋

**대규모 변경 (메이저 버전):**
- Branch by Abstraction 패턴 적용
- 피처 플래그로 점진적 전환

### 4단계: 검증
```bash
go test -race ./...
go vet ./...
golangci-lint run
```

## Go 버전 업그레이드

### go.mod 업데이트
```bash
# Go 버전 변경
go mod edit -go=1.23

# 새 기능 활용 가능 여부 확인
go build ./...
go test ./...
```

### 주요 버전별 변경 사항 확인
- 새 표준 라이브러리 함수
- 삭제/변경된 동작
- 새 린트 규칙

## 의존성 업그레이드 전략

### 안전한 순서
```
1. 패치 버전 업그레이드 (1.2.3 → 1.2.4) — 버그 수정
2. 마이너 버전 업그레이드 (1.2.x → 1.3.0) — 하위 호환 기능 추가
3. 메이저 버전 업그레이드 (v1 → v2) — 브레이킹 체인지 가능
```

### Dependabot / Renovate 활용
```yaml
# .github/dependabot.yml
version: 2
updates:
  - package-ecosystem: gomod
    directory: /
    schedule:
      interval: weekly
    reviewers:
      - team/backend
```

## 브레이킹 체인지 대응 패턴

### Adapter Pattern
```go
// 구 API를 새 API로 래핑
type LegacyAdapter struct {
    newClient *NewClient
}

func (a *LegacyAdapter) OldMethod(args OldArgs) OldResult {
    newArgs := convertArgs(args)
    newResult := a.newClient.NewMethod(newArgs)
    return convertResult(newResult)
}
```

### Feature Flag
```go
func handler(w http.ResponseWriter, r *http.Request) {
    if config.UseNewAPI {
        newHandler(w, r)
    } else {
        oldHandler(w, r)
    }
}
```

### Parallel Run (Shadow Traffic)
```go
func handler(w http.ResponseWriter, r *http.Request) {
    oldResult := oldHandler(r)

    // 백그라운드에서 새 구현 실행 (비교용)
    go func() {
        newResult := newHandler(r)
        compareResults(oldResult, newResult)
    }()

    respond(w, oldResult) // 구 결과 반환
}
```

## 마이그레이션 체크리스트

- [ ] 릴리스 노트/마이그레이션 가이드 확인
- [ ] 영향받는 파일 목록 작성
- [ ] 기준선 테스트 통과 확인
- [ ] 점진적 마이그레이션 (한 번에 전체 변경 금지)
- [ ] 각 단계 후 테스트 실행
- [ ] 롤백 계획 수립
- [ ] 호환 레이어 정리 (마이그레이션 완료 후)
