# Autopus-ADK 기술 스택 및 개발 가이드

## 기술 스택 개요

| 카테고리 | 기술 | 버전 | 용도 |
|---------|------|------|------|
| **프로그래밍 언어** | Go | 1.23+ | 전체 애플리케이션 |
| **CLI 프레임워크** | Cobra | v1.9.1 | 명령어 인터페이스 |
| **설정 형식** | YAML | v3.0.1 | 설정 파일 파싱 |
| **해싱** | xxhash | v2.3.0 | 체크섬 계산 |
| **테스트** | testify | v1.11.1 | Assertion 및 Mock |
| **빌드** | Make | (내장) | 자동화 |

---

## 핵심 의존성 상세

### 1. Go 1.23
**역할**: 전체 애플리케이션의 기반 언어

**선택 이유**:
- 빠른 컴파일 및 실행 속도
- 크로스 플랫폼 지원 (Windows, macOS, Linux)
- 강력한 표준 라이브러리
- 멀티 플랫폼 배포 용이

**필수 모듈** (go.mod):
```
require (
    github.com/cespare/xxhash/v2 v2.3.0
    github.com/spf13/cobra v1.9.1
    github.com/stretchr/testify v1.11.1
    gopkg.in/yaml.v3 v3.0.1
)
```

**Go 버전 선택**:
- 1.23: 최신 안정 버전 (권장)
- 1.22: 호환성 유지 가능
- 1.21 이하: 일부 기능 제약

---

### 2. Cobra CLI Framework (v1.9.1)
**역할**: 명령어 라인 인터페이스 구축

**기능**:
- 명령어 트리 구조 (root → subcommand)
- 플래그 관리 (전역, 명령어 수준)
- 자동 help 생성
- 명령어 별 실행 로직

**사용 패턴**:
```go
// 명령어 정의
var initCmd = &cobra.Command{
    Use: "init",
    Short: "하네스 설치",
    RunE: func(cmd *cobra.Command, args []string) error {
        // 명령어 로직
        return nil
    },
}

// 플래그 추가
initCmd.Flags().StringVar(&mode, "mode", "full", "설치 모드")
```

**주요 명령어**:
- `auto init`: 하네스 설치
- `auto update`: 업데이트
- `auto doctor`: 검증
- `auto arch`: 아키텍처 분석
- `auto spec`: SPEC 생성

---

### 3. YAML v3 (yaml.v3)
**역할**: 설정 파일 파싱 및 직렬화

**사용**:
```go
// 파일 읽기
data, err := ioutil.ReadFile("config.yaml")

// 파싱
var config HarnessConfig
yaml.Unmarshal(data, &config)

// 직렬화
bytes, err := yaml.Marshal(config)
```

**설정 예시**:
```yaml
mode: full
platform: claude
features:
  search: true
  lsp: true
  arch: true
```

---

### 4. xxhash/v2
**역할**: 고속 체크섬 계산

**사용**:
```go
import "github.com/cespare/xxhash/v2"

h := xxhash.New64()
h.Write(data)
checksum := h.Sum64()
```

**용도**:
- 파일 체크섬 계산
- 데이터 무결성 검증
- 빠른 해시 계산 (SHA256보다 10배 빠름)

---

### 5. testify (v1.11.1)
**역할**: 테스트 작성 및 Mock 지원

**주요 기능**:
- `assert`: 조건 검증
- `require`: 조건 검증 (실패 시 즉시 중단)
- `mock`: Mock 객체 생성

**사용 패턴**:
```go
import "github.com/stretchr/testify/assert"

func TestExample(t *testing.T) {
    result := someFunction()
    assert.Equal(t, expected, result, "메시지")
    assert.NoError(t, err)
}
```

---

## 빌드 시스템 (Makefile)

### 빌드 프로세스

**명령어**:
```bash
make build        # 바이너리 생성
make test         # 테스트 실행
make lint         # 코드 검증
make coverage     # 커버리지 리포트
make clean        # 빌드 결과 삭제
make install      # 설치
```

### LDFLAGS를 통한 버전 주입

