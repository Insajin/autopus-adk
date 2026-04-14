---
name: competitive-analysis
description: SWOT, competitive landscape analysis, and battlecard generation
triggers:
  - competitive
  - competitor
  - 경쟁 분석
  - swot
  - battlecard
  - 배틀카드
category: strategy
level1_metadata: "SWOT analysis, Porter's Five Forces, competitive battlecard, market positioning"
---

# Competitive Analysis Skill

경쟁 환경을 구조적으로 분석하고 포지셔닝 전략을 도출하는 스킬입니다.

## 분석 프레임워크

### 1. SWOT

| 축 | 질문 |
|---|---|
| Strengths | 우리가 경쟁사보다 명확히 잘하는 것은 무엇인가? |
| Weaknesses | 현재 약점과 제약은 무엇인가? |
| Opportunities | 시장 변화가 여는 기회는 무엇인가? |
| Threats | 경쟁, 대체재, 규제가 만드는 위협은 무엇인가? |

SWOT은 항목 나열로 끝내지 않고 아래 전략으로 연결합니다.

| 전략 | 조합 | 의미 |
|------|------|------|
| SO | Strengths × Opportunities | 강점으로 기회를 확대 |
| WO | Weaknesses × Opportunities | 약점 보완 후 기회 포착 |
| ST | Strengths × Threats | 강점으로 위협 방어 |
| WT | Weaknesses × Threats | 회피 / 축소 / 집중 |

### 2. Competitive Landscape

```markdown
| 기준 | Our Product | Competitor A | Competitor B |
|------|-------------|-------------|-------------|
| Core Value | | | |
| Target Users | | | |
| Pricing | | | |
| Key Feature 1 | ✅/⚠️/❌ | ✅/⚠️/❌ | ✅/⚠️/❌ |
| Moat | | | |
```

경쟁사는 다음으로 구분합니다.
- Direct: 같은 문제를 같은 방식으로 해결
- Indirect: 같은 문제를 다른 방식으로 해결
- Potential: 인접 시장에서 진입 가능성이 큰 플레이어

### 3. Porter's Five Forces

| Force | 분석 질문 | 강도 |
|-------|----------|------|
| 경쟁자 간 경쟁 | 차별화가 낮고 경쟁자가 많은가? | Low / Med / High |
| 신규 진입 위협 | 진입 장벽이 낮은가? | Low / Med / High |
| 대체재 위협 | 다른 해결 방식이 쉬운가? | Low / Med / High |
| 공급자 교섭력 | 핵심 의존성 집중도가 높은가? | Low / Med / High |
| 구매자 교섭력 | 전환 비용이 낮은가? | Low / Med / High |

### 4. Battlecard

```markdown
## Battlecard: Our Product vs Competitor

### Where We Win
| 영역 | 우리 | 경쟁사 | Talk Track |

### Where They Win
| 영역 | 우리 | 경쟁사 | Counter |

### Common Objections
| 반론 | 대응 |
```

## 분석 프로세스

1. `.autopus/project/product.md`와 사용자 요청에서 분석 목적을 정리합니다.
2. 경쟁사 목록과 비교 기준을 명시합니다.
3. 웹 검색 또는 제공된 자료로 최신 근거를 수집합니다.
4. SWOT, Landscape, Battlecard 중 필요한 프레임워크를 적용합니다.
5. 최종적으로 차별화 포인트와 전략 액션을 3개 이내로 제안합니다.

## 출력 형식

```markdown
## Competitive Analysis

### 범위
- 대상: {경쟁사 목록}
- 목적: {전략 수립 / 기능 비교 / 세일즈 지원}

### SWOT
{요약}

### Landscape
{비교 표}

### Key Insights
1. {핵심 인사이트}
2. {핵심 인사이트}

### Recommended Actions
1. {실행 액션}
2. {실행 액션}
```
