# Autopus-ADK 데이터 흐름

## 시스템 전체 데이터 흐름

```
┌──────────────────────────────────────────────────────────────────┐
│                    사용자 입력                                    │
│              (CLI 커맨드 + 옵션 + 인자)                          │
└─────────────────────────┬──────────────────────────────────────┘
                          │
                    ┌─────▼──────┐
                    │  플래그     │
                    │  파싱       │
                    │(Cobra)      │
                    └─────┬──────┘
                          │
         ┌────────────────┼────────────────┐
         │                │                │
    ┌────▼─────┐  ┌──────▼──────┐  ┌──────▼──────┐
    │   init   │  │   update    │  │   doctor   │
    │ 워크플로우 │  │ 워크플로우  │  │ 워크플로우 │
    └────┬─────┘  └──────┬──────┘  └──────┬──────┘
         │                │                │
         └────────────────┼────────────────┘
                          │
                    ┌─────▼──────────┐
                    │  최종 출력      │
                    │  (성공/오류)    │
                    └────────────────┘
```

---

## 1. init 워크플로우 (상세)

### 단계 1: 플래그 파싱

```
입력: auto init --mode full --platform claude-code
      │
      ├─ --mode "full" 파싱
      │   └─ 설정 모드: Full Mode 선택
      │
      ├─ --platform "claude-code" 파싱
      │   └─ 플랫폼: Claude Code 지정
      │
      └─ --verbose 확인
          └─ 상세 출력: 활성화 여부
```

### 단계 2: 플랫폼 감지

```
PlateformAdapter.Detect() 호출 (각 플랫폼별)
│
├─ Claude Code 감지
│   ├─ PATH 검색: claude 바이너리
│   ├─ 존재 여부: 파일시스템 확인
│   └─ 결과: detected = true/false
│
├─ Codex 감지
│   ├─ PATH 검색: codex 바이너리
│   ├─ 존재 여부: 파일시스템 확인
│   └─ 결과: detected = true/false
│
├─ Gemini CLI 감지
├─ OpenCode 감지
└─ Cursor 감지

결과 수집:
detected_platforms = [
  PlatformAdapter{Name: "claude-code"},
  PlatformAdapter{Name: "cursor"},
  ...
]
```

### 단계 3: 설정 로드

```
설정 로드 경로:
│
├─ CLI 플래그 확인
│   └─ --config 경로 지정되었는가?
│
├─ YES: YAML 파일에서 로드
│   │
│   ├─ 파일 읽기: filepath.Read()
│   ├─ YAML 파싱: yaml.Unmarshal()
│   ├─ 검증: schema.Validate()
│   └─ 결과: HarnessConfig
│
└─ NO: 기본값 생성
    │
    ├─ --mode 확인
    │   ├─ "full" → DefaultFullConfig()
    │   │   └─ 모든 기능 포함
    │   └─ "lite" → DefaultLiteConfig()
    │       └─ 최소 필수 기능만
    │
    └─ 결과: HarnessConfig

HarnessConfig 구조:
{
  Mode: "full",
  Platforms: ["claude-code", "cursor"],
  ContentOptions: {
    IncludeAgents: true,
    IncludeSkills: true,
    IncludeHooks: true,
    IncludeWorkflows: true,
    ...
  }
}
```

### 단계 4: 플랫폼별 파일 생성

