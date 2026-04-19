# SPEC-SPECWR-001 수락 기준 (축소판)

본 문서의 시나리오는 Gherkin(Given/When/Then) 형식을 따른다. 시나리오별 우선순위는 `Priority:` 메타 라인으로 기술한다. `autopus-adk/pkg/spec/gherkin_parser.go:12-19`의 파서 규칙(시나리오 헤더 `### S{N}: ...`, Priority 어휘 `Must|Should|Nice`)과 호환된다.

## 시나리오

### S1: 체크리스트 문서가 5 차원 구조로 작성된다

Priority: Must

Given `[NEW] autopus-adk/content/rules/spec-quality.md`가 신규로 작성된 상태이다.
When 문서를 읽어 차원(상위 섹션) 목록을 추출한다.
Then 정확히 5개의 주요 차원 섹션이 존재하며 각 섹션 제목은 `correctness`, `completeness`, `feasibility`, `style`, `security` 중 하나에 해당한다.
And 5 차원은 `autopus-adk/pkg/spec/types.go:84-90`의 `FindingCategory` 상수 5개와 1:1 대응한다.

### S2: 각 체크 항목에 고유 ID와 PASS/FAIL 기준이 부여된다

Priority: Must

Given `spec-quality.md`의 각 체크 항목이 작성된 상태이다.
When 임의의 항목을 선택해 본문을 파싱한다.
Then 항목은 `Q-{CATEGORY}-{NN}` 형식의 ID(예: `Q-CORR-01`)를 가진다.
And 해당 항목은 "PASS 기준" 블록과 "FAIL 기준" 블록을 모두 포함한다.

### S3: Priority 축과 EARS type 축이 별도 체크 항목으로 분리된다

Priority: Must

Given `spec-quality.md`의 `style` 및 `completeness` 차원 섹션이 작성된 상태이다.
When 섹션 내 항목을 조사한다.
Then EARS type 어휘(Ubiquitous/EventDriven/StateDriven/Unwanted/Optional) 검사 항목과 Priority 어휘(Must/Should/Nice) 검사 항목이 각각 독립된 체크 항목으로 존재한다.
And 두 축을 동일 FAIL 규칙에 묶은 항목은 존재하지 않는다.

### S4: Priority 어휘가 Must/Should/Nice로만 제한된다

Priority: Must

Given `spec-quality.md`의 Priority 검사 항목이 작성된 상태이다.
When 항목 본문의 허용 어휘 목록을 확인한다.
Then 허용 어휘는 정확히 `Must`, `Should`, `Nice` 세 값이며 `Could`, `P0`, `P1`, `P2` 등은 금지 어휘로 명시된다.
And 이 규칙은 `autopus-adk/pkg/spec/gherkin_parser.go:17-18`의 `rePriority` 정규식 수용 집합과 일치한다.

### S5: spec-writer가 생성 직후 자체 검증 루프를 실행한다

Priority: Must

Given `autopus-adk/content/agents/spec-writer.md`가 업데이트되어 "자체 검증 루프" 단계를 포함한다.
When spec-writer 에이전트가 새 SPEC 4개 파일 작성을 완료한다.
Then 에이전트는 즉시 `content/rules/spec-quality.md`를 읽어 체크리스트를 로드한다.
And 에이전트는 자기 생성 문서에 대해 각 체크 항목의 PASS/FAIL을 자연어로 판정한다.
And `research.md`에는 `## Self-Verify Summary` 섹션이 존재하며 각 항목이 `Q-* | 상태 | 시도 회차 | 관련 파일 | 사유` 형식으로 기록된다.

### S6: FAIL 발생 시 차원 단위로 필요한 모든 파일이 수정된다

Priority: Must

Given 자체 검증 루프가 하나 이상의 FAIL 항목을 보고한 상태이다.
When spec-writer가 수정 작업을 수행한다.
Then 수정 범위는 파일 단위가 아니라 FAIL이 발생한 차원(체크 항목)이 요구하는 모든 관련 파일이 된다.
And 예를 들어 REQ↔AC 추적성(`Q-COMP-02`) FAIL은 spec.md와 acceptance.md를 동시에 수정할 수 있다.

### S7: 재시도 한도가 2회로 제한된다

Priority: Must

Given 자체 검증 루프가 첫 판정에서 FAIL을 보고한 상태이다.
When spec-writer가 수정 후 재검증을 시도한다.
Then 1차 재시도(총 2회차)와 2차 재시도(총 3회차) 판정까지만 수행된다.
And 3차 재시도는 수행되지 않는다.

### S8: 재시도 소진 시 Open Issues 섹션이 기록된다

Priority: Must

Given 2회 재시도를 소진하고도 FAIL 항목이 남은 상태이다.
When spec-writer가 작업을 완료한다.
Then spec.md 말미에 `## Open Issues` 섹션이 추가된다.
And 각 항목은 `Q-* | category | scope | attempt | reason` 필드를 포함한 구조화된 리스트로 기록된다.

### S9: planned addition은 코드 정합성 FAIL에서 제외된다

Priority: Must

Given SPEC 본문이 아직 존재하지 않는 파일을 `[NEW]` 마커와 함께 언급한 상태이다 (예: `[NEW] autopus-adk/content/rules/spec-quality.md`).
When 체크리스트 `Q-CORR-02` (코드 정합성) 항목이 적용된다.
Then `[NEW]` 마커가 붙은 참조는 실제 파일 존재 검증을 생략하고 PASS 판정된다.
And `[NEW]` 마커가 없는 참조만 실제 경로 존재를 요구한다.

### S10: spec-review skill 문서가 체크리스트 참조를 포함한다

Priority: Must

Given `autopus-adk/content/skills/spec-review.md`가 업데이트된 상태이다.
When 문서 상단 Overview 또는 Pre-review 노트를 읽는다.
Then `content/rules/spec-quality.md`를 가리키는 명시적 참조 링크가 존재한다.
And self-check가 멀티 프로바이더 리뷰 이전에 선행됨이 기술된다.

### S11: REQ description에 모호어가 없고 REQ/AC 문장이 종결부호로 완결된다

Priority: Must

Given 본 SPEC의 모든 REQ description과 acceptance 시나리오 문장이 작성된 상태이다.
When `autopus-adk/pkg/spec/validator.go:9,52-60`의 `ambiguousWords` 검사와 종결부호 점검을 적용한다.
Then 어떤 REQ description에서도 `should`, `might`, `could`, `possibly`, `maybe`, `perhaps` 문자열이 검출되지 않는다.
And 모든 REQ 문장과 AC 문장은 마침표 또는 `다.`로 완결된다.
And Priority 값 `Should`는 REQ description이 아닌 메타 라인(`Priority: ...`)에만 배치된다.

### S12: 변경 범위가 content/ 하위로 한정된다

Priority: Must

Given 본 SPEC의 구현이 완료된 상태이다.
When git diff로 변경 파일 경로를 확인한다.
Then 변경된 파일은 모두 `autopus-adk/content/` 하위 경로에 속한다.
And `autopus-adk/pkg/spec/` 하위 Go 파일은 편집되지 않는다.

### S13: 부록 cohesion 축이 본체와 분리된다

Priority: Should

Given `spec-quality.md`에 부록 섹션이 추가로 작성된 상태이다.
When 부록 내 `cohesion` 축 기술을 확인한다.
Then 부록 헤더에 "본체 5 차원과 분리된 보조 축"이라는 문구가 포함된다.
And `cohesion`은 5 차원 본체 섹션에는 포함되지 않는다.
