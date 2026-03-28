# SPEC-ORCH-016 수락 기준

## 시나리오

### S1: Round 2 진입 시 stale surface 감지 및 재생성

- Given: Round 1이 완료되었고 opencode 프로바이더의 CLI 프로세스가 종료되어 surface가 stale 상태
- When: Round 2 `executeRound`가 호출됨
- Then: ReadScreen이 에러를 반환하여 surface가 invalid로 판정되고, 새 pane이 생성되어 CLI가 재실행되며, Round 2 프롬프트가 정상 전송됨

### S2: Claude 프로바이더는 surface 검증 skip

- Given: 3-provider debate (claude, opencode, gemini)에서 Round 1 완료
- When: Round 2 surface 검증 단계 진입
- Then: claude 프로바이더는 `needsSurfaceCheck`가 false를 반환하여 검증을 건너뛰고, opencode와 gemini만 검증 수행

### S3: Pane 재생성 실패 시 graceful degradation

- Given: opencode의 surface가 stale이고 SplitPane이 에러 반환
- When: `recreatePane`이 호출됨
- Then: 해당 프로바이더가 `skipWait = true`로 마킹되고, 나머지 프로바이더는 정상 진행되며, WARNING 로그가 출력됨

### S4: SendLongText 실패 시 recreatePane fallback

- Given: Round 2에서 surface가 ReadScreen으로는 valid하지만 SendLongText가 실패
- When: paste-buffer가 exit status 1 반환
- Then: 기존 retry 대신 recreatePane이 1회 시도되고, 성공 시 새 surface로 프롬프트 재전송

### S5: 전체 3-round debate 정상 완료

- Given: debate --rounds 3 with claude + opencode + gemini
- When: opencode가 Round 1, 2 후 각각 CLI 종료
- Then: 매 라운드 시작 시 surface 검증 → 재생성이 동작하여 3라운드 모두 3개 프로바이더의 응답이 수집됨

### S6: Round 1에서는 surface 검증 미실행

- Given: debate --rounds 2 시작
- When: Round 1 `executeRound` 호출
- Then: surface 검증 로직이 실행되지 않음 (round == 1이므로 skip)
