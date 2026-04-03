# SPEC-LEARN-001 구현 계획

## 태스크 목록

- [ ] T1: `pkg/learn/types.go` — LearningEntry 구조체, 타입 상수, Severity enum 정의
- [ ] T2: `pkg/learn/store.go` — JSONL Store (Read, Append, NextID, UpdateReuseCount)
- [ ] T3: `pkg/learn/query.go` — 관련성 매칭 엔진 (파일 경로, 패키지, 키워드 스코어링)
- [ ] T4: `pkg/learn/prune.go` — 시간 기반 자동 정리 (90일, sync 시 자동 호출)
- [ ] T5: `pkg/learn/summary.go` — 학습 요약 생성 (반복 패턴 Top N, 신규 항목 통계, 개선 영역 비교)
- [ ] T6: Gate/Phase 자동 기록 훅 — Gate FAIL, Review REQUEST_CHANGES, Executor 연속 실패 시 자동 기록
- [ ] T7: 스킬 통합 — auto-router.md.tmpl의 go/fix/sync 섹션에 learnings 자동 트리거 삽입
- [ ] T8: 세션 시작 통합 — Context Load에 learnings 알림 조건 추가
- [ ] T9: 테스트 — T1~T6 각 패키지별 단위 테스트

## 구현 전략

### 기존 코드 활용

- **store 패턴**: `pkg/lore/writer.go`의 파일 append 패턴을 참고하여 JSONL writer 구현
- **Gate 훅**: 파이프라인 Gate 결과에 learning callback 추가 (구조체 변경 없이 외부 훅)
- **타입 정의**: `pkg/lore/types.go`와 유사한 구조

### 변경 범위

| 범위 | 파일 수 | 변경 유형 |
|------|---------|-----------|
| 신규 Go 패키지 | 5 | `pkg/learn/` 전체 신규 |
| Gate 통합 | 1-2 | 기존 파일에 훅 추가 |
| 스킬 템플릿 | 1 | `auto-router.md.tmpl` 섹션 수정 |
| 테스트 | 5 | 각 Go 파일에 대응하는 테스트 |

### 의존 관계

```
T1 (types) ← T2 (store) ← T3 (query)
                         ← T4 (prune)
                         ← T5 (summary)
T2 ← T6 (gate hooks)
T3 ← T7 (skill injection points)
T5 ← T7 (sync summary display)
T7 ← T8 (session start notification)
```

### 병렬 실행 가능 그룹

- **Group A** (순차→병렬): T1 → T2 완료 후 T3, T4, T5 병렬
- **Group B** (T2 완료 후): T6 (gate hooks)
- **Group C** (T3+T5 완료 후): T7 (skill integration), T8 (session start)
- **Group D** (전체 완료 후): T9 (테스트)

### CLI 커맨드: 없음

기존 plan의 T5 (`internal/cli/learn.go`)와 T10 (`auto-router.md.tmpl` learn 라우팅)은 삭제.
모든 기능은 기존 워크플로우 내부에서 자동 트리거.
