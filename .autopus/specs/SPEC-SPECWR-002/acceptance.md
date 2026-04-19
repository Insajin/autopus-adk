# SPEC-SPECWR-002 수락 기준

## 시나리오

### S1: 체크리스트 파일이 embed FS에 있을 때 프롬프트에 주입된다
- Given: `content/rules/spec-quality.md`가 embed FS에 포함되어 빌드됨 (기존 `rules/*.md` 패턴이 자동 커버)
- Given: `BuildReviewPrompt`를 임의의 `SpecDocument`와 유효한 `specDir`로 호출
- When: 함수가 반환한 프롬프트 문자열을 검사
- Then: 프롬프트에 `## Quality Checklist` 헤더가 포함됨
- Then: 체크리스트 본문의 첫 줄이 `## Quality Checklist` 헤더 바로 뒤에 나타남
- Then: 삽입 위치가 `### Existing Code Context` 블록 이후, `### Instructions` 섹션 이전임
- Priority: Must

### S2: 체크리스트 파일이 없을 때 soft-fallback
- Given: embed FS에서 `rules/spec-quality.md` 제거된 상태(테스트 격리용 별도 embed FS 주입)
- Given: 디스크에도 해당 경로 파일 부재
- When: `BuildReviewPrompt` 호출
- Then: 함수는 에러를 반환하지 않음
- Then: 표준 에러에 `경고: 체크리스트 로드 실패` 문자열 포함 로그 1회 출력
- Then: 반환된 프롬프트에 `## Quality Checklist` 헤더가 부재
- Then: 나머지 프롬프트 섹션(SPEC 원문, plan/research/acceptance, FINDING 지시문)은 정상 포함
- Priority: Must

### S3: 체크리스트 본문이 200 라인을 초과하면 trim
- Given: `content/rules/spec-quality.md`가 300 라인인 테스트 fixture
- Given: `ReviewPromptOptions.DocContextMaxLines = 200`
- When: `BuildReviewPrompt` 호출
- Then: 프롬프트에 포함된 체크리스트 본문이 정확히 200 라인 + `... (trimmed 100 more lines)` 트림 공지로 끝남
- Then: 기존 `trimToLines` 헬퍼의 포맷과 동일
- Priority: Must

### S4: 리뷰어 응답의 CHECKLIST 라인이 구조화 파싱된다
- Given: provider 응답 문자열에 다음 라인들 포함:
  ```
  VERDICT: REVISE
  CHECKLIST: Q-CORR-01 | PASS
  CHECKLIST: Q-COMP-03 | FAIL | "acceptance.md에 error path 시나리오 부재"
  FINDING: [major] [completeness] autopus-adk/.autopus/specs/SPEC-X-001/acceptance.md Error path 미커버
  ```
- When: `ParseVerdict`를 provider="claude", revision=0, priorFindings=nil로 호출
- Then: 반환된 `ReviewResult.ChecklistOutcomes` 길이가 2
- Then: 첫 번째 outcome은 `{ID: "Q-CORR-01", Status: "PASS", Reason: "", Provider: "claude", Revision: 0}`
- Then: 두 번째 outcome은 `{ID: "Q-COMP-03", Status: "FAIL", Reason: "acceptance.md에 error path 시나리오 부재", Provider: "claude", Revision: 0}`
- Then: 기존 `Verdict`, `Findings` 파싱 결과는 본 SPEC 구현 전과 동일
- Priority: Must

### S5: CLI 최종 출력에 체크리스트 요약 렌더링
- Given: `runSpecReview`가 체크리스트 outcomes 3건(PASS 2, FAIL 1)을 포함한 최종 결과를 생성
- When: stdout 내용을 검사
- Then: `체크리스트 결과: 3건 (PASS: 2, FAIL: 1)` 라인 포함
- Then: 바로 뒤에 `- [FAIL] Q-COMP-03: acceptance.md에 error path 시나리오 부재` 형식의 FAIL 목록 포함
- Then: `ChecklistOutcomes`가 비어있을 경우 위 라인 모두 출력되지 않음
- Priority: Must

