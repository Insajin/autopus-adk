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
2. **단일 모듈 대상**: `{target-module}/.autopus/brainstorms/`에 BS 파일 생성
3. **크로스-모듈 (2+ 모듈)**: 루트 `.autopus/brainstorms/`에 BS 파일 생성 (meta repo 커밋 대상)
4. 감지 실패 시 루트 `.autopus/brainstorms/`에 저장
5. BS ID는 프로젝트 전체에서 유일해야 함: `.autopus/brainstorms/BS-*` AND `*/.autopus/brainstorms/BS-*` 스캔

Ref: `.claude/rules/autopus/doc-storage.md` for full storage rules.

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

#### Opportunity-Solution Tree (선택)

아이디어가 기존 제품 개선인 경우, OST 프레임워크로 구조화:

```
Outcome (목표)
  └─ Opportunity (기회/문제)
       ├─ Solution A
       │    └─ Experiment (검증 방법)
       ├─ Solution B
       │    └─ Experiment
       └─ Solution C
            └─ Experiment
```

- **Outcome**: 달성하려는 비즈니스/사용자 목표
- **Opportunity**: 사용자의 unmet need 또는 pain point
- **Solution**: 기회를 해결하는 구체적 방안
- **Experiment**: 솔루션의 가정을 검증하는 최소 실험

#### Assumption Identification

아이디어의 핵심 가정을 4축으로 식별:

| 축 | 질문 | 예시 |
|---|---|---|
| **Value** | 사용자가 이것을 원하는가? | "사용자가 자동 분석을 필요로 한다" |
| **Usability** | 사용자가 이것을 쓸 수 있는가? | "CLI 인터페이스로 충분하다" |
| **Feasibility** | 기술적으로 구현 가능한가? | "LLM API 지연이 허용 범위 내다" |
| **Viability** | 비즈니스적으로 지속 가능한가? | "API 비용이 수익 내에서 감당 가능하다" |

가장 위험한 가정(높은 Impact × 높은 Uncertainty)을 상위 3개 식별합니다.

### ⛔ 2-Round Debate Enforcement (HARD REQUIREMENT)

IMPORTANT: 멀티 프로바이더 아이디어 토론은 **반드시 2라운드**를 완료해야 합니다. 이 규칙은 절대 위반할 수 없습니다.

**위반 탐지 체크리스트** (Step 4 진입 전 메인 세션이 반드시 확인):

- [ ] Round 1 실행됨: 각 프로바이더/분석자가 독립적으로 발산 (Step 3 완료)
- [ ] Round 2 실행됨: 각 프로바이더/분석자가 다른 참가자의 Round 1 결과를 읽고 인정/통합/리스크 3단계로 응답 (Step 3.5-3.6 완료)
- [ ] Round 2 결과 수집됨: 모든 프로바이더의 Round 2 응답이 수집됨 (Step 3.6 완료)

WHEN 위 체크리스트의 어느 항목이라도 미완료 상태에서 Step 4(Judge)로 진행하려 하면:
- **HARD BLOCK**: Step 4 진입을 차단하고, 누락된 단계로 되돌아감
- Round 1만 완료하고 Round 2를 건너뛰는 것은 **명시적으로 금지**됨
- Fallback 모드(orchestra 실패 시 Agent 기반 토론)에서도 2라운드 의무는 동일하게 적용

**Why**: Round 1만으로는 각 분석자가 자기 관점에만 갇힘. Round 2 교차 수분(cross-pollination)이 품질 향상의 핵심 — MAD 연구에서 one-shot revision이 대부분의 품질 향상을 가져옴.

### [REQUIRED] Step 3: Orchestra Round 1 (MUST call Bash tool)

IMPORTANT: 이 단계는 반드시 Bash 툴로 CLI를 실행해야 합니다. Sequential Thinking이나 단일 모델 시뮬레이션으로 대체 금지.

#### Multi-Perspective Brainstorming

Orchestra 프롬프트에 3가지 관점을 포함하여 다각적 발산을 유도:

- **PM 관점**: 사용자 가치, 비즈니스 임팩트, 우선순위
- **Designer 관점**: UX, 접근성, 사용자 여정, 인터랙션 패턴
- **Engineer 관점**: 기술적 실현 가능성, 아키텍처, 성능, 보안

