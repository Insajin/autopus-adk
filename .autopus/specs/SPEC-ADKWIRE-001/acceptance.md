# SPEC-ADKWIRE-001 수락 기준

## 시나리오

### S1: Learn CLI — query 서브커맨드
- Given: `.autopus/learnings/pipeline.jsonl`에 3개 이상의 learning entry 존재
- When: `auto learn query --packages "pkg/worker" --keywords "timeout"` 실행
- Then: 관련도순으로 매칭 엔트리 출력, 매칭 없으면 "No matching entries" 메시지

### S2: Learn CLI — record 서브커맨드
- Given: 프로젝트 디렉토리에 `.autopus/learnings/` 존재
- When: `auto learn record --type gate_fail --pattern "lint timeout on large files" --phase phase3 --severity high` 실행
- Then: pipeline.jsonl에 새 엔트리 추가, ID는 `L-{NNN}` 형식, 성공 메시지 출력

### S3: Learn CLI — prune 서브커맨드
- Given: 90일 이상 된 엔트리 2개와 최근 엔트리 3개 존재
- When: `auto learn prune --days 90` 실행
- Then: "Pruned 2 entries" 출력, pipeline.jsonl에 최근 3개만 남음

### S4: Learn CLI — summary 서브커맨드
- Given: 다양한 type의 learning entry 10개 존재
- When: `auto learn summary --top 3` 실행
- Then: 총 엔트리 수, 타입별 카운트, 상위 3개 패턴, 개선 영역 출력

### S5: TokenRefresher 자동 시작
- Given: LoopConfig에 AuthToken과 CredentialsPath가 설정됨
- When: WorkerLoop.Start(ctx) 호출
- Then: TokenRefresher.Start()가 별도 goroutine에서 실행됨, ctx 취소 시 종료

### S6: TokenRefresher 토큰 갱신 전파
- Given: TokenRefresher가 실행 중이고 토큰이 5분 내 만료 예정
- When: TokenRefresher가 백엔드에서 새 토큰 수령
- Then: A2A Server의 auth token이 새 토큰으로 업데이트됨

### S7: TaskPoller fallback 활성화
- Given: A2A WebSocket 연결이 끊기고 모든 reconnect 재시도 실패
- When: OnReconnectFailed 콜백 호출
- Then: TaskPoller.Start()가 REST polling 시작, 수신된 task는 handleTask()로 전달

### S8: NetMonitor 네트워크 변경 감지
- Given: NetMonitor가 실행 중이고 네트워크 인터페이스 주소가 변경됨
- When: onValidate() 호출 시 WebSocket ping 실패
- Then: a2a.Transport.Reconnect() 호출됨

### S9: Pipeline Dashboard 실제 데이터
- Given: `.autopus-checkpoint.yaml`에 `phase: phase2`, task_status에 T1=done, T2=in_progress 존재
- When: `auto pipeline dashboard SPEC-TEST-001` 실행
- Then: Planning=done, Test Scaffold=done, Implementation=running, Testing/Review=pending 표시

### S10: Pipeline Dashboard 파일 없음 fallback
- Given: `.autopus-checkpoint.yaml` 파일이 존재하지 않음
- When: `auto pipeline dashboard SPEC-TEST-001` 실행
- Then: 모든 phase=pending으로 표시 + "checkpoint file not found, showing default" 경고
