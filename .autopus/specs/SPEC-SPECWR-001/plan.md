# SPEC-SPECWR-001 구현 계획 (축소판)

## 태스크 목록

- [x] **T1** — 신규 체크리스트 문서 `[NEW] autopus-adk/content/rules/spec-quality.md` 작성. 5 차원(`correctness` / `completeness` / `feasibility` / `style` / `security`) 섹션을 생성하고 각 차원마다 최소 3개의 체크 항목을 포함한다. 항목 ID는 `Q-CORR-01`, `Q-COMP-01`, `Q-FEAS-01`, `Q-STYLE-01`, `Q-SEC-01` 형식을 따른다.
- [x] **T2** — 각 체크 항목에 대해 "PASS 기준"과 "FAIL 기준"을 자연어 블록으로 나란히 기술한다. 판정이 애매한 항목은 예시 1개를 첨부한다.
- [x] **T3** — Priority 축(Must/Should/Nice)과 EARS type 축(Ubiquitous/EventDriven/StateDriven/Unwanted/Optional)을 분리 점검하는 항목 2개를 각각 작성한다. 예: `Q-STYLE-02` (Priority 어휘 제한), `Q-COMP-03` (EARS type 표기 완전성).
- [x] **T4** — 코드 정합성 점검 항목(`Q-CORR-02`)에 `[NEW]` 마커 예외 규칙을 명시한다. "기존 참조는 실제 경로 존재를 검증하며, `[NEW]` 마커가 붙은 planned addition은 정합성 FAIL 대상에서 제외한다."
- [x] **T5** — Security 차원(`Q-SEC-01` 이상)에는 프롬프트 인젝션, 파일 경로 traversal, 비밀값 노출 등 표준 위협 범주를 체크 포인트로 포함한다. 문서 편집만 하는 SPEC의 경우 "해당 없음(N/A)" 판정을 허용하는 규칙을 명기한다.
- [x] **T6** — 부록(Appendix)에 도메인 응집도(`cohesion`) 축을 별도 절로 추가한다. 5 차원 본체와 혼동하지 않도록 "본체 5 차원과 분리된 보조 축"이라는 헤더 문구를 명시한다.
- [x] **T7** — `autopus-adk/content/skills/spec-review.md` 상단 Overview에 self-check 참조 문단을 추가한다. 기존 "Step 1: Load SPEC" 직전에 "Step 0: spec-writer self-verify (see `content/rules/spec-quality.md`)" 노트를 삽입한다.
- [x] **T8** — `autopus-adk/content/agents/spec-writer.md`의 "작업 절차" 섹션을 개편한다. 기존 "3. SPEC 파일 생성" 뒤에 "4. 자체 검증 루프" 단계를 추가하고, 기존 "4. 디렉토리 생성"을 "5. 디렉토리 생성"으로 재번호한다. 자체 검증 루프 본문은 체크리스트 로드 → 차원별 자연어 적용 → FAIL 수정 → 최대 2회 재시도 → Open Issues 기록 순으로 기술한다.
- [x] **T9** — spec-writer가 `## Open Issues` 섹션을 기록할 때 사용하는 마크다운 포맷을 예시로 문서에 포함한다. 예: `- Q-COMP-02: REQ-003에 추적성 AC가 없음 (2회 수정 시도 후에도 해소 실패)`.
- [x] **T10** — 작성된 3개 문서(신규 1 + 수정 2)를 spec-quality 체크리스트로 self-assess하여 dogfooding을 수행한다. 결과를 본 SPEC의 research.md "Self-Assessment 결과" 섹션에 요약 기록한다.

## 구현 전략

### 접근 방법

1. **문서 중심(Document-only) 구현** — Go 코드 변경 없이 `autopus-adk/content/` 하위 3개 파일만 수정/생성하여 효과를 낸다. spec-writer 에이전트 프롬프트가 매 호출 시 `content/agents/spec-writer.md`를 읽으므로, 문서 업데이트만으로 행동 변화를 유도할 수 있다.
2. **체크리스트의 Go 타입 정렬** — `autopus-adk/pkg/spec/types.go:84-90`의 `FindingCategory` 5개 상수에 1:1 매핑되는 구조를 문서에 부여하여, 후속 SPEC-SPECWR-002에서 런타임 통합 시 매핑 작업 없이 그대로 파싱/주입이 가능하도록 한다.
3. **축 분리의 문서적 강제** — Priority와 EARS type이 혼합되지 않도록 체크리스트 항목 자체를 두 축 각각에 대해 별도로 둔다. 스타일 위반 검출을 운용자 판단에 맡기지 않고 항목화한다.

### 기존 코드 활용

- **`autopus-adk/pkg/spec/types.go:5-14`** — EARS type 5종을 체크리스트 어휘의 출처로 인용한다. 문서가 이 코드 진실의 원천과 동기화되어야 한다.
- **`autopus-adk/pkg/spec/types.go:35`** — Criterion.Priority 필드 주석을 체크리스트의 Priority 어휘 표준으로 인용한다.
- **`autopus-adk/pkg/spec/gherkin_parser.go:17-18`** — `rePriority` 정규식이 인정하는 3값(`Must|Should|Nice`)을 체크리스트 FAIL 기준 근거로 제시한다.
- **`autopus-adk/pkg/spec/validator.go:9`** — `ambiguousWords = []string{"should", "might", "could", "possibly", "maybe", "perhaps"}`를 REQ description 금지어 목록으로 재사용한다. 체크리스트 `Q-STYLE-01`이 이 목록을 그대로 참조한다.
- **`autopus-adk/content/agents/spec-writer.md` 기존 구조** — "작업 절차" 번호 체계를 유지하면서 새 단계를 삽입한다.

### 변경 범위

| 파일 | 변경 타입 | 예상 증분 라인 수 |
|------|-----------|-------------------|
| `[NEW] autopus-adk/content/rules/spec-quality.md` | 신규 | ~180줄 (5 차원 × 3~4 항목 + 부록) |
| `autopus-adk/content/skills/spec-review.md` | 편집 | ~10줄 추가 |
| `autopus-adk/content/agents/spec-writer.md` | 편집 | ~25줄 추가 (자체 검증 루프 단계) |

3개 파일 × 평균 ~70줄 = 약 215줄 증분. 모두 마크다운 문서이므로 `file-size-limit` 규칙의 300줄 상한은 여유 있게 준수된다. Go 소스는 건드리지 않으므로 테스트 재실행도 요구되지 않는다.

### 검증 전략

- **정적 검증** — `auto spec validate .autopus/specs/SPEC-SPECWR-001`로 EARS 형식, 모호어 포함 여부를 점검한다. 본 SPEC의 모든 REQ description은 `ambiguousWords` 검사를 통과해야 한다 (Priority 값은 description과 분리된 메타 라인에 배치).
- **체크리스트 dogfooding** — 본 SPEC 자체를 `spec-quality.md` 체크리스트로 점검한다. 결과는 research.md에 기록한다.
- **후속 SPEC 연결** — SPEC-SPECWR-002 초안을 동시에 작성 대기 상태로 둔다. 런타임 통합 요구가 발생하면 즉시 진행 가능하도록 인터페이스만 문서 수준에서 정의한다.
