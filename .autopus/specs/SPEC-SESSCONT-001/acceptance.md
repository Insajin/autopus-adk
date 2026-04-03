# SPEC-SESSCONT-001 수락 기준

## 시나리오

### S1: 페이즈 간 세션 ID 통합

- Given: PipelineExecutor가 taskID "task-abc"로 초기화됨
- When: Execute가 planner → executor → tester → reviewer 순으로 4개 페이즈를 실행
- Then: 모든 페이즈의 --resume 플래그에 "pipeline-task-abc"가 전달됨
- And: 각 페이즈의 TaskConfig.TaskID는 "task-abc-planner", "task-abc-executor" 등으로 구분됨

### S2: 두 번째 페이즈에서 이전 컨텍스트 활용

- Given: planner 페이즈가 "pipeline-task-abc" 세션에서 완료됨
- When: executor 페이즈가 동일 세션 ID로 시작됨
- Then: CLI subprocess의 --resume 인자가 "pipeline-task-abc"임
- And: stdin으로 executor 역할 프롬프트만 전달됨 (시스템 프롬프트 재전송 없음)

### S3: 페이즈 실패 후 재시도 세션 유지 (P1)

- Given: executor 페이즈가 "pipeline-task-abc" 세션에서 실패함
- When: 재시도가 트리거됨
- Then: 재시도 시 동일한 "pipeline-task-abc" 세션 ID가 사용됨
- And: PhaseResult.RetryCount가 1로 기록됨

### S4: 최대 재시도 초과 (P1)

- Given: executor 페이즈가 3회 연속 실패
- When: 재시도 한도(2회)를 초과
- Then: 파이프라인이 에러를 반환하며 중단됨
- And: 에러 메시지에 재시도 횟수가 포함됨

### S5: 세션 메타데이터 캐싱 (P2)

- Given: 파이프라인이 총 $0.15, 3000ms로 완료됨
- When: 동일 provider+workDir로 새 파이프라인이 시작됨
- Then: 캐시에서 예상 비용/시간 추정값을 조회할 수 있음

### S6: 기존 단일 실행 모드 미영향

- Given: WorkerLoop.handleTask가 PipelineExecutor를 사용하지 않는 단일 실행 모드
- When: 태스크가 executeSubprocess로 직접 실행됨
- Then: SessionID 동작이 변경되지 않음 (기존 loop.go 경로 무영향)
