---
name: product-discovery
description: Product discovery workflow with OST, assumption testing, and interview scripts
triggers:
  - discover
  - discovery
  - 디스커버리
  - 제품 발견
  - assumption test
  - 가정 검증
category: workflow
level1_metadata: "Opportunity-Solution Tree, assumption testing, interview script, user research"
---

# Product Discovery Skill

Opportunity-Solution Tree를 중심으로 사용자 문제를 탐색하고 솔루션 가설을 검증하는 디스커버리 워크플로우입니다.

## 핵심 흐름

### 1. Outcome 정의

- Business Outcome: 어떤 비즈니스 지표를 움직이려는가?
- Product Outcome: 사용자 행동이 어떻게 바뀌어야 하는가?
- Constraint: 시간, 기술, 리소스 제약은 무엇인가?

Outcome은 반드시 측정 가능하고 기간이 명시되어야 합니다.

### 2. Opportunity 탐색

사용자의 unmet need와 friction을 구조화합니다.

추천 소스:
- 사용자 피드백 / CS 티켓
- 인터뷰 인사이트
- 경쟁사 분석 (`competitive-analysis` 스킬 연계)
- 퍼널 데이터와 사용 패턴

```markdown
| ID | Opportunity | Source | Impact | Confidence |
|----|-------------|--------|--------|------------|
| O1 | 사용자가 X를 찾는 데 오래 걸린다 | 인터뷰 3건 | High | Medium |
| O2 | Y 기능의 실패율이 높다 | 지원 티켓 | Medium | High |
```

### 3. Solution Ideation

각 Opportunity마다 최소 3개의 솔루션 후보를 만듭니다.

- PM 관점: 제품 전략과 우선순위
- Designer 관점: UX 단순화
- Engineer 관점: 구현 가능성과 비용

### 4. Assumption Mapping

각 솔루션의 핵심 가정을 4축으로 정리합니다.

| 축 | 검증 질문 |
|---|---|
| Value | 사용자가 정말 원하고 있는가? |
| Usability | 이해하고 쓸 수 있는가? |
| Feasibility | 현재 스택과 팀으로 구현 가능한가? |
| Viability | 비즈니스적으로 지속 가능한가? |

Impact × Uncertainty가 높은 가정을 먼저 검증합니다.

### 5. Experiment Design

가정별로 최소 비용 실험을 설계합니다.

| 실험 유형 | 적합한 가정 | 특징 |
|----------|-------------|------|
| Fake Door | Value | 관심도 빠른 검증 |
| Concierge | Value / Usability | 사람 기반 수동 검증 |
| Wizard of Oz | Usability / Feasibility | 내부 수동 처리 숨김 |
| Prototype | Usability | UX 검증 |
| MVP | 전체 | 비용 크지만 검증력 높음 |

```markdown
### Experiment: {name}
- Assumption: {검증 대상}
- Type: {실험 유형}
- Metric: {측정 지표}
- Success Criteria: {수치 기준}
- Duration: {기간}
```

### 6. Interview Script 생성

필요 시 인터뷰 스크립트를 다음 구조로 만듭니다.

- Warm-up: 맥락과 최근 경험 파악
- Context: 현재 해결 방식과 pain point 확인
- Deep Dive: 핵심 가정 검증 질문
- Wrap-up: 후속 인터뷰 / 추가 사례 확보

원칙:
- 유도 질문 금지
- 미래 의향보다 과거 행동 중심
- 5 Whys로 근본 동기 파악

### 7. Discovery Summary 저장

결과는 BS 파일 또는 PRD/SPEC 연구 문서로 저장합니다.

```markdown
# BS-{ID}: Discovery — {title}

## Outcome
{목표}

## Opportunities
{우선순위 포함 목록}

## Key Assumptions
| # | Assumption | Axis | Impact | Uncertainty | Action |

## Experiments
{실험 카드}
```

## 출력 형식

```markdown
## Discovery Summary

### Outcome
{목표}

### Opportunities
{핵심 기회 3-5개}

### Key Assumptions
{우선 검증할 가정}

### Experiments
{실험 설계}

### Next Step
/auto plan --from-idea BS-{ID} "feature description"
```