### S6: self-verification 로그 append
- Given: 빈 `.autopus/specs/SPEC-TEST-001/` 디렉토리
- When: `AppendSelfVerifyEntry`를 `{Dimension: "completeness", Status: "FAIL", Reason: "REQ-003 미종결"}` 엔트리로 호출
- Then: `.self-verify.log` 파일이 생성됨
- Then: 파일 내용이 JSON 한 줄이며 `json.Unmarshal`로 역직렬화 가능
- Then: 동일 함수를 99회 더 호출 → 100 라인
- Then: 101번째 호출 후 파일 라인 수는 정확히 100
- Then: 가장 오래된(1번째) 엔트리가 제거되고 2~101번째 엔트리가 남음
- Priority: Should

### S7: CLI 서브커맨드로 로그 기록
- Given: `auto spec self-verify --record SPEC-TEST-001 --dimension correctness --status FAIL --reason "추상용어 미정의"` 실행
- When: 커맨드 종료 코드 확인
- Then: 종료 코드 0
- Then: `.autopus/specs/SPEC-TEST-001/.self-verify.log`에 해당 내용이 JSON 라인 한 개로 append됨
- Then: `--status` 값이 `PASS`/`FAIL` 외일 때 종료 코드 1 + stderr에 검증 오류 메시지
- Priority: Should

### S8: Priority enum 변경 없음
- Given: 본 SPEC 구현 전후의 `pkg/spec/types.go`를 diff
- When: `Criterion.Priority` 필드와 관련 상수를 비교
- Then: `Criterion.Priority string` 타입 변경 없음
- Then: `gherkin_parser.go`의 `rePriority = regexp.MustCompile(\`(?i)^\s*Priority:\s*(Must|Should|Nice)\`)` 변경 없음
- Priority: Must

### S9: 기존 embed 패턴이 신규 체크리스트 파일을 자동 포함함을 회귀 검증
- Given: `content/embed.go`의 기존 `rules/*.md` 패턴 (변경 없음)
- Given: 테스트 fixture로 `content/rules/spec-quality.md` 파일 존재
- When: `go test ./content/... ./pkg/spec/...` 실행
- Then: 기존에 성공하던 모든 테스트 여전히 통과
- Then: `content.FS`에서 `rules/spec-quality.md` 파일 읽기 가능
- Then: `content/embed.go`의 `//go:embed` 디렉티브는 수정되지 않음(diff 0)
- Priority: Must

### S10: soft-fallback 동작 중에도 판정 파이프라인 정상 종료
- Given: 체크리스트 파일 부재로 soft-fallback 활성화
- Given: provider가 `VERDICT: PASS`와 FINDING 없음 응답
- When: `runSpecReview` 전체 실행
- Then: 종료 코드 0
- Then: `발견 사항: 0건` 또는 해당 라인 부재(기존 동작과 동일)
- Then: SPEC 상태가 기존 로직대로 `approved`로 업데이트됨
- Priority: Must

### S11: 체크리스트 응답 예시가 프롬프트에 포함됨
- Given: `BuildReviewPrompt`를 체크리스트 주입이 성공하는 상태로 호출
- When: 프롬프트 문자열 검사
- Then: `CHECKLIST: <항목 ID> | PASS` 패턴의 예시 라인이 프롬프트에 1회 이상 포함됨
- Then: `CHECKLIST: <항목 ID> | FAIL | <reason>` 패턴의 예시 라인이 프롬프트에 1회 이상 포함됨
- Then: 이 예시는 `writeChecklistExamples` 함수에서 생성되며 `writeFindingExamples`와 별도 섹션 헤더(`### Checklist Response Format`)로 구분됨
- Priority: Must

### S12: gitignore 적용 확인
- Given: 본 SPEC 구현 후의 저장소 상태
- When: 임의 SPEC 디렉토리에 `.self-verify.log` 생성 후 `git status`
- Then: 해당 파일이 untracked/modified 목록에 나타나지 않음
- Then: `git check-ignore -v <path>` 명령이 0을 반환하고 `.gitignore` 라인을 출력
- Priority: Should
