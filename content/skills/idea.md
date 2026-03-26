---
name: idea
description: 멀티 프로바이더 아이디어 토론 및 발산 스킬
triggers:
  - idea
  - 아이디어 토론
  - brainstorm idea
  - 아이디어 발산
category: workflow
level1_metadata: "멀티 프로바이더 아이디어 토론, SCAMPER/HMW/ICE, BS 파일 생성"
---

# Idea Skill

멀티 프로바이더 오케스트라를 활용해 아이디어를 구조화하고 발산 후 BS 파일로 저장합니다.

## 사용법

```
/auto idea "설명" [--strategy debate|consensus|pipeline|fastest] [--providers list] [--auto]
```

**플래그:**
- `--strategy` — 오케스트레이션 전략 지정 (기본값: `debate`)
- `--providers` — 사용할 프로바이더 목록 (기본값: orchestra 설정 전체)
- `--auto` — 완료 후 `/auto plan --from-idea BS-{ID}` 자동 체이닝

## 저장 위치 규칙

BS 파일은 **대상 모듈** 기준으로 저장합니다.

1. 아이디어 설명에서 관련 코드를 검색하여 대상 서브모듈을 자동 감지
2. `{target-module}/.autopus/brainstorms/`에 BS 파일 생성
3. 감지 실패 시 CWD 기준 `.autopus/brainstorms/`에 저장
4. BS ID는 프로젝트 전체에서 유일해야 함: `.autopus/brainstorms/BS-*` AND `*/.autopus/brainstorms/BS-*` 스캔

이 규칙은 monorepo, submodule, 독립 repo 모든 경우에 동일하게 적용됩니다.

## 5단계 파이프라인

### Step 1: Parse Input and Flags

```
# Parse user input
input = args[0]            # required: idea description
strategy = flags.strategy  # default: "debate"
providers = flags.providers # default: all configured providers
auto_chain = flags.auto    # default: false
```

### Step 2: Structure Idea as What/Why/Who/When

입력을 아래 4개 축으로 구조화합니다:

- **What**: 무엇을 만드는가?
- **Why**: 왜 필요한가? (문제/기회)
- **Who**: 누구를 위한 것인가? (대상 사용자)
- **When**: 언제 필요한가? (타임라인/맥락)

### [REQUIRED] Step 3: Call Orchestra Brainstorm (MUST call Bash tool)

IMPORTANT: 이 단계는 반드시 Bash 툴로 CLI를 실행해야 합니다. Sequential Thinking이나 단일 모델 시뮬레이션으로 대체 금지.

```bash
auto orchestra brainstorm "{structured idea}" --strategy {strategy}
```

- `orchestra.timeout_seconds` 설정에서 프로바이더별 타임아웃 적용
- 프로바이더 실패 시 graceful degradation — 나머지 프로바이더 결과로 계속 진행
- Bash 호출이 에러를 반환한 경우에만 사용자에게 fallback 여부를 확인

> **⏭ POST-STEP**: Orchestra 결과 수신 후 Step 4로 진행. Step 5로 건너뛰지 말 것.

### Step 4: ICE Scoring and Top N Selection

브레인스토밍 결과를 파싱하고 ICE 점수로 수렴합니다:

| 항목 | 설명 | 범위 |
|------|------|------|
| Impact | 영향력 | 1-10 |
| Confidence | 확신도 | 1-10 |
| Ease | 실행 용이성 | 1-10 |

`Score = (Impact × Confidence × Ease) / 100`

상위 N개 아이디어를 선별하여 BS 파일에 기록합니다.

### Step 5: Save and Guide Next Steps

BS-{ID} 파일 저장 후 Workflow Lifecycle 바 표시 및 다음 단계 안내.

**ID 자동 증분**: `{target-module}/.autopus/brainstorms/BS-{ID}.md` 파일이 이미 존재하면 ID를 증분합니다. 전체 프로젝트 스캔으로 ID 유일성을 보장합니다.

## BS 파일 형식

`{target-module}/.autopus/brainstorms/BS-{ID}.md`:

```markdown
# BS-{ID}: {title}

**Created**: {date}
**Strategy**: {strategy}
**Providers**: {provider list}
**Status**: active

## 원본 아이디어
- What: {description}
- Why: {motivation}
- Who: {target users}
- When: {timeline}

## 프로바이더별 발산 결과
{raw brainstorm output}

## ICE 스코어링 — Top N
| Rank | Idea | Impact | Confidence | Ease | Score |
|------|------|--------|------------|------|-------|

## 추천 방향
{judge's recommendation}

## 다음 단계
`/auto plan --from-idea BS-{ID} "feature description"`
```

## 완료 후 출력

```
🐙 Workflow: BS-{ID}
  ● idea  →  ○ plan  →  ○ go  →  ○ sync
```

`--auto` 플래그가 있으면 자동으로 `/auto plan --from-idea BS-{ID}`로 체이닝합니다.