```bash
auto orchestra brainstorm "{structured idea}" --strategy debate --no-judge --yield-rounds --context --timeout 300 --no-detach
```

- `--no-judge --yield-rounds`: Round 1만 실행 후 JSON 결과 출력, pane 유지
- `--context`: 프로젝트 컨텍스트(ARCHITECTURE.md, product.md, structure.md)를 brainstorm 프롬프트에 주입하여 프로바이더가 프로젝트를 이해한 상태에서 발산
- 메인 세션이 직접 judge 역할 수행 (프로젝트 전체 컨텍스트 활용 가능)
- Bash 호출이 에러를 반환한 경우에만 사용자에게 fallback 여부를 확인

#### JSON 출력 파싱

Orchestra는 stdout에 JSON을 출력합니다. 파싱하여 각 프로바이더의 Round 1 응답과 pane ID를 추출합니다.

> **⏭ POST-STEP**: Round 1 JSON 수신 후 Step 3.5로 진행.

### [REQUIRED] Step 3.5: 구조화된 Round 2 준비 및 주입 (메인 세션)

메인 세션이 Round 1 결과를 정리하고, 각 프로바이더 pane에 **구조화된 정보 기반 수정(informed revision)** 프롬프트를 직접 주입합니다. 적대적 "반박"이 아닌 **교차 수분(cross-pollination)** 방식입니다.

Ref: Multi-Agent Debate 연구 — one-shot revision이 대부분의 품질 향상을 가져오며, 적대적 반박은 방어적 태도와 아첨(sycophancy)을 유발할 수 있음.

#### Round 2 입력 소스 우선순위

Round 1 결과를 준비할 때, 아래 우선순위로 소스를 선택합니다:

1. **1순위 — `--yield-rounds` JSON output**: orchestra가 수집한 cleaned output 사용. JSON의 `round_history[0].responses[].output` 필드에 이미 TUI 노이즈가 제거된 텍스트가 포함됨.
2. **2순위 — `auto orchestra collect <session-id> --clean`**: JSON output이 빈 프로바이더가 있을 경우, collect 명령으로 pane scrollback을 읽고 Go 엔진의 sanitizer(`CleanScreenForCrossPollination`)를 적용. ICE 점수도 자동 제거됨.
3. **3순위 — `cmux read-screen` 직접 읽기**: collect 명령도 실패할 경우에만 사용. 반드시 `SanitizeScreenOutput` 수준의 정제를 수동 적용해야 함.

IMPORTANT: `grep` 패턴 매칭으로 핵심만 추출하는 것은 금지. 원문의 대부분이 누락됨. TUI 노이즈만 제거하고 나머지는 원문 그대로 전달해야 합니다.

#### Round 2 입력 준비 가이드라인

1. **익명화 필수**: 프로바이더 이름 대신 "토론자 A", "토론자 B"로 표기
2. **ICE 점수 제거**: Round 1의 자체 ICE 점수를 제거하고 아이디어 내용만 전달 (자신감 전파/confidence cascade 방지). `CleanScreenForCrossPollination`이 자동 처리.
3. **원문에 가깝게**: 핵심 주장, 구체적 제안, 근거를 축약하지 않고 전달 (TUI 노이즈만 제거). grep/요약 금지.

#### 구조화된 Round 2 프롬프트

각 프로바이더에게 동일한 3단계 구조로 응답하도록 요청합니다:

```
다른 토론자들의 Round 1 아이디어를 읽었습니다.

## 토론자 {X}의 핵심 주장:
{cleaned output — ICE 점수 제거, TUI 노이즈 제거}

## 토론자 {Y}의 핵심 주장:
{cleaned output}

아래 3단계 구조로 응답해주세요:

### 1단계: 인정 (Acknowledge)
다른 토론자의 아이디어 중 가장 강한 점 2-3개를 식별하고,
**왜 강한지 구체적 근거**를 제시하세요.
주의: 무비판적으로 수용하지 마세요. 근거 없는 인정은 가치 없습니다.

### 2단계: 통합 (Integrate)
자신의 Round 1 핵심 아이디어는 **반드시 유지**하되,
다른 토론자의 강점을 결합하여 **개선된 통합 제안**을 만드세요.
자기 아이디어를 포기하지 마세요 — 보강하세요.

### 3단계: 리스크 (Risk)
통합된 제안에 남아있는 **약점, 리스크, 실현 장벽**을 지적하세요.
```

