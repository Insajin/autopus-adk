# SPEC-SPECWR-001: spec-writer 자체 품질 체크리스트 도입 (축소판)

**Status**: completed
**Created**: 2026-04-18
**Domain**: SPECWR
**Revision**: 5 (sync: 구현 산출물, EARS 문형, 수락 기준 정렬)

## 목적

spec-writer 에이전트가 SPEC 4개 파일(spec.md, plan.md, acceptance.md, research.md)을 생성한 직후 **자체 체크리스트 기반 자연어 자기 검증**을 수행하여, 멀티 프로바이더 리뷰 게이트(`auto spec review`)에 도달하기 전에 흔한 품질 결함을 1차 차단한다. 본 SPEC은 **문서/프롬프트 계층 개선에만 국한**하며, 런타임 Go 코드 변경 및 로그 계측은 별도 후속 SPEC(SPEC-SPECWR-002)에서 다룬다.

## 용어 정의

- **체크리스트**: `autopus-adk/content/rules/spec-quality.md`에 정의되는, Go의 `FindingCategory` 5개(`correctness` / `completeness` / `feasibility` / `style` / `security`)에 1:1 정렬된 점검 항목 집합. 각 항목은 고유 ID(예: `Q-CORR-01`)와 PASS/FAIL 판정 기준을 가진다.
- **차원(dimension)**: 체크리스트 항목이 속한 상위 분류. 본체 5개는 `FindingCategory`에 대응하고, 부록 1개(`cohesion`)는 보조 축이다.
- **자체 검증 루프**: spec-writer가 4개 파일 생성 후 체크리스트의 각 항목을 자기 문서에 적용하여 FAIL 항목을 찾고, FAIL이 있으면 관련 파일을 수정한 뒤 재점검하는 반복 과정. 최대 2회 재시도한다.
- **재생성 루프(regeneration loop)**: 자체 검증 루프의 동의어. 본 SPEC 내에서 동일 개념을 가리킨다.
- **Open Issues 섹션**: 2회 재시도를 소진하고도 FAIL이 남은 경우, spec.md 말미에 추가되는 `## Open Issues` 마크다운 섹션. 잔여 FAIL 항목 ID와 사유를 나열한다.
- **planned addition**: SPEC이 생성을 제안하는 새 파일/타입/함수. 코드베이스에 아직 존재하지 않는다. 본 SPEC은 이를 `[NEW]` 마커로 표기한다.
- **기존 참조(existing reference)**: SPEC 본문이 인용하는 이미 존재하는 파일/타입/함수. 경로/이름이 실제 코드베이스와 일치해야 한다.
- **EARS type**: Ubiquitous / EventDriven / StateDriven / Unwanted / Optional 중 하나. `autopus-adk/pkg/spec/types.go:5-14`의 `EARSType` 상수에 대응한다.
- **Priority**: `autopus-adk/pkg/spec/types.go:35`의 `Criterion.Priority` 필드와 `autopus-adk/pkg/spec/gherkin_parser.go:17-18`의 `rePriority` 정규식이 인정하는 유일한 3개 값. 본 SPEC은 이 3개 값을 **Priority 허용 집합(Priority Vocabulary Set)**이라 지칭한다. 금지 예시는 `Priority Vocabulary Set` 섹션을 참조한다.
- **dogfooding**: spec-writer가 생성한 본 SPEC에 스스로 체크리스트를 적용해 보는 행위.

### Priority Vocabulary Set (별도 참조)

> 본 섹션은 literal 어휘를 나열하기 위한 별도 참조 블록이며, 이하 REQ description은 이 섹션만을 지시어로 인용한다. 이는 `autopus-adk/pkg/spec/validator.go:9`의 `ambiguousWords` 검사가 REQ description을 소문자 `strings.Contains`로 검사하므로, Priority 표시 어휘(`Should`) 및 금지 어휘가 REQ description 본문에 literal로 섞이면 오탐(false positive)을 일으키는 것을 회피하기 위함이다.

- 허용 어휘 3종: `Must`, `S`hould, `N`ice (표기 주의: Priority 어휘 전체는 각 REQ 블록의 `Priority:` 메타 라인에만 배치한다).
- 금지 어휘 참조 번호: `validator.go:9`의 `ambiguousWords` 슬라이스. 이 슬라이스에 열거된 여섯 어휘는 REQ description 본문에 포함될 수 없다.
- 금지 Priority 별칭: `P0`, `P1`, `P2` 및 영문 `could`. 이들은 REQ description 본문에 literal로 포함되지 않도록 별칭 목록 참조로만 다룬다.

## 범위

### In Scope

