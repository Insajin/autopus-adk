# SPEC-SESSCONT-001: Session Continuity Optimization

**Status**: draft
**Created**: 2026-04-02
**Domain**: SESSCONT
**Phase**: Phase 3 (v0.15~v0.16) — Optimization
**ICE Score**: 2.88 (Impact 4, Confidence 8, Ease 9)

## 목적

Worker 파이프라인(PipelineExecutor)은 각 페이즈(planner, executor, tester, reviewer)마다
독립적인 세션 ID를 생성한다(`pipeline-{taskID}-{phase}`). 이로 인해:

1. 동일한 시스템 프롬프트와 SecurityPolicy가 4회 반복 전송됨
2. 이전 페이즈의 컨텍스트가 CLI --resume으로 이어지지 않음
3. 페이즈 실패 시 재시도가 새 세션으로 시작되어 컨텍스트 손실 발생

세션 ID를 태스크 단위로 통합하면 Claude Code의 --resume이 이전 페이즈
컨텍스트를 자연스럽게 이어받아, 프롬프트 중복 전송 없이 연속적 실행이 가능하다.

## 요구사항

### P0: 페이즈 간 세션 재사용

- R1: WHEN PipelineExecutor.runPhase가 호출될 때, THE SYSTEM SHALL 페이즈 접미사 없이
  `pipeline-{taskID}` 형식의 단일 세션 ID를 사용한다.
- R2: WHEN 동일 태스크의 후속 페이즈가 실행될 때, THE SYSTEM SHALL 이전 페이즈와
  동일한 세션 ID를 --resume 플래그에 전달하여 컨텍스트 연속성을 유지한다.
- R3: WHEN 세션이 재사용될 때, THE SYSTEM SHALL 각 페이즈의 역할 프롬프트(plannerPrompt 등)만
  stdin으로 전달하고, 시스템 프롬프트 재전송은 하지 않는다.

### P1: 재시도 시 세션 연속성

- R4: WHEN 페이즈 실행이 실패하여 재시도될 때, THE SYSTEM SHALL 동일한 세션 ID를
  유지하여 이전 실행 컨텍스트를 보존한다.
- R5: WHEN 재시도가 발생할 때, THE SYSTEM SHALL 재시도 카운트를 PhaseResult에 기록한다.

### P2: 세션 메타데이터 캐싱

- R6: WHEN 파이프라인이 성공적으로 완료될 때, THE SYSTEM SHALL 총 비용과 소요 시간을
  세션 메타데이터로 캐싱한다.
- R7: WHEN 새 파이프라인이 시작될 때, THE SYSTEM SHALL 캐싱된 메타데이터를 참조하여
  예산 추정값을 제공한다.

## 주의사항

- SPEC-COMPRESS-001과 연계 필수: 세션 재사용 시 컨텍스트 누적으로 overflow 위험 증가
- P0은 독립 실행 가능하나, 프로덕션 배포 시 SPEC-COMPRESS-001과 함께 배포 권장

## 생성 파일 상세

| 파일 | 변경 유형 | 역할 |
|------|----------|------|
| `pkg/worker/pipeline.go` | 수정 | runPhase의 sessionID 생성 로직 변경 (L99) |
| `pkg/worker/pipeline.go` | 수정 | Execute에 재시도 로직 추가 (P1) |
| `pkg/worker/session_cache.go` | 신규 | 세션 메타데이터 캐싱 (P2) |
| `pkg/worker/pipeline_test.go` | 수정 | 세션 재사용 검증 테스트 |
| `pkg/worker/session_cache_test.go` | 신규 | 캐시 테스트 |
