# SPEC-SPECWR-001 리서치 (축소판)

## 기존 코드 분석

### EARS type과 Priority 어휘의 진실 원천

- **`autopus-adk/pkg/spec/types.go:5-14`** — `EARSType` 상수 5종을 정의한다.
  ```
  EARSUbiquitous, EARSEventDriven, EARSStateDriven, EARSUnwanted, EARSOptional
  ```
  본 SPEC의 REQ 메타 라인은 이 상수 문자열 값(`Ubiquitous`, `EventDriven`, `StateDriven`, `Unwanted`, `Optional`)을 그대로 인용한다.

- **`autopus-adk/pkg/spec/types.go:31-37`** — `Criterion` 구조체는 `Priority` 필드를 `string`으로 가지며 주석으로 `"Must", "Should", "Nice" (default: "Must")`만 허용됨을 명시한다.

- **`autopus-adk/pkg/spec/gherkin_parser.go:17-18`** — `rePriority` 정규식 `(?i)^\s*Priority:\s*(Must|Should|Nice)`이 파서가 인정하는 유일한 3값을 고정한다. 체크리스트는 이 집합과 일치해야 한다.

- **`autopus-adk/pkg/spec/validator.go:9`** — `ambiguousWords = []string{"should", "might", "could", "possibly", "maybe", "perhaps"}`. 본 SPEC의 체크리스트 항목(`Q-STYLE-01` 예정)은 이 목록을 그대로 FAIL 어휘 기준으로 참조한다. 검사는 `strings.Contains(strings.ToLower(description), word)`로 수행되므로(`validator.go:52-54`) **단어 경계를 따지지 않는다**. 즉, REQ description 본문에 대문자 `Should`를 Priority 의미로 쓰더라도 `strings.ToLower`에 의해 `"should"`와 충돌한다. 본 SPEC은 Priority 값의 literal 나열을 별도 참조 블록("Priority Vocabulary Set")으로 격리하고 REQ description 본문에서는 그 블록을 지시어로만 참조하여 이 충돌을 회피한다.

- **`autopus-adk/pkg/spec/parser.go:22-48` (`ParseEARS`)** — 입력 텍스트를 줄 단위로 스캔한다. 빈 줄, `#`으로 시작하는 헤더 줄, `-`으로 시작하는 리스트 줄은 건너뛴다. 따라서 본 SPEC의 `- **REQ-001** — EARS type: ...` 메타 라인은 파서가 **무시**하며, 들여쓰기된 description 라인만 description으로 저장된다. 이 동작 덕에 메타 라인 내 `Should` 문자열은 `ambiguousWords` 검사 대상이 아니다.

### 리뷰 프롬프트 조립 지점

- **`autopus-adk/pkg/spec/prompt.go:17-76`** — `BuildReviewPrompt`는 런타임에 리뷰 프롬프트를 조립한다. 이 함수는 `doc.RawContent`, Acceptance Criteria, aux docs(plan/research/acceptance), code context를 순서대로 합친 뒤 `buildDiscoverInstructions` 또는 `buildVerifyInstructions`로 마감한다. 체크리스트를 이 프롬프트에 주입하려면 Go 코드 수정이 필수이므로 **SPEC-SPECWR-002로 이관**한다.
- `BuildReviewPrompt`는 `doc.RawContent`를 포함하므로, `spec.md` 본문에 체크리스트 ID(`Q-CORR-01` 등)를 적어두면 리뷰 모델에 자연스럽게 노출된다. 이는 SPEC-SPECWR-002 없이도 간접적 효과를 낸다.

### 리뷰 분류(Finding Category) 정렬

- **`autopus-adk/pkg/spec/types.go:81-90`** — `FindingCategory` 상수는 정확히 다음 5개이다.
  ```
  correctness, completeness, feasibility, style, security
  ```
  이전 revision(40건 REVISE)에서 `security`에 대응하는 체크 차원이 없었던 것이 구조적 결함 #5였다. 본 SPEC은 5개 모두를 체크리스트 본체 차원으로 포함하도록 REQ-001과 S1에 명시한다.

