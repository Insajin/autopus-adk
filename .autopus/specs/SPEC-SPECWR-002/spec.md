# SPEC-SPECWR-002: Spec-Review Runtime Checklist Injection (Go)

**Status**: completed
**Created**: 2026-04-18
**Domain**: SPECWR
**Module**: autopus-adk
**Package**: `pkg/spec/`, `internal/cli/`, `content/`
**Depends on**: SPEC-SPECWR-001 (산출물 `content/rules/spec-quality.md`를 입력 자산으로 소비한다. SPEC-001이 실제 리포 구조인 평탄 `content/rules/` 하위에 파일을 배치하는 결정을 반영한다)
**Related**: SPEC-ORCH-019, SPEC-ACCGATE-001

## 용어 정의

본 SPEC에서 반복되는 용어는 다음과 같이 정의한다. 추상 용어는 사용 전 정의 선행 규칙에 따라 명시한다.

- **런타임 리뷰 프롬프트**: `pkg/spec/prompt.go`의 `BuildReviewPrompt` 함수가 반환하는 문자열. `auto spec review` 실행 시 각 provider에게 전달되는 최종 프롬프트이다.
- **체크리스트 섹션**: 런타임 리뷰 프롬프트 내부에 삽입되는 `## Quality Checklist` 구획. `content/rules/spec-quality.md` 원문을 그대로 또는 trim 후 포함한다.
- **체크리스트 주입 (checklist injection)**: SPEC-001이 정의하는 체크리스트 파일의 내용을 런타임 리뷰 프롬프트에 삽입하는 Go 코드 경로. embed FS 또는 디스크 읽기로 소스 파일을 로드한다.
- **체크리스트 결과 (checklist outcome)**: 리뷰어가 체크리스트 항목별로 반환하는 `CHECKLIST: <ID> | <PASS|FAIL> | <reason?>` 라인의 파싱 결과. 신규 구조체 `ChecklistOutcome`으로 표현한다.
- **self-verification 로그**: spec-writer가 4개 파일 생성 직후 자체 체크리스트 결과를 JSON Lines 형식으로 기록하는 파일. 경로는 `.autopus/specs/SPEC-{ID}/.self-verify.log`이며 `.gitignore`로 제외한다.
- **Go 헬퍼 호출**: LLM이 로그 파일을 직접 편집하는 방식이 아니라, `auto spec self-verify --record` 같은 CLI 서브커맨드를 통해 로그 엔트리를 추가하는 결정적 경로.
- **로그 retention**: `.self-verify.log` 파일의 보존 정책. 단일 SPEC당 최대 100 라인 보관, 초과 시 오래된 엔트리부터 제거한다.
- **fail-fast**: 체크리스트 파일 로딩 실패 시 리뷰 전체를 중단하고 오류 메시지를 반환하는 동작.
- **soft-fallback**: 체크리스트 파일 로딩 실패 시 경고 로그만 남기고 체크리스트 없이 리뷰를 진행하는 동작.

## 목적

SPEC-SPECWR-001은 체크리스트를 `content/rules/spec-quality.md`로 외부화하고 spec-writer 에이전트와 spec-review skill이 동일 파일을 참조하도록 명시한다. 그러나 실제 `auto spec review` 실행 경로를 조사하면 다음 사실이 확인된다.

1. 런타임 리뷰 프롬프트는 `pkg/spec/prompt.go:BuildReviewPrompt`에서 Go 코드로 조립된다.
2. 현재 구현은 `spec.md` 원문, `plan.md`, `research.md`, `acceptance.md`, 코드 컨텍스트, 판정 규칙, FINDING 포맷 예시만 주입한다.
3. skill 문서(`content/skills/*.md`)는 `/auto spec review` 명령의 상위 오케스트레이션을 설명하지만, 실제 프롬프트 문자열을 조립하지는 않는다.

따라서 skill 문서만 갱신해도 런타임 리뷰어는 체크리스트를 보지 못한다. 본 SPEC은 Go 코드 경로를 수정하여 체크리스트를 프롬프트에 실제 삽입하고, 리뷰어 응답에서 체크 항목별 PASS/FAIL을 구조화 파싱하며, spec-writer 자체 검증 로그를 결정적 Go 헬퍼로 기록하는 세 가지 변경을 다룬다.

SPEC-001과의 의존 방향은 단방향이다. SPEC-001이 체크리스트 md를 작성하면, SPEC-002가 그것을 읽어 프롬프트에 주입한다. SPEC-001의 R08이 "Go 함수로 구현하는 자동 점검 스크립트는 후속 SPEC에서 다룬다"고 명시한 부분이 본 SPEC의 범위이다.

## 요구사항

### P0 — Must Have

#### R01 — 체크리스트 파일 로더
WHEN `BuildReviewPrompt`가 호출되면, THE SYSTEM SHALL `content/rules/spec-quality.md`를 embed FS에서 로드한다. 파일이 embed FS에 존재하지 않으면 디스크 fallback 경로(`autopus-adk/content/rules/spec-quality.md` 상대 경로)를 시도한다.

