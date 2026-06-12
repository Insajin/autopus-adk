# SPEC-SPECREV-002 수락 기준

각 Must 시나리오는 구체적 입력과 구체적 기대값(concrete expected output)을 가진 oracle
시나리오다. 구조적 신호(파일 존재 여부, heading 유무) 단독으로는 Must를 닫지 않는다. 리프
헬퍼 단위 검증(S5·S7)과 별도로 실제 소비 배선(S11·S12·S13)을 함께 oracle로 고정한다.

## Test Scenarios

### S1: N/A 체크리스트 라인이 reason과 함께 파싱된다 (REQ-001, INV-001)

Given provider 출력에 라인 `CHECKLIST: Q-SEC-01 | N/A | doc-only SPEC, no trust boundary`가 포함된다
When `spec.ParseVerdict("SPEC-X-001", output, "claude", 0, nil)`를 호출한다
Then `result.ChecklistOutcomes`의 길이는 1이다
And outcome은 `{ID: "Q-SEC-01", Status: ChecklistStatusNA, Reason: "doc-only SPEC, no trust boundary", Provider: "claude", Revision: 0}`와 정확히 일치한다

### S2: PASS·FAIL·N/A가 한 출력에서 모두 카운트된다 (REQ-001, INV-001)

Given 출력에 세 라인 `CHECKLIST: Q-CORR-01 | PASS`, `CHECKLIST: Q-COMP-03 | FAIL | "근거"`, `CHECKLIST: Q-SEC-01 | N/A | "doc-only"`가 포함된다
When `ParseVerdict`로 outcome을 파싱하고 `spec.CountChecklistStatuses`로 집계한다
Then `ChecklistOutcomes`의 길이는 3이다
And 집계 결과는 정확히 pass=1, fail=1, na=1이다

### S3: N/A 엔트리의 빈 reason은 거부된다 (REQ-002, INV-002)

Given 엔트리 `SelfVerifyEntry{Dimension: "security", Status: ChecklistStatusNA, Reason: ""}`가 주어진다
When 임시 spec 디렉토리에서 `spec.AppendSelfVerifyEntry(specDir, entry)`를 호출한다
Then 반환된 error는 non-nil이며 메시지에 부분 문자열 `reason`을 포함한다
And `.self-verify.log` 파일은 생성되지 않는다 (기록 전 거부)

### S4: reason이 있는 N/A 엔트리는 수락된다 (REQ-002, INV-002)

Given 엔트리 `SelfVerifyEntry{Dimension: "security", Status: ChecklistStatusNA, Reason: "doc-only SPEC, no trust boundary"}`가 주어진다
When `spec.AppendSelfVerifyEntry(specDir, entry)`를 호출한다
Then 반환된 error는 nil이다
And `.self-verify.log` 내용은 부분 문자열 `"status":"N/A"`를 포함한다

### S5: loopAwareMaxRevisions 리프 헬퍼가 floor를 적용한다 (REQ-003, INV-003)

Given `loopModeMinRevisions`는 5이다
When `loopAwareMaxRevisions(2, true)`와 `loopAwareMaxRevisions(8, true)`를 호출한다
Then 첫 호출은 정확히 5를 반환한다 (expected value 5)
And 둘째 호출은 정확히 8을 반환한다 (floor 이상이면 불변, expected value 8)

### S6: --loop 미설정 시 리비전 한도는 불변이다 (REQ-003, INV-003)

Given 설정 `maxRevisions`는 2이다
When `loopAwareMaxRevisions(2, false)`를 호출한다
Then 반환값은 정확히 2이다 (expected value 2, 설정 그대로)

### S7: 인식되지 않은 SHALL 라인이 ParseEARSWithWarnings에서 경고가 된다 (REQ-004, INV-004)

Given 텍스트 라인 `The button SHALL respond to the click`가 주어진다 (주어가 system이 아니고 WHEN/IF/WHERE/THEN 없음)
When `spec.ParseEARSWithWarnings(text)`를 호출한다
Then 반환된 requirements의 길이는 0이다
And warnings의 길이는 1이며 warnings[0]은 부분 문자열 `The button SHALL respond to the click`를 포함한다

### S8: 인식되는 EARS 라인은 경고를 만들지 않는다 (REQ-004, INV-004)

Given 텍스트 `WHEN the user clicks THEN the system SHALL respond`가 주어진다
When `spec.ParseEARSWithWarnings(text)`를 호출한다
Then requirements의 길이는 1이고 `requirements[0].Type`은 `EARSEventDriven`이다
And warnings의 길이는 0이다

### S9: wrapSpecLoadError는 본문 비어있음을 단정하지 않는다 (REQ-005, INV-005)

Given 센티넬 오류 `errBoom = errors.New("parse spec.md: malformed frontmatter")`가 주어진다
When `wrapSpecLoadError("SPEC-SPECREV-002", errBoom)`를 호출한다
Then 반환된 메시지는 부분 문자열 `SPEC-SPECREV-002`와 `parse spec.md: malformed frontmatter`를 포함한다
And `errors.Is(result, errBoom)`는 true이며 메시지는 `본문이 비어있습니다`를 포함하지 않는다

