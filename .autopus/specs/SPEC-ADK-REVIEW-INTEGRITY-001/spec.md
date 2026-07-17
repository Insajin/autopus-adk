# SPEC-ADK-REVIEW-INTEGRITY-001: Fail-closed review observation integrity

**Status**: completed
**Created**: 2026-07-17
**Domain**: ADK

## 목적

멀티-프로바이더 SPEC 리뷰 게이트가 문서를 전부 관측하지 못했거나 프로바이더 정족수를 채우지 못한 상태에서 무자격 PASS를 반환하고 `status: approved`까지 승격하는 문제를 봉합한다. 두 결함을 하나의 관측 무결성(observation integrity) 속성으로 함께 닫는다: (A) 보조문서 head-only 절단이 리뷰어 몰래 일어나 false-PASS를 만든다, (B) provider degraded(구성 3개 중 1개만 응답) PASS가 정상 PASS와 구분 없이 status 승격까지 이어진다.

증거: 루트 `SPEC-DESKTOP-DEVICE-SETUP-001`의 `review.md`는 `PASS (degraded — 1/3 providers responded)`인데도 승격되었고, 그 시점 `plan.md`(358) `research.md`(429) `acceptance.md`(404)는 모두 200줄 주입 캡을 초과했다. 즉 게이트는 부분 관측한 문서를 승인했다.

## Outcome Boundary

- User-visible outcome: `auto spec review`가 (1) 문서를 전부 보지 못한 리뷰와 (2) 프로바이더 정족수 미달 리뷰를 무자격 PASS로 승격하지 않는다. verdict/`review.md`/`review-findings.json`에 관측 커버리지와 degraded 사유가 명시되고, `status: approved` 승격은 완전 관측 + 정족수 충족 또는 명시적 override에서만 일어난다.
- Mandatory requirements: 주입 커버리지 계측, fail-closed 절단, 구조 보존 주입, 프로바이더 정족수 정책, override 경로, authoring guard, 플랫폼 parity.
- Explicit non-goals: debate/consensus 전략 의미 변경, 프로바이더 추가/제거, orchestra 엔진 재작업, `review-findings.json` 파괴적 스키마 변경, 기존 526개 SPEC 재리뷰.
- Completion evidence: 캡 초과 fixture로 (a) 무자격 PASS 불가 (b) 커버리지 메타데이터 기록 (c) degraded 시 승격 차단 (d) override 경로 동작을 보이는 Go 오라클 테스트, `validate --strict` 경고, `pkg/adapter` parity 테스트 green.

## Requirements

### Observation Coverage

**REQ-RINT-COV-01**
Priority: Must
Type: Ubiquitous
THE SYSTEM SHALL record, per injected auxiliary document (`plan.md`, `research.md`, `acceptance.md`), the injected line count, the source total line count, an integer coverage percent, and a complete flag, SHALL render them into `review.md`, and SHALL persist them as an additive optional `DocCoverages` field inside the existing `review-findings.json` sidecar so prior sidecars without the field still load with empty coverage.

**REQ-RINT-FULL-02**
Priority: Must
Type: Ubiquitous
THE SYSTEM SHALL inject every required auxiliary document in full by default under an adaptive total context budget sized so typical SPEC document sets are fully injected at 100 percent coverage, and SHALL apply structure-preserving compression only as a fallback when that total budget is exceeded.

**REQ-RINT-STRUCT-03**
Priority: Must
Type: State-Driven
WHERE an auxiliary document exceeds the total context budget and must be compacted, THEN THE SYSTEM SHALL preserve tail-critical sections (Self-Verify Summary, Traceability Matrix, Reviewer Brief, Completion Debt, Evolution Ideas, Open Issues) instead of discarding the document tail.

### Fail-closed Gating

**REQ-RINT-TRUNC-04**
Priority: Must
Type: Unwanted
IF a required auxiliary document is injected below 100 percent coverage because the document set exceeded the total context budget, THEN THE SYSTEM SHALL annotate the verdict with the degraded reason `partial_doc_context` and SHALL NOT auto-promote the SPEC status to `approved` without an explicit override, so truncation becomes a visible exceptional path rather than the silent default.

**REQ-RINT-QUORUM-05**
Priority: Must
Type: Unwanted
IF the number of providers that returned a usable review is below the configured minimum quorum, THEN THE SYSTEM SHALL annotate the verdict with the degraded reason `provider_quorum` and SHALL NOT auto-promote the SPEC status to `approved` without an explicit override.