```
for each detected_platform in detected_platforms:
  │
  ├─ adapter := registry.Get(platform.Name)
  │
  ├─ adapter.Generate(context, harness_config)
  │   │
  │   ├─ 플랫폼별 파일 목록 결정
  │   │   ├─ Claude Code:
  │   │   │   ├─ .claude/CLAUDE.md
  │   │   │   ├─ .claude/agents/*.md
  │   │   │   ├─ .claude/skills/*.md
  │   │   │   ├─ .claude/hooks/*.py
  │   │   │   └─ .claude/rules/moai/*.md
  │   │   │
  │   │   ├─ Codex:
  │   │   │   ├─ .codex/config.json
  │   │   │   ├─ .codex/agents/
  │   │   │   └─ ...
  │   │   │
  │   │   └─ [기타 플랫폼별 파일]
  │   │
  │   ├─ 각 파일별 템플릿 렌더링
  │   │   │
  │   │   ├─ template.Parse(template_source)
  │   │   ├─ template.Execute(buffer, harness_config)
  │   │   └─ content := buffer.Bytes()
  │   │
  │   ├─ 파일 메타데이터 생성
  │   │   │
  │   │   ├─ SourceTemplate: "agents/expert-backend.md"
  │   │   ├─ TargetPath: ".claude/agents/expert-backend.md"
  │   │   ├─ OverwritePolicy: "always"
  │   │   ├─ Checksum: SHA256(content)
  │   │   └─ Content: content (바이너리)
  │   │
  │   └─ PlatformFiles 반환
  │       {
  │         Files: [FileMapping, ...],
  │         Checksum: SHA256(all_files)
  │       }
  │
  └─ platform_files[platform.Name] = PlatformFiles
```

### 단계 5: 파일 시스템에 쓰기

```
WriteFiles(platform_files)
│
└─ for each platform_name, platform_files in platform_files_map:
    │
    ├─ for each file_mapping in platform_files.Files:
    │   │
    │   ├─ 타겟 경로 준비
    │   │   ├─ target_dir := filepath.Dir(file_mapping.TargetPath)
    │   │   ├─ os.MkdirAll(target_dir)
    │   │   └─ full_path := filepath.Join(cwd, file_mapping.TargetPath)
    │   │
    │   ├─ 파일 존재 여부 확인
    │   │   └─ exists := os.Stat(full_path) == nil
    │   │
    │   ├─ OverwritePolicy 확인
    │   │   │
    │   │   ├─ "always":
    │   │   │   └─ 파일 덮어쓰기 (백업 생성 후)
    │   │   │
    │   │   ├─ "never":
    │   │   │   ├─ 파일 존재 → 스킵
    │   │   │   └─ 파일 미존재 → 생성
    │   │   │
    │   │   └─ "marker":
    │   │       ├─ AUTOPUS:BEGIN 찾기
    │   │       ├─ AUTOPUS:END 찾기
    │   │       ├─ 사이 내용 교체
    │   │       └─ 외부 내용 보존
    │   │
    │   ├─ 파일 쓰기
    │   │   ├─ os.WriteFile(full_path, content, 0644)
    │   │   └─ 성공: 카운터 증가
    │   │
    │   └─ 검증 (선택적)
    │       ├─ SHA256 확인
    │       └─ 내용 검증
    │
    └─ 플랫폼별 완료 메시지
        └─ "Claude Code: 12 files written"
```

### 단계 6: 완료 및 출력

```
모든 플랫폼 처리 완료
│
├─ 통계 수집
│   ├─ 총 파일 수: sum(files_written)
│   ├─ 성공한 플랫폼: [platform_names]
│   ├─ 실패한 플랫폼: [platform_names]
│   └─ 처리 시간: end_time - start_time
│
└─ 결과 출력
    ├─ "Successfully initialized Autopus-ADK!"
    ├─ "Installed on 2 platforms:"
    ├─ "  - claude-code: 12 files"
    ├─ "  - cursor: 12 files"
    └─ "Next: auto doctor"
```

---

## 2. update 워크플로우 (상세)

```
auto update
│
├─ 단계 1: 기존 설정 로드
│   └─ LoadConfig() → HarnessConfig
│
├─ 단계 2: 감지된 플랫폼 목록 조회
│   └─ registry.DetectAll() → [PlatformAdapter, ...]
│
├─ 단계 3: 각 플랫폼별 Update 실행
│   │
│   └─ for each adapter in detected_adapters:
│       │
│       ├─ adapter.Update(context, harness_config)
│       │   │
│       │   ├─ 새로운 파일 내용 생성 (Generate와 동일)
│       │   │
│       │   └─ PlatformFiles 반환 (마커 정보 포함)
│       │
│       └─ WriteFiles() (마커 기반 부분 업데이트)
│           │
│           └─ for each file_mapping in platform_files.Files:
│               │
│               ├─ policy == "marker"
│               │   │
│               │   ├─ 기존 파일 읽기
│               │   ├─ "AUTOPUS:BEGIN" 라인 찾기
│               │   ├─ "AUTOPUS:END" 라인 찾기
│               │   ├─ 사이 내용을 새로운 내용으로 교체
│               │   ├─ 마커 외부 사용자 코드 보존
│               │   └─ 파일 다시 쓰기
│               │
│               └─ 검증: 마커 존재 확인
│
└─ 완료 메시지 출력
```

