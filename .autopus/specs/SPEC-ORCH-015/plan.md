# SPEC-ORCH-015 구현 계획

## 태스크 목록

### Phase 1: 기존 핫픽스 테스트 보강 (R1-R4)

- [ ] T1: `stripANSI` + `isPromptVisible` ANSI 입력 테스트 (R1)
  - ANSI 컬러 코드 포함 프롬프트 문자열에서 패턴 매칭 검증
  - `interactive_detect_test.go`에 추가
- [ ] T2: `waitAndCollectResults` fresh context ReadScreen 테스트 (R2)
  - 취소된 context에서 최종 ReadScreen이 fresh context로 성공하는지 검증
  - mock Terminal 사용
- [ ] T3: opencode `PromptViaArgs=false` 설정 검증 테스트 (R3)
  - `defaults.go`, `migrate.go`, `orchestra_helpers.go`의 opencode 설정값 확인
  - `migrate_test.go`에 추가
- [ ] T4: `DefaultCompletionPatterns` 프로바이더별 패턴 매칭 테스트 (R4)
  - 각 프로바이더의 실제 TUI 출력 샘플에 대한 패턴 매칭 검증
  - `interactive_detect_test.go`에 추가

### Phase 2: 신규 구현 (R5-R8)

- [ ] T5: `waitForSessionReady` 전용 패턴셋 구현 (R5)
  - `SessionReadyPatterns()` 함수 추가 (shell `$`, `#` 패턴 제외)
  - `waitForSessionReady`에서 `DefaultCompletionPatterns` 대신 사용
- [ ] T6: 프로바이더별 startup timeout 구현 (R6)
  - `ProviderConfig`에 `StartupTimeout time.Duration` 필드 추가
  - `waitForSessionReady`에서 프로바이더별 timeout 적용
  - defaults에 claude=15s, gemini=10s, opencode=5s 설정
- [ ] T7: opencode 완료 감지 idle fallback 구현 (R7)
  - `waitForCompletion`에 pipe-pane idle detection fallback 추가
  - 2-phase 매치 실패 시 `isOutputIdle(outputFile, 15s)` 체크
  - `paneInfo`에 `outputFile` 경로 전달 확인
- [ ] T8: debate `executeRound` 에러 핸들링 강화 (R8)
  - `_ =` 에러 무시를 `log + retry` 로직으로 교체
  - 빈 응답에 `EmptyOutput=true` 설정
  - 1회 재시도 실패 시 `skipWait` 마킹

### Phase 3: 통합 테스트

- [ ] T9: 프로바이더별 end-to-end 시나리오 테스트
  - mock Terminal로 claude/gemini/opencode 시나리오 커버
  - debate 멀티라운드 빈 응답 + 재시도 시나리오

## 구현 전략

- T1-T4는 기존 코드 변경 없이 테스트만 추가 (확인 목적)
- T5-T8은 기존 파일 수정, 각 태스크 독립 실행 가능
- T5와 T6은 `interactive.go`의 `waitForSessionReady` 수정으로 연관 — T5 먼저 진행
- T7은 `interactive_completion.go` 단독 수정
- T8은 `interactive_debate.go` 단독 수정
- 300줄 파일 제한 준수: 신규 함수 추가 시 기존 파일 라인 수 확인
