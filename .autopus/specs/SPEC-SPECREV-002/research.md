# SPEC-SPECREV-002 리서치

## 기존 코드 분석

모든 결함은 SPEC 작성 전 file:line으로 재검증했다. 검증 도구: `rg`, `Read`, 그리고 실제
`spec.ParseEARS`를 호출하는 임시 테스트(ground truth).

- `pkg/spec/reviewer.go:14` — `reChecklist` status alternation이 `PASS|FAIL`뿐. `parseChecklistOutcomes`
  (reviewer.go:169-196)는 line 188에서 `ChecklistStatus(strings.ToUpper(...))`로 매핑하므로 정규식이
  N/A를 캡처하면 `ChecklistStatusNA`로 정확히 매핑된다. 누락 지점은 정규식 한 곳.
- `pkg/orchestra/output_parser.go:60-66` — orchestra `ParseReviewer`는 이미 `"N/A"`를 수용. 비대칭이 핵심.
- `pkg/spec/types.go:118-125` — `ChecklistStatusPass/Fail/NA` 상수 존재(`ChecklistStatusNA = "N/A"`).
- `pkg/spec/checklist_render.go:33-51` — `RenderChecklistSection`은 빈 reason을 `emptyNotePlaceholder`
  (`-`)로 렌더한다. N/A 빈 reason도 PASS 빈칸과 동일하게 `-`로 표시되어 구분 불가 → REQ-007 대상.
- `pkg/spec/selfverify.go:31-37` — `AppendSelfVerifyEntry`는 N/A status는 수용하나 reason 비어있음을
  검증하지 않는다. `Reason`은 `json:"reason,omitempty"`(line 23).
- `internal/cli/global_flags.go:18, 83` — `LoopMode bool` 정의·할당. non-test 소비처 0건(grep 확인). inert.
- `internal/cli/spec_review.go:94-97` — `maxRevisions`는 `gate.MaxRevisions`(없으면 `defaultMaxRevisions=3`).
  `flags := globalFlagsFromContext(ctx)`(line 79)로 LoopMode 접근 가능.
- `internal/cli/spec_review.go:68-77` — line 71이 모든 `spec.Load` 실패를 `SPEC 본문이 비어있습니다`로
  보고. `%w`로 원인은 보존되나 prefix가 빈 본문을 단정. line 64는 이미 `SPEC 로드 실패: %w` 사용.
- `pkg/spec/template.go:36-74` — `Load → parseSpecMd`. `parseSpecMd`는 `doc.ID == ""`이면 line 65에서
  에러 반환, 그 전 line 71에서 `doc.RawContent = content`. ID는 content의 `# SPEC-...` 헤더에서 파싱되므로
  content에 ID가 있으면 RawContent는 비어있지 않다. 따라서 **성공한 Load에서 `RawContent == ""`는 도달
  불가능**하며 spec_review.go:75-77 가드는 방어코드다 (Q-FEAS-03 근거).
- `pkg/spec/parser.go:34-37` — `detectEARSType(line) == ""`이면 `continue`로 무신호 skip. 라인 필터
  (line 30)는 빈 줄·`#`·`-` 접두를 먼저 제외하므로 skip 대상은 비-불릿·비-주석 라인이다.
- `pkg/spec/parser.go:14-19` — 실제 정규식: reOptional `WHEN.+IF.+THEN.+`, reEventDriven `WHEN.+THEN.+`,
  reStateDriven `WHERE.+THEN.+`, reUnwanted `IF.+THEN.+`, reUbiquitous `(시스템|system|The system) SHALL`.
  Revision 1에서 모든 REQ 문장을 이 정규식으로 실측 검증함(아래 closure).
- `pkg/spec/validator.go:12-66` — `ValidateSpec`는 `[]ValidationError`(Field/Message/Level)를 반환하고
  `auto spec validate`(internal/cli/spec.go:74-86)가 warning 레벨을 stderr로 출력. EARS 경고 소비처.
