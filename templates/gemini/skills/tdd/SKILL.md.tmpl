---
name: auto-tdd
---

# TDD (Test-Driven Development) Skill

테스트를 먼저 작성하고 구현하는 RED-GREEN-REFACTOR 사이클을 적용하는 스킬입니다.

## 핵심 원칙

**테스트 없이 코드를 작성하지 않는다.** 이 규칙을 위반하면 작업을 거부합니다.

## RED-GREEN-REFACTOR 사이클

### RED 단계: 실패하는 테스트 작성

```
1. 구현하려는 동작을 테스트로 먼저 작성
2. 테스트가 실패하는지 확인 (컴파일 에러 포함)
3. 올바른 실패 이유인지 확인
```

테스트 작성 원칙:
- 테스트는 하나의 동작만 검증
- 이름은 `Test[Subject]_[Scenario]_[Expected]` 형식
- Given-When-Then 구조 사용
- Table-driven tests로 다양한 케이스 커버

```go
func TestCalculate_WithZeroInput_ReturnsError(t *testing.T) {
    t.Parallel()
    // Given
    input := 0
    // When
    _, err := Calculate(input)
    // Then
    assert.Error(t, err)
    assert.ErrorIs(t, err, ErrInvalidInput)
}
```

### GREEN 단계: 최소 구현으로 테스트 통과

```
1. 테스트를 통과시키는 가장 단순한 코드 작성
2. 과도한 최적화나 일반화 금지
3. 테스트 통과만을 목표로
```

### REFACTOR 단계: 코드 품질 개선

```
1. 중복 제거 (DRY 원칙)
2. 명명 개선
3. 복잡도 감소
4. 테스트는 항상 그린 상태 유지
```

## Go 테스트 패턴

```go
func TestMyFunction(t *testing.T) {
    t.Parallel()

    tests := []struct {
        name    string
        input   int
        want    int
        wantErr bool
    }{
        {"정상 입력", 5, 25, false},
        {"영 입력", 0, 0, true},
        {"음수 입력", -1, 0, true},
    }

    for _, tt := range tests {
        tt := tt
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()
            got, err := MyFunction(tt.input)
            if tt.wantErr {
                require.Error(t, err)
                return
            }
            require.NoError(t, err)
            assert.Equal(t, tt.want, got)
        })
    }
}
```

## 완료 기준

- [ ] 모든 새 코드에 테스트 존재
- [ ] 테스트 커버리지 85% 이상
- [ ] `go test -race ./...` 통과
- [ ] 각 단계에서 커밋 생성
