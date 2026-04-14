---
name: metrics
description: North Star Metric, input metrics, and success dashboard design
triggers:
  - metrics
  - 지표
  - north star
  - 노스스타
  - KPI
  - success metrics
  - 성공 지표
category: strategy
level1_metadata: "North Star Metric, input metrics, success criteria dashboard, metric-driven development"
---

# Metrics Skill

North Star Metric을 정의하고 입력 지표와 guard rail을 설계하는 스킬입니다.

## Metrics Hierarchy

```text
North Star Metric
  ├─ Input Metric 1
  ├─ Input Metric 2
  ├─ Input Metric 3
  └─ Health Metrics
```

## 3단계 설계 프로세스

### 1. North Star Metric 정의

좋은 North Star Metric의 조건:
- 고객 가치를 반영한다
- 팀이 영향을 줄 수 있다
- 측정 가능하다
- 시계열 비교가 가능하다

예시:

| 비즈니스 모델 | North Star 후보 |
|--------------|----------------|
| SaaS | Weekly Active Users |
| Marketplace | Transactions completed |
| E-commerce | Purchase frequency |
| Developer Tool | Deployments / Builds per week |

```markdown
### North Star Metric
- Metric: {지표명}
- Definition: {정의}
- Current Value: {현재 값}
- Target: {목표 값 + 기간}
- Measurement: {측정 방법}
```

### 2. Input Metrics 도출

North Star에 직접 영향을 주는 3-5개 입력 지표를 정의합니다.

```markdown
| Input Metric | Definition | Owner | Current | Target | Leading/Lagging |
|-------------|------------|-------|---------|--------|----------------|
| New Signups | 주간 신규 가입자 수 | Growth | 500 | 800 | Leading |
| Activation Rate | 핵심 기능 활성화 비율 | Product | 35%% | 50%% | Leading |
| W1 Retention | 1주차 재방문율 | Product | 40%% | 55%% | Lagging |
```

유형 분류:
- Acquisition
- Activation
- Engagement
- Retention
- Revenue

### 3. Health Metrics 설정

North Star를 올리더라도 깨지면 안 되는 guard rail을 정의합니다.

```markdown
| Health Metric | Threshold | Alert Condition |
|--------------|-----------|-----------------|
| Error Rate | < 0.1%% | > 0.5%% for 1h |
| P99 Latency | < 500ms | > 1000ms for 30m |
| Support Tickets | < 50/day | > 100/day for 3 days |
```

## Success Dashboard

```markdown
## Success Dashboard: {Feature}

### Primary Metric
- Baseline: {현재 값}
- Target: {목표}
- Measurement window: {기간}

### Secondary Metrics
| Metric | Baseline | Target | Status |

### Guard Rails
| Metric | Acceptable Range | Current |
```

## PRD / SPEC 연동

- PRD의 `Goals & Success Metrics` 섹션에 직접 삽입합니다.
- SPEC의 `acceptance.md`에 측정 가능한 수락 기준으로 연결합니다.

예시:

```markdown
## Acceptance Criteria — Metrics
- [ ] {Primary metric}이 {target} 이상 달성
- [ ] {Guard rail metric}이 {threshold} 미만 유지
- [ ] 측정 이벤트와 대시보드가 정의됨
```

## 출력 형식

```markdown
## Metrics Design

### North Star
{North Star 카드}

### Input Metrics
{입력 지표 테이블}

### Health Metrics
{Guard Rail 테이블}

### Measurement Plan
- Tool: {analytics tool}
- Events: {추적 이벤트}
- Review cadence: {주기}
```