- 호출자 그래프: `ParseEARS`는 `pkg/spec/template.go:69`와 `parser_test.go`만 호출 → 시그니처 보존 필요.

## Outcome Lock

- **User-visible outcome**: provider가 N/A 체크리스트를 출력하면 `pkg/spec` 경로에서도 손실 없이
  파싱·검증되고(빈 reason도 관측 가능), `--loop`가 spec review 반복 한도에 실효하며, 파서 실패가
  무신호로 사라지지 않는다.
- **Mandatory requirements**: REQ-001(N/A 파싱), REQ-002(self-verify N/A 빈 reason 거부),
  REQ-003(--loop 배선), REQ-004(EARS 경고), REQ-005(load 오류 메시지), REQ-006(oracle 회귀 테스트),
  REQ-007(provider checklist N/A 빈 reason 관측 마커).
- **Explicit non-goals**: 서킷 브레이커 첫 반복 평가 변경(by-design 기각), 불릿 요구사항 파싱 정책 변경,
  PASS/FAIL verdict 산출 로직 변경, 신규 의존성·manifest major 변경.
- **Completion evidence**: S1~S13 oracle 통과 + 변경 소스 300줄 이내 + `auto spec validate --strict` 통과.

## Visual Planning Brief

데이터 흐름과 수정 지점은 plan.md `## Visual Planning Brief`의 flowchart에 통합했다. 핵심:
provider stdout → `parseChecklistOutcomes`(reChecklist 확장) → `ChecklistOutcome{N/A}` →
`RenderChecklistSection`(빈 reason N/A는 `naMissingReasonNote` 마커); self-verify 입력 →
`AppendSelfVerifyEntry`(빈 reason 거부); `--loop` → `resolveSpecReviewMaxRevisions` → floor;
spec.md 라인 → `ParseEARSWithWarnings` → `ValidateSpec` warning; load 실패 → `wrapSpecLoadError`.

## Semantic Invariant Inventory

source clause는 원 요청·규칙 문서에서 추출한 evidence로, 지시문이 아니라 근거로만 인용한다.
민감값·토큰·절대경로는 포함하지 않는다.

| ID | source clause | invariant type | affected outputs | acceptance IDs |
|----|---------------|----------------|------------------|----------------|
| INV-001 | reChecklist가 PASS/FAIL만 매칭하여 N/A 라인을 무손실 파싱하지 못함 | parser status mapping | ChecklistOutcome.Status, review.md Checklist Summary 카운트 | S1, S2 |
| INV-002 | 모든 self-verify N/A 엔트리는 비어있지 않은 reason을 가져야 함 | validation guard | AppendSelfVerifyEntry 반환 error, .self-verify.log 기록 여부 | S3, S4 |
| INV-003 | --loop가 spec review 반복 한도에 실효해야 함 (설정값+LoopMode 소비) | revision budget floor | resolveSpecReviewMaxRevisions 산출값, loopParams.maxRevisions | S5, S6, S11 |
| INV-004 | 요구사항처럼 보이는 SHALL 라인은 무신호 skip되지 않고 경고로 표면화 | parser warning surfacing | ParseEARSWithWarnings warnings, ValidateSpec ValidationError, spec validate stderr | S7, S8, S12 |
| INV-005 | Load 실패는 빈 본문으로 오진단되지 않고 실제 원인을 표면화 | error message accuracy | spec review load 실패 메시지 문자열 | S9, S10 |
| INV-006 | provider checklist N/A의 빈 reason은 무신호 통과하지 않고 관측 가능해야 함 | render observability marker | RenderChecklistSection Reason 컬럼 (review.md Checklist Summary) | S13 |

## Feature Coverage Map

