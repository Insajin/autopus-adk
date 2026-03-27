# SPEC-ORCH-010 수락 기준

## 시나리오

### S1: Claude pane에서 permission 프롬프트가 발생하지 않음

- Given: Claude provider가 orchestra debate에 참여하고, 프로젝트 디렉토리에 파일이 존재할 때
- When: `auto brainstorm --multi --rounds 2` 실행
- Then: Claude pane에서 "Do you want to proceed?" 프롬프트가 나타나지 않고, 응답이 정상 수집됨

### S2: buildInteractiveLaunchCmd에 permission bypass 플래그 포함

- Given: Provider binary가 "claude"인 ProviderConfig
- When: `buildInteractiveLaunchCmd(provider)` 호출
- Then: 반환된 명령어 문자열에 `--dangerously-skip-permissions`가 포함됨

### S3: 비-Claude 프로바이더에 permission 플래그 미추가

- Given: Provider binary가 "opencode" 또는 "gemini"인 ProviderConfig
- When: `buildInteractiveLaunchCmd(provider)` 호출
- Then: 반환된 명령어 문자열에 `--dangerously-skip-permissions`가 포함되지 않음

### S4: Topic isolation으로 기존 파일 토론 방지

- Given: 프로젝트에 `.autopus/brainstorms/BS-001.md` 파일이 존재하고, debate 주제가 "Go vs Rust" 일 때
- When: Round 1 프롬프트가 provider에 전달됨
- Then: 프롬프트에 "Do NOT read, reference, or analyze any existing files" 문구가 포함됨

### S5: Rebuttal 프롬프트에도 topic isolation 적용

- Given: Round 2 rebuttal 프롬프트 빌드 시
- When: `buildRebuttalPrompt(original, others, round)` 호출
- Then: 반환된 프롬프트 시작 부분에 topic isolation instruction이 포함됨

### S6: Completion detection이 2회 연속 match를 요구

- Given: Provider가 응답 생성 중이고, 출력 중간에 `>` 패턴이 일시적으로 나타날 때
- When: `waitForCompletion()` polling 중 첫 번째 prompt match 감지
- Then: 즉시 completion으로 판정하지 않고, 2초 후 재확인하여 연속 match일 때만 completion 확정

### S7: 단일 prompt match로는 completion 미확정

- Given: 첫 번째 poll에서 prompt match, 2초 후 두 번째 poll에서 prompt 미감지 (AI가 다시 출력 중)
- When: `waitForCompletion()` 실행
- Then: Completion으로 판정하지 않고 polling 계속

### S8: Round 3 이후 rebuttal 프롬프트가 요약됨

- Given: Round 3의 rebuttal 프롬프트 빌드 시, 이전 라운드 응답이 각 2000자
- When: `buildRebuttalPrompt(original, others, 3)` 호출
- Then: 각 provider 응답이 500자 + "[...truncated]"로 요약됨

### S9: Round 2까지는 전체 응답 포함

- Given: Round 2의 rebuttal 프롬프트 빌드 시
- When: `buildRebuttalPrompt(original, others, 2)` 호출
- Then: 각 provider 응답이 truncation 없이 전체 포함됨

### S10: Per-round timeout 최소 45초 보장

- Given: totalSeconds=60, rounds=3 (계산상 20초/라운드)
- When: `perRoundTimeout(60, 3)` 호출
- Then: 반환값이 45초 (최소값 적용)

### S11: Consensus threshold가 config에서 읽힘

- Given: OrchestraConfig.ConsensusThreshold가 0.8으로 설정됨
- When: `consensusReached(responses, cfg)` 호출
- Then: 0.8 threshold로 consensus 판정 수행 (기본 0.66이 아님)