### 마커 기반 업데이트 예시

```
기존 파일:

# Custom Section Start
# (사용자 작성 코드)
print("My custom code")
# Custom Section End

# @MX:NOTE: This is Autopus managed section
# AUTOPUS:BEGIN:v1.0.0
# Old content here
# This will be replaced
# AUTOPUS:END:v1.0.0

# More custom code
# (사용자 작성 코드)

→ 업데이트 후:

# Custom Section Start
# (사용자 작성 코드 - 보존됨)
print("My custom code")
# Custom Section End

# @MX:NOTE: This is Autopus managed section
# AUTOPUS:BEGIN:v1.0.0
# New content from v1.0.1
# This is the updated version
# AUTOPUS:END:v1.0.0

# More custom code
# (사용자 작성 코드 - 보존됨)
```

---

## 3. doctor 워크플로우 (상세)

```
auto doctor
│
├─ 단계 1: 설정 로드
│   └─ LoadConfig() → HarnessConfig
│
├─ 단계 2: 각 플랫폼별 검증
│   │
│   └─ for each adapter in registry.List():
│       │
│       ├─ adapter.Validate(context)
│       │   │
│       │   ├─ 필수 파일 확인
│       │   │   ├─ .claude/CLAUDE.md 존재?
│       │   │   ├─ .claude/agents/*.md 개수?
│       │   │   └─ 기타 파일들
│       │   │
│       │   ├─ 파일 내용 검증
│       │   │   ├─ 문법 검증 (YAML, Go 등)
│       │   │   ├─ 필수 필드 확인
│       │   │   └─ 형식 일관성
│       │   │
│       │   ├─ 설정 일관성 검사
│       │   │   ├─ 마커 쌍 확인 (BEGIN/END)
│       │   │   ├─ 버전 정보 확인
│       │   │   └─ 의존성 확인
│       │   │
│       │   ├─ 권한 확인
│       │   │   └─ 파일 읽기/실행 권한
│       │   │
│       │   └─ ValidationError[] 반환
│       │       └─ {File, Message, Level: "error"/"warning"}
│       │
│       └─ 결과 수집: validation_results[platform.Name]
│
├─ 단계 3: 결과 분류 및 집계
│   │
│   ├─ for each platform, errors in validation_results:
│   │   ├─ errors 분류
│   │   │   ├─ error_count += count(level == "error")
│   │   │   └─ warning_count += count(level == "warning")
│   │   │
│   │   └─ 플랫폼별 상태: 성공/경고/실패
│   │
│   └─ 전체 상태 결정
│       ├─ error_count > 0 → 실패
│       ├─ warning_count > 0 → 경고
│       └─ 모두 0 → 성공
│
├─ 단계 4: 리포트 생성
│   │
│   ├─ 헤더
│   │   └─ "Autopus-ADK Health Report"
│   │
│   ├─ 플랫폼별 상태
│   │   │
│   │   └─ for each platform in platforms:
│   │       ├─ "Claude Code: ✓ PASSED"
│   │       ├─ "  - Agent files: 8/8"
│   │       ├─ "  - Skill files: 12/12"
│   │       └─ "  - Hook files: 4/4"
│   │
│   ├─ 발견된 문제 목록
│   │   │
│   │   └─ for each error in errors:
│   │       ├─ "[ERROR] .claude/agents/expert-backend.md"
│   │       ├─ "  Line 15: Missing required field 'description'"
│   │       └─ "  Fix: Add field as per CLAUDE.md"
│   │
│   └─ 권장사항
│       ├─ "Run: auto update"
│       ├─ "Run: auto init --platform [platform]"
│       └─ "Review: .claude/CLAUDE.md"
│
└─ 단계 5: 출력 및 종료
    │
    ├─ 콘솔에 리포트 출력 (또는 JSON)
    │
    └─ 종료 코드 설정
        ├─ 0: 성공
        ├─ 1: 오류 발견
        └─ 2: 경고만 발견
```