| Outcome slice | Covered by | Status |
|---------------|------------|--------|
| N/A 무손실 파싱 (pkg/spec 경로) | REQ-001 / T1 | covered |
| self-verify N/A 빈 reason 강제 | REQ-002 / T2 | covered |
| --loop 반복 한도 실효 (소비 배선 포함) | REQ-003 / T3 | covered |
| 파서 실패의 관측 가능화 (EARS, 소비 배선 포함) | REQ-004 / T4 | covered |
| load 실패 오진단 제거 | REQ-005 / T5 | covered |
| 회귀 잠금 (oracle 테스트) | REQ-006 / T6 | covered |
| provider checklist N/A 빈 reason 관측 | REQ-007 / T7 | covered |
| 서킷 브레이커 첫 반복 평가 | by-design (기각) | non-goal |

## 설계 결정

1. **N/A 강제·관측의 2개 경로 분리**: self-verify 게이트(`AppendSelfVerifyEntry`)는 빈 reason N/A를
   hard reject한다(REQ-002). provider 파싱 경로는 review 중간에 데이터를 버릴 수 없으므로 무손실
   보존하되 `RenderChecklistSection`이 전용 마커(`naMissingReasonNote`)로 관측 가능하게 한다(REQ-007).
   이로써 Outcome Lock의 "provider N/A가 손실 없이 파싱·검증된다"가 양쪽 경로에서 닫힌다.
2. **--loop = floor 의미론, 값 5, 소비는 resolveSpecReviewMaxRevisions**: `defaultMaxRevisions`(3)보다
   큰 `loopModeMinRevisions`(5)를 floor로 써 기본 설정에서도 효과가 관측된다. 리프 함수
   `loopAwareMaxRevisions`와 별도로, gate.MaxRevisions 폴백까지 포함한 `resolveSpecReviewMaxRevisions`를
   `runSpecReviewWithOptions`가 호출하게 해 소비 배선을 oracle(S11)로 고정한다. 서킷 브레이커가 진행
   없음 시 여전히 조기 종료하므로 ceiling 상향은 안전하다.
3. **EARS 경고 = uppercase SHALL 토큰 한정, 소비는 ValidateSpec**: `(?i)`로 lowercase까지 잡으면 산문
   오탐이 늘어난다. 대문자 `SHALL`만 트리거. 경고는 `ValidateSpec`가 warning `ValidationError`로
   표면화하고 기존 `auto spec validate` stderr 경로로 흘러 inert seam이 아니다(S12).
4. **load 메시지 = 중립 prefix + %w**: line 64와 일관된 `SPEC 로드 실패` 계열 prefix로 교체하고 원인은
   `%w`로 보존. 성공 Load의 `RawContent == ""`는 도달 불가 방어코드이므로(기존 코드 분석 참조) acceptance는
   도달 가능한 실패 경로(SPEC ID 헤더 부재)를 사용한다(S10).
5. **EARS 문장의 파서 정합**: 모든 REQ 문장을 실제 `detectEARSType`로 검증해 선언 타입과 일치시켰다
   (closure 참조). Ubiquitous substring 의존을 제거했다.
6. **순수 함수 우선**: `loopAwareMaxRevisions`, `resolveSpecReviewMaxRevisions`, `ParseEARSWithWarnings`,
   `wrapSpecLoadError`를 순수 함수로 추출해 oracle 테스트가 I/O 없이 결정적으로 단언하게 한다.

## Rejected Findings

| Finding | 재검증 결과 | 사유 |
|---------|-------------|------|
| 서킷 브레이커 첫 반복 미평가 (spec_review_loop.go:208 `revision > 0` 가드) | by-design, 기각 | discover 모드에서 revision 0의 `priorFindings`는 비어 있다. `ShouldTripCircuitBreaker(empty, curr)`(merge.go:62-72)는 `currCount >= prevCount`를 반환하는데 prevCount=0이라 항상 true가 되어, 첫 발견 직후 루프가 무조건 조기 종료된다. 따라서 `revision > 0` 가드는 spurious trip을 막는 올바른 설계다. 보강 불필요. |

finding 6(오진단 에러)은 부분 유효로 채택: `%w`로 원인이 이미 보존되므로 "원인 완전 소실"은
아니지만, 하드코딩 prefix가 빈 본문을 단정해 오진단한다. REQ-005로 prefix만 정정한다.