**Makefile 설정**:
```makefile
VERSION ?= $(shell git describe --tags --always --dirty)
COMMIT  ?= $(shell git rev-parse --short HEAD)
DATE    ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -ldflags "-s -w \
    -X github.com/anthropics/autopus-adk/pkg/version.version=$(VERSION) \
    -X github.com/anthropics/autopus-adk/pkg/version.commit=$(COMMIT) \
    -X github.com/anthropics/autopus-adk/pkg/version.date=$(DATE)"
```

**효과**:
- 실행 시 `auto version` 명령어에서 정확한 버전 표시
- 바이너리 크기 최소화 (-s -w 옵션)
- 디버그 정보 제거

---

### 바이너리 생성

**아웃풋**:
- 위치: `./bin/auto`
- 크기: ~5-10MB (프레임워크 불포함)

**크로스 컴파일**:
```bash
GOOS=darwin GOARCH=arm64 make build    # macOS ARM64
GOOS=linux GOARCH=amd64 make build     # Linux x86_64
GOOS=windows GOARCH=amd64 make build   # Windows x86_64
```

---

## 테스트 (Testing)

### 테스트 실행

**전체 테스트** (race condition 감지):
```bash
go test -race ./...
```

**특정 패키지 테스트**:
```bash
go test -race ./pkg/adapter/
```

**커버리지 리포트**:
```bash
make coverage
go tool cover -html=coverage.out
```

### 테스트 구조

**파일 명명 규칙**:
- 소스: `adapter.go`
- 테스트: `adapter_test.go`
- 통합: `integration_test.go`
- 헬퍼: `testhelper_test.go`

**테스트 작성 패턴**:

**단위 테스트**:
```go
func TestAdapterDetect(t *testing.T) {
    // Arrange
    adapter := NewAdapter()

    // Act
    detected, err := adapter.Detect(context.Background())

    // Assert
    assert.NoError(t, err)
    assert.True(t, detected)
}
```

**테이블 기반 테스트**:
```go
func TestPlatformDetection(t *testing.T) {
    tests := []struct {
        name     string
        platform string
        expected bool
    }{
        {"claude", "claude", true},
        {"codex", "codex", true},
        {"unknown", "unknown", false},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := Detect(tt.platform)
            assert.Equal(t, tt.expected, result)
        })
    }
}
```

**Mock 테스트**:
```go
func TestWithMock(t *testing.T) {
    mockAdapter := new(MockAdapter)
    mockAdapter.On("Detect", mock.Anything).Return(true, nil)

    // 테스트 로직
    assert.True(t, mockAdapter.AssertCalled(t, "Detect"))
}
```

### 테스트 커버리지 목표

**요구사항**:
- 전체 패키지: 85%+ 커버리지
- 핵심 패키지 (adapter, config, content): 90%+
- 공개 인터페이스: 100% 테스트 필수

**커버리지 확인**:
```bash
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out | grep total
```

---

## 아키텍처 패턴

### 1. Adapter Pattern (어댑터 패턴)
**목적**: 여러 플랫폼에 동일한 인터페이스 제공

**구조**:
```
┌─────────────────────────┐
│  PlatformAdapter 인터페이스 │
└────────┬────────────────┘
         ├─ ClaudeAdapter
         ├─ CodexAdapter
         ├─ GeminiAdapter
         ├─ OpenCodeAdapter
         └─ CursorAdapter
```

**적용 예**:
```go
type PlatformAdapter interface {
    Name() string
    Detect(ctx context.Context) (bool, error)
    Generate(ctx context.Context, cfg *config.HarnessConfig) (*PlatformFiles, error)
}
```

---

### 2. Strategy Pattern (전략 패턴)
**목적**: 런타임에 동작 전환

**예**: Full/Lite 설치 전략
```go
type InstallStrategy interface {
    GetFeatures() map[string]bool
    GetContentSize() int
    Install() error
}

type FullStrategy struct{}
type LiteStrategy struct{}
```

---

### 3. Registry Pattern (레지스트리 패턴)
**목적**: 객체 중앙 집중식 관리

