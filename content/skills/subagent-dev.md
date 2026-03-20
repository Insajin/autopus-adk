---
name: subagent-dev
description: 서브에이전트 개발 및 오케스트레이션 스킬
triggers:
  - subagent
  - 서브에이전트
  - agent development
  - 에이전트 개발
  - orchestration
category: agentic
level1_metadata: "서브에이전트 설계, 오케스트레이션 패턴, 병렬 실행"
---

# Subagent Development Skill

효과적인 서브에이전트를 설계하고 오케스트레이션하는 스킬입니다.

## 에이전트 정의 형식 (Claude Code 2.0+)

에이전트는 `.claude/agents/<name>.md` 에 YAML 프론트매터로 정의합니다.

### 프론트매터 필드

| 필드 | 필수 | 기본값 | 설명 |
|------|------|--------|------|
| `name` | Yes | - | 고유 식별자, 소문자+하이픈 |
| `description` | Yes | - | 에이전트 역할 설명 (언제 위임할지 기준) |
| `tools` | No | 전체 상속 | 허용 도구 목록 (allowlist) |
| `disallowedTools` | No | 없음 | 차단 도구 목록 (denylist, tools와 상호배타) |
| `model` | No | 상속 | opus, sonnet, haiku |
| `permissionMode` | No | default | 권한 모드 (아래 표 참조) |
| `maxTurns` | No | 무제한 | 최대 에이전틱 턴 수 |
| `skills` | No | 없음 | 에이전트 컨텍스트에 주입할 스킬 목록 |
| `hooks` | No | 없음 | 에이전트 스코프 라이프사이클 훅 |
| `memory` | No | 없음 | 영속 메모리 스코프 (user, project, local) |
| `isolation` | No | 없음 | "worktree" — 격리된 git worktree에서 실행 |
| `background` | No | false | 백그라운드 실행 (대화 비차단) |

### 권한 모드 (permissionMode)

| 모드 | 동작 | 적합한 에이전트 |
|------|------|----------------|
| `default` | 표준 권한 확인 | 범용 에이전트 |
| `acceptEdits` | 파일 편집 자동 승인 | 구현 에이전트 |
| `plan` | 읽기 전용, 쓰기 불가 | 분석/리서치 에이전트 |
| `dontAsk` | 모든 권한 요청 자동 거부 | 샌드박스 에이전트 |
| `bypassPermissions` | 모든 권한 검사 생략 | 완전 신뢰 자동화 |

### 정의 예시

```markdown
---
name: expert-backend
description: 백엔드 API 구현 전문 에이전트. REST/gRPC 엔드포인트, 비즈니스 로직, DB 연동을 담당한다.
model: sonnet
tools: Read, Write, Edit, Grep, Glob, Bash
permissionMode: acceptEdits
maxTurns: 50
skills:
  - tdd
  - ddd
---

역할 지침:
1. 무엇을 해야 하는가
2. 어떤 파일을 수정할 수 있는가
3. 완료 기준은 무엇인가
4. 결과를 어떻게 보고하는가
```

## 서브에이전트 설계 원칙

### 단일 책임 원칙
각 에이전트는 하나의 명확한 역할을 가집니다:
- 구현 에이전트: `permissionMode: acceptEdits`, `tools`에 Write/Edit 포함
- 분석 에이전트: `permissionMode: plan`, 읽기 도구만 포함

### 격리 원칙
에이전트는 독립적으로 실행됩니다:
- 이전 대화 히스토리에 접근 불가
- 필요한 모든 컨텍스트를 spawn prompt에 포함
- 결과는 구조화된 형식으로 반환
- `isolation: worktree` — 여러 에이전트가 동시에 파일 수정 시 충돌 방지

### 병렬 실행 원칙
독립적인 작업은 병렬로 실행합니다:
```
독립적 → 병렬 실행 (단일 메시지에 여러 Agent() 호출)
의존적 → 순차 실행 (이전 결과를 다음 입력으로)
```

## 오케스트레이션 패턴

### Fan-Out / Fan-In
```
조율자 → [에이전트 A, 에이전트 B, 에이전트 C] (병렬)
         → 결과 통합 → 조율자
```

### Pipeline
```
에이전트 A → 결과 → 에이전트 B → 결과 → 에이전트 C
```

### Supervisor
```
감독자 에이전트 → 실행자 에이전트 모니터링
               → 실패 시 재시도 또는 대안 전략
```

## 에이전트 간 통신

### 서브에이전트 방식 (기본)
- `Agent()` 호출로 생성, 결과 반환 후 종료
- 서브에이전트는 다른 서브에이전트를 생성할 수 없음
- 중단된 에이전트는 `SendMessage({to: agentId})`로 재개

### Agent Teams 방식 (실험적)
- `CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1` 필요
- `TeamCreate`로 팀 생성, `SendMessage`로 팀원 간 통신
- `TaskCreate/Update/List/Get`으로 작업 공유
- 완료 후 `TeamDelete`로 팀 리소스 해제

## 완료 기준

- [ ] 에이전트 정의에 CC 2.0+ 프론트매터 사용
- [ ] `description`에 명확한 역할과 위임 기준 포함
- [ ] 최소 권한 원칙 (`tools`, `permissionMode` 적절히 설정)
- [ ] `maxTurns` 설정으로 무한 루프 방지
- [ ] 병렬 실행 기회 파악
- [ ] 에러 처리 전략 포함 (최대 3회 재시도 후 사용자 개입 요청)
