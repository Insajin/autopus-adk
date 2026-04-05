# SPEC-HARN-PIPE-001 수락 기준

## 시나리오

### S1: 기본 순차 파이프라인 실행 (REQ-1, REQ-2, REQ-3)

- Given: `SPEC-FEAT-001` 디렉토리에 `spec.md`, `plan.md`, `acceptance.md`, `research.md`가 존재
- And: `autopus.yaml`에 codex 프로바이더가 설정되어 있음
- When: `auto pipeline run SPEC-FEAT-001 --platform codex` 실행
- Then: Plan → TestScaffold → Implement → Validate → Review 5개 Phase가 순차 실행
- And: 각 Phase가 codex CLI subprocess로 호출됨
- And: Phase 1 (Plan) 결과가 Phase 2 (Implement) 프롬프트에 포함됨

### S2: --platform 생략 시 자동 감지 (REQ-3)

- Given: 현재 환경에 gemini CLI만 설치되어 있음
- When: `auto pipeline run SPEC-FEAT-001` 실행 (--platform 생략)
- Then: gemini가 자동으로 선택됨

### S3: 병렬 실행 (REQ-4, REQ-5)

- Given: `plan.md`에 3개의 독립 실행 태스크가 정의됨
- And: tmux/cmux 세션이 활성화되어 있음
- When: `auto pipeline run SPEC-FEAT-001 --strategy parallel` 실행
- Then: Phase 2 (Implement)에서 3개의 executor가 각각 별도 git worktree에서 동시 실행
- And: 각 worktree가 실행 완료 후 main worktree로 머지됨

### S4: Quality Gate 재시도 (REQ-6)

- Given: Phase 2 (Implement) 완료 후 Validation Gate 실행
- When: validator가 FAIL 판정
- Then: Phase 2를 최대 3회 재시도
- And: 3회 모두 실패 시 파이프라인이 FAIL 상태로 종료

### S5: Checkpoint 저장 및 Resume (REQ-7)

- Given: Phase 2까지 성공적으로 완료된 파이프라인
- When: Phase 3 실행 중 사용자가 Ctrl+C로 중단
- Then: `.autopus/pipeline-state/SPEC-FEAT-001.yaml`에 Phase 2 완료 상태가 저장됨
- When: 이후 `auto pipeline run SPEC-FEAT-001 --continue` 실행
- Then: Phase 3부터 재개됨

### S6: Dry-run 모드 (REQ-12)

- Given: `SPEC-FEAT-001`이 존재
- When: `auto pipeline run SPEC-FEAT-001 --platform claude --dry-run` 실행
- Then: 각 Phase에 전달될 프롬프트가 stdout에 출력됨
- And: 실제 subprocess 호출은 발생하지 않음

### S7: Claude Code 공존 (REQ-13, REQ-17)

- Given: Claude Code 환경에서 실행 중
- When: `auto go SPEC-FEAT-001` 실행 (기존 방식)
- Then: Agent() 기반 파이프라인이 실행됨 (기존 동작 유지)
- When: `auto pipeline run SPEC-FEAT-001 --engine subprocess` 실행
- Then: subprocess 기반 파이프라인이 실행됨

### S8: JSONL 이벤트 발행 (REQ-11)

- Given: 파이프라인 실행 중
- When: Phase 1에서 Phase 2로 전환
- Then: `phase_end` 이벤트와 `phase_start` 이벤트가 JSONL 로그에 기록됨
- And: 이벤트에 timestamp, phase 이름, agent 이름이 포함됨

### S9: 터미널 pane 모니터링 (REQ-10)

- Given: cmux 세션이 활성화되어 있음
- And: `--strategy parallel` 모드
- When: Phase 2에서 3개 executor가 병렬 실행
- Then: 각 executor가 별도 cmux pane에 표시됨
- And: 대시보드 pane에 전체 진행 상태가 표시됨

### S10: 파일 크기 제한 준수

- Given: SPEC-HARN-PIPE-001의 모든 구현 파일
- When: `wc -l`로 줄 수 확인
- Then: 모든 소스 코드 파일이 300줄 미만