**예**: 플랫폼 어댑터 레지스트리
```go
type AdapterRegistry struct {
    adapters map[string]PlatformAdapter
}

func (r *AdapterRegistry) Register(name string, adapter PlatformAdapter) {}
func (r *AdapterRegistry) Get(name string) PlatformAdapter {}
```

---

### 4. Template Method Pattern (템플릿 메서드)
**목적**: 알고리즘의 뼈대 정의, 세부사항은 서브클래스 구현

**예**: 명령어 실행 패턴
```go
type Command interface {
    Validate() error
    Execute(ctx context.Context) error
    Report(output io.Writer)
}
```

---

### 5. Factory Pattern (팩토리 패턴)
**목적**: 객체 생성 로직 캡슐화

**예**: 플랫폼 감지 후 적절한 어댑터 생성
```go
func NewAdapter(platform string) (PlatformAdapter, error) {
    switch platform {
    case "claude":
        return NewClaudeAdapter(), nil
    case "codex":
        return NewCodexAdapter(), nil
    // ...
    }
}
```

---

## 코드 품질 관리

### Linting (코드 검증)

**go vet 실행**:
```bash
go vet ./...
```

**golangci-lint 실행** (권장):
```bash
golangci-lint run ./...
```

**주요 검사 항목**:
- 미사용 변수 감지
- 타입 안정성 검증
- 포맷 문자열 검증
- 코드 복잡도 분석

---

### Formatting (코드 포맷팅)

**gofmt 적용**:
```bash
gofmt -w ./
```

**goimports 적용** (import 정렬):
```bash
goimports -w ./
```

---

### Race Condition 감지

**-race 플래그 사용**:
```bash
go test -race ./...
```

**목적**: 동시성 관련 데이터 경합 감지

---

## 개발 환경 설정

### 최소 요구사항

**필수**:
- Go 1.23+
- make
- git

**권장**:
- golangci-lint (코드 검증)
- docker (개발 환경 격리)
- VS Code 또는 GoLand (IDE)

### 개발 환경 설정

**1. 저장소 클론**:
```bash
git clone https://github.com/anthropics/autopus-adk.git
cd autopus-adk
```

**2. 의존성 설치**:
```bash
go mod download
go mod verify
```

**3. 빌드**:
```bash
make build
```

**4. 테스트**:
```bash
make test
make coverage
```

**5. Lint**:
```bash
make lint
```

---

## 코드 스타일 가이드

### 명명 규칙 (Go Conventions)

**패키지명**: 소문자, 단어 (예: adapter, config, content)

**상수명**: 대문자 (예: OverwriteAlways)

**함수명**: CamelCase, 공개 함수는 대문자 시작 (예: Detect, Generate)

**변수명**: camelCase (예: platformFiles, hasError)

**인터페이스명**: -er 또는 -able 접미사 (예: PlatformAdapter, Validator)

---

### 코드 구조

**함수 길이**: 한 함수는 한 가지 일만 수행 (단일 책임 원칙)

**매개변수**: 3개 이상이면 구조체 사용

**에러 처리**: 명시적 에러 처리 (에러 무시 금지)

**커멘트**: 공개 함수/타입에 godoc 필수 (영어)

---

### 에러 처리 예시

**올바른 방법**:
```go
func Generate(ctx context.Context, cfg *config.HarnessConfig) (*PlatformFiles, error) {
    if err := validateConfig(cfg); err != nil {
        return nil, fmt.Errorf("config validation failed: %w", err)
    }
    // ...
}
```

**피해야 할 방법**:
```go
// 에러 무시 금지
_ = someFunction()

// panic 사용 금지
if err != nil {
    panic(err)
}
```

---

## CI/CD 파이프라인

### GitHub Actions (권장)

**워크플로우 구성**:
1. Lint (golangci-lint)
2. Test (go test -race)
3. Build (make build)
4. Release (Tag 생성 시)

**예시 워크플로우** (.github/workflows/ci.yml):
```yaml
name: CI
on: [push, pull_request]
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v4
        with:
          go-version: 1.23
      - run: go test -race ./...
      - run: golangci-lint run ./...
      - run: make build
```

---

## 배포 및 배포 후 검증

### 로컬 검증

**설치**:
```bash
make install
# 또는
cp bin/auto /usr/local/bin/auto
```

