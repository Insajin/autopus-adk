# SPEC-ORCH-022 Acceptance: Claude Code 안 hook-IPC headless 멀티프로바이더 실행 경로

## Test Scenarios

### Scenario S1: claude Stop hook이 settings.json에 등록됨 (REQ-001 / INV-001)
Given autopus-adk 프로젝트에서 claude-code 어댑터로 auto init을 실행한다.
When .claude/settings.json을 읽는다.
Then hooks 객체에 Stop 키가 존재한다.
And concrete expected output으로 hooks.Stop[0].hooks[0].command 값이 정확히 ".claude/hooks/autopus/hook-claude-stop.sh"이다.
And hooks.Stop[0].hooks[0].type 값이 "command"이다.

### Scenario S2: gemini AfterAgent와 codex Stop이 등록됨 (REQ-002 / INV-002)
Given antigravity-cli(gemini) 어댑터로 auto init을 실행한다.
When .agents/hooks.json의 autopus 스펙을 읽는다.
Then AfterAgent 이벤트 엔트리가 존재하고 그 command가 gemini afteragent 스크립트 경로를 가리킨다.
And codex 어댑터로 init했을 때 예상 출력은 Stop 이벤트 엔트리가 codex stop 스크립트 경로를 가리키는 것이다.

### Scenario S3: 등록 후 isHookModeAvailable가 true로 전이 (REQ-003 / INV-003)
Given 임시 디렉토리의 .claude/settings.json에 autopus 문자열과 Stop 키가 모두 포함되어 있다.
When isHookModeAvailable를 호출한다.
Then 반환값이 true이다.
And settings.json에서 Stop 키를 제거하면 반환값이 false이다.

### Scenario S4: headless에서 done-file 수집 시 detector가 FileIPCDetector (REQ-004 / INV-004)
Given HookMode가 true이고 HookSession이 비-nil인 OrchestraConfig가 주어진다.
When 완료 detector를 NewCompletionDetectorWithConfig(non-signal terminal, true, session)로 해석한다.
Then 반환된 detector의 구체 타입이 FileIPCDetector이다.
And SignalCapable 터미널이 주어지면 SignalDetector가 우선 선택된다.

### Scenario S5: CLAUDECODE + hook 가용 + cmux 설치 시 멀티플렉서 진입 (REQ-005 / INV-005)
Given CLAUDECODE 환경변수가 "1"이고 hookAvailable=true, muxInstalled=true이다.
When paneInteractiveContext(claudeCode "1", ci "", hookAvailable true, muxInstalled true)를 호출한다.
Then 반환값이 true이다.
And detectStructuredTerminal가 PlainAdapter가 아닌 cmux 또는 tmux 터미널을 반환한다.

### Scenario S5b: CLAUDECODE + hook 미가용 시 plain 유지 (REQ-005 / REQ-008 / INV-005)
Given CLAUDECODE 환경변수가 "1"이고 hookAvailable=false이다.
When paneInteractiveContext(claudeCode "1", ci "", hookAvailable false, muxInstalled true)를 호출한다.
Then 반환값이 false이다.
And detectStructuredTerminal가 PlainAdapter를 반환한다.

### Scenario S6: PaneBackend가 HookMode일 때 실 HookSession 사용 (REQ-006 / INV-006)
Given OrchestraConfig.HookMode가 true이고 SessionID가 설정된 InteractivePaneBackend가 주어진다.
When Execute가 완료 detector를 해석하는 시점을 관측한다.
Then 전달된 hookSession이 nil이 아니다.
And 해석된 완료 detector가 FileIPCDetector이다.
And HookMode가 false이면 예상 출력은 hookSession이 nil이고 detector가 ScreenPollDetector인 것이다.

### Scenario S7: done-file 미수신 시 bounded timeout으로 결정적 false (REQ-007 / INV-007)
Given FileIPCDetector와 done-file이 절대 생성되지 않는 세션 디렉토리가 주어진다.
When 200ms deadline을 가진 context로 WaitForCompletion을 호출한다.
Then explicit tolerance로 호출이 deadline 직후(200ms 이상 1s 이하) 반환되고 무한 대기하지 않는다.
And 반환된 completed 값이 false이다.

### Scenario S8: hook 미설치 시 subprocess backend로 degrade (REQ-008 / INV-008)
Given SubprocessMode가 false이고 Terminal이 PlainAdapter인 OrchestraConfig가 주어진다.
When SelectBackend를 호출한다.
Then concrete expected output으로 반환된 backend의 Name() 값이 정확히 "subprocess"이다.
And cmux/tmux가 설치되지 않은 환경에서 detectStructuredTerminal가 PlainAdapter를 반환한다.

### Scenario S9: hook도 subprocess도 불가하면 actionable 에러 (REQ-009 / INV-009)
Given cmux/tmux 미설치, hook 미등록, API 키 부재 조건이 동시에 성립한다.
When spec review 또는 orchestra run 실행 경로가 backend를 결정한다.
Then 반환된 에러 메시지에 복구 지침이 포함된다.
And 예상 출력으로 그 메시지에 "auto init"과 "cmux"와 "API" 키워드가 모두 등장한다.

### Scenario S10: 세션 디렉토리 0o700, result 0o600, traversal 거부 (REQ-010 / INV-010)
Given session-id가 "orch-123"인 NewHookSession을 호출한다.
When 생성된 /tmp/autopus/orch-123 디렉토리의 권한 비트를 os.Stat로 읽는다.
Then concrete expected output으로 디렉토리 권한이 정확히 0o700이다.
And session-id가 "../evil"이면 sanitizeProviderName이 슬래시와 점을 제거하여 "evil"이 되고 /tmp/autopus 상위로 벗어나지 않는다.
And hook 스크립트가 작성하는 result.json의 권한이 0o600이다.

## Edge Cases

### Edge Case 1: cmux daemon은 살아 있으나 new-split 실패
Given cmux는 설치되어 있으나 surface 생성이 실패한다.
When headless hook runner가 launch를 시도한다.
Then subprocess fallback으로 degrade하고 무한 대기하지 않는다.

### Edge Case 2: round-scoped done 파일
Given AUTOPUS_ROUND가 2로 설정된 멀티라운드 세션이다.
When Stop hook이 완료 신호를 쓴다.
Then claude-round2-done 파일이 생성되고 FileIPCDetector가 round-scoped 신호를 감시한다.

## Oracle Acceptance Notes

Must 시나리오는 파일 존재, heading, exit code, non-empty output만으로 닫지 않고 concrete expected output 또는 explicit tolerance를 포함한다. 구체적으로 S1은 settings.json command 문자열(`.claude/hooks/autopus/hook-claude-stop.sh`), S4/S6은 detector 구체 타입(FileIPCDetector / ScreenPollDetector), S5/S5b는 터미널 이름, S7은 timeout 시간 경계(explicit tolerance 200ms~1s), S8은 backend Name() 문자열("subprocess"), S9는 에러 메시지 키워드 집합, S10은 디렉토리/파일 권한 비트(0o700/0o600)와 traversal sanitize 결과("evil")를 예상 출력으로 검증한다. 이 oracle들은 research.md의 Semantic Invariant Inventory INV-001~INV-010과 양방향으로 매핑된다.

## Definition of Done

- [ ] S1 ~ S10 + Edge Case 1~2 통과
- [ ] 신규 소스 300줄 이하
- [ ] 코드 리뷰 완료
