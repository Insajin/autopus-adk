# Autopus-ADK 프로젝트 구조

## 전체 디렉토리 트리

```
autopus-adk/
├── cmd/
│   └── auto/
│       └── main.go                 # CLI 진입점
├── internal/
│   ├── cli/
│   │   ├── root.go                 # 루트 명령어 정의
│   │   ├── version.go              # version 명령어
│   │   ├── init.go                 # init 명령어 (하네스 설치)
│   │   ├── update.go               # update 명령어 (하네스 업데이트)
│   │   ├── doctor.go               # doctor 명령어 (설치 검증)
│   │   ├── platform.go             # platform 명령어 (플랫폼 정보)
│   │   ├── arch.go                 # arch 명령어 (아키텍처 분석)
│   │   ├── lore.go                 # lore 명령어 (결정 기록)
│   │   ├── spec.go                 # spec 명령어 (SPEC 생성)
│   │   ├── lsp.go                  # lsp 명령어 (LSP 통합)
│   │   ├── search.go               # search 명령어 (지식 검색)
│   │   ├── docs.go                 # docs 명령어 (문서 생성)
│   │   ├── hash.go                 # hash 명령어 (해시 계산)
│   │   ├── skill.go                # skill 명령어 (스킬 생성)
│   │   ├── testhelper_test.go      # 테스트 헬퍼
│   │   └── *_test.go               # 각 명령어의 테스트 파일
│   └── cli/.autopus/               # 내부 Autopus 설정
├── pkg/
│   ├── adapter/
│   │   ├── adapter.go              # PlatformAdapter 인터페이스
│   │   ├── claude/                 # Claude Code 어댑터
│   │   │   ├── adapter.go
│   │   │   ├── detector.go
│   │   │   └── adapter_test.go
│   │   ├── codex/                  # Codex 어댑터
│   │   ├── gemini/                 # Gemini CLI 어댑터
│   │   ├── opencode/               # OpenCode 어댑터
│   │   └── cursor/                 # Cursor 어댑터
│   ├── arch/
│   │   ├── analyzer.go             # 아키텍처 분석기
│   │   ├── generator.go            # ARCHITECTURE.md 생성기
│   │   ├── linter.go               # 아키텍처 린터
│   │   ├── types.go                # 아키텍처 타입 정의
│   │   └── *_test.go               # 테스트 파일
│   ├── config/
│   │   ├── config.go               # HarnessConfig 구조체
│   │   ├── loader.go               # YAML 로더
│   │   ├── defaults.go             # Full/Lite 기본값
│   │   └── *_test.go               # 테스트 파일
│   ├── content/
│   │   ├── router.go               # 콘텐츠 라우터
│   │   ├── agent.go                # 에이전트 콘텐츠 생성
│   │   ├── skill.go                # 스킬 콘텐츠 생성
│   │   ├── hook.go                 # 훅 콘텐츠 생성
│   │   ├── workflow.go             # 워크플로우 콘텐츠 생성
│   │   ├── methodology.go          # TDD/DDD 방법론 생성
│   │   ├── session.go              # 세션 관리 콘텐츠
│   │   ├── mx.go                   # MX 태그 생성
│   │   ├── intent.go               # 의도 라우팅 생성
│   │   └── *_test.go               # 테스트 파일
│   ├── detect/
│   │   ├── detector.go             # 플랫폼 감지기
│   │   └── detector_test.go        # 테스트
│   ├── lore/
│   │   ├── protocol.go             # 9-trailer 프로토콜 정의
│   │   ├── writer.go               # 결정 기록 작성기
│   │   ├── parser.go               # 결정 기록 파서
│   │   ├── query.go                # 결정 조회
│   │   ├── validator.go            # 프로토콜 검증
│   │   └── *_test.go               # 테스트 파일
│   ├── lsp/
│   │   ├── client.go               # LSP 클라이언트
│   │   ├── detector.go             # 언어 서버 감지
│   │   ├── diagnostic.go           # 진단 데이터 처리
│   │   ├── commands.go             # LSP 명령어
│   │   └── *_test.go               # 테스트 파일
│   ├── plugin/
│   │   └── plugin.go               # 플러그인 인터페이스
│   ├── search/
│   │   ├── context7.go             # Context7 MCP 통합
│   │   ├── exa.go                  # Exa API 통합
│   │   ├── hash.go                 # 해시 기반 검색
│   │   ├── client.go               # 통합 검색 클라이언트
│   │   └── *_test.go               # 테스트 파일
│   ├── spec/
│   │   ├── parser.go               # EARS 형식 파서
│   │   ├── validator.go            # SPEC 검증기
│   │   ├── generator.go            # SPEC 생성기
│   │   ├── templates.go            # SPEC 템플릿
│   │   └── *_test.go               # 테스트 파일
│   ├── template/
│   │   ├── engine.go               # 템플릿 엔진
│   │   ├── funcmap.go              # 커스텀 함수 라이브러리
│   │   └── *_test.go               # 테스트 파일
│   └── version/
│       └── version.go              # 버전 정보 저장소
├── templates/
│   ├── shared/                     # 공통 템플릿
│   │   ├── config.yml.tmpl
│   │   ├── architecture.md.tmpl
│   │   └── ...
│   ├── claude/                     # Claude Code 전용 템플릿
│   │   ├── settings.json.tmpl
│   │   ├── hooks.md.tmpl
│   │   └── ...
│   ├── codex/                      # Codex 전용 템플릿
│   ├── gemini/                     # Gemini CLI 전용 템플릿
│   ├── opencode/                   # OpenCode 전용 템플릿
│   └── cursor/                     # Cursor 전용 템플릿
├── content/                        # 내장 콘텐츠 (에이전트, 스킬 등)
│   ├── agents/                     # 에이전트 정의
│   ├── skills/                     # 스킬 정의
│   ├── hooks/                      # 훅 정의
│   └── methodology/                # 방법론 가이드
├── configs/                        # 설정 파일 예제
│   ├── full-config.yaml           # Full 모드 기본 설정
│   └── lite-config.yaml           # Lite 모드 기본 설정
├── go.mod                          # Go 모듈 정의
├── go.sum                          # Go 모듈 체크섬
├── Makefile                        # 빌드 자동화
├── .gitignore                      # git 무시 규칙
└── .claude/                        # Claude Code 설정
    ├── agents/                     # 에이전트 정의
    ├── skills/                     # 스킬 정의
    ├── hooks/                      # 훅 정의
    └── rules/                      # 프로젝트 규칙
```

