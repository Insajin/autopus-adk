# SPEC-LEARN-001 구현 계획

## 태스크 목록

- [ ] T1: `pkg/learn/types.go` — LearningEntry 구조체, 타입 상수, Severity enum 정의
- [ ] T2: `pkg/learn/store.go` — JSONL Store (Read, Append, NextID, UpdateReuseCount)
- [ ] T3: `pkg/learn/query.go` — 관련성 매칭 엔진 (파일 경로, 패키지, 키워드 스코어링)
- [ ] T4: `pkg/learn/prune.go` — 시간 기반 정리 (기본 90일, configurable)
- [ ] T5: `internal/cli/learn.go` — `auto learn` 서브커맨드 (list/add/prune) cobra 등록
- [ ] T6: `internal/cli/gate.go` 확장 — Gate FAIL 시 learning 자동 기록 훅
- [ ] T7: `content/skills/agent-pipeline.md` 수정 — Phase 1 learnings 주입 프롬프트 추가
- [ ] T8: `content/skills/debugging.md` 수정 — learnings 참조 단계 추가
- [ ] T9: 테스트 — T1~T6 각 패키지별 단위 테스트
- [ ] T10: 통합 — `templates/claude/commands/auto-router.md.tmpl`에 learn 서브커맨드 라우팅 추가

## 구현 전략

### 기존 코드 활용

- **store 패턴**: `pkg/lore/writer.go`의 파일 append 패턴을 참고하여 JSONL writer 구현
- **CLI 등록**: `internal/cli/root.go`의 cobra 서브커맨드 등록 패턴 따름
- **Gate 훅**: `internal/cli/gate.go`의 `GateResult`에 learning callback 추가 (GateResult 구조체 변경 없이 외부 훅)
- **타입 정의**: `pkg/lore/types.go`와 유사한 구조 (필드 + 메타데이터)

### 변경 범위

| 범위 | 파일 수 | 변경 유형 |
|------|---------|-----------|
| 신규 Go 패키지 | 4 | `pkg/learn/` 전체 신규 |
| CLI 확장 | 1 | `internal/cli/learn.go` 신규 |
| Gate 통합 | 1 | `internal/cli/gate.go` 또는 별도 훅 파일 |
| 스킬 프롬프트 | 3 | 기존 md 파일 수정 (섹션 추가) |
| 테스트 | 4-5 | 각 Go 파일에 대응하는 테스트 |

### 의존 관계

```
T1 (types) ← T2 (store) ← T3 (query)
                         ← T4 (prune)
T1 + T2 ← T5 (CLI)
T2 ← T6 (gate hook)
T3 ← T7 (pipeline skill injection)
T3 ← T8 (debugging skill injection)
T5 ← T10 (router template)
```

### 병렬 실행 가능 그룹

- **Group A** (병렬): T1 → T2, T3, T4 (types 완료 후 store/query/prune 병렬)
- **Group B** (병렬): T5, T6 (CLI와 gate hook 독립)
- **Group C** (병렬): T7, T8, T10 (스킬 프롬프트 수정 독립)
- **Group D** (순차): T9 (모든 Go 코드 완료 후 테스트)