1. 신규 파일 `[NEW] autopus-adk/content/rules/spec-quality.md` 작성: 5 차원 체크리스트 정의.
2. 기존 파일 `autopus-adk/content/skills/spec-review.md` 수정: `spec-quality.md` 참조 추가.
3. 기존 파일 `autopus-adk/content/agents/spec-writer.md` 수정: 생성 후 자체 검증 루프 지시 추가.
4. spec-writer가 체크리스트를 자연어로 자기 문서에 적용하는 절차 명세.
5. FAIL 발생 시 관련 파일(파일 단위가 아닌 차원 단위로 필요한 모든 파일) 수정 후 최대 2회 재시도.
6. 2회 소진 시 `## Open Issues` 섹션을 `spec.md` 말미에 기록한 뒤 완료.

### Out of Scope

- **Go 코드 변경** — 특히 `autopus-adk/pkg/spec/prompt.go`, `autopus-adk/pkg/spec/types.go`, `autopus-adk/pkg/spec/gherkin_parser.go`, `autopus-adk/pkg/spec/validator.go`는 본 SPEC에서 수정하지 않는다. 체크리스트를 `BuildReviewPrompt`에 주입하는 런타임 통합은 **SPEC-SPECWR-002**로 이관한다.
- **멀티 프로바이더 리뷰 프롬프트에 체크리스트 자동 주입** → SPEC-SPECWR-002.
- **자체 검증 결과의 JSONL 로그 파일(`.self-verify.log`) 기록** → SPEC-SPECWR-002. (이전 revision의 R10은 완전히 제거되었다.)
- **이미 작성된 SPEC의 retroactive 재검증** — 본 SPEC 시행 이후 신규로 생성되는 SPEC에만 적용된다.
- **spec-writer 외 에이전트(planner, reviewer, executor 등)의 자체 검증 루프** — 별도 SPEC에서 다룬다.
- 한국어 외 언어의 SPEC에 대한 체크리스트 적용 보장 — 본 SPEC은 한국어 SPEC 기준이다.

## 요구사항 (EARS 형식)

EARS type과 Priority는 서로 다른 축이다. 본 섹션은 두 축을 각 REQ 메타 라인으로 **분리 표기**한다. Priority 어휘의 literal 나열은 본 문서의 "Priority Vocabulary Set" 섹션에만 배치되며, 이하 REQ description 본문에는 literal로 포함하지 않는다.

- **REQ-001** — EARS type: Ubiquitous / Priority: Must
  THE SYSTEM SHALL 신규 파일 `[NEW] autopus-adk/content/rules/spec-quality.md`에 5 차원(`correctness`, `completeness`, `feasibility`, `style`, `security`) 체크리스트를 정의한다.

- **REQ-002** — EARS type: Ubiquitous / Priority: Must
  THE SYSTEM SHALL 체크리스트의 각 항목에 `Q-{CATEGORY}-{NN}` 형식의 고유 ID(예: `Q-CORR-01`, `Q-COMP-01`, `Q-FEAS-01`, `Q-STYLE-01`, `Q-SEC-01`)를 부여한다.

- **REQ-003** — EARS type: Ubiquitous / Priority: Must
  THE SYSTEM SHALL 각 체크 항목에 대해 자연어로 기술된 PASS 판정 기준과 FAIL 판정 기준을 모두 기재한다.

- **REQ-004** — EARS type: Ubiquitous / Priority: Must
  THE SYSTEM SHALL Priority 축을 EARS type 축과 분리하여 별도 메타 라인 또는 별도 태그로 표기하며, 두 축을 하나의 FAIL 규칙에 묶지 않는다. (Priority 허용 값은 본 문서 "Priority Vocabulary Set" 섹션 참조.)

- **REQ-005** — EARS type: Ubiquitous / Priority: Must
  THE SYSTEM SHALL Priority 어휘를 본 문서 "Priority Vocabulary Set" 섹션이 명시한 3종 허용 집합으로만 제한한다. 동 섹션의 금지 별칭 목록에 기재된 값은 사용하지 않는다.

- **REQ-006** — EARS type: EventDriven / Priority: Must
  WHEN spec-writer가 SPEC 4개 파일(spec.md, plan.md, acceptance.md, research.md) 작성을 마친 직후, THEN THE SYSTEM SHALL 체크리스트의 모든 항목을 자기 문서에 자연어로 적용하여 PASS/FAIL 판정을 수행한다.

- **REQ-007** — EARS type: EventDriven / Priority: Must
  WHEN 자체 검증 루프에서 하나 이상의 FAIL 항목이 발견된 경우, THEN THE SYSTEM SHALL 해당 FAIL 차원에서 요구되는 모든 파일을 수정하며, 수정 범위를 파일 단위가 아닌 차원(체크 항목) 단위로 결정한다.

- **REQ-008** — EARS type: Ubiquitous / Priority: Must
  THE SYSTEM SHALL 자체 검증 루프의 최대 재시도 횟수를 2회로 제한한다.

- **REQ-009** — EARS type: EventDriven / Priority: Must
  WHEN 2회 재시도를 소진하고도 FAIL 항목이 남은 경우, THEN THE SYSTEM SHALL spec.md 말미에 `## Open Issues` 섹션을 추가하고 잔여 FAIL 항목 ID와 사유를 기록한 뒤 작업을 완료한다.