---

## 각 디렉토리/패키지 역할 상세 설명

### cmd/auto/ — 진입점 (Entry Point)
**역할**: CLI 애플리케이션의 메인 진입점

**파일 구성**:
- `main.go`: Go 프로그램의 `main()` 함수 호출, CLI 라우터 초기화

**책임**:
- 애플리케이션 시작
- 초기 설정 로드
- CLI 라우터(Cobra) 실행

**외부 의존성**: `internal/cli` 패키지

---

### internal/cli/ — CLI 명령어 (Commands)
**역할**: Cobra 기반 모든 CLI 명령어 구현

**주요 파일**:
- `root.go`: 루트 명령어 및 플래그 정의 (--help, --version 등)
- `version.go`: `auto version` 명령어
- `init.go`: `auto init` 명령어 (하네스 설치)
- `update.go`: `auto update` 명령어 (업데이트)
- `doctor.go`: `auto doctor` 명령어 (설치 검증)
- `platform.go`: `auto platform` 명령어 (플랫폼 정보 조회)
- `arch.go`: `auto arch` 명령어 (아키텍처 분석)
- `lore.go`: `auto lore` 명령어 (결정 기록)
- `spec.go`: `auto spec` 명령어 (SPEC 생성)
- `lsp.go`: `auto lsp` 명령어 (LSP 통합)
- `search.go`: `auto search` 명령어 (지식 검색)
- `docs.go`: `auto docs` 명령어 (문서 생성)
- `hash.go`: `auto hash` 명령어 (해시 계산)
- `skill.go`: `auto skill` 명령어 (스킬 생성)