### spec-writer 에이전트 문서

- **`autopus-adk/content/agents/spec-writer.md`** — 실제 파일은 149줄이며 "작업 절차" 섹션이 1→2→3→4(SPEC 파일 생성 → 디렉토리 생성) 순으로 구성된다. 본 SPEC은 기존 "3" 다음에 "4. 자체 검증 루프"를 삽입하고 기존 "4"는 "5. 디렉토리 생성"으로 재번호한다.

### skill 문서

- **`autopus-adk/content/skills/spec-review.md`** — 77줄로 간결하다. Step 1~5로 리뷰 흐름을 기술한다. 본 SPEC은 Step 1 이전에 "Step 0: spec-writer self-verify" 노트를 삽입한다.

### 경로 불일치 확인 (과제 지시 vs 실제 구조)

- 과제 지시는 `autopus-adk/content/rules/autopus/spec-quality.md`, `autopus-adk/content/skills/autopus/spec-review.md`, `autopus-adk/content/agents/autopus/spec-writer.md` 경로를 제시했다.
- 실제 `autopus-adk/content/` 구조에는 `autopus/` 중첩 디렉토리가 없다. `content/rules/`, `content/skills/`, `content/agents/` 바로 아래에 `.md` 파일이 평탄(flat)하게 배치된다. 확인 명령: `ls autopus-adk/content/rules/` → `branding.md`, `context7-docs.md`, ... 11개 파일.
- 반면 `.claude/rules/autopus/`, `.gemini/rules/autopus/`, `.codex/rules/autopus/` 등 **설치 대상(install target)** 디렉토리에는 `autopus/` 중첩이 있다. 이는 `content/`가 소스 템플릿이고, 플랫폼별 `.claude/` 등은 복제본이기 때문이다.
- 결정: 본 SPEC은 **소스**인 `content/` 하위의 실제 구조를 따라 `content/rules/spec-quality.md`에 작성한다. 설치 시점(`auto install` 또는 이에 상응하는 sync)에 플랫폼별 `rules/autopus/spec-quality.md`로 배포된다. 이 복제 경로는 본 SPEC의 out-of-scope이며 기존 설치 메커니즘에 위임한다.

## 설계 결정

### D1: Go 코드 변경 배제 (축소판의 핵심)

이전 REVISE 40건에서 반복적으로 지적된 구조적 결함 #1은 "체크리스트를 skill/agent md에만 넣으면 `pkg/spec/prompt.go`의 런타임 리뷰 프롬프트에 주입되지 않는다"였다. 본 SPEC은 이 지적을 인정하되, **범위를 분할**하는 방식으로 대응한다.

- 본 SPEC(001): 문서 계층에서 spec-writer의 **자기 점검(self-verify)**을 유도한다. 런타임 리뷰 게이트와 독립적으로 동작한다.
- 후속 SPEC(002): `BuildReviewPrompt`에 체크리스트를 정식으로 주입하여 **리뷰어 프롬프트**에도 체크리스트를 반영한다.

이 분할의 트레이드오프:
- 장점: 001의 변경 영향은 `content/` 하위 3개 md 파일로 국한되며 테스트·빌드 재실행이 불필요하다. 신속한 병합이 가능하고 회귀 위험이 낮다.
- 단점: 001만으로는 리뷰어(LLM 프로바이더)가 체크리스트를 명시적으로 보지 못한다. spec-writer self-verify 통과분만 품질 향상된다. 이는 "허약한 반쪽 해결"처럼 보일 수 있으나, 002로 이어지면 완전해진다. 두 SPEC을 묶어서 계획하는 것을 전제로 한다.

### D2: EARS type과 Priority 축 분리

