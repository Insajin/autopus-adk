# Autopus-ADK 모듈 설명

## 모듈 카탈로그

### 1. cmd/auto - 애플리케이션 진입점

**책임**: 프로그램의 메인 진입점 제공

**주요 파일**:
- `main.go`: 패키지 진입점, `cli.Execute()` 호출

**공개 인터페이스**:
- `main()`: 프로그램 실행

**의존성**:
- `internal/cli`: CLI 실행 로직

---

### 2. internal/cli - CLI 커맨드 핸들러

**책임**: Cobra 기반 커맨드 정의 및 실행, 사용자 입력 처리

**주요 파일**:
- `root.go`: 루트 커맨드 정의, 모든 서브커맨드 등록
- `init.go`: init 커맨드 (플랫폼 설치)
- `update.go`: update 커맨드 (파일 업데이트)
- `doctor.go`: doctor 커맨드 (유효성 검증)
- `arch.go`: arch 커맨드 (아키텍처 분석)
- `spec.go`: spec 커맨드 (SPEC 파싱)
- `lore.go`: lore 커맨드 (의사결정 추적)
- `lsp.go`: lsp 커맨드 (LSP 통합)
- `search.go`: search 커맨드 (지식 검색)
- `platform.go`: platform 커맨드 (플랫폼 목록)
- `skill.go`: skill 커맨드 (스킬 관리)
- `hash.go`: hash 커맨드 (해시 계산)
- `docs.go`: docs 커맨드 (문서 생성)

**공개 인터페이스**:
- `Execute()`: CLI 실행의 진입점
- `NewRootCmd()`: 루트 Cobra 커맨드 생성

**설정 플래그**:
- `--verbose, -v`: 상세 출력 활성화
- `--config`: 설정 파일 경로 지정

**의존성**:
- `pkg/adapter`: 플랫폼 어댑터
- `pkg/config`: 설정 로드
- `pkg/version`: 버전 정보
- `pkg/arch`: 아키텍처 분석
- `pkg/spec`: SPEC 파싱
- 외 다수

---

### 3. pkg/adapter - 플랫폼 추상화 계층

**책임**: 플랫폼 독립적인 인터페이스 정의, 구현체 등록 및 조회

**주요 파일**:
- `adapter.go`: PlatformAdapter 인터페이스, 관련 타입 정의
- `registry.go`: Registry 구현 (스레드 안전)

**공개 인터페이스**:

```go
type PlatformAdapter interface {
  Name() string
  Version() string
  CLIBinary() string
  Detect(ctx context.Context) (bool, error)
  Generate(ctx context.Context, cfg *config.HarnessConfig) (*PlatformFiles, error)
  Update(ctx context.Context, cfg *config.HarnessConfig) (*PlatformFiles, error)
  Validate(ctx context.Context) ([]ValidationError, error)
  Clean(ctx context.Context) error
  SupportsHooks() bool
  InstallHooks(ctx context.Context, hooks []HookConfig) error
}

type Registry struct { ... }
func NewRegistry() *Registry
func (r *Registry) Register(a PlatformAdapter)
func (r *Registry) Get(name string) (PlatformAdapter, error)
func (r *Registry) List() []PlatformAdapter
func (r *Registry) DetectAll(ctx context.Context) []PlatformAdapter
```

**핵심 타입**:
- `PlatformAdapter`: 어댑터 인터페이스
- `PlatformFiles`: 생성된 파일 목록
- `FileMapping`: 단일 파일 매핑
- `OverwritePolicy`: 파일 덮어쓰기 정책 (always, never, marker)
- `ValidationError`: 검증 오류
- `HookConfig`: 훅 설정
- `Registry`: 어댑터 레지스트리

**동시성**:
- RWMutex를 사용한 스레드 안전 구현

---

### 4. pkg/adapter/claude - Claude Code 어댑터

**책임**: Claude Code IDE에 하네스 설치

**주요 파일**:
- `adapter.go`: Claude Code 어댑터 구현
- 하위 디렉토리: Full/Lite 모드 템플릿

**구현 메서드**:
- `Name()`: "claude-code" 반환
- `Detect()`: Claude Code 설치 여부 확인
- `Generate()`: 하네스 파일 생성
- `Update()`: 기존 파일 업데이트 (마커 기반)
- `Validate()`: 파일 유효성 검증
- `Clean()`: 하네스 파일 제거
- `SupportsHooks()`: 훅 지원 여부 (true/false)
- `InstallHooks()`: 훅 설치

---

### 5. pkg/adapter/codex - Codex 어댑터

**책임**: Codex IDE에 하네스 설치

**특성**:
- Codex 특화 설정
- Codex 플랫폼 요구사항 맞춤

---

### 6. pkg/adapter/gemini - Gemini CLI 어댑터

**책임**: Gemini CLI에 하네스 설치

**특성**:
- Gemini 특화 설정
- Gemini 플랫폼 요구사항 맞춤

---

### 7. pkg/adapter/opencode - OpenCode 어댑터

**책임**: OpenCode IDE에 하네스 설치

---

### 8. pkg/adapter/cursor - Cursor 어댑터

**책임**: Cursor IDE에 하네스 설치

---

### 9. pkg/config - 설정 관리

**책임**: 설정 스키마 정의, YAML 로딩, 기본값 제공

**주요 파일**:
- `schema.go`: HarnessConfig 스키마 정의
- `loader.go`: YAML 로더 구현
- `defaults.go`: 기본 설정값

**공개 인터페이스**:

```go
type HarnessConfig struct {
  Mode string // "full" 또는 "lite"
  // 기타 설정 필드
}

func LoadConfig(path string) (*HarnessConfig, error)
func DefaultFullConfig() *HarnessConfig
func DefaultLiteConfig() *HarnessConfig
```

**주요 책임**:
- Full Mode: 모든 기능 포함 (agents, skills, hooks, workflows 등)
- Lite Mode: 최소 필수 기능만 (기본 구조)

---

### 10. pkg/content - 콘텐츠 생성

**책임**: Autopus 하네스 콘텐츠 자동 생성

**생성 대상**:
- Agents 정의
- Skills 정의
- Hooks 설정
- Workflows 정의
- Methodology 문서
- MX 태그 규칙
- Intent 정의
- Session 구성
- Router 설정

**핵심 함수**:
- `GenerateAgents()`: 에이전트 파일 생성
- `GenerateSkills()`: 스킬 파일 생성
- `GenerateHooks()`: 훅 파일 생성
- 외 다수

---

### 11. pkg/arch - 아키텍처 분석

**책임**: 코드 구조 분석, 아키텍처 문서 생성, 린팅

**핵심 기능**:
- `Analyze()`: 디렉토리 구조 분석
- `Lint()`: 아키텍처 규칙 린팅
- `Generate()`: ARCHITECTURE.md 생성

**분석 대상**:
- 디렉토리 구조
- 모듈 의존성
- 파일 조직

---

### 12. pkg/spec - SPEC 엔진

**책임**: SPEC 문서 파싱, 템플릿 렌더링, 검증

**핵심 기능**:
- `Parse()`: SPEC 파일 파싱 (EARS 형식)
- `Template()`: SPEC 템플릿 렌더링
- `Validate()`: SPEC 유효성 검증

**SPEC 형식**:
- EARS (Easy Approach to Requirements Syntax)
- Ubiquitous, Event-driven, State-driven, Unwanted, Optional 요구사항

---

### 13. pkg/lore - 의사결정 추적

**책임**: Git 기반 의사결정 지식 관리

**핵심 기능**:
- `Query()`: Git 커밋 로그 쿼리
- `Writer`: 커밋 트레일러 작성
- 9-트레일러 프로토콜 구현

**트레일러 종류**:
- Decision-Reason: 의사결정 이유
- Decision-Consequence: 결과
- Decision-Alternative: 검토된 대안
- 외 다수

---

### 14. pkg/lsp - LSP 통합

**책임**: Language Server Protocol 통합

**기능**:
- LSP 클라이언트/서버 통신
- 코드 분석 요청 처리

---

### 15. pkg/search - 외부 지식 검색

**책임**: 외부 지식 소스 검색

**검색 대상**:
- Context7 문서 검색
- Exa 검색 API
- 해시 기반 검색

---

### 16. pkg/detect - 플랫폼 감지

**책임**: 설치된 코딩 CLI 감지

**감지 방법**:
- PATH 환경 변수 스캔
- 실행 파일 존재 여부 확인

**감지 대상**:
- claude-code
- codex
- gemini-cli
- opencode
- cursor

---

### 17. pkg/template - 템플릿 엔진

**책임**: Go text/template 래퍼 제공

**기능**:
- 템플릿 파싱
- 함수맵(FuncMap) 정의
- 템플릿 렌더링

---

### 18. pkg/version - 버전 정보

**책임**: 빌드 메타데이터 제공

**정보**:
- 버전 번호
- Git 커밋 해시
- 빌드 날짜
- 빌드 환경

**주입 방식**:
- ldflags를 통한 동적 주입
- 빌드 시점에 설정

---

## 모듈 책임 매트릭스

| 모듈 | 책임 | 의존성 |
|------|------|--------|
| cmd/auto | 진입점 | internal/cli |
| internal/cli | 커맨드 처리 | 모든 pkg/* |
| pkg/adapter | 플랫폼 추상화 | pkg/config |
| pkg/adapter/* | 플랫폼 구현 | pkg/config, pkg/template |
| pkg/config | 설정 관리 | - |
| pkg/content | 콘텐츠 생성 | pkg/template |
| pkg/arch | 아키텍처 분석 | - |
| pkg/spec | SPEC 처리 | pkg/template |
| pkg/lore | 의사결정 추적 | - |
| pkg/lsp | LSP 통합 | - |
| pkg/search | 지식 검색 | - |
| pkg/detect | 플랫폼 감지 | - |
| pkg/template | 템플릿 엔진 | - |
| pkg/version | 버전 정보 | - |

---

## 핵심 인터페이스

### PlatformAdapter

모든 플랫폼 어댑터가 구현해야 하는 메인 인터페이스입니다. 이 인터페이스는 플랫폼 독립적인 작업을 정의하고, 각 플랫폼별 구현이 이를 따릅니다.

### Registry

스레드 안전한 어댑터 관리 인터페이스입니다. 런타임에 어댑터를 등록하고 조회할 수 있습니다.

---

## 설계 패턴 활용

### 어댑터 패턴
PlatformAdapter 인터페이스를 통해 플랫폼별 구현을 추상화합니다.

### 레지스트리 패턴
Registry를 사용하여 어댑터를 중앙에서 관리합니다.

### 팩토리 패턴
`DefaultFullConfig()`, `DefaultLiteConfig()` 함수로 설정을 생성합니다.

### 전략 패턴
Full/Lite 모드로 설정 전략을 선택합니다.

### 템플릿 패턴
Go text/template을 사용하여 파일을 동적으로 생성합니다.
