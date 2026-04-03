# SPEC-BUDGET-001 수락 기준

## 시나리오

### S1: EventToolCall 스트림 파싱

- Given: Claude 서브프로세스가 `{"type": "tool_use", "name": "Bash", ...}` JSON 라인을 출력
- When: StreamParser가 해당 라인을 파싱
- Then: Event.Type이 "tool_call"이고 Event.Subtype이 비어있는 EventToolCall 이벤트가 반환된다

### S2: stdin 파이프 유지

- Given: 서브프로세스가 프롬프트를 받고 실행 중
- When: 프롬프트 쓰기가 완료된 후
- Then: stdin 파이프가 열린 상태로 유지되어 추가 메시지 쓰기가 가능하다

### S3: 70% 경고 메시지 주입

- Given: IterationBudget limit=100, 현재 counter=70
- When: 새로운 EventToolCall 이벤트가 수신되어 counter가 70에 도달
- Then: stdin에 "[BUDGET WARNING] 도구 호출 예산 70% 소진. 남은 예산: 30회. 효율적으로 작업을 완료하세요." 메시지가 쓰인다

### S4: 90% 위험 메시지 주입

- Given: IterationBudget limit=100, 현재 counter=89
- When: 새로운 EventToolCall 이벤트가 수신되어 counter가 90에 도달
- Then: stdin에 "[BUDGET CRITICAL] 예산 거의 소진. 남은 예산: 10회. 핵심 작업만 완료하세요." 메시지가 쓰인다

### S5: 100% EmergencyStop 호출

- Given: IterationBudget limit=100, 현재 counter=99
- When: 새로운 EventToolCall 이벤트가 수신되어 counter가 100에 도달
- Then: EmergencyStop.Stop("iteration_budget_exceeded")가 호출되고 서브프로세스가 종료된다

### S6: Pipeline Phase 예산 분배

- Given: 전체 budget=200, 4-phase pipeline 실행
- When: PipelineExecutor.Execute가 호출
- Then: Planning phase에 20회, Execution phase에 120회, Testing phase에 40회, Review phase에 20회가 분배된다

### S7: 미사용 예산 이월

- Given: Planning phase에 20회 할당, 실제 12회 사용하여 8회 미사용
- When: Planning phase가 완료되고 Execution phase가 시작
- Then: Execution phase 예산이 120+8=128회로 증가한다

### S8: Codex/Gemini tool_call 매핑

- Given: Codex 서브프로세스가 `{"type": "tool_call", "function": "exec", ...}` 출력
- When: CodexAdapter.ParseEvent가 해당 라인을 파싱
- Then: StreamEvent.Type이 "tool_call"인 이벤트가 반환된다

### S9: 경고 없이 정상 완료

- Given: IterationBudget limit=100, 서브프로세스가 총 50회 도구 호출 후 완료
- When: 서브프로세스가 result 이벤트를 출력하고 종료
- Then: 경고 메시지가 한 번도 주입되지 않고 정상 결과가 반환된다

### S10: Budget 미설정 시 무제한

- Given: IterationBudget이 nil 또는 limit=0
- When: EventToolCall 이벤트가 수신
- Then: 카운팅만 수행하고 경고/중단 없이 정상 진행된다