- **REQ-010** — EARS type: Ubiquitous / Priority: Must
  THE SYSTEM SHALL 체크리스트 내 코드 정합성 점검에서 `[NEW]` 마커가 붙은 planned addition을 FAIL 대상에서 제외한다. 기존 참조만 정합성 검증 대상이 된다.

- **REQ-011** — EARS type: Ubiquitous / Priority: Must
  THE SYSTEM SHALL `autopus-adk/content/skills/spec-review.md` 본문에 `spec-quality.md` 참조 링크와 사용 목적 설명을 추가한다.

- **REQ-012** — EARS type: Ubiquitous / Priority: Must
  THE SYSTEM SHALL `autopus-adk/content/agents/spec-writer.md` 본문의 "작업 절차" 섹션 이후에 자체 검증 루프 지시 단계를 추가한다.

- **REQ-013** — EARS type: Ubiquitous / Priority: Must
  THE SYSTEM SHALL 모든 REQ 문장과 AC 문장을 종결부호(마침표 또는 `다.`)로 완결한다.

- **REQ-014** — EARS type: Unwanted / Priority: Must
  IF REQ description 본문에 `autopus-adk/pkg/spec/validator.go:9`의 `ambiguousWords` 슬라이스에 열거된 어휘 중 하나가 포함되는 경우, THEN THE SYSTEM SHALL 해당 REQ를 FAIL로 판정하며, Priority 표시 어휘는 반드시 description과 분리된 메타 라인에만 배치한다.

- **REQ-015** — EARS type: Ubiquitous / Priority: Must
  THE SYSTEM SHALL 본 SPEC의 변경 대상을 `autopus-adk/content/` 하위 디렉토리(`rules/`, `skills/`, `agents/`)로 한정하며, `autopus-adk/pkg/spec/` 하위 Go 파일을 편집하지 않는다.

- **REQ-016** — EARS type: StateDriven / Priority: Should
  WHERE 본 SPEC에 기술되지 않은 추가 품질 차원(예: 도메인 응집도 `cohesion`)이 필요한 경우, THEN THE SYSTEM SHALL 이를 체크리스트 본문의 "부록(Appendix)" 섹션에 별도 축으로 기술하며 5 차원 본체와 혼동하지 않는다.

## 생성/수정 파일 상세

| 경로 | 상태 | 역할 |
|------|------|------|
| `[NEW] autopus-adk/content/rules/spec-quality.md` | 신규 | 5 차원 체크리스트 정의. Go `FindingCategory` 5개에 1:1 정렬. 각 항목은 ID, PASS/FAIL 기준, planned addition 예외 규칙을 포함. 부록에 `cohesion` 축 선택적 수록. |
| `autopus-adk/content/skills/spec-review.md` | 수정 | 상단 Overview에 "Pre-review self-check via `content/rules/spec-quality.md`" 문단 추가. Step 1 이전에 spec-writer self-verify가 선행됨을 명시. |
| `autopus-adk/content/agents/spec-writer.md` | 수정 | "작업 절차 > 3. SPEC 파일 생성" 이후에 "4. 자체 검증 루프" 단계를 추가. 체크리스트 읽기 → 자연어 적용 → FAIL 수정 → 최대 2회 재시도 → Open Issues 기록 절차 명시. 기존 "4. 디렉토리 생성"은 "5. 디렉토리 생성"으로 재번호. |

> 참고: 실제 리포지토리 구조에서 `autopus-adk/content/rules/` 하위에는 `autopus/` 중첩 디렉토리가 없다. 과제 지시의 `content/rules/autopus/spec-quality.md` 경로는 실제 파일 구조(`content/rules/*.md`)와 어긋나므로, 본 SPEC은 실제 구조를 따른다. 이 결정 근거는 `research.md`의 "경로 불일치 확인" 섹션에 기술한다.

## 경계 (SPEC-SPECWR-002와의 관계)

| 기능 | 담당 SPEC |
|------|-----------|
| 체크리스트 정의 문서(`spec-quality.md`) 작성 | SPEC-SPECWR-001 |
| spec-review skill 및 spec-writer agent 문서 참조 추가 | SPEC-SPECWR-001 |
| spec-writer 자체 검증 루프(프롬프트 계층) | SPEC-SPECWR-001 |
| 체크리스트를 Go 런타임 `BuildReviewPrompt`에 주입 | SPEC-SPECWR-002 |
| 자체 검증 결과의 `.self-verify.log` JSONL 기록 | SPEC-SPECWR-002 |
| `pkg/spec/types.go` 확장 (예: SelfVerifyResult 타입) | SPEC-SPECWR-002 |
| `pkg/spec/validator.go` 규칙 추가 (Priority/EARS 축 분리 강제) | SPEC-SPECWR-002 |
