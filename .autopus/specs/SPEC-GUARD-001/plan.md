# SPEC-GUARD-001 구현 계획

## 재시도 제한 통합 테이블

이 SPEC의 모든 태스크는 `auto-router.md.tmpl`과 `auto.md` 두 파일만 수정한다.
태스크 간 의존성이 있으므로 순차 실행한다.

## 태스크 목록

- [x] T1: Plan Step 3 IF-THEN 다이어그램 변환 (P0-1, R13)
  - 위치: 템플릿 154-188줄 (Step 3: Review Gate Decision)
  - 산문형 조건부 분기를 IF-THEN 다이어그램으로 교체
  - `[CONDITIONAL]` 태그 추가
  - true/false 분기 경로 명시

- [x] T2: Phase 2.1 Worktree Merge 승격 (P0-2, R14, R15)
  - 위치: 템플릿 411-442줄 (Step 2.3.1 + executor 결과 표시)
  - "Step 2.3.1"을 `[REQUIRED] Phase 2.1 -- Worktree Merge`로 승격
  - executor 결과 표시의 "다음: 검증 단계로 진행"을 "다음: Phase 2.1 Worktree Merge (병합 필수)"로 변경
  - Gate 2 직전에 `[CHECKPOINT]` 마커 추가

- [x] T3: Error Recovery 인라인화 (P0-3, R16)
  - 위치: 템플릿 533-569줄 (Error Recovery 섹션)
  - Validation failure 블록을 Gate 2 직후(466줄 부근)로 이동
  - Review failure 블록을 Gate 4 직후(515줄 부근)로 이동
  - Worktree merge failure 블록을 Phase 2.1 직후로 이동
  - Subagent failure 블록을 각 Agent 스폰 직후로 이동
  - 기존 Error Recovery 섹션 제거

- [x] T4: Post-Agent Continuation Markers 삽입 (Layer 1, R1-R6)
  - plan 서브커맨드: spec-writer Agent 스폰 후 (173줄 부근)
  - go 서브커맨드: planner 스폰 후 (360줄 부근)
  - go 서브커맨드: executor 스폰 후 (410줄 부근)
  - go 서브커맨드: validator 스폰 후 (466줄 부근)
  - go 서브커맨드: tester 스폰 후 (497줄 부근)
  - go 서브커맨드: reviewer 스폰 후 (522줄 부근)

- [x] T5: Pre-Completion Verification Checklist 추가 (Layer 2, R7-R9)
  - plan 서브커맨드: Step 5 (Completion Guidance) 직전에 체크리스트 삽입
  - go 서브커맨드: Completion Guidance 직전에 체크리스트 삽입
  - 미평가 항목 발견 시 되돌아가기 지시 포함

- [x] T6: REQUIRED/GATE/CONDITIONAL 마커 전체 적용 (Layer 3, R10-R12, R18)
  - plan 서브커맨드의 모든 Step에 [REQUIRED] 또는 [CONDITIONAL] 태그
  - go 서브커맨드의 모든 Phase, Gate에 [REQUIRED] 태그
  - 조건부 건너뛰기 시 [INTENDED SKIP] 태그 패턴 추가

- [x] T7: 재시도 제한 통합 테이블 (P1-1, R17)
  - go 서브커맨드 파이프라인 시작부에 재시도 제한 요약 테이블 추가
  - 기존 분산된 재시도 언급은 테이블 참조로 변경

- [x] T8: auto.md 동기화
  - 템플릿 변경 완료 후 `auto` CLI로 렌더링하거나 수동 동기화
  - 렌더링 결과물(.claude/commands/auto.md)이 템플릿 변경사항 반영 확인

## 구현 전략

### 접근 방법
1. 템플릿(auto-router.md.tmpl)을 먼저 수정한다
2. 변경은 기존 파이프라인 로직을 유지하면서 에이전트 지시 텍스트만 구조화한다
3. 렌더링된 auto.md는 템플릿 변경 후 동기화한다

### 실행 순서
T1 → T2 → T3 (P0 수정) → T4 → T5 → T6 (Layer 적용) → T7 (P1 수정) → T8 (동기화)

### 위험 요소
- auto-router.md.tmpl이 SPEC-LOOP-001과 동일 파일을 수정하므로 충돌 가능
  - 대응: SPEC-GUARD-001을 먼저 구현하여 기반을 확립
- 템플릿 조건분기({{if}}) 내부 수정 시 Go 템플릿 문법 오류 주의
  - 대응: 수정 후 `auto render` 또는 동등한 검증 실행