**테스트 파일**:
- `*_test.go`: 각 명령어의 단위 테스트
- `integration_test.go`: 통합 테스트
- `testhelper_test.go`: 테스트 헬퍼 함수

**책임**:
- 사용자 입력 파싱
- 명령어 로직 구현
- 에러 처리 및 사용자 피드백
- 플래그 검증

**외부 의존성**: `pkg/*` 패키지들

---

### pkg/adapter/ — 플랫폼 어댑터 (Platform Adapters)
**역할**: 각 코딩 CLI에 맞춘 플랫폼 구현

**구조**:
```
adapter/
├── adapter.go              # PlatformAdapter 인터페이스
├── claude/                 # Claude Code 구현
├── codex/                  # Codex 구현
├── gemini/                 # Gemini CLI 구현
├── opencode/               # OpenCode 구현
└── cursor/                 # Cursor 구현
```

**PlatformAdapter 인터페이스 메서드**:
- `Name()`: 어댑터 이름 반환
- `Version()`: 어댑터 버전
- `CLIBinary()`: CLI 실행 파일명
- `Detect(ctx)`: 설치 여부 감지
- `Generate(ctx, cfg)`: 플랫폼 파일 생성
- `Update(ctx, cfg)`: 파일 업데이트
- `Validate(ctx)`: 설치 검증
- `Clean(ctx)`: 파일 제거
- `SupportsHooks()`: 훅 지원 여부
- `InstallHooks(ctx, hooks)`: 훅 설치

**각 플랫폼별 파일**:
- `adapter.go`: PlatformAdapter 구현
- `detector.go`: 플랫폼 감지 로직
- `adapter_test.go`: 테스트

**책임**:
- 플랫폼별 경로 및 구조 이해
- 플랫폼 파일 생성 및 업데이트
- 설치 검증

---

### pkg/config/ — 설정 관리 (Configuration)
**역할**: 하네스 설정 로드 및 관리

**파일**:
- `config.go`: `HarnessConfig` 구조체 정의
  - 플랫폼 설정
  - 모드 (Full/Lite)
  - 활성화된 기능
  - 출력 경로
- `loader.go`: YAML 파일 로드 및 파싱
- `defaults.go`: Full/Lite 기본값 정의
- `config_test.go`: 테스트

**주요 구조**:
```go
type HarnessConfig struct {
    Mode                string              // "full" or "lite"
    Platform            string              // "claude", "codex" 등
    Features            map[string]bool     // 기능 활성화 맵
    OutputDir           string              // 출력 디렉토리
    Metadata            map[string]string   // 메타데이터
}
```

**책임**:
- 설정 파일 로드
- 기본값 적용
- 설정 검증

---

### pkg/content/ — 콘텐츠 생성 (Content Generation)
**역할**: 에이전트, 스킬, 훅 등의 콘텐츠 자동 생성

**파일**:
- `router.go`: 콘텐츠 타입별 라우팅
- `agent.go`: 에이전트 마크다운 생성
- `skill.go`: 스킬 마크다운 생성
- `hook.go`: 훅 파일 생성
- `workflow.go`: 워크플로우 문서 생성
- `methodology.go`: TDD/DDD 가이드 생성
- `session.go`: 세션 관리 관련 콘텐츠
- `mx.go`: MX 태그 생성기
- `intent.go`: 의도 라우팅 설정 생성

**책임**:
- 템플릿 기반 콘텐츠 생성
- 변수 주입
- 파일 작성

---

### pkg/arch/ — 아키텍처 분석 (Architecture Analysis)
**역할**: 프로젝트 구조 분석 및 문서화

**파일**:
- `analyzer.go`: 디렉토리 스캔 및 모듈 관계도 생성
- `generator.go`: ARCHITECTURE.md 생성
- `linter.go`: 아키텍처 패턴 검증 (순환 의존성 감지 등)
- `types.go`: 아키텍처 데이터 타입 정의

**분석 대상**:
- 디렉토리 계층 구조
- 파일 크기 및 복잡도
- 의존성 그래프
- 모듈 응집도

**책임**:
- 프로젝트 구조 이해
- 다이어그램 생성
- 개선 권고