---

## 4. 아키텍처 분석 (arch) 워크플로우

```
auto arch --path ./src
│
├─ 단계 1: 플래그 파싱
│   └─ path = "./src"
│
├─ 단계 2: 코드 구조 분석
│   │
│   └─ arch.Analyze(path)
│       │
│       ├─ filepath.Walk(path, ...)
│       │   │
│       │   └─ for each file in path:
│       │       ├─ 파일 확장자 분류
│       │       │   ├─ .go → Go 파일
│       │       │   ├─ .py → Python 파일
│       │       │   ├─ .ts → TypeScript 파일
│       │       │   └─ 기타
│       │       │
│       │       └─ 파일 메타데이터 수집
│       │           ├─ 경로, 크기
│       │           ├─ 수정 시간
│       │           ├─ 라인 수
│       │           └─ import/require 추출
│       │
│       ├─ 의존성 그래프 구성
│       │   │
│       │   ├─ for each file, imports in files:
│       │   │   ├─ import "pkg/adapter" 찾기
│       │   │   ├─ import "github.com/spf13/cobra" 찾기
│       │   │   └─ 의존성 링크 생성
│       │   │
│       │   └─ dependencies = {
│       │       "internal/cli": ["pkg/adapter", "pkg/config", ...],
│       │       "pkg/adapter": ["pkg/config"],
│       │       ...
│       │     }
│       │
│       ├─ 모듈 계층 분석
│       │   ├─ 레이어 0: import 없음 (pkg/version)
│       │   ├─ 레이어 1: 표준 라이브러리만 (pkg/detect)
│       │   ├─ 레이어 2: 기본 패키지 의존 (pkg/adapter)
│       │   └─ 레이어 N: 모든 의존성 (internal/cli)
│       │
│       └─ ArchitectureAnalysis 객체 반환
│
├─ 단계 3: 아키텍처 린팅
│   │
│   └─ arch.Lint(analysis)
│       │
│       ├─ 규칙 1: 순환 의존성 검사
│       │   └─ if has_cycle() → warning
│       │
│       ├─ 규칙 2: 의존성 역전 검사
│       │   └─ if high_layer depends on low_layer → error
│       │
│       ├─ 규칙 3: 패키지 크기 검사
│       │   └─ if package_lines > 1000 → warning
│       │
│       ├─ 규칙 4: 복잡도 검사
│       │   └─ if cyclomatic_complexity > 15 → warning
│       │
│       └─ LintResult[] 반환
│           ├─ {Type: "warning/error", Message, File, Line}
│
├─ 단계 4: 문서 생성
│   │
│   └─ arch.Generate(analysis, lint_results)
│       │
│       ├─ ARCHITECTURE.md 생성
│       │   ├─ 개요 섹션
│       │   │   └─ 프로젝트 요약, 핵심 설계 원칙
│       │   │
│       │   ├─ 모듈 구조
│       │   │   └─ 각 패키지의 책임 설명
│       │   │
│       │   ├─ 의존성 그래프
│       │   │   ├─ 텍스트 형식
│       │   │   ├─ 다이어그램 (Mermaid 또는 ASCII)
│       │   │   └─ 계층 분석
│       │   │
│       │   ├─ 린팅 결과
│       │   │   ├─ 발견된 문제
│       │   │   ├─ 권장사항
│       │   │   └─ 수정 방법
│       │   │
│       │   └─ 모듈별 세부 정보
│       │       ├─ 파일 목록
│       │       ├─ 책임
│       │       └─ 의존성
│       │
│       └─ 파일 쓰기: ARCHITECTURE.md
│
└─ 완료 메시지
    └─ "Architecture analysis complete"
```

---

## 5. SPEC 분석 (spec) 워크플로우

