# SPEC-ORCH-015 수락 기준

## 시나리오

### S1: ANSI 코드 포함 프롬프트 감지 (R1)

- Given: ReadScreen 출력에 `\x1b[32m❯\x1b[0m` 형태의 ANSI 컬러 프롬프트가 포함됨
- When: `isPromptVisible`가 해당 screen 문자열로 호출됨
- Then: ANSI 코드가 strip된 후 `❯` 패턴이 매칭되어 `true` 반환

### S2: 취소된 context 이후 최종 ReadScreen (R2)

- Given: `waitForCompletion`이 timeout으로 인해 parent context가 취소됨
- When: `waitAndCollectResults`가 최종 screen 캡처를 시도함
- Then: `context.Background()` 기반 5초 timeout context로 ReadScreen이 성공하여 비어있지 않은 output 반환

### S3: opencode stdin 모드 전달 (R3)

- Given: opencode provider가 interactive pane mode로 설정됨
- When: 프롬프트를 전달할 때
- Then: `PromptViaArgs=false`이므로 `SendLongText`를 통해 stdin으로 전달 (CLI arg 아님)

### S4: 프로바이더별 패턴 매칭 정확성 (R4)

- Given: claude TUI에서 `❯` 프롬프트가 표시됨
- When: `isPromptVisible`가 호출됨
- Then: claude 전용 패턴 `^❯\s*$`가 매칭되어 `true` 반환

- Given: gemini TUI에서 `> Type your message...` 프롬프트가 표시됨
- When: `isPromptVisible`가 호출됨
- Then: gemini 전용 패턴 `^\s*>\s+(Type your|@)`가 매칭되어 `true` 반환

- Given: opencode TUI에서 `Ask anything` placeholder가 표시됨
- When: `isPromptVisible`가 호출됨
- Then: opencode 전용 패턴 `Ask anything`가 매칭되어 `true` 반환

### S5: waitForSessionReady shell 오탐 방지 (R5)

- Given: pane이 생성 직후이고 shell `$` 프롬프트만 표시됨 (CLI 미시작)
- When: `waitForSessionReady`가 폴링을 시작함
- Then: shell `$` 패턴이 전용 패턴셋에 포함되지 않으므로 매칭되지 않고 폴링 계속

- Given: CLI가 시작되어 claude `❯` 프롬프트가 표시됨
- When: `waitForSessionReady`가 폴링 중
- Then: CLI 전용 패턴 `❯`가 매칭되어 ready 반환

### S6: 프로바이더별 startup timeout (R6)

- Given: claude provider에 `StartupTimeout=15s`가 설정됨
- When: `waitForSessionReady`가 claude pane을 폴링함
- Then: 15초 timeout이 적용됨 (기본 30초가 아님)

- Given: opencode provider에 `StartupTimeout=5s`가 설정됨
- When: opencode CLI가 5초 내 시작하지 않음
- Then: 5초 후 timeout되어 다음 단계로 진행

### S7: opencode idle fallback 완료 감지 (R7)

- Given: opencode provider가 응답을 완료했으나 TUI 렌더링으로 `Ask anything` 재표시가 지연됨
- When: 2-phase consecutive match가 30초 내 성공하지 못함
- Then: pipe-pane output file의 idle detection (15초 미변경)으로 완료가 감지됨

### S8: debate 라운드 프롬프트 전달 실패 재시도 (R8)

- Given: debate round 2에서 provider A에 대한 `SendLongText` 호출이 실패함
- When: 에러가 발생함
- Then: 에러가 로깅되고, 1회 재시도가 실행됨

- Given: 재시도도 실패함
- When: provider A가 해당 라운드에서 skip됨
- Then: 다른 provider들의 응답은 정상 수집되고, provider A는 `EmptyOutput=true`로 결과에 포함됨

### S9: debate 빈 응답 핸들링 (R8)

- Given: debate round에서 provider B가 빈 응답을 반환함
- When: 응답 수집이 완료됨
- Then: provider B의 `ProviderResponse`에 `EmptyOutput=true`가 설정되고, partial merge에 포함됨