이전 REVISE의 구조적 결함 #2는 `Ubiquitous`(EARS type)와 `Must`(Priority)를 동일 FAIL 규칙에 묶은 것이었다. 두 축은 서로 직교한다.
- EARS type은 **요구사항의 문법적 형태**(조건부/비조건부/예외/상태/복합)이다.
- Priority는 **요구사항의 중요도**(구현 필수/권장/선택)이다.
- 동일한 Ubiquitous 요구사항이 Must일 수도, Nice일 수도 있다. 역도 성립한다.
본 SPEC은 REQ 메타 라인에 두 축을 각각 별도 필드로 기재하고, 체크리스트에서도 별도 항목(`Q-COMP-XX` EARS type 완전성, `Q-STYLE-XX` Priority 어휘 제한)으로 분리한다.

### D3: "FAIL 차원 단위 수정" vs "FAIL 파일 단위 수정"

이전 REVISE의 구조적 결함 #4는 "FAIL 파일만 수정" 제약이 교차 문서 수정을 차단한다는 것이었다. 예: REQ↔AC 추적성 FAIL은 spec.md와 acceptance.md를 동시에 수정해야 하는데, "FAIL 파일만 수정" 규칙은 이 중 하나만 손대도록 강제한다.
- 해소: 본 SPEC은 수정 단위를 **차원(체크 항목) 기준**으로 정의한다. 해당 차원이 요구하는 모든 파일을 spec-writer가 수정할 수 있다. REQ-007과 S6에 명시한다.

### D4: planned addition과 기존 참조의 분리

이전 REVISE의 구조적 결함 #6은 "코드 정합성 규칙이 신규 파일/타입/함수 추가도 FAIL시킴"이었다. SPEC은 본질적으로 **미래의 변경**을 기술하므로, SPEC이 제안하는 신규 파일은 당연히 코드베이스에 아직 없다. 이를 정합성 FAIL로 묶으면 SPEC은 영원히 FAIL한다.
- 해소: `[NEW]` 마커를 명시 규약으로 도입한다. 체크리스트 `Q-CORR-02`(코드 정합성)는 기존 참조만 검증 대상으로 삼고, `[NEW]` 접두는 건너뛴다. REQ-010 및 S9에 명시한다.
- 본 SPEC 자체에서도 `[NEW] autopus-adk/content/rules/spec-quality.md`를 일관되게 표기한다.

### D5: R10(`.self-verify.log` JSONL) 제거

이전 revision의 R10은 spec-writer가 검증 결과를 `.self-verify.log`에 JSONL로 기록하도록 요구했다. 구조적 결함 #7은 "LLM 프롬프트만으로 안정된 JSONL 기록을 보장할 수 없다"였다. 프롬프트가 반복될 때마다 JSON 포맷 깨짐, 필드 누락, 파일 경로 오기 등이 발생할 수 있다.
- 해소: R10을 **본 SPEC에서 완전히 제거**하고 로그 계측을 SPEC-SPECWR-002로 이관한다. Go 코드로 구조화된 기록을 수행하면 포맷 불안정성을 제거할 수 있다.

### D6: security 차원 포함 (N/A 허용)

이전 REVISE의 구조적 결함 #5는 `FindingCategory`의 5번째 값인 `security`에 대응하는 체크 차원이 없었다는 것이었다. 본 SPEC은 `security` 차원을 체크리스트 본체에 포함하되, 문서 편집만 하는 SPEC에는 "해당 없음(N/A)" 판정을 명시적으로 허용한다(REQ-001 → plan.md T5).
- 대안 검토: security 차원을 생략하고 5개가 아닌 4개 차원으로 축소하는 방안도 있었으나, Go 타입과의 1:1 정렬을 깨면 002 통합 시 매핑 비용이 발생한다. 1:1 정렬 유지를 택했다.

### D7: cohesion 축을 부록으로 격리