---

### pkg/spec/ — SPEC 생성 (SPEC Generation)
**역할**: EARS 형식 요구사항 → SPEC 문서 변환

**파일**:
- `parser.go`: EARS 형식 파싱
- `validator.go`: SPEC 검증
- `generator.go`: SPEC 문서 생성
- `templates.go`: SPEC 템플릿 정의

**지원 형식**:
- Ubiquitous (항상 활성)
- Event-driven (이벤트 기반)
- State-driven (상태 기반)
- Unwanted (금지)
- Optional (선택적)

**책임**:
- 요구사항 파싱
- SPEC 검증
- 문서 생성

---

### pkg/lore/ — 결정 기록 (Decision Tracking)
**역할**: 아키텍처 의사결정 기록 및 추적

**파일**:
- `protocol.go`: 9-trailer 프로토콜 정의
- `writer.go`: 결정 기록 작성
- `parser.go`: 기록 파싱
- `query.go`: 결정 조회
- `validator.go`: 프로토콜 검증

**프로토콜**: 9가지 필드
1. 타입 및 제목
2. 본문
3. 이유 (Reason)
4. 영향 (Impact)
5. 대안 (Alternatives)
6. 리스크 (Risks)
7. 팀 (Team)
8. 참고 (References)
9. 버전 (Version)

**책임**:
- 의사결정 형식 정의
- 기록 저장/조회
- 검증

---

### pkg/lsp/ — LSP 통합 (Language Server Protocol)
**역할**: IDE의 언어 서버와 통합

**파일**:
- `client.go`: LSP 클라이언트
- `detector.go`: 언어 서버 감지
- `diagnostic.go`: 진단 데이터 처리
- `commands.go`: LSP 명령어 구현

**지원 언어 서버**:
- Python (pylance, pyright)
- TypeScript/JavaScript (typescript-language-server)
- Go (gopls)
- Rust (rust-analyzer)

**책임**:
- 언어 서버 연결
- 진단 수집
- 오류/경고 분류

---

### pkg/search/ — 지식 검색 (Knowledge Search)
**역할**: Context7, Exa, 해시 기반 검색 통합

**파일**:
- `context7.go`: Context7 MCP 통합
- `exa.go`: Exa API 통합
- `hash.go`: 해시 기반 검색
- `client.go`: 통합 검색 클라이언트

**검색 소스**:
- Context7: 공식 라이브러리 문서
- Exa: 웹 검색
- Hash: 프로젝트 내 패턴 검색

**책임**:
- API 호출
- 결과 파싱
- 검색 최적화

---

### pkg/detect/ — 플랫폼 감지 (Platform Detection)
**역할**: 설치된 코딩 CLI 자동 감지

**파일**:
- `detector.go`: 플랫폼 감지 로직

**감지 대상**:
- Claude Code
- Codex
- Gemini CLI
- OpenCode
- Cursor

**감지 방법**:
- 바이너리 경로 확인
- 설정 파일 존재 확인
- 버전 명령어 실행

**책임**:
- 설치 여부 판단
- 버전 정보 수집

---

### pkg/template/ — 템플릿 엔진 (Template Engine)
**역할**: Go text/template 확장 및 렌더링

**파일**:
- `engine.go`: 템플릿 로드 및 렌더링
- `funcmap.go`: 커스텀 함수 라이브러리 (플랫폼별)

**커스텀 함수** (예):
- `upper(s)`: 대문자 변환
- `join(slice, sep)`: 배열 조인
- `indent(s, spaces)`: 들여쓰기
- `yaml(v)`: YAML 인코딩

**책임**:
- 템플릿 렌더링
- 변수 주입
- 플랫폼별 커스터마이제이션

---

### pkg/version/ — 버전 관리 (Version Management)
**역할**: 버전, 커밋, 빌드 일시 저장

**파일**:
- `version.go`: 버전 정보 저장소

**정보**:
- `version`: 태그 버전 (예: v0.0.1)
- `commit`: 커밋 해시 (단축)
- `date`: 빌드 일시 (ISO 8601)