**REQ-RINT-PROMO-06**
Priority: Must
Type: Event-Driven
WHEN a review completes with a PASS verdict and no active blocking findings, THEN THE SYSTEM SHALL promote the status to `approved` only when every required auxiliary document is fully observed and the provider quorum is met, otherwise THE SYSTEM SHALL leave the prior status unchanged.

**REQ-RINT-OVERRIDE-07**
Priority: Must
Type: Event-Driven
WHEN the operator passes `--allow-degraded`, THEN THE SYSTEM SHALL permit promotion of a degraded-observation PASS and SHALL record the override reason in `review.md` as an audit line.

### Authoring And Parity

**REQ-RINT-AUTHOR-08**
Priority: Should
Type: Event-Driven
WHEN `auto spec validate <dir> --strict` runs and `plan.md`, `research.md`, or `acceptance.md` exceeds the review injection line cap, THEN THE SYSTEM SHALL emit a warning-level finding that names the file, its line count, and the cap, and SHALL exit zero so authors see the risk before review.

**REQ-RINT-PARITY-09**
Priority: Must
Type: Ubiquitous
THE SYSTEM SHALL author every reviewer-facing guidance change in the `content/` source of truth so it renders to claude-code, codex, antigravity-cli, and opencode surfaces with `pkg/adapter` parity tests green, while the enforcement logic itself stays platform-neutral in Go runtime.

**REQ-RINT-COMPAT-10**
Priority: Must
Type: Unwanted
IF a `review-findings.json` written by the prior schema is loaded, THEN THE SYSTEM SHALL parse it without error, treating new coverage fields as additive and optional.

## 생성 파일 상세

- `[NEW] pkg/spec/doc_coverage.go`: `DocCoverage` 타입, 커버리지 계산, `## Observation Coverage` 렌더.
- `[NEW] pkg/spec/doc_compaction.go`: 섹션 인지 구조 보존 압축 (기존 `trimToLines` head-only 대체).
- `pkg/spec/prompt.go`: `injectAuxDocs`가 커버리지 레코드를 반환하도록 확장, 압축 경로 연결.
- `pkg/spec/types.go`: `ReviewResult`에 `DocCoverages`와 degraded 사유 필드 추가.
- `pkg/spec/review_persist.go`: `review.md`에 커버리지/사유/override 감사 라인 렌더.
- `pkg/spec/provider_health.go`: 정족수 helper (`MeetsProviderQuorum`, `DefaultMinProviders`).
- `pkg/spec/quality_preflight.go`: 캡 초과 warning (REQ-RINT-AUTHOR-08).
- `pkg/config/schema_spec.go` / `pkg/config/defaults.go`: `min_providers` 필드.
- `internal/cli/spec_review.go`: `--allow-degraded` 플래그.
- `internal/cli/spec_review_loop.go`: 커버리지·정족수 집계, degraded 사유 세팅.
- `internal/cli/spec_review_runtime.go`: `syncReviewedSpecStatus`에 관측 무결성 게이트.
- `content/skills/spec-review.md`, `content/rules/spec-quality.md`: 새 fail-closed 의미 기술 (SoT → 4 플랫폼).

## Related SPECs

None. Primary SPEC가 Outcome Lock을 단독으로 닫는다. `SPEC-SPECREV-001`(provider health, degraded label)과 `SPEC-REVCONV-001`(수렴) 위에 관측 무결성 게이트를 추가하며 그 계약을 파괴하지 않는다.

## Traceability Matrix

| Requirement | Plan Task | Acceptance Scenario | Semantic Invariant |
|-------------|-----------|---------------------|--------------------|
| REQ-RINT-COV-01 | T1, T2 | AC-RINT-COV-1 | INV-001 |
| REQ-RINT-FULL-02 | T2 | AC-RINT-COV-1 | INV-002 |
| REQ-RINT-STRUCT-03 | T3 | AC-RINT-STRUCT-3 | INV-003 |
| REQ-RINT-TRUNC-04 | T4, T6 | AC-RINT-TRUNC-2 | INV-004 |
| REQ-RINT-QUORUM-05 | T5, T6 | AC-RINT-QUORUM-4 | INV-005 |
| REQ-RINT-PROMO-06 | T6 | AC-RINT-TRUNC-2, AC-RINT-QUORUM-4 | INV-006 |
| REQ-RINT-OVERRIDE-07 | T7 | AC-RINT-OVERRIDE-5 | INV-007 |
| REQ-RINT-AUTHOR-08 | T8 | AC-RINT-AUTHOR-6 | INV-008 |
| REQ-RINT-PARITY-09 | T9 | AC-RINT-PARITY-7 | INV-009 |
| REQ-RINT-COMPAT-10 | T1 | AC-RINT-COMPAT-8 | INV-010 |
