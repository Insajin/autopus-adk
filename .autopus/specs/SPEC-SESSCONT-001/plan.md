# SPEC-SESSCONT-001 구현 계획

## 태스크 목록

### P0: 페이즈 간 세션 재사용

- [ ] T1: `pipeline.go:99` sessionID 포맷 변경 — `pipeline-{taskID}-{phase}` → `pipeline-{taskID}`
- [ ] T2: `pipeline.go:101` TaskConfig.TaskID에서 phase 접미사 제거 여부 결정 및 적용
- [ ] T3: 페이즈별 프롬프트에 `[Phase: {name}]` 접두사 추가하여 세션 내 페이즈 경계 명시
- [ ] T4: 기존 테스트 업데이트 — sessionID 포맷 변경에 따른 assertion 수정
- [ ] T5: 통합 테스트 — 4개 페이즈가 동일 sessionID로 순차 실행되는지 검증

### P1: 재시도 시 세션 연속성

- [ ] T6: `PhaseResult`에 `RetryCount int` 필드 추가
- [ ] T7: `runPhase`에 재시도 로직 추가 (최대 2회, 지수 백오프)
- [ ] T8: 재시도 시 동일 sessionID 사용 보장 — 새 세션 생성 방지
- [ ] T9: 재시도 테스트 — 실패 후 재시도가 동일 세션에서 이루어지는지 검증

### P2: 세션 메타데이터 캐싱

- [ ] T10: `session_cache.go` 신규 — SessionMetadata 구조체 및 인메모리 캐시
- [ ] T11: PipelineExecutor.Execute 완료 시 캐시에 메타데이터 저장
- [ ] T12: 새 파이프라인 시작 시 캐시에서 예산 추정값 조회하는 API
- [ ] T13: 캐시 유닛 테스트

## 구현 전략

### T1 핵심 변경 (1줄 수정)

```go
// Before (pipeline.go:99)
sessionID := fmt.Sprintf("pipeline-%s-%s", taskID, phase)

// After
sessionID := fmt.Sprintf("pipeline-%s", taskID)
```

이 1줄 변경으로 Claude Code --resume이 동일 세션을 이어받는다.
각 페이즈의 역할 구분은 stdin으로 전달되는 phase prompt가 담당.

### TaskID 접미사 유지

`TaskConfig.TaskID`는 phase 접미사를 유지한다 (`{taskID}-{phase}`).
이는 로그 추적과 비용 집계에 필요. SessionID만 통합.

### 재시도 전략 (P1)

- 최대 2회 재시도 (총 3회 시도)
- 백오프: 5초 → 15초
- 동일 sessionID 유지 — Claude Code가 실패 컨텍스트를 기억하므로 재시도 효율 향상

### 캐시 전략 (P2)

- 인메모리 sync.Map 기반 단순 캐시
- 키: provider+workDir 조합
- 값: 최근 5개 파이프라인의 평균 비용/시간
- 프로세스 재시작 시 초기화 (영속성 불필요)