## Completion Debt

| Item | Blocks | Required resolution |
|------|--------|---------------------|
| None | - | - |

Primary SPEC의 REQ-001~REQ-007이 Outcome Lock을 전부 닫는다. 보안·데이터 무결성·필수 workflow를
막는 누락 작업 없음.

## Evolution Ideas

아래는 Outcome Lock 밖의 선택적 개선이며 sync completion을 막지 않는다. ID·task·후속 문서를
자동 생성하지 않는다.

| Idea | Why not required now | Promotion trigger |
|------|----------------------|-------------------|
| 불릿(`-`) 접두 요구사항 라인도 EARS 파싱 대상에 포함 | 현재 `ParseEARS` 필터는 불릿을 제외하며 본 결함 범위 밖. 동작 변경 시 다수 기존 문서 파싱 결과가 바뀜 | 사용자가 불릿 요구사항 지원을 명시 요청할 때 |
| EARS 경고에 RFC-2119 `MUST` 토큰도 포함 | EARS 표준은 `SHALL`만 사용. 범위 확장은 오탐 위험 | 작성자 피드백으로 누락이 반복 확인될 때 |
| provider 파싱 경로의 빈 reason N/A를 hard reject로 승격 | review 중간 데이터 손실 위험이 있어 현재는 관측 마커로만 처리 | 운영에서 빈 reason N/A 남용이 실제 보고될 때 |

## Sibling SPEC Decision

| Decision | Reason | Sibling SPEC IDs |
|----------|--------|------------------|
| none | Primary SPEC가 단일 모듈 내 7개 요구사항으로 Outcome Lock을 닫는다 | None |

## Reference Discipline

| Reference | Type | Verification |
|-----------|------|--------------|
| pkg/spec/reviewer.go reChecklist, parseChecklistOutcomes | existing | rg/Read로 line 14, 169-196 확인 |
| pkg/orchestra/output_parser.go ParseReviewer N/A 수용 | existing | Read line 60-66 확인 |
| pkg/spec/types.go ChecklistStatusNA | existing | rg line 125 확인 |
| pkg/spec/checklist_render.go RenderChecklistSection, emptyNotePlaceholder | existing | Read line 33-51 + provider_health.go:28 확인 |
| pkg/spec/selfverify.go AppendSelfVerifyEntry | existing | Read line 27-63 확인 |
| internal/cli/global_flags.go LoopMode | existing | rg line 18, 83 + non-test 소비처 0 확인 |
| internal/cli/spec_review.go line 71, 75-77, 94-97 | existing | Read 확인 |
| pkg/spec/parser.go ParseEARS, detectEARSType, 정규식 | existing | Read line 14-69 + 실제 ParseEARS 호출 검증 |
| pkg/spec/template.go Load, parseSpecMd (RawContent 도달성) | existing | Read line 36-74 확인 |
| pkg/spec/validator.go ValidateSpec | existing | Read line 12-66 확인 |
| merge.go ShouldTripCircuitBreaker | existing | Read line 62-72 확인 |
| loopAwareMaxRevisions, loopModeMinRevisions, resolveSpecReviewMaxRevisions | [NEW] planned addition | internal/cli/spec_review.go 신규 |
| ParseEARSWithWarnings | [NEW] planned addition | pkg/spec/parser.go 신규 |
| wrapSpecLoadError | [NEW] planned addition | internal/cli/spec_review.go 신규 |
| naMissingReasonNote | [NEW] planned addition | pkg/spec/checklist_render.go 신규 상수 |
| selfverify_na_test.go, spec_review_revisions_test.go, checklist_render_test.go | [NEW] planned addition | 신규 테스트 파일 |

## Reviewer Brief

- **Intended scope**: SPEC-SPECREV-001의 N/A 계약 미반영 지점 + inert `--loop` + 파서 무신호 skip을
  닫는 brownfield 정합성 보강. 단일 모듈(autopus-adk), `pkg/spec` + `internal/cli`.