**빌드 시 주입** (Makefile LDFLAGS):
```
-X github.com/anthropics/autopus-adk/pkg/version.version=$(VERSION)
-X github.com/anthropics/autopus-adk/pkg/version.commit=$(COMMIT)
-X github.com/anthropics/autopus-adk/pkg/version.date=$(DATE)
```

---

### templates/ — 플랫폼 템플릿 (Platform Templates)
**역할**: 플랫폼별 설정 파일 템플릿

**구조**:
- `shared/`: 모든 플랫폼 공통
- `claude/`: Claude Code 특화
- `codex/`: Codex 특화
- `gemini/`: Gemini CLI 특화
- `opencode/`: OpenCode 특화
- `cursor/`: Cursor 특화

**템플릿 형식**: Go text/template 문법

**예시** (claude/settings.json.tmpl):
```json
{
  "organization": "{{ .Organization }}",
  "model": "{{ .Model }}",
  "features": {
    "search": {{ .Features.Search }},
    "lsp": {{ .Features.LSP }}
  }
}
```

---

### content/ — 내장 콘텐츠 (Embedded Content)
**역할**: 에이전트, 스킬, 훅 등 내장 콘텐츠

**구조**:
- `agents/`: 에이전트 마크다운 정의
- `skills/`: 스킬 마크다운 정의
- `hooks/`: 훅 정의
- `methodology/`: TDD/DDD 방법론 가이드

**저장 형식**: YAML 프론트매터 + 마크다운

---

### configs/ — 설정 예제 (Configuration Examples)
**역할**: Full/Lite 모드 기본 설정 제공

**파일**:
- `full-config.yaml`: Full 모드 기본값
- `lite-config.yaml`: Lite 모드 기본값

---

## 주요 파일 위치 및 역할

### 진입점
- `/cmd/auto/main.go`: 애플리케이션 시작점

### 인터페이스
- `/pkg/adapter/adapter.go`: PlatformAdapter 인터페이스

### 설정
- `/pkg/config/config.go`: HarnessConfig 구조체
- `/configs/full-config.yaml`: Full 모드 기본값
- `/configs/lite-config.yaml`: Lite 모드 기본값

### 템플릿
- `/templates/`: 모든 플랫폼 템플릿
- `/pkg/template/`: 템플릿 엔진

### 콘텐츠
- `/content/`: 내장 콘텐츠 (에이전트, 스킬 등)
- `/pkg/content/`: 콘텐츠 생성 로직

### 명령어
- `/internal/cli/`: Cobra 기반 모든 CLI 명령어

---

## 패키지 간 의존성 관계

```
cmd/auto/main.go
    ↓
internal/cli/
    ├→ pkg/adapter/         (플랫폼 처리)
    ├→ pkg/config/          (설정 로드)
    ├→ pkg/content/         (콘텐츠 생성)
    ├→ pkg/arch/            (아키텍처 분석)
    ├→ pkg/spec/            (SPEC 생성)
    ├→ pkg/lore/            (결정 기록)
    ├→ pkg/lsp/             (LSP 통합)
    ├→ pkg/search/          (지식 검색)
    ├→ pkg/detect/          (플랫폼 감지)
    ├→ pkg/template/        (템플릿 렌더링)
    └→ pkg/version/         (버전 정보)

pkg/adapter/
    └→ pkg/template/        (플랫폼 파일 렌더링)

pkg/content/
    └→ templates/           (템플릿 파일)

pkg/spec/
    └→ pkg/template/        (SPEC 템플릿 렌더링)
```

---

## 파일별 책임 원칙

- **src/파일**: 특정 도메인의 핵심 로직 구현
- **src/*_test.go**: 해당 파일의 단위 테스트
- **integration_test.go**: 통합 테스트 (패키지 수준)
- **testhelper_test.go**: 테스트 헬퍼 함수

---

## 빌드 산출물

**바이너리**: `./bin/auto`
- 플랫폼별 크로스 컴파일 가능
- 버전/커밋/빌드 일시 포함

**파일 크기**:
- Full: ~500KB
- Lite: ~100KB (옵션 지정 시)

---

*마지막 업데이트: 2026-03-20*
