---
name: auto-ast-refactoring
---

# AST Refactoring Skill

Abstract Syntax Tree(AST)를 활용하여 코드를 안전하게 리팩토링하는 스킬입니다.

## AST 기반 리팩토링 원칙

### 왜 AST인가?
- 텍스트 기반 치환: `sed`, 정규식 → 오탐, 부분 매칭 위험
- AST 기반 변환: 문법을 이해하고 변환 → 안전

### Go AST 도구

```bash
# gorename: 심볼 안전 이름 변경
gorename -from "github.com/org/pkg.OldName" -to NewName

# gofmt -r: 패턴 기반 변환
gofmt -r 'a.Foo(b, c) -> a.Bar(b, c)' -w ./...

# gotools: 패키지 이동
gomv github.com/org/pkg/old github.com/org/pkg/new
```

## 리팩토링 패턴

### Extract Function
```go
// Before: 복잡한 함수
func ProcessOrder(order Order) error {
    // 유효성 검사 (30줄)
    // 가격 계산 (20줄)
    // DB 저장 (15줄)
    return nil
}

// After: 분리된 함수
func ProcessOrder(order Order) error {
    if err := validateOrder(order); err != nil {
        return err
    }
    price := calculatePrice(order)
    return saveOrder(order, price)
}
```

### Extract Interface
```go
// 구체 타입에 인터페이스 추출
type UserRepository interface {
    FindByID(ctx context.Context, id string) (*User, error)
    Save(ctx context.Context, user *User) error
}
```

### Move Package
```bash
# 패키지 이동 시 모든 참조 자동 업데이트
# gopls를 통한 LSP rename 사용
```

## 안전한 리팩토링 절차

```
1. 기존 테스트 실행 (GREEN 확인)
2. 리팩토링 실행
3. 테스트 재실행 (GREEN 유지 확인)
4. 린터 실행
5. 커밋
```

**원칙**: 기능 변경과 리팩토링을 동일 커밋에 혼합하지 않습니다.

## 체크리스트

- [ ] 리팩토링 전 테스트 GREEN 확인
- [ ] 단일 책임 원칙 충족
- [ ] 인터페이스 추출로 테스트 용이성 향상
- [ ] 변수/함수 명명이 의도를 명확히 표현
- [ ] 리팩토링 후 테스트 GREEN 확인
- [ ] 성능 회귀 없음 (벤치마크 비교)
