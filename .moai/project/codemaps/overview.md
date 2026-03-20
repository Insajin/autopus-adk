# Autopus-ADK 아키텍처 개요

## 프로젝트 소개

Autopus-ADK(Agentic Development Kit)는 코딩 CLI(Claude Code, Codex, Gemini CLI, OpenCode 등)에 Autopus 하네스를 설치하는 Go 기반 커맨드라인 도구입니다. AI 에이전트가 코딩 작업을 자동화하고 일관된 개발 환경을 유지할 수 있도록 지원합니다.

## 핵심 설계 철학

### 1. 어댑터 패턴 (Adapter Pattern)
```
PlatformAdapter 인터페이스
├── Claude Code 어댑터
├── Codex 어댑터
├── Gemini CLI 어댑터
├── OpenCode 어댑터
└── Cursor 어댑터
```

플랫폼별 차이점을 추상화하여 코어 로직이 구체적인 구현에 의존하지 않도록 설계했습니다.

### 2. 레지스트리 패턴 (Registry Pattern)
모든 어댑터는 스레드 안전한 Registry에 등록되어 동적으로 조회됩니다. RWMutex를 사용하여 동시 접근을 안전하게 처리합니다.

### 3. 전략 패턴 (Strategy Pattern)
- Full Mode: 완전한 하네스 설치 (모든 기능 포함)
- Lite Mode: 경량 하네스 설치 (최소 필수 기능만)

### 4. 템플릿 패턴 (Template Pattern)
Go의 text/template을 활용하여 플랫폼별 맞춤 파일을 동적으로 생성합니다.

### 5. 팩토리 패턴 (Factory Pattern)
`DefaultFullConfig()`와 `DefaultLiteConfig()` 함수로 설정 객체를 생성합니다.

## 시스템 경계

```
┌─────────────────────────────────────────────────────────────┐
│                    Autopus-ADK CLI                           │
│  (cmd/auto/main.go → internal/cli/root.go)                  │
└────────────────────┬────────────────────────────────────────┘
                     │
        ┌────────────┴────────────┐
        │                         │
   ┌────▼─────────────┐   ┌──────▼─────────────┐
   │   CLI Commands   │   │  Configuration    │
   │  (init, update,  │   │   Management      │
   │   doctor, arch,  │   │  (schema, load)   │
   │   spec, lore)    │   │                   │
   └────┬─────────────┘   └────────────────────┘
        │
   ┌────▼──────────────────────────────────────┐
   │   Platform Abstraction Layer              │
   │   (pkg/adapter + PlatformAdapter)         │
   └────┬──────────────────────────────────────┘
        │
        ├─ Claude Code  ├─ Codex    ├─ Gemini CLI
        ├─ OpenCode     └─ Cursor
```

## 모듈 구조

### 코어 모듈 (Core)
- **cmd/auto**: 진입점 (main.go)
- **internal/cli**: 12개의 Cobra 기반 커맨드 핸들러

### 플랫폼 추상화 (Platform Abstraction)
- **pkg/adapter**: PlatformAdapter 인터페이스 + Registry
- **pkg/adapter/[platform]**: 5개 플랫폼별 구현체
  - claude/: Claude Code 어댑터
  - codex/: Codex 어댑터
  - gemini/: Gemini CLI 어댑터
  - opencode/: OpenCode 어댑터
  - cursor/: Cursor 어댑터

### 설정 관리 (Configuration)
- **pkg/config**: 설정 스키마, 로더, 기본값

### 콘텐츠 생성 (Content Generation)
- **pkg/content**: 에이전트, 스킬, 훅, 워크플로우 등 콘텐츠 생성

### 아키텍처 분석 (Architecture Analysis)
- **pkg/arch**: 코드 구조 분석, 린팅, 문서 생성

### SPEC 엔진 (SPEC Engine)
- **pkg/spec**: SPEC 문서 파싱, 템플릿 렌더링, 검증

### 의사결정 추적 (Decision Tracking)
- **pkg/lore**: Git 기반 의사결정 지식 (9-트레일러 프로토콜)

### LSP 통합 (LSP Integration)
- **pkg/lsp**: Language Server Protocol 통합

### 외부 지식 검색 (External Knowledge)
- **pkg/search**: Context7, Exa, 해시 기반 검색

### 플랫폼 감지 (Platform Detection)
- **pkg/detect**: PATH 스캔을 통한 플랫폼 감지

### 템플릿 엔진 (Template Engine)
- **pkg/template**: Go text/template 래퍼

### 버전 정보 (Version Info)
- **pkg/version**: 빌드 메타데이터 (ldflags를 통한 주입)

## 데이터 흐름 개요

### 초기화 (init) 워크플로우
1. 플랫폼 자동 감지 (DetectPlatforms)
2. 설정 로드 (LoadConfig)
3. 플랫폼 파일 생성 (adapter.Generate)
4. 파일 쓰기 (WriteFiles)

### 업데이트 (update) 워크플로우
1. 기존 설정 로드
2. 새로운 파일 생성 (adapter.Update)
3. AUTOPUS:BEGIN/END 마커를 통해 사용자 수정 보존
4. 파일 업데이트

### 의사 (doctor) 워크플로우
1. 어댑터 검증 (adapter.Validate)
2. 오류/경고 리포트

### 아키텍처 (arch) 워크플로우
1. 코드 구조 분석 (arch.Analyze)
2. 문서 생성 (arch.Generate)
3. ARCHITECTURE.md 작성

## 주요 설계 원칙

### 1. 플랫폼 독립성
모든 플랫폼별 로직은 PlatformAdapter 인터페이스 뒤에 숨겨져 있어, 코어 로직이 특정 플랫폼에 의존하지 않습니다.

### 2. 확장성
새로운 코딩 CLI를 지원하려면 PlatformAdapter를 구현하고 Registry에 등록하기만 하면 됩니다.

### 3. 안정성
- 마커 기반 파일 업데이트로 사용자 수정 보존
- 검증을 통한 설치 상태 확인
- 에러 처리 및 복구 메커니즘

### 4. 테스트 가능성
- 인터페이스 기반 설계로 모킹 가능
- 각 모듈의 독립적인 테스트
- 통합 테스트로 전체 워크플로우 검증

### 5. 성능
- 병렬 플랫폼 감지
- 효율적인 파일 I/O
- 스레드 안전한 레지스트리

## 버전 정보

- **Go 버전**: 1.25+
- **라이선스**: Apache 2.0
- **빌드 메커니즘**: ldflags를 통한 동적 버전 주입

## 다음 단계

더 자세한 정보는 다음 문서들을 참조하세요:
- `modules.md`: 각 모듈의 자세한 설명
- `dependencies.md`: 모듈 간 의존성 그래프
- `entry-points.md`: CLI 진입점 및 커맨드
- `data-flow.md`: 상세한 데이터 흐름
