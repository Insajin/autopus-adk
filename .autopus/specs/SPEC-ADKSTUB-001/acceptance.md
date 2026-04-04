# SPEC-ADKSTUB-001 수락 기준

## 시나리오

### S1: agent_run 정상 실행
- Given: `.autopus/runs/test-task/context.yaml`에 유효한 task context가 존재하고, claude adapter가 등록되어 있을 때
- When: `auto agent run test-task`를 실행하면
- Then: provider subprocess가 구동되고, stream 이벤트가 파싱되며, `result.yaml`에 실제 cost_usd, duration_ms, session_id가 기록된다

### S2: agent_run 실패 전파
- Given: provider subprocess가 비정상 종료(exit code != 0)할 때
- When: `auto agent run test-task`를 실행하면
- Then: `result.yaml`의 status가 "failed"이고, error 필드에 실패 원인이 포함된다

### S3: agent_run context 미존재
- Given: `.autopus/runs/missing-task/context.yaml` 파일이 없을 때
- When: `auto agent run missing-task`를 실행하면
- Then: "task context not found: missing-task" 에러가 반환된다 (기존 동작 유지)

### S4: TUI pause 토글
- Given: Worker TUI가 실행 중이고 connStatus가 Connected일 때
- When: 사용자가 `p` 키를 누르면
- Then: `paused` 상태가 true로 전환되고, header에 "PAUSED"가 표시되며, `OnPauseToggle` 콜백이 호출된다
- When: 사용자가 다시 `p`를 누르면
- Then: `paused` 상태가 false로 전환되고, "PAUSED" 표시가 사라진다

### S5: TUI cancel 동작
- Given: Worker TUI에서 currentTask가 실행 중일 때
- When: 사용자가 `c` 키를 누르면
- Then: `OnCancelTask` 콜백이 currentTask.ID와 함께 호출되고, currentTask가 nil로 설정된다

### S6: TUI cancel 무동작
- Given: Worker TUI에서 currentTask가 nil일 때
- When: 사용자가 `c` 키를 누르면
- Then: 아무 동작도 수행되지 않는다 (nil panic 없음)

### S7: TelemetryConf 제거 후 파싱
- Given: `autopus.yaml`에 `telemetry:` 섹션이 존재하지 않을 때
- When: config를 로드하면
- Then: 에러 없이 파싱되고, `HarnessConfig`에 `Telemetry` 필드가 존재하지 않는다

### S8: TelemetryConf 제거 후 레거시 호환
- Given: `autopus.yaml`에 기존 `telemetry:` 섹션이 여전히 남아 있을 때
- When: config를 로드하면
- Then: YAML의 unknown field로 인해 파싱이 실패하지 않는다 (yaml.v3 기본 동작: unknown 무시)

### S9: Constraints config 연결
- Given: `constraints.enabled: true`, `constraints.path: ".autopus/context/custom-constraints.yaml"`로 설정되어 있을 때
- When: constraint 체크가 실행되면
- Then: 지정된 경로의 constraints 파일이 로드되어 검사에 사용된다

### S10: IssueReport.AutoSubmit 자동 트리거
- Given: `issue_report.auto_submit: true`이고 pipeline run이 FAIL 상태로 종료될 때
- When: pipeline error handler가 실행되면
- Then: `auto issue report --auto-submit` 로직이 자동으로 실행된다

### S11: Hints.Platform 연결
- Given: `hints.platform: false`로 설정되어 있을 때
- When: pipeline 성공 후 `CheckAndShow()`가 호출되면
- Then: hint가 표시되지 않는다