도메인 응집도(cohesion) — 단일 SPEC이 여러 디렉토리/모듈을 건드리지 말아야 한다는 원칙 — 는 품질 차원이긴 하나 Go `FindingCategory` 5개에는 포함되지 않는다. 본체에 섞으면 1:1 정렬이 깨지고 과적합 위험이 생긴다.
- 해소: 부록(Appendix)으로 격리한다. REQ-016과 S13에 명시한다. 본체 5 차원과 혼동하지 않도록 부록 헤더에 경계 문구를 둔다.

### D8: Priority 어휘 literal의 REQ description 배제

`validator.go:9`의 `ambiguousWords`는 `strings.Contains`로 소문자 부분 문자열 검사를 수행한다. Priority 어휘 중 `Should`는 소문자 `should`와 충돌하고, 금지 별칭 `could` 역시 마찬가지이다. 따라서 REQ description 본문에 Priority 어휘 또는 금지 별칭을 literal로 나열하면 자기 SPEC이 자기 validator의 경고를 유발한다.
- 해소: 본 SPEC은 Priority 어휘 literal을 `## 용어 정의` 아래 별도 블록("Priority Vocabulary Set")으로 격리하고, REQ-004/005/014는 그 블록을 지시어(reference)로만 참조하도록 재작성했다. dogfooding 점검에서 이 조치 적용 전 REQ-004/005/014가 FAIL 판정을 받았고, 적용 후 16개 REQ 모두 PASS로 전환되었다.

## Self-Assessment 결과 (dogfooding)

본 SPEC 자체에 5 차원 체크리스트를 적용한 결과이다. 체크리스트 문서(`spec-quality.md`)가 아직 작성되지 않았으므로 본 평가는 본 SPEC이 **규정하는** 기준에 대해 자연어로 수행되었으며, `ambiguousWords` 검사는 Python으로 `strings.Contains` 동작을 재현해 실제 시뮬레이션했다.
추가로 `ParseEARS`는 `- **REQ-xxx**` 메타 라인을 무시하므로, 아래 EARS 관련 평가는 각 description 라인의 실제 `WHEN/IF/WHERE ... THEN THE SYSTEM SHALL ...` 문형을 기준으로 수행했다.

### 1차 평가 (수정 전)

| 차원 | 판정 | 근거 |
|------|------|------|
| correctness | PASS | REQ의 인용 경로와 라인 번호(`types.go:5-14`, `types.go:35`, `gherkin_parser.go:17-18`, `validator.go:9`, `parser.go:22-48`)를 Read로 직접 확인했다. |
| completeness | PASS | 16개 REQ가 S1~S13 AC로 커버된다. 용어 정의 섹션이 10개 용어를 정의한다. |
| feasibility | PASS | 변경 대상이 `content/` 하위 3개 md 파일이다. |
| **style** | **FAIL** | Python 시뮬레이션에서 REQ-004(`should` 검출), REQ-005(`should`, `could` 검출), REQ-014(`should`, `might`, `could`, `possibly`, `maybe`, `perhaps` 전부 검출)의 description이 `ambiguousWords`에 걸렸다. Priority 어휘 literal을 description 본문에 나열한 것이 원인. |
| security | N/A | md 편집만 수행. |

### 수정 (FAIL 해소)

- Priority 어휘 literal을 `## 용어 정의` 하위의 "Priority Vocabulary Set" 별도 참조 블록으로 격리.
- REQ-004, REQ-005, REQ-014의 description을 수정: literal 어휘 나열 대신 해당 참조 블록 또는 `validator.go:9`의 슬라이스 이름을 지시어로만 인용.
- 재시뮬레이션: 16개 REQ 모두 `ambiguousWords` 검사 통과. 결과는 이 research.md와 동 디렉토리의 audit 흐름에 반영.

### 2차 평가 (수정 후, 최종)