```
auto spec --file ./SPEC-XXX.md
│
├─ 단계 1: 플래그 파싱
│   └─ file = "./SPEC-XXX.md"
│
├─ 단계 2: SPEC 파일 읽기
│   └─ content := ReadFile(file)
│
├─ 단계 3: SPEC 파싱 (EARS 형식)
│   │
│   └─ spec.Parse(content)
│       │
│       ├─ YAML 프론트매터 추출
│       │   ├─ title, description
│       │   ├─ author, date
│       │   └─ 메타데이터
│       │
│       ├─ EARS 요구사항 파싱
│       │   │
│       │   ├─ Ubiquitous 요구사항
│       │   │   └─ "The system shall always..."
│       │   │
│       │   ├─ Event-driven 요구사항
│       │   │   └─ "When X occurs, the system shall..."
│       │   │
│       │   ├─ State-driven 요구사항
│       │   │   └─ "While X is true, the system shall..."
│       │   │
│       │   ├─ Unwanted 요구사항
│       │   │   └─ "The system shall not..."
│       │   │
│       │   └─ Optional 요구사항
│       │       └─ "The system should where possible..."
│       │
│       └─ Requirement[] 반환
│           └─ {Type, Description, Priority, ...}
│
├─ 단계 4: 템플릿 렌더링
│   │
│   └─ spec.Template(parsed_spec)
│       │
│       ├─ 템플릿 로드: spec_template.txt
│       ├─ 변수 주입
│       │   ├─ {{.Title}}
│       │   ├─ {{.Description}}
│       │   ├─ {{.Requirements}}
│       │   └─ 기타
│       │
│       └─ 렌더링된 문서 반환
│
├─ 단계 5: SPEC 검증
│   │
│   └─ spec.Validate(parsed_spec)
│       │
│       ├─ 필드 검증
│       │   ├─ title 필수
│       │   ├─ description 필수
│       │   └─ requirements 필수
│       │
│       ├─ 형식 검증
│       │   ├─ EARS 형식 확인
│       │   ├─ 마크다운 문법 확인
│       │   └─ 링크 검증
│       │
│       ├─ 일관성 검사
│       │   ├─ 중복 요구사항 확인
│       │   ├─ 모순 검사
│       │   └─ 참조 검증
│       │
│       └─ ValidationError[] 반환
│
└─ 단계 6: 리포트 출력
    │
    ├─ "SPEC Analysis Results"
    ├─ "Title: SPEC-XXX-001"
    ├─ "Requirements: 15"
    ├─ "  - Ubiquitous: 5"
    ├─ "  - Event-driven: 7"
    ├─ "  - State-driven: 2"
    ├─ "  - Unwanted: 1"
    └─ "Validation: PASSED ✓"
```

---

## 상태 관리 및 전환

### 파일 상태 변환

```
상태 다이어그램:

[init]
  ├─ 성공 → [ready]
  └─ 실패 → [error]

[ready]
  ├─ update 실행 → [updated]
  ├─ doctor 실행 → [validated]
  └─ 파일 삭제 → [missing]

[updated]
  ├─ 성공 → [ready]
  └─ 실패 → [error]

[validated]
  ├─ 성공 → [ready]
  ├─ 경고 → [warning]
  └─ 실패 → [error]

[warning]
  ├─ update 실행 → [updated]
  └─ fix 실행 → [ready]

[error]
  ├─ 수동 수정 → [ready]
  ├── re-init → [ready]
  └─ clean 실행 → [missing]

[missing]
  └─ init 실행 → [ready]
```

---

## 데이터 구조 흐름

### HarnessConfig 객체 흐름

```
LoadConfig(path) 또는 DefaultFullConfig()
  │
  ├─ 필드 정보
  │   ├─ Mode: "full" | "lite"
  │   ├─ Platforms: ["claude-code", "cursor", ...]
  │   ├─ ContentOptions: {
  │   │   ├─ IncludeAgents: true/false
  │   │   ├─ IncludeSkills: true/false
  │   │   ├─ IncludeHooks: true/false
  │   │   ├─ IncludeWorkflows: true/false
  │   │   └─ IncludeSpecs: true/false
  │   │ }
  │   ├─ FileSettings: {
  │   │   ├─ BackupOldFiles: true/false
  │   │   ├─ PreserveUserCode: true/false
  │   │   └─ OverwritePolicy: "marker" | "never" | "always"
  │   │ }
  │   └─ OutputPath: "./"
  │
  ├─ 사용처
  │   ├─ adapter.Generate(ctx, config) 호출
  │   ├─ adapter.Update(ctx, config) 호출
  │   └─ adapter.Validate(ctx) 호출
  │
  └─ 저장
      └─ YAML 파일로 직렬화 (변경 시)
```