#### Round 2 주입 절차

```bash
cmux set-buffer "{structured round 2 prompt for provider}"
cmux paste-buffer --surface "{surface_id}"
sleep 1
cmux send --surface "{surface_id}" "\n"
```

#### 익명화 매핑

| 실제 프로바이더 | Round 2 내 표기 |
|----------------|-----------------|
| claude | 토론자 A |
| codex | 토론자 B |
| gemini | 토론자 C |

매핑은 라운드 간 일관 유지. 메인 세션만 매핑을 보유.

> **⏭ POST-STEP**: 3개 프로바이더에 Round 2 프롬프트 주입 완료 후 Step 3.6으로 진행.

### [REQUIRED] Step 3.6: Round 2 결과 수집 (메인 세션)

CC21 경로에서는 고정 sleep polling 대신 provider별 `Monitor`를 먼저 사용합니다.
명령 템플릿과 line-buffered `grep` 규칙은 `.claude/skills/autopus/monitor-patterns.md`를 따릅니다.

```python
Monitor(
  description="wait for claude round2 idle",
  command='cmux read-screen --surface "{surface_id}" --follow | stdbuf -oL grep -E "^(❯|codex>|> Type your|gemini>|opencode›)"',
  timeout_ms=180000,
  persistent=False,
)
```

규칙:

- provider당 1개 Monitor를 띄우고, 모두 idle이 되면 즉시 scrollback을 수집합니다.
- Monitor timeout 또는 비-Claude-Code 런타임이면 기존 `cmux read-screen --scrollback` polling fallback으로 전환합니다.
- timeout fallback이 발생하면 로그에 `monitor_timeout=true`와 provider 이름을 남깁니다.

> **⏭ POST-STEP**: Round 2 결과 수집 후 Step 3.7로 진행.

### [REQUIRED] Step 3.7: Pane 정리

```bash
cmux close-surface --surface "{surface_id}"
```

모든 프로바이더 pane을 닫습니다.

> **⏭ POST-STEP**: Pane 정리 후 Step 4로 진행.

### ⛔ Pre-Step 4 Gate: 2-Round Completion Verification

BEFORE proceeding to Step 4, THE SYSTEM SHALL verify the 2-Round Debate Enforcement checklist:

```
IF Round_1_completed == false:
  → HARD BLOCK: "Step 3 미완료. Round 1을 먼저 실행하세요."
  → Return to Step 3

IF Round_2_completed == false:
  → HARD BLOCK: "Step 3.5-3.6 미완료. Round 2 교차 수분을 먼저 실행하세요."
  → Return to Step 3.5

IF Round_2_results_collected == false:
  → HARD BLOCK: "Round 2 결과 미수집. Step 3.6을 완료하세요."
  → Return to Step 3.6
```

이 게이트를 통과하지 못하면 Step 4로 진입할 수 없습니다. `--auto` 모드에서도 예외 없음.

### [REQUIRED] Step 4: Blind Synthesis Judge (MUST call Agent tool)

IMPORTANT: 편향 방지를 위해 최종 판정은 **서브에이전트에 위임**합니다. 메인 세션이 직접 scoring하지 않습니다. Judge는 토론자와 **다른 모델**을 사용하여 모델 편향을 방지합니다.

Ref: MAD 연구 — "judge와 debaters는 서로 다른 LLM을 사용해야 한다", 같은 모델이면 해당 모델의 문체/사고 패턴을 무의식적으로 선호.

#### 4.1: 익명화된 입력 준비 (메인 세션)

메인 세션은 Round 1 + Round 2 결과를 **익명화**하여 서브에이전트에 전달합니다:

- 프로바이더 이름을 **토론자 A, B, C**로 치환
- 매핑 테이블은 메인 세션만 보유 (서브에이전트에 전달 금지)
- TUI 노이즈(배너, 프롬프트 echo, 시스템 메시지)만 제거하고 **응답 원문은 축약하지 않음**
- Round 1의 SCAMPER 분석, HMW 질문 + Round 2의 인정/통합/리스크 **모든 내용을 원문 그대로** 전달
- 프로젝트 컨텍스트(ARCHITECTURE.md, product.md)를 함께 주입