- **Explicit non-goals**: 서킷 브레이커 첫 반복 평가, 불릿 요구사항 파싱, PASS/FAIL verdict 로직,
  신규 의존성. 리뷰어는 이들로 scope를 확장하지 말 것.
- **Self-verified**: 모든 REQ 문장을 실제 `detectEARSType`로 파싱 검증(선언 타입 일치),
  Traceability Matrix(REQ↔Task↔S↔INV), Semantic Invariant(INV-001~006), 리프+소비 배선 oracle
  (S5/S7 리프, S11/S12/S13 배선), S10의 도달 가능한 실패 경로, existing/[NEW] reference discipline,
  서킷 브레이커 기각 근거.
- **Reviewer should focus on**: correctness(N/A 매핑·메시지 정확성·EARS 타입 일치), convergence safety
  (floor 상향이 서킷 브레이커와 충돌하지 않음), regression risk(PASS/FAIL·기존 호출자 후방호환),
  Completion Debt 유무.

## Self-Verify Summary

- Q-CORR-01 | status: PASS | attempt: 1 | files: research.md, spec.md | reason: 모든 기존 참조를 rg/Read로 file:line 확인했다.
- Q-CORR-03 | status: FAIL | attempt: 1 | files: spec.md | reason: REQ 문장이 선언 타입과 달리 Ubiquitous substring으로 파싱되었다(codex 지적).
- Q-CORR-03 | status: PASS | attempt: 2 | files: spec.md | reason: 7개 REQ 문장을 실제 detectEARSType로 검증해 선언 타입(Event/Unwanted/State/Ubiquitous)과 일치시켰다.
- Q-CORR-04 | status: PASS | attempt: 1 | files: research.md | reason: existing 참조와 [NEW] planned addition을 Reference Discipline에서 분리했다.
- Q-COMP-01 | status: PASS | attempt: 1 | files: spec.md, plan.md, acceptance.md, research.md | reason: 4개 문서가 목적·역할을 갖고 상호 보완한다.
- Q-COMP-02 | status: FAIL | attempt: 1 | files: acceptance.md | reason: REQ-003/004가 리프 헬퍼만 검증하고 소비 배선을 oracle로 고정하지 않았다(codex 지적).
- Q-COMP-02 | status: PASS | attempt: 2 | files: spec.md, plan.md, acceptance.md | reason: S11(resolveSpecReviewMaxRevisions)·S12(ValidateSpec)·S13(RenderChecklistSection) 배선 oracle을 추가하고 Traceability/T6/T7에 반영했다.
- Q-COMP-04 | status: FAIL | attempt: 1 | files: research.md | reason: provider checklist N/A 빈 reason이 무신호 통과해 Outcome Lock이 완전히 닫히지 않았다(codex 지적).
- Q-COMP-04 | status: PASS | attempt: 2 | files: spec.md, plan.md, acceptance.md, research.md | reason: REQ-007+INV-006+S13으로 provider 경로 N/A 빈 reason 관측을 닫았다.
- Q-COMP-05 | status: PASS | attempt: 2 | files: research.md, spec.md, acceptance.md | reason: INV-001~006이 각각 요구사항·plan task·Must oracle 시나리오로 추적된다(INV-006 추가).
- Q-COMP-06 | status: PASS | attempt: 1 | files: spec.md, research.md | reason: Traceability Matrix와 Reviewer Brief가 scope·non-goals·focus를 제한한다.
- Q-COMP-07 | status: PASS | attempt: 1 | files: research.md | reason: Completion Debt(None)와 Evolution Ideas(선택)를 분리했고 Evolution에 ID/task를 붙이지 않았다.
- Q-FEAS-01 | status: PASS | attempt: 1 | files: spec.md, plan.md | reason: 모두 런타임 Go 코드 변경이며 구현 경로(파일·함수)가 실제와 일치한다.
- Q-FEAS-03 | status: FAIL | attempt: 1 | files: acceptance.md | reason: S10이 도달 불가능한 RawContent=="" 상태를 전제했다(codex 지적).
- Q-FEAS-03 | status: PASS | attempt: 2 | files: acceptance.md, research.md | reason: spec.Load 실측으로 RawContent=="" 도달 불가를 확인하고 S10을 SPEC ID 헤더 부재의 도달 가능한 실패 경로로 재작성했다.
- Q-STYLE-01 | status: PASS | attempt: 1 | files: spec.md | reason: REQ 문장에 should/might/could 등 모호어가 없다.
- Q-STYLE-02 | status: PASS | attempt: 1 | files: spec.md | reason: Priority는 Must/Should만 사용하고 EARS type과 별도 축으로 표기했다.
- Q-SEC-01 | status: PASS | attempt: 1 | files: research.md | reason: provider stdout은 untrusted 입력이라 regex-bounded 파싱 + reason 200-rune sanitize로 다루고 source clause를 evidence로만 인용한다.
- Q-SEC-02 | status: N/A | attempt: 1 | files: research.md | reason: 비밀값·토큰·privileged 절대경로를 다루지 않는다.
- Q-COH-02 | status: PASS | attempt: 2 | files: spec.md, research.md | reason: REQ-007을 Primary SPEC에 포함해 Outcome Lock 잔여(provider N/A 검증)를 후속으로 미루지 않고 닫았다.