### PlatformFiles 객체 흐름

```
adapter.Generate(ctx, harness_config)
  │
  ├─ Files: [
  │   │
  │   ├─ FileMapping {
  │   │   ├─ SourceTemplate: "agents/expert-backend.md"
  │   │   ├─ TargetPath: ".claude/agents/expert-backend.md"
  │   │   ├─ OverwritePolicy: "always" | "never" | "marker"
  │   │   ├─ Checksum: "sha256:abc123..."
  │   │   └─ Content: [...bytes...]
  │   │ },
  │   │
  │   └─ ... (더 많은 FileMapping)
  │ ]
  │
  ├─ Checksum: "sha256:combined_all_files..."
  │
  ├─ 사용처
  │   └─ WriteFiles(platform_files)
  │       ├─ 각 FileMapping 처리
  │       ├─ TargetPath에 쓰기
  │       └─ 검증
  │
  └─ 저장
      └─ metadata.json에 기록 (다음 update 시 참조)
```

---

## 에러 처리 및 복구

### 에러 흐름

```
에러 발생
  │
  ├─ 타입 분류
  │   ├─ ErrFileNotFound
  │   │   └─ 권장: init 커맨드 실행
  │   │
  │   ├─ ErrPermissionDenied
  │   │   └─ 권장: 파일 권한 확인
  │   │
  │   ├─ ErrInvalidConfig
  │   │   └─ 권장: 설정 파일 검증
  │   │
  │   ├─ ErrPlatformNotDetected
  │   │   └─ 권장: 플랫폼 수동 지정
  │   │
  │   └─ ErrUnknown
  │       └─ 권장: --verbose 플래그로 상세 정보 확인
  │
  ├─ 메시지 생성
  │   ├─ 에러 설명
  │   ├─ 원인 분석
  │   └─ 해결 방법
  │
  ├─ 콘솔 출력
  │   └─ stderr에 에러 메시지 출력
  │
  ├─ 부분 복구 (가능한 경우)
  │   ├─ 백업 파일 복원
  │   ├─ 트랜잭션 롤백
  │   └─ 상태 정리
  │
  └─ 종료
      └─ exit(1) 또는 해당 코드
```

### 복구 전략

```
실패 → 재시도 → 성공

1. 첫 번째 시도: 표준 실행
2. 실패 시:
   - 에러 로깅
   - 상태 검사
   - 가능하면 부분 복구
3. 재시도:
   - 백업에서 복원
   - 다른 방식으로 재시도
   - 사용자 개입 요청
```

---

## 성능 고려사항

### 데이터 흐름 최적화

```
초기화 (init):
├─ 병렬 플랫폼 감지
│   └─ goroutines 사용 (최대 5개)
│   └─ 예상 시간: 100ms
│
├─ 순차 파일 생성
│   └─ 각 어댑터별
│   └─ 예상 시간: 50-100ms
│
├─ 파일 쓰기 (병렬 가능)
│   └─ 개선 기회
│   └─ 예상 시간: 200-500ms
│
└─ 전체 시간: 500-1000ms

업데이트 (update):
├─ 마커 기반 부분 업데이트
│   └─ 전체 파일 대신 부분만 처리
│   └─ 예상 시간: 100-300ms
│
└─ 의사 (doctor):
├─ 병렬 검증
│   └─ 각 플랫폼별 독립적
│   └─ 예상 시간: 200-500ms
```

---

## 요약

Autopus-ADK의 데이터 흐름은 명확한 단계별 처리와 강력한 에러 처리를 특징으로 합니다:

1. **입력 처리**: CLI 플래그 파싱
2. **준비 단계**: 플랫폼 감지, 설정 로드
3. **처리 단계**: 파일 생성, 템플릿 렌더링
4. **출력 단계**: 파일 쓰기, 검증
5. **완료 단계**: 보고, 종료

각 단계는 독립적이고 테스트 가능하며, 실패 시 명확한 복구 경로를 제공합니다.
