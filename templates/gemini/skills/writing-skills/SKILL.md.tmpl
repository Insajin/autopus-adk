---
name: auto-writing-skills
---

# Writing Skills

명확하고 유용한 기술 문서와 Claude Code 구성 요소를 작성하는 스킬입니다.

## Claude Code 스킬 작성 (2.0+)

### 스킬 파일 구조

스킬은 `.gemini/skills/<skill-name>/SKILL.md` 에 정의합니다.

### 프론트매터 필드

**표준 필드:**

| 필드 | 필수 | 설명 |
|------|------|------|
| `name` | Yes | 스킬 식별자 (소문자, 하이픈) |
| `description` | Yes | 목적 설명 (YAML folded scalar `>` 사용, 최대 1024자) |
| `allowed-tools` | No | 쉼표 구분 도구 목록 (자동 승인) |
| `user-invocable` | No | `/` 메뉴 노출 여부 (기본: true) |
| `effort` | No | 모델 effort 오버라이드: low, medium, high |
| `model` | No | 모델 오버라이드: opus, sonnet, haiku |
| `context` | No | "fork" — 포크된 서브에이전트에서 실행 |
| `agent` | No | context: fork 시 사용할 서브에이전트 타입 |
| `hooks` | No | 스킬 스코프 라이프사이클 훅 |
| `disable-model-invocation` | No | true — Claude 자동 로딩 방지 |

**주의:** CC 2.1.19부터 `allowed-tools`가 없는 스킬은 도구 사용 시 사용자 승인이 필요합니다.

### 템플릿 변수

- `$ARGUMENTS` — 사용자가 전달한 전체 인자
- `$ARGUMENTS[N]` / `$N` — N번째 위치 인자
- `${CLAUDE_SKILL_DIR}` — 스킬 자체 디렉터리 경로 (모듈 참조에 유용)

### 스킬 예시

```yaml
---
name: my-skill
description: >
  스킬이 하는 일을 설명합니다.
  여러 줄은 YAML folded scalar (>)를 사용합니다.
allowed-tools: Read, Grep, Glob, Bash
user-invocable: true
effort: medium
---

# My Skill

스킬 본문 내용...
```

## Claude Code 에이전트 작성 (2.0+)

에이전트 정의 가이드는 `subagent-dev` 스킬을 참조하세요.

핵심 프론트매터: `name`, `description`, `model`, `tools`, `permissionMode`, `maxTurns`, `skills`, `isolation`, `background`, `memory`

## 문서 유형별 가이드

### README 작성
```markdown
# 프로젝트명

한 줄 설명

## 설치

\```bash
go get github.com/...
\```

## 빠른 시작

\```go
// 핵심 사용 예시
\```

## API

[주요 API 설명]

## 기여하기

[기여 방법]
```

### API 문서
각 엔드포인트에 대해:
- **Method**: HTTP 메서드
- **Path**: URL 경로
- **Request**: 요청 형식 (Body, Params)
- **Response**: 응답 형식
- **Errors**: 에러 코드 목록
- **Example**: 실제 예시

### 기술 설계 문서
```markdown
## 문제 정의
[해결할 문제]

## 제약 조건
[기술적, 비즈니스 제약]

## 설계 결정
[결정 사항과 이유]

## 대안 검토
[고려한 대안과 기각 이유]

## 구현 계획
[단계별 구현 계획]
```

## 작성 원칙

### 독자 중심
- 독자가 누구인가? (초보자 vs 전문가)
- 독자가 무엇을 알고 싶어하는가?
- 독자가 무엇을 하고 싶어하는가?

### 명확성
- 짧은 문장 (20단어 이하 권장)
- 능동태 사용
- 전문 용어는 처음 사용 시 정의

### 실용성
- 모든 개념에 예시 포함
- 복사-붙여넣기 가능한 코드 스니펫
- 실제 사용 시나리오 기반

## 검토 체크리스트

- [ ] 핵심 독자가 이해할 수 있는가?
- [ ] 설치/실행 방법이 명확한가?
- [ ] 모든 예시가 동작하는가?
- [ ] 오타 및 문법 오류가 없는가?
- [ ] 링크가 유효한가?