#### R02 — embed 자동 포함 회귀 검증
WHEN 본 SPEC이 구현되면, THE SYSTEM SHALL `content/embed.go`의 기존 `//go:embed rules/*.md` 패턴이 SPEC-SPECWR-001 산출물 `spec-quality.md`를 자동 포함함을 회귀 테스트로 확인한다. 테스트는 `content.FS`에서 `rules/spec-quality.md` 파일을 성공적으로 읽을 수 있는지 검증한다. embed 디렉티브 자체는 변경하지 않는다.

#### R03 — 체크리스트 섹션 주입
WHEN `BuildReviewPrompt`가 체크리스트 파일을 성공적으로 로드하면, THE SYSTEM SHALL 프롬프트의 `### Existing Code Context` 이후, `buildVerifyInstructions` 또는 `buildDiscoverInstructions` 호출 이전 지점에 `## Quality Checklist` 헤더와 체크리스트 원문을 삽입한다. 체크리스트 원문은 `DocContextMaxLines`(기본 200)로 trim하되 `trimToLines` 기존 헬퍼를 재사용한다.

#### R04 — 로딩 실패 동작 결정
WHEN 체크리스트 파일 로딩이 실패하면, THE SYSTEM SHALL soft-fallback 동작을 수행한다. 구체적으로 표준 에러에 `경고: 체크리스트 로드 실패 ({path}): {err}`를 출력하고 체크리스트 섹션 없이 프롬프트를 반환하며, 리뷰 파이프라인은 중단하지 않는다. fail-fast 대신 soft-fallback을 선택한 근거는 research.md의 "로딩 실패 처리 결정"을 참조한다.

#### R05 — 리뷰 응답 포맷 확장
WHEN `buildVerifyInstructions` 또는 `buildDiscoverInstructions`가 지시문을 작성할 때 체크리스트 섹션이 프롬프트에 포함되어 있으면, THE SYSTEM SHALL 리뷰어에게 각 체크 항목에 대해 다음 형식으로 응답하도록 요구한다: `CHECKLIST: <항목 ID> | PASS` 또는 `CHECKLIST: <항목 ID> | FAIL | <reason>`. 응답 형식 예시는 기존 `writeFindingExamples`와 별도의 `writeChecklistExamples` 함수로 분리한다.

#### R06 — 체크리스트 응답 파싱
WHEN `ParseVerdict`가 provider 응답을 파싱할 때, THE SYSTEM SHALL `CHECKLIST: ...` 라인을 정규식 `reChecklist = regexp.MustCompile(\`(?i)CHECKLIST:\s*([A-Z0-9-]+)\s*\|\s*(PASS|FAIL)(?:\s*\|\s*(.+))?\`)`로 매칭하고, 신규 타입 `ChecklistOutcome{ID, Status, Reason, Provider, Revision}`으로 구조화하여 `ReviewResult.ChecklistOutcomes` 필드에 누적한다.

#### R07 — ReviewResult 타입 확장
WHEN `pkg/spec/types.go`가 본 SPEC 구현 시 갱신되면, THE SYSTEM SHALL `ReviewResult` 구조체에 `ChecklistOutcomes []ChecklistOutcome` 필드를 추가하고, `ChecklistOutcome` 타입 및 `ChecklistStatus` enum(`"PASS" | "FAIL"`)을 정의한다. 기존 `ReviewFinding` 구조체는 수정하지 않는다.

#### R08 — CLI 출력 통합
WHEN `runSpecReview`가 최종 결과를 출력할 때 `ChecklistOutcomes`가 비어있지 않으면, THE SYSTEM SHALL `체크리스트 결과: {total}건 (PASS: {pass}, FAIL: {fail})` 요약 라인을 기존 `발견 사항` 라인 바로 뒤에 출력하고, FAIL 항목은 각각 `- [FAIL] {ID}: {reason}` 형식으로 열거한다.

### P1 — Should Have

#### R09 — Priority enum 유지
WHEN 본 SPEC이 구현될 때, THE SYSTEM SHALL `pkg/spec/types.go`의 `Criterion.Priority` 문자열 필드와 `pkg/spec/gherkin_parser.go`의 `rePriority` 정규식을 변경하지 않는다. 현재 `Must / Should / Nice` 어휘를 SPEC-SPECWR-001의 체크리스트 항목도 동일하게 사용하므로 enum 확장 없이 정렬을 유지한다. 결정 근거는 research.md "Priority enum 결정"에 기록한다.

#### R10 — self-verification 로그 헬퍼
WHEN spec-writer가 자체 검증 결과를 기록해야 할 때, THE SYSTEM SHALL `pkg/spec/selfverify.go`(신규)에 `AppendSelfVerifyEntry(specDir string, entry SelfVerifyEntry) error` 함수를 제공한다. 이 함수는 `.autopus/specs/SPEC-{ID}/.self-verify.log`에 JSON Lines 한 줄을 append하고, 파일이 100 라인을 초과하면 오래된 엔트리를 제거하여 총 라인 수를 100 이하로 유지한다.

