---
name: performance
description: 프로파일링, 벤치마크, 성능 최적화 기법
triggers:
  - performance
  - 성능
  - profiling
  - benchmark
  - 최적화
  - bottleneck
category: quality
level1_metadata: "pprof 프로파일링, 벤치마크, 메모리 최적화, 캐싱 전략"
---

# Performance Skill

Go 애플리케이션의 성능을 측정하고 최적화하는 스킬입니다.

## 벤치마크 작성

```go
func BenchmarkFunction(b *testing.B) {
    // 셋업 (타이머에 포함되지 않음)
    data := prepareTestData()
    b.ResetTimer()

    for i := 0; i < b.N; i++ {
        Function(data)
    }
}

// 메모리 할당 추적
func BenchmarkFunction_Allocs(b *testing.B) {
    b.ReportAllocs()
    for i := 0; i < b.N; i++ {
        Function(input)
    }
}
```

실행:
```bash
go test -bench=. -benchmem ./pkg/...
go test -bench=BenchmarkFunction -count=5 -benchtime=3s ./...
```

## pprof 프로파일링

### CPU 프로파일
```go
import _ "net/http/pprof"

// 서버에 추가
go func() {
    http.ListenAndServe("localhost:6060", nil)
}()
```

```bash
# 30초 CPU 프로파일 수집
go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30

# 힙 메모리 프로파일
go tool pprof http://localhost:6060/debug/pprof/heap

# 고루틴 프로파일
go tool pprof http://localhost:6060/debug/pprof/goroutine
```

### 테스트에서 프로파일
```bash
go test -cpuprofile=cpu.out -memprofile=mem.out -bench=. ./...
go tool pprof -http=:8080 cpu.out
```

## 일반적인 최적화 패턴

### 메모리 할당 줄이기
```go
// Before: 반복 할당
func process(items []Item) []Result {
    var results []Result
    for _, item := range items {
        results = append(results, transform(item))
    }
    return results
}

// After: 사전 할당
func process(items []Item) []Result {
    results := make([]Result, 0, len(items))
    for _, item := range items {
        results = append(results, transform(item))
    }
    return results
}
```

### sync.Pool 활용
```go
var bufPool = sync.Pool{
    New: func() interface{} {
        return new(bytes.Buffer)
    },
}

func process() {
    buf := bufPool.Get().(*bytes.Buffer)
    defer bufPool.Put(buf)
    buf.Reset()
    // buf 사용
}
```

### 문자열 연결
```go
// Bad: O(n^2) 할당
s := ""
for _, item := range items {
    s += item.String()
}

// Good: O(n)
var b strings.Builder
for _, item := range items {
    b.WriteString(item.String())
}
s := b.String()
```

## 캐싱 전략

| 전략 | 용도 | 구현 |
|------|------|------|
| In-memory | 읽기 빈도 높은 소량 데이터 | `sync.Map`, LRU 캐시 |
| Redis | 분산 캐시, 세션 | go-redis |
| HTTP 캐시 | API 응답 캐싱 | ETag, Cache-Control |

## 성능 최적화 원칙

1. **측정 먼저**: 추측하지 말고 프로파일링
2. **병목 집중**: 전체 시간의 80%를 차지하는 20% 코드
3. **벤치마크 비교**: 최적화 전후 수치 비교
4. **회귀 방지**: CI에 벤치마크 포함

## 체크리스트

- [ ] 병목 지점 프로파일링으로 식별
- [ ] 벤치마크 테스트 작성
- [ ] 최적화 전후 수치 비교
- [ ] 메모리 할당 최소화
- [ ] 불필요한 고루틴 생성 확인
- [ ] 캐싱 전략 검토