| 차원 | 판정 | 근거 |
|------|------|------|
| correctness | PASS | 기존 참조 5곳 모두 실제 경로/라인 번호 일치 확인. Priority 허용 집합(Must/Should/Nice)과 EARS type 5종이 Go 상수와 일치. |
| completeness | PASS | 16개 REQ가 S1~S13 AC로 모두 커버된다: REQ-001→S1, REQ-002→S2, REQ-003→S2, REQ-004→S3, REQ-005→S4, REQ-006→S5, REQ-007→S6, REQ-008→S7, REQ-009→S8, REQ-010→S9, REQ-011→S10, REQ-012→S5, REQ-013→S11, REQ-014→S11, REQ-015→S12, REQ-016→S13. 용어 정의 섹션이 체크리스트·차원·자체 검증 루프·재생성 루프·Open Issues 섹션·planned addition·기존 참조·EARS type·Priority·dogfooding 10개 용어를 정의한다. 모든 REQ가 `다.` 또는 `.`로 종결되고, S11이 REQ/AC 종결부호와 모호어 배제를 함께 검증한다. |
| feasibility | PASS | 변경 대상이 `content/` 하위 3개 md 파일이며, sync 과정에서 `.autopus/specs/SPEC-SPECWR-001/` 문서를 실제 산출물에 맞게 정렬했다. Go 코드 수정 없음 → 빌드/테스트 재실행 불필요. 약 215줄 증분으로 file-size-limit(300줄) 여유. 모든 단계가 기존 에이전트 툴(Read/Grep/Write)로 수행 가능하다. 후속 SPEC-SPECWR-002로 이관된 항목은 경계 표에 명시했다. |
| style | PASS | 본문은 한국어. 모든 REQ description이 실제 parser 기준 EARS 패턴(`THE SYSTEM SHALL ...`, `WHEN ... THEN THE SYSTEM SHALL ...`, `IF ... THEN THE SYSTEM SHALL ...`, `WHERE ... THEN THE SYSTEM SHALL ...`)을 따른다. Priority는 별도 메타 라인에만 배치. `ambiguousWords` 검사 16/16 통과(Python 재현 시뮬레이션). |
| security | N/A | 본 SPEC은 md 파일 편집만 수행한다. 프롬프트 인젝션 경로는 spec-writer 프롬프트가 `content/rules/spec-quality.md`를 읽을 때 발생 가능하나, 해당 파일은 리포지토리 내부 신뢰 경로이며 외부 입력 통로가 아니다. 파일 경로 traversal은 `..`/절대 경로 조작 입력이 없으므로 해당 없다. 비밀값 노출 없음. 명시적 N/A 처리. |

## 결함 해소 매핑 (이전 40건 REVISE → 본 SPEC 섹션)

| # | 이전 결함 | 본 SPEC 해소 위치 |
|---|-----------|-------------------|
| 1 | 체크리스트 skill/agent md만으로는 런타임 리뷰 프롬프트 미반영 | spec.md "경계" 표, research.md D1, REQ-015 (Go 코드 배제), Out of Scope 첫 항목 |
| 2 | EARS type과 Priority 혼합 | spec.md REQ-004, research.md D2, acceptance.md S3 |
| 3 | Priority 어휘가 Must/Should/Nice로 제한 (Go 진실) | spec.md 용어 정의 Priority 항, Priority Vocabulary Set 섹션, REQ-005, research.md "EARS type과 Priority 어휘의 진실 원천" + D8, acceptance.md S4 |
| 4 | "FAIL 파일만 수정" 제약이 교차 문서 수정 차단 | spec.md REQ-007, research.md D3, acceptance.md S6 |
| 5 | security 차원 누락 | spec.md REQ-001, plan.md T5, research.md D6, acceptance.md S1 |
| 6 | 신규 추가도 FAIL시키는 코드 정합성 규칙 | spec.md 용어 정의 planned addition 항, REQ-010, research.md D4, acceptance.md S9 |
| 7 | R10 (`.self-verify.log` JSONL) 불안정성 | spec.md Out of Scope 세 번째 항목(R10 완전 제거 선언), research.md D5 |