#### R11 — CLI 서브커맨드로 래핑
WHEN R10의 Go 헬퍼가 구현되면, THE SYSTEM SHALL `auto spec self-verify --record <SPEC-ID> --dimension <dim> --status <PASS|FAIL> --reason <text>` CLI 서브커맨드를 추가하여 spec-writer 에이전트가 Bash 도구로 호출할 수 있도록 한다. 에이전트가 로그 파일을 직접 편집하지 않는다.

#### R12 — gitignore 엔트리 추가
WHEN 본 SPEC이 구현되면, THE SYSTEM SHALL 저장소 루트의 `.gitignore`에 `**/.autopus/specs/**/.self-verify.log` 패턴을 추가하여 로그 파일이 커밋되지 않도록 한다.

### P2 — Could Have

#### R13 — 체크리스트 결과 persist
WHEN `ChecklistOutcomes`가 파싱되면, THE SYSTEM SHALL `review.md` 출력 섹션에 `## Checklist Outcomes` 표(ID | Status | Reason | Provider)를 추가로 기록할 수 있다. 이 기능은 P2이며 본 SPEC 범위에서 구현 여부는 reviewer 판단으로 결정한다.

#### R14 — 체크리스트 토큰 예산 관리
WHEN 체크리스트 원문이 `DocContextMaxLines * 4` 문자 수를 초과하면, THE SYSTEM SHALL 경고 로그를 출력하고 trim을 적용한다. 이는 프롬프트 토큰 폭증 방지용 안전장치이다.

## 생성/수정 파일 상세

기존 파일 수정:
- `autopus-adk/pkg/spec/prompt.go` — `BuildReviewPrompt`에 체크리스트 주입 로직 추가, `writeChecklistExamples` 분리.
- `autopus-adk/pkg/spec/types.go` — `ChecklistOutcome`, `ChecklistStatus` 타입 추가, `ReviewResult.ChecklistOutcomes` 필드 추가.
- `autopus-adk/pkg/spec/reviewer.go` — `ParseVerdict`에서 `CHECKLIST:` 라인 파싱 추가.
- `autopus-adk/internal/cli/spec_review.go` — 최종 출력에 체크리스트 결과 요약 렌더링.
- `autopus-adk/internal/cli/spec.go` — `newSpecSelfVerifyCmd` 등록 (R11).
- `.gitignore` (저장소 루트) — `.self-verify.log` 제외 패턴 추가.

embed 변경 없음:
- `autopus-adk/content/embed.go` — 기존 `rules/*.md` 평탄 패턴이 이미 `spec-quality.md`를 자동 포함하므로 수정 불필요. R02에서 회귀 테스트로만 확인한다.

신규 파일:
- `autopus-adk/pkg/spec/checklist.go` [NEW] — 체크리스트 파일 로더, 주입 헬퍼.
- `autopus-adk/pkg/spec/selfverify.go` [NEW] — `AppendSelfVerifyEntry`, retention 로직.
- `autopus-adk/internal/cli/spec_self_verify.go` [NEW] — `auto spec self-verify` 서브커맨드.
- `autopus-adk/pkg/spec/checklist_test.go` [NEW] — 로더/주입 테스트.
- `autopus-adk/pkg/spec/reviewer_checklist_test.go` [NEW] — `CHECKLIST:` 파싱 회귀 테스트.
- `autopus-adk/pkg/spec/selfverify_test.go` [NEW] — append/retention 테스트.
- `autopus-adk/internal/cli/spec_self_verify_test.go` [NEW] — CLI self-verify 회귀 테스트.
- `autopus-adk/internal/cli/spec_review_checklist_test.go` [NEW] — CLI 체크리스트 요약 출력 회귀 테스트.

## 의존성 방향 요약

```
SPEC-SPECWR-001 (md 체크리스트 정의)
        │ outputs
        ▼
content/rules/spec-quality.md           ──── 입력 자산 ────┐
                                                          │
                                                          ▼
SPEC-SPECWR-002 (본 SPEC) ──── reads ──── pkg/spec/checklist.go
                                                          │
                                                          ▼
                                          BuildReviewPrompt → provider
```

SPEC-002는 SPEC-001을 hard dependency로 한다. SPEC-001이 구현되기 전이라도 SPEC-002의 Go 코드는 merge 가능하나, 파일 부재로 soft-fallback 경로만 활성화된다(체크리스트 미주입).

## Out of Scope

- 체크리스트 md 파일 본문 정의 (SPEC-001 담당).
- Review provider(claude/codex/gemini) 오케스트라 엔진 내부 변경.
- `pkg/spec/validator.go`의 기존 정적 검증 재작성.
- 리뷰 전략(debate/consensus) 변경.
- 체크리스트 결과에 따른 판정 임계값 자동 조정(P3 후속 SPEC).
