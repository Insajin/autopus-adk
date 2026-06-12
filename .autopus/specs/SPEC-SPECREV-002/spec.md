# SPEC-SPECREV-002: SPEC 리뷰 파이프라인 정합성 보강 (N/A 파서·--loop 배선·파서 견고성)

**Status**: completed
**Created**: 2026-06-12
**Domain**: SPECREV

## 목적

SPEC-SPECREV-001은 체크리스트 표면 전반에 `N/A` 상태를 도입했다. 그러나 감사 결과
`pkg/spec` 리뷰 경로의 일부 파서/검증 지점이 그 계약을 따라가지 못해 정보가 조용히
사라지거나, 글로벌 `--loop` 플래그가 어디에서도 소비되지 않아 동작에 영향을 주지 못하고,
파서 실패가 관측 신호 없이 묻히는 결함이 남아 있다. 본 문서는 file:line 증거로 재검증한
결함만을 닫아 리뷰 파이프라인의 관측 가능성과 정합성을 회복한다. 이는 기존 PASS/FAIL
경로와 기존 호출자 동작을 후방호환으로 보존하는 brownfield 변경이다.

## Outcome Boundary

- **User-visible outcome**: provider가 `N/A` 체크리스트를 출력하면 `pkg/spec` 경로에서도
  손실 없이 파싱·검증되고(빈 reason도 관측 가능), `--loop`가 spec review 반복 한도에
  실효하며, 파서 실패가 무신호로 사라지지 않는다.
- **Mandatory requirements (Primary SPEC)**: REQ-001 ~ REQ-007.
- **Explicit non-goals**:
  - 서킷 브레이커의 첫 반복 평가 변경 (재검증 결과 by-design, research.md `## Rejected Findings` 참조).
  - 불릿(`-`) 접두 요구사항 라인의 파싱 정책 변경 (기존 `ParseEARS` 필터 동작 보존).
  - PASS/FAIL 파싱·병합·verdict 산출 로직 변경.
  - 새로운 외부 의존성 추가 또는 manifest major 버전 변경.
- **Completion evidence**: REQ-001 ~ REQ-007이 모두 oracle 수락 시나리오(S1 ~ S13)로
  검증되고, 변경된 소스 파일이 300줄 하드리밋을 넘지 않으며, `auto spec validate --strict`가
  통과한다.

## Requirements

각 요구사항 문장은 `pkg/spec/parser.go::detectEARSType`의 실제 정규식으로 검증되어 선언된
EARS 타입으로 파싱됨을 확인했다 (research.md `## Revision 1 closure` 참조).

### REQ-001 (Event-driven / Priority: Must)

WHEN provider review 출력에 CHECKLIST 라인이 N/A 상태로 포함되면, THEN the system SHALL parseChecklistOutcomes로 이를 ChecklistStatusNA 값의 ChecklistOutcome으로 파싱하고 라인을 누락하지 않는다.

관측 지점: `spec.ParseVerdict(...).ChecklistOutcomes`가 N/A outcome을 포함하고 reason 텍스트가 보존된다 (S1, S2).

### REQ-002 (Unwanted behavior / Priority: Must)

IF self-verify 엔트리의 status가 ChecklistStatusNA이고 reason이 비어 있거나 공백뿐이면, THEN the system SHALL AppendSelfVerifyEntry에서 해당 엔트리를 거부하고 reason 누락을 알리는 오류를 반환한다.

관측 지점: `AppendSelfVerifyEntry`가 non-nil 오류(메시지에 `reason` 포함)를 반환하고 `.self-verify.log`가 기록되지 않는다 (S3, S4).

### REQ-003 (Event-driven / Priority: Must)

WHEN auto spec review가 글로벌 loop 플래그와 함께 실행되면, THEN the system SHALL resolveSpecReviewMaxRevisions로 유효 최대 리비전 수를 loopModeMinRevisions 이상으로 산출하여 리뷰 루프에 전달한다.

관측 지점: `resolveSpecReviewMaxRevisions(gate, LoopMode=true)`가 `max(설정 한도, loopModeMinRevisions)`를 반환하고, `runSpecReviewWithOptions`가 이 값을 `specReviewLoopParams.maxRevisions`로 전달한다 (S5, S6, S11).

### REQ-004 (State-driven / Priority: Should)

WHERE spec.md의 비-불릿 라인이 대문자 SHALL 토큰을 포함하지만 어떤 EARS 타입에도 매칭되지 않으면, THEN the system SHALL ParseEARSWithWarnings로 이를 감지하고 ValidateSpec에서 warning 레벨 ValidationError로 표면화한다.

관측 지점: `ValidateSpec(doc)`가 해당 라인을 명시하는 `Level == "warning"` `ValidationError`를 반환하고 `auto spec validate`가 stderr로 출력한다 (S7, S8, S12).

### REQ-005 (Unwanted behavior / Priority: Must)

IF auto spec review 실행 중 spec.Load가 실패하면, THEN the system SHALL 본문 비어있음을 단정하지 않는 정확한 메시지로 원인 오류를 래핑하여 반환한다.

관측 지점: load 실패 메시지가 래핑된 원인과 specID를 포함하고 `본문이 비어있습니다` 문구를 포함하지 않는다 (S9, S10).

### REQ-006 (Ubiquitous / Priority: Must)