**기본 명령어 테스트**:
```bash
auto version
auto doctor
auto platform
```

### 릴리스 체크리스트

- [ ] 모든 테스트 통과 (go test -race ./...)
- [ ] 코드 품질 확인 (go vet, golangci-lint)
- [ ] 커버리지 >= 85%
- [ ] 버전 태그 생성 (git tag v0.0.1)
- [ ] CHANGELOG.md 업데이트
- [ ] GitHub Releases 생성

---

## 성능 최적화

### 컴파일 최적화

**LDFLAGS 옵션**:
- `-s`: 심볼 테이블 제거 (디버그 정보 제거)
- `-w`: DWARF 정보 제거
- 결과: 바이너리 크기 40-50% 감소

**빌드 캐싱**:
```bash
go build -a -installsuffix cgo ./...  # 캐시 무시 (완전 재빌드)
```

---

## 보안 고려사항

### 입력 검증

**설정 파일 검증**:
```go
func ValidateConfig(cfg *HarnessConfig) error {
    if cfg.Platform == "" {
        return errors.New("platform must not be empty")
    }
    if cfg.Mode != "full" && cfg.Mode != "lite" {
        return fmt.Errorf("invalid mode: %s", cfg.Mode)
    }
    return nil
}
```

### 파일 경로 보안

**경로 검증** (경로 통과 공격 방지):
```go
import "path/filepath"

targetPath := filepath.Join(basePath, userInput)
// 검증: targetPath가 basePath 내에 있는지 확인
if !strings.HasPrefix(targetPath, basePath) {
    return errors.New("path traversal attack detected")
}
```

---

## 의존성 관리

### go.mod 관리

**현재 의존성**:
```
github.com/cespare/xxhash/v2 v2.3.0     (해싱)
github.com/spf13/cobra v1.9.1           (CLI)
github.com/stretchr/testify v1.11.1     (테스트)
gopkg.in/yaml.v3 v3.0.1                 (YAML)
```

**의존성 업데이트**:
```bash
go get -u ./...         # 최신 버전 업데이트
go mod tidy             # 정리
go mod verify           # 검증
```

**보안 취약점 확인**:
```bash
go list -u -m all       # 업데이트 가능한 모듈 확인
```

---

## 디버깅

### 디버그 빌드

```bash
go build -gcflags="all=-N -l" -o bin/auto ./cmd/auto
```

### Delve 디버거 사용

```bash
# 설치
go install github.com/go-delve/delve/cmd/dlv@latest

# 디버깅 시작
dlv debug ./cmd/auto

# 중단점 설정
(dlv) break main.main
(dlv) continue
```

### 로깅

**기본 로깅** (log 패키지):
```go
import "log"
log.Printf("상태: %v", status)
```

**구조화된 로깅** (권장):
```go
// zap 패키지 사용 예시
logger.Info("작업 완료", zap.String("파일", path))
```

---

## 성능 측정

### Benchmarking

```bash
go test -bench=. -benchmem ./...
```

**벤치마크 작성 예시**:
```go
func BenchmarkAdapterDetect(b *testing.B) {
    adapter := NewAdapter()
    ctx := context.Background()

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        adapter.Detect(ctx)
    }
}
```

---

## 문제 해결

### 일반 빌드 오류

**문제**: `go version 1.22: module requires 1.23`
**해결**: `go version 1.23+` 설치

**문제**: `import cycle`
**해결**: 패키지 의존성 구조 검토, 순환 참조 제거

### 테스트 실패

**문제**: Race condition 감지
**해결**: 고루틴 동기화 메커니즘 확인 (mutex, channel 등)

**문제**: 테스트 타임아웃
**해결**: 컨텍스트 타임아웃 설정, 대기 시간 조정

---

## 참고 자료

- Go 공식 문서: https://golang.org/doc/
- Cobra 가이드: https://cobra.dev/
- Go 테스트: https://golang.org/pkg/testing/
- Go 에러 처리: https://golang.org/doc/effective_go#errors

---

*마지막 업데이트: 2026-03-20*
*버전: Go 1.23, Cobra v1.9.1*
