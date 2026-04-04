# SPEC-ADKWIRE-002 수락 기준

## 시나리오

### S1: Audit 로거가 태스크 이벤트를 기록한다
- Given: LoopConfig.AuditLogPath가 설정되고 WorkerLoop가 시작됨
- When: 태스크가 수신되어 실행이 시작되고 완료됨
- Then: audit.jsonl 파일에 "started"와 "completed" 이벤트가 JSON Lines 형식으로 기록되며, 각 이벤트에 taskID, timestamp, durationMS가 포함됨

### S2: Audit 로그가 설정되지 않으면 기록을 건너뛴다
- Given: LoopConfig.AuditLogPath가 빈 문자열
- When: 태스크가 실행됨
- Then: audit 관련 코드가 no-op으로 동작하며 에러가 발생하지 않음

### S3: Scheduler가 cron 일정에 따라 태스크를 트리거한다
- Given: WorkspaceID가 설정되고 백엔드가 cron 스케줄 목록을 반환함
- When: 현재 시각이 cron 표현식과 매치됨
- Then: onTrigger 콜백이 호출되어 해당 payload로 handleTask가 실행됨

### S4: Scheduler가 동일 분(minute) 내 중복 트리거를 방지한다
- Given: 스케줄이 매분 실행으로 설정됨
- When: 같은 분 내에 tick이 2번 발생함
- Then: 태스크가 1번만 트리거됨

### S5: 병렬 태스크가 세마포어로 동시성이 제한된다
- Given: MaxConcurrency가 3으로 설정됨
- When: 5개 태스크가 동시에 수신됨
- Then: 최대 3개만 동시 실행되고, 나머지 2개는 슬롯이 해제될 때까지 대기함

### S6: MaxConcurrency가 1이면 순차 실행을 유지한다
- Given: MaxConcurrency가 0 또는 1
- When: 태스크가 수신됨
- Then: 세마포어 없이 기존 순차 실행 경로를 그대로 사용함

### S7: Worktree 격리가 병렬 태스크에 적용된다
- Given: MaxConcurrency > 1이고 WorktreeIsolation이 true
- When: 태스크 실행이 시작됨
- Then: `.worktrees/worker-{taskID}` 경로에 worktree가 생성되고, 실행 완료 후 제거됨

### S8: Worktree 생성 실패 시 in-place로 폴백한다
- Given: WorktreeIsolation이 true이지만 git worktree 명령이 실패함
- When: 태스크 실행이 시작됨
- Then: 경고 로그가 출력되고 기존 WorkDir에서 in-place 실행됨

### S9: Knowledge 검색이 KnowledgeCtx를 채운다
- Given: KnowledgeSync가 true이고 백엔드 Knowledge API가 결과를 반환함
- When: description이 "deploy production"인 태스크가 수신됨
- Then: KnowledgeSearcher.Search("deploy production")가 호출되고, 결과가 TaskPayload.KnowledgeCtx에 포맷되어 들어감

### S10: Knowledge 검색 실패 시 빈 KnowledgeCtx로 진행한다
- Given: Knowledge API가 에러를 반환함
- When: 태스크가 수신됨
- Then: KnowledgeCtx가 빈 문자열이고, 태스크 실행은 정상 진행됨

### S11: FileWatcher가 변경 파일을 백엔드에 동기화한다
- Given: KnowledgeSync가 true이고 FileWatcher가 시작됨
- When: WorkDir 내 파일이 수정됨
- Then: Syncer.SyncFile이 호출되어 변경된 파일의 SHA256 해시와 내용이 백엔드에 전송됨

### S12: Multi-workspace 모드가 워크스페이스별 A2A 서버를 스폰한다
- Given: Workspaces에 2개 워크스페이스가 설정됨
- When: WorkerLoop가 시작됨
- Then: 각 워크스페이스별 독립 A2A goroutine이 실행되고, MultiWorkspace.List()가 2개 항목을 반환함

### S13: 단일 워크스페이스일 때 기존 동작을 유지한다
- Given: Workspaces가 비어있거나 1개 항목만 있음
- When: WorkerLoop가 시작됨
- Then: MultiWorkspace가 생성되지 않고 기존 단일 A2A Server 경로가 사용됨

### S14: 서비스 종료 순서가 역순으로 실행된다
- Given: 모든 서비스가 시작된 상태
- When: context가 cancel됨
- Then: workspace → scheduler → knowledge → audit 순서로 종료되며, 5초 내에 모든 서비스가 정리됨

### S15: 수정된 파일이 300줄을 초과하지 않는다
- Given: 모든 구현이 완료됨
- When: 수정/생성된 .go 파일의 줄 수를 확인함
- Then: 모든 파일이 300줄 미만임