IMPORTANT: 응답을 요약하거나 축약하면 judge가 충분한 맥락 없이 판단하게 됩니다. TUI 노이즈만 제거하고, 아이디어 내용 자체는 원문 보존이 원칙입니다.

#### 4.2: 서브에이전트 blind judge 호출

Judge는 토론자(claude/codex/gemini)와 다른 판단 프로필을 사용합니다. 가장 저렴한 티어는 피하고, 표준 리뷰 모델 또는 토론자에 포함되지 않은 모델을 지정합니다.

```
Agent(
  subagent_type = "general-purpose",
  prompt = """
    ## 프로젝트 컨텍스트
    {ARCHITECTURE.md 전문 또는 product.md — 프로젝트 구조, 기술 스택, 핵심 도메인}

    ## 토론 결과 (익명 — 어떤 AI 모델이 작성했는지 알 수 없음)
    아래 3명의 토론자가 동일 주제에 대해:
    - Round 1: 독립적으로 아이디어를 발산
    - Round 2: 서로의 아이디어를 인정/통합/리스크 분석
    한 결과입니다. 원문 그대로 제공됩니다.

    ### 토론자 A
    **Round 1 (독립 발산 원문)**:
    {cleaned full output — 축약 금지}

    **Round 2 (인정/통합/리스크 원문)**:
    {cleaned full output — 축약 금지}

    ### 토론자 B
    **Round 1 (독립 발산 원문)**: {원문}
    **Round 2 (인정/통합/리스크 원문)**: {원문}

    ### 토론자 C
    **Round 1 (독립 발산 원문)**: {원문}
    **Round 2 (인정/통합/리스크 원문)**: {원문}

    ## 과제: 합의 기반 통합 판정

    ### 1. 합의 영역 추출
    2명 이상의 토론자가 Round 2에서 **공통으로 통합한** 아이디어를 추출하세요.
    공통 통합 = 높은 확신도 신호입니다.

    ### 2. 고유 인사이트 식별
    1명만 제안하고 다른 토론자가 통합하지 않은 독창적 아이디어를 식별하세요.
    독창적이지만 위험할 수 있으므로 별도 표기합니다.

    ### 3. 교차 리스크 분석
    복수 토론자가 Round 2 Step 3에서 **공통으로 지적한 리스크**를 정리하세요.
    공통 리스크 = 실재할 가능성이 높은 위험입니다.

    ### 4. 최종 통합 ICE 스코어링
    위 분석을 기반으로 Top 5 아이디어를 선정하고 ICE 스코어링하세요:
    - Impact (1-10): 프로젝트 컨텍스트를 고려한 실질적 영향력
    - Confidence (1-10): **합의 수준 반영** — 3명 합의 > 2명 > 1명
    - Ease (1-10): 현재 코드베이스에서의 구현 용이성, 교차 리스크 반영
    - Score = (Impact × Confidence × Ease) / 100

    나머지 아이디어는 부록에 포함하세요.
    아이디어의 내용만으로 평가하세요. 토론자의 정체는 알 수 없으며 알 필요도 없습니다.
  """
)
```

#### 4.3: 결과 수신 및 매핑 복원

서브에이전트의 판정 결과를 수신한 후, 메인 세션이 익명 매핑을 복원하여 BS 파일에 기록합니다:
- 토론자 A → {실제 프로바이더 이름} (BS 파일의 프로바이더별 발산 결과 섹션용)
- ICE Top N은 익명 상태 그대로 기록 (어떤 프로바이더가 제안했는지는 부차적)
- 합의 영역 / 고유 인사이트 / 교차 리스크를 BS 파일에 별도 섹션으로 기록

#### Assumption Risk Overlay

ICE Top N 아이디어 각각에 대해 Step 2에서 식별한 가정의 위험도를 오버레이:

| Rank | Idea | ICE Score | Top Risk Assumption | Risk Level |
|------|------|-----------|---------------------|------------|
| 1 | ... | 7.2 | "사용자가 X를 원한다" (Value) | HIGH |
| 2 | ... | 6.8 | "API 지연 < 500ms" (Feasibility) | MEDIUM |

HIGH 위험 가정이 있는 아이디어는 `/auto plan` 전에 검증 실험을 권장합니다.

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
