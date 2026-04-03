# SPEC-LEARN-001 수락 기준

## 시나리오

### S1: Gate 실패 시 자동 기록

- Given: 파이프라인이 Phase 2를 완료하고 Gate 2 (Validation)를 실행한다
- When: Gate 2가 FAIL을 반환한다 (예: 테스트 파일 없음)
- Then: `.autopus/learnings/pipeline.jsonl`에 type=gate_fail 항목이 append된다
- And: 항목에 phase=gate2, pattern, resolution 필드가 포함된다

### S2: Coverage Gap 기록

- Given: 파이프라인이 Phase 3를 완료하고 Gate 3를 실행한다
- When: 커버리지가 85% 미만이다 (예: 72%)
- Then: `.autopus/learnings/pipeline.jsonl`에 type=coverage_gap 항목이 append된다
- And: 항목에 커버리지 갭(13%)과 미커버 패키지 목록이 포함된다

### S3: Review REQUEST_CHANGES 기록

- Given: 파이프라인 Phase 4에서 reviewer가 실행된다
- When: reviewer가 REQUEST_CHANGES를 반환한다
- Then: `.autopus/learnings/pipeline.jsonl`에 type=review_issue 항목이 하나 이상 append된다
- And: 각 항목에 리뷰 이슈의 pattern과 관련 파일이 포함된다

### S4: Executor 연속 실패 기록

- Given: Phase 2에서 executor가 task T3을 실행한다
- When: T3이 2회 연속 실패한다 (첫 시도 + 재시도)
- Then: `.autopus/learnings/pipeline.jsonl`에 type=executor_error 항목이 append된다
- And: 항목에 실패 원인과 최종 결과(성공/포기)가 포함된다

### S5: Planning 시 Learning 주입

- Given: `.autopus/learnings/pipeline.jsonl`에 pkg/foo 관련 gate_fail 항목이 있다
- When: `/auto go SPEC-FOO-001`이 Phase 1 (Planning)을 시작하고, SPEC이 pkg/foo를 참조한다
- Then: planner 프롬프트에 해당 learning 항목이 "Previous Learnings" 섹션으로 주입된다
- And: 주입된 토큰이 2000 이하이다

### S6: Fix 시 Learning 참조

- Given: `.autopus/learnings/pipeline.jsonl`에 pkg/bar의 "nil pointer in handler" 패턴이 있다
- When: `/auto fix`가 pkg/bar에서 nil pointer 에러를 디버깅한다
- Then: 디버깅 프롬프트에 해당 learning이 참조로 주입된다

### S7: CLI — learn list

- Given: `.autopus/learnings/pipeline.jsonl`에 5개 항목이 있다
- When: `auto learn list`를 실행한다
- Then: 5개 항목이 테이블 형태로 출력된다 (id, type, pattern, timestamp)

### S8: CLI — learn list with filter

- Given: `.autopus/learnings/pipeline.jsonl`에 gate_fail 2개, review_issue 3개가 있다
- When: `auto learn list --type gate_fail`을 실행한다
- Then: gate_fail 항목 2개만 출력된다

### S9: CLI — learn add

- Given: `.autopus/learnings/` 디렉토리가 존재한다
- When: `auto learn add "항상 context.Context를 첫 번째 파라미터로 전달할 것"`을 실행한다
- Then: `patterns.jsonl`에 type=manual 항목이 추가된다
- And: 항목의 pattern 필드에 입력 텍스트가 저장된다

### S10: CLI — learn prune

- Given: `pipeline.jsonl`에 100일 전 항목 2개와 오늘 항목 3개가 있다
- When: `auto learn prune`을 실행한다 (기본 90일)
- Then: 100일 전 항목 2개가 제거되고 3개만 남는다
- And: 제거된 항목 수가 출력된다

### S11: Relevance Scoring

- Given: learnings에 파일 경로 "pkg/foo/handler.go" 항목과 "pkg/bar/service.go" 항목이 있다
- When: SPEC의 plan.md가 "pkg/foo/handler.go"를 참조한다
- Then: foo 항목이 bar 항목보다 높은 점수로 매칭된다
- And: 무관한 항목은 주입되지 않는다

### S12: Reuse Count Tracking

- Given: learning L-003의 reuse_count가 0이다
- When: L-003이 planner 프롬프트에 주입된다
- Then: L-003의 reuse_count가 1로 업데이트된다