## Technology Stack Decision

| Mode | Selected stack | Resolved versions | Source refs | Checked at | Rejected alternatives |
|------|----------------|-------------------|-------------|------------|-----------------------|
| brownfield | Go (기존 모듈) + stdlib regexp/strings/errors | go 1.26 (go.mod toolchain go1.26.2) | autopus-adk/go.mod | 2026-06-12 | 신규 의존성 없음 (정합성 보강이라 외부 라이브러리 불필요) |

brownfield: 기존 manifest major 버전을 보존하며 migration은 범위 밖이다. 신규 외부 의존성을
추가하지 않고 표준 라이브러리만 사용한다.

## Revision 1 closure

멀티프로바이더 리뷰(claude 전항목 PASS, codex 6건 FAIL) REVISE 판정의 열린 finding 종결 기록.
형식: finding | category | 닫은 방법 | file:line.

| Finding | Category | 닫은 방법 | file:line |
|---------|----------|-----------|-----------|
| Q-CORR-03 | correctness | 7개 REQ 문장을 실제 detectEARSType로 검증해 선언 타입과 일치(Event/Unwanted/State/Ubiquitous); Ubiquitous substring 의존 제거 | pkg/spec/parser.go:14-19 (기준), spec.md REQ-001~007 |
| Q-COMP-02 | completeness | --loop·EARS 경고의 소비 배선 oracle 추가(S11 resolveSpecReviewMaxRevisions, S12 ValidateSpec) + Traceability/T6 반영 | acceptance.md S11-S12, spec.md Traceability, plan.md T6 |
| Q-COMP-05 | completeness | INV-006 추가 및 INV-003/004 acceptance ID를 배선 oracle까지 확장(S5,S6,S11 / S7,S8,S12) | research.md Semantic Invariant Inventory |
| Q-COMP-04 | completeness | provider 경로 N/A 빈 reason 관측을 REQ-007로 신설(RenderChecklistSection 마커), Outcome Lock 완결 | spec.md REQ-007, plan.md T7 |
| Q-COH-02 | cohesion | REQ-007을 Primary SPEC에 포함해 Outcome Lock 잔여를 후속으로 미루지 않음 | spec.md Outcome Boundary, research.md Outcome Lock |
| Q-FEAS-03 | feasibility | spec.Load 실측으로 RawContent=="" 도달 불가 확인 후 S10을 SPEC ID 헤더 부재의 도달 가능 실패 경로로 재작성 | pkg/spec/template.go:36-74 (근거), acceptance.md S10 |