### S10: 파싱 불가 spec.md의 Load 실패가 실제 원인으로 보고된다 (REQ-005, INV-005)

Given SPEC ID 헤더(`# SPEC-...`)가 없는 spec.md를 가진 임시 spec 디렉토리가 주어진다 (도달 가능한 실제 실패 경로)
When `spec.Load(dir)`를 호출하고 그 오류를 `wrapSpecLoadError("SPEC-SPECREV-002", err)`로 감싼다
Then `spec.Load`는 부분 문자열 `SPEC ID를 찾을 수 없습니다`를 포함하는 non-nil error를 반환한다
And `wrapSpecLoadError` 결과 메시지는 `SPEC-SPECREV-002`와 그 원인 문자열을 포함하고 `본문이 비어있습니다`를 포함하지 않는다

### S11: --loop 한도 산출이 설정값과 LoopMode를 함께 소비한다 (REQ-003, INV-003)

Given `loopModeMinRevisions`는 5이고 `defaultMaxRevisions`는 3이다
When `resolveSpecReviewMaxRevisions(config.ReviewGateConf{MaxRevisions: 2}, true)`, `resolveSpecReviewMaxRevisions(config.ReviewGateConf{MaxRevisions: 0}, true)`, `resolveSpecReviewMaxRevisions(config.ReviewGateConf{MaxRevisions: 2}, false)`를 호출한다
Then 결과는 각각 정확히 5, 5, 2이다 (expected values: 설정 2는 floor 5로, 설정 0은 default 3 폴백 후 floor 5로, LoopMode=false는 설정 2 보존)
And `runSpecReviewWithOptions`가 이 산출값을 `specReviewLoopParams.maxRevisions`로 전달한다 (소비 배선)

### S12: ValidateSpec이 인식 불가 SHALL 라인을 warning ValidationError로 표면화한다 (REQ-004, INV-004)

Given `doc.RawContent`가 라인 `The button SHALL respond to the click`를 포함하고 ID/Title/Requirements/AcceptanceCriteria가 채워진 `SpecDocument`가 주어진다
When `spec.ValidateSpec(doc)`를 호출한다
Then 반환된 `[]ValidationError` 중 `Level == "warning"`이고 Message가 부분 문자열 `The button SHALL respond to the click`를 포함하는 항목이 정확히 하나 존재한다 (concrete expected output)

### S13: 빈 reason N/A는 전용 누락 마커로 렌더된다 (REQ-007, INV-006)

Given outcomes `[{ID: "Q-SEC-01", Status: ChecklistStatusNA, Reason: "", Provider: "claude"}, {ID: "Q-CORR-01", Status: ChecklistStatusPass, Reason: "", Provider: "claude"}]`가 주어진다
When `spec.RenderChecklistSection(outcomes)`를 호출한다
Then Q-SEC-01 행의 Reason 컬럼은 부분 문자열 `reason missing`(naMissingReasonNote)을 포함하고 `-`(emptyNotePlaceholder)가 아니다
And Q-CORR-01 PASS 행의 Reason 컬럼은 `-`(emptyNotePlaceholder)이다 (예상 출력: N/A 빈 reason만 마커로 구분)
And 집계 라인은 부분 문자열 `N/A: 1`을 포함한다 (expected value)

## Oracle Acceptance Notes

- S1~S13은 모두 concrete expected output을 명시한다: struct 동등성(S1), 정수 집계 pass/fail/na(S2),
  부분 문자열 포함/부재(S3·S4·S9·S10·S12·S13), 정확한 정수 반환값(S5·S6·S11), 슬라이스 길이와
  타입(S7·S8).
- 리프 헬퍼(S5·S7)와 소비 배선(S11 resolveSpecReviewMaxRevisions, S12 ValidateSpec, S13
  RenderChecklistSection)을 모두 oracle로 고정해, 헬퍼 반환값만 검증하고 배선이 inert로 남는 것을
  방지한다.
- 본 SPEC은 numeric formula 도메인이 아니므로 explicit numeric tolerance는 N/A다. 대신 결정적 정수
  동등성(예상 출력 5, 8, 2)과 문자열 동등성을 oracle로 사용한다.
- S10은 `spec.Load`의 도달 가능한 실제 실패 경로(SPEC ID 헤더 부재)를 사용한다. 성공한 Load에서
  `RawContent == ""`는 도달 불가능하므로(research.md `## Revision 1 closure` 참조) 전제하지 않는다.
- 회귀 잠금: 기존 `TestParseVerdict_ParsesChecklistOutcomes`(PASS/FAIL 경로)와
  `TestSpecSelfVerifyCmd_AcceptsNAStatus`(reason 포함 N/A)가 계속 통과해야 한다.