the system SHALL REQ-001 부터 REQ-005 및 REQ-007 까지의 각 변경에 대해 구체적 입력과 구체적 기대값을 가진 oracle 테스트를 추가하여 회귀로부터 잠근다.

관측 지점: 신규/확장된 테스트가 S1 ~ S13의 기대값을 단언하고 `go test ./pkg/spec/... ./internal/cli/...`로 실행 가능하다.

### REQ-007 (Unwanted behavior / Priority: Must)

IF provider checklist 파싱 경로로 들어온 N/A outcome이 빈 reason을 가지면, THEN the system SHALL RenderChecklistSection에서 이를 전용 누락 마커로 표기하여 PASS 빈칸 placeholder와 구분한다.

관측 지점: `RenderChecklistSection`이 빈 reason N/A 행의 Reason 컬럼에 `-`(emptyNotePlaceholder)가 아닌 전용 누락 마커를 출력하고, Status 컬럼은 `N/A`를 유지한다 (S13).

## 생성·수정 파일 상세

- `pkg/spec/reviewer.go` (수정): `reChecklist` 정규식 status 그룹에 `N/A` 추가. `parseChecklistOutcomes`는 기존 `strings.ToUpper` 매핑으로 `ChecklistStatusNA`를 산출 (추가 변경 불필요, 재검증 완료). (REQ-001)
- `pkg/spec/selfverify.go` (수정): `AppendSelfVerifyEntry`에 N/A 빈 reason 거부 가드 추가. (REQ-002)
- `internal/cli/spec_review.go` (수정): `[NEW] loopModeMinRevisions` 상수, `[NEW] loopAwareMaxRevisions` 순수 함수, `[NEW] resolveSpecReviewMaxRevisions`(gate 한도+LoopMode 소비) 도입 후 `runSpecReviewWithOptions` 배선. `spec.Load` 실패 메시지를 `[NEW] wrapSpecLoadError`로 교체. (REQ-003, REQ-005)
- `pkg/spec/parser.go` (수정): `[NEW] ParseEARSWithWarnings` 추가, `ParseEARS`는 후방호환 wrapper로 유지. (REQ-004)
- `pkg/spec/validator.go` (수정): `ValidateSpec`가 `ParseEARSWithWarnings` 경고를 warning 레벨 `ValidationError`로 표면화. (REQ-004)
- `pkg/spec/checklist_render.go` (수정): `RenderChecklistSection`이 빈 reason N/A를 `[NEW] naMissingReasonNote` 마커로 표기. (REQ-007)
- 테스트 (수정/`[NEW]`): `pkg/spec/reviewer_checklist_test.go`(N/A 파싱), `[NEW] pkg/spec/selfverify_na_test.go`(빈 reason 거부), `[NEW] internal/cli/spec_review_revisions_test.go`(loopAwareMaxRevisions + resolveSpecReviewMaxRevisions + wrapSpecLoadError), `pkg/spec/parser_test.go`(ParseEARSWithWarnings), `pkg/spec/validator_test.go`(ValidateSpec 경고), `pkg/spec/checklist_render_test.go`(N/A 마커). (REQ-006)

## Related SPECs

- 선행: SPEC-SPECREV-001 (N/A 표면 통일·context limit·verdict denom). 본 문서는 그 N/A 계약의
  미반영 지점을 닫는 후속 정합성 보강이다.
- Sibling SPEC: None. Primary SPEC 단독으로 Outcome Lock을 닫는다 (research.md `## Sibling SPEC Decision` 참조).

## Traceability Matrix

| Requirement | Plan Task | Acceptance Scenario | Semantic Invariant |
|-------------|-----------|---------------------|--------------------|
| REQ-001 | T1 | S1, S2 | INV-001 |
| REQ-002 | T2 | S3, S4 | INV-002 |
| REQ-003 | T3 | S5, S6, S11 | INV-003 |
| REQ-004 | T4 | S7, S8, S12 | INV-004 |
| REQ-005 | T5 | S9, S10 | INV-005 |
| REQ-006 | T6 | S1, S3, S5, S7, S9, S13 | INV-001, INV-002, INV-003, INV-004, INV-005, INV-006 |
| REQ-007 | T7 | S13 | INV-006 |

## Completion Verdict

- Outcome Lock: satisfied — provider N/A 체크리스트가 pkg/spec 경로에서 손실 없이 파싱·검증되고(`reChecklist` N/A + 빈 reason 거부 + render 마커), `--loop`가 spec review 반복 한도에 실효하며(`resolveSpecReviewMaxRevisions` floor 5), EARS 미인식 SHALL 라인과 Load 실패가 무신호로 사라지지 않는다.
- Mandatory requirements: 6/6 Must (REQ-001/002/003/005/006/007), Should 1/1 (REQ-004)
- Must acceptance: S1~S13 전부 oracle 테스트 green (`go test ./pkg/spec/... ./internal/cli/... -race`)
- Review: multi-provider debate PASS (45/46, Rev1 closure 후), Phase 4 reviewer APPROVE + security-auditor PASS
- Completion Debt: none
- Evolution Ideas: advisory로만 잔존 (S11 배선 단언 보강, resetUnsignedWarnOnce 테스트 파일 이동 등 LOW 4건은 research 참고)
