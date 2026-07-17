# SPEC-ADK-REVIEW-INTEGRITY-001 리서치

## 기존 코드 분석

- `pkg/spec/prompt.go:118` `injectAuxDocs` → `prompt.go:149` `trimToLines`가 `lines[:maxLines]`(head)만 유지하고 tail을 버린 뒤 trim notice를 붙인다. 기본 캡은 `prompt.go:10` `defaultDocContextMaxLines = 200`, 설정은 `pkg/config/schema_spec.go:28` `DocContextMaxLines`, 기본값 `pkg/config/defaults.go:73`.
- 완전 주입 fail-closed 경로(`RequireCompleteDocuments`, `pkg/spec/prompt_documents.go`, `pkg/spec/prompt_context_delivery.go`)는 존재하나 `internal/cli/spec_review_context_scope.go:15` `requireCompleteGPTReviewDocuments`가 codex/openai/gpt-only로 게이트한다. 기본 `[claude, codex, gemini]` 혼합 세트는 절단 경로를 쓴다.
- 승격: `internal/cli/spec_review_runtime.go:29` `syncReviewedSpecStatus`는 `Verdict==PASS && !hasActiveFindings`면 `spec.UpdateStatus(specDir,"approved")`. `ProviderStatuses` degraded 상태를 읽지 않는다.
- degraded 계측은 이미 존재: `pkg/spec/provider_health.go` `BuildProviderStatuses`/`ShouldLabelDegraded`/`DegradedLabel`(label만), `review_persist.go:35` verdict 라인에 반영. 정족수 승격 게이트는 없다.
- 집계 지점: `internal/cli/spec_review_loop.go:118` `BuildProviderStatuses`, `:119` `failedCount`. 여기서 커버리지·정족수를 함께 계산할 수 있다.
- authoring guard: `pkg/spec/quality_preflight.go:11` `ValidateSpecSet`(strict 경로), `internal/cli/spec.go:66` `--strict`.

## Outcome Lock

- User-visible outcome: `auto spec review`가 부분 관측(문서 절단) 또는 정족수 미달 리뷰를 무자격 PASS로 승격하지 않고, 커버리지·degraded 사유를 아티팩트에 남기며, `approved` 승격은 완전 관측+정족수 충족 또는 명시 override에서만 일어난다.
- Mandatory requirements: REQ-RINT-COV-01, FULL-02, STRUCT-03, TRUNC-04, QUORUM-05, PROMO-06, OVERRIDE-07, AUTHOR-08, PARITY-09, COMPAT-10.
- Explicit non-goals: 전략 의미 변경, 프로바이더 add/remove, orchestra 재작업, 파괴적 스키마 변경, 기존 526 SPEC 재리뷰.
- Completion evidence: 캡 초과 fixture 오라클 테스트(무자격 PASS 불가·커버리지 기록·degraded 승격 차단·override 승격), `validate --strict` 경고, `pkg/adapter` parity green.

## Visual Planning Brief

승격 결정 data-flow (compact):

```
inject aux docs --(try full within token budget)--> coverage{injected,total,percent,complete}
                --(over budget)--> section-aware compaction (keep tail-critical)
providers -> successful_count -> quorum_ok = successful >= n/2+1
verdict PASS + no findings
  -> observed_ok = all(complete) ; quorum_ok
  -> promote iff observed_ok && quorum_ok, else block  (override bypasses with audit)
```

## 설계 결정

- 채택 모델(budget-default-full): 보조문서는 기본 전량 주입하되 adaptive TOTAL 예산(plan+research+acceptance 합산, char/token 기반) 하에 둔다. 예산은 일반 SPEC 세트(본 SPEC 62~133줄, DEVICE-SETUP 358/429/404줄 포함)가 100% 주입되도록 넉넉히 잡고, 구조 보존 압축은 총 예산 초과 시에만 발동하는 fallback이다. 무결성 판정은 캡 숫자가 아니라 실제 커버리지가 한다. 따라서 well-formed SPEC은 override 없이 approved에 도달하고 병리적 대형 문서만 degraded되며 AUTHOR-08 validate --strict가 사전 경고한다. 이는 `SPEC-DESKTOP-DEVICE-SETUP-001`의 head-only 손실을 반증한다.
- 정족수 기본값은 과반(`n/2+1`)으로 잡아 단일 프로바이더 로컬 리뷰(min=1)를 깨지 않으면서 3중 구성에서 1/3 승격을 차단한다.
- override(`--allow-degraded`)는 승격을 막지 않되 감사 라인으로 기록해 결정을 관측 가능하게 남긴다.
- 대안 검토: (a) 캡만 상향(라인 캡 유지) → 더 큰 문서에서 동일 head-loss 재현하므로 기각하고, `doc_context_max_lines`를 압축 fallback threshold로 재해석해 기존 config 하위호환을 유지하면서 budget-default-full 모델로 대체(승격 게이트가 절단을 조용히 두지 않고 가시화). (b) codex 전량 경로를 모든 프로바이더로 확장 → 토큰 초과 리스크, 압축+커버리지가 더 안전. (c) degraded면 무조건 REVISE → 로컬 단일 프로바이더 워크플로 파손, 기각.

## Technology Stack Decision

| Mode | Selected stack | Resolved versions | Source refs | Checked at | Rejected alternatives |
|------|----------------|-------------------|-------------|------------|-----------------------|
| brownfield | 기존 Go 모듈 + 표준 라이브러리만 | go 1.26 / toolchain go1.26.5 (기존 `go.mod` 제약 유지) | `autopus-adk/go.mod:3,5` | 2026-07-17 | 신규 의존성 (미도입, minimality) |

## Minimality Decision Matrix

| Ladder step | Evidence | Decision | Receipt item |
|-------------|----------|----------|--------------|
| actual need | Outcome Lock: 부분 관측/정족수 미달 무자격 PASS 차단 | proceed | 관측 무결성 게이트 |
| existing code/helper/pattern | `provider_health.go` degraded, `syncReviewedSpecStatus`, `ValidateSpecSet`, `injectAuxDocs` 확인 | reuse | 기존 구조 확장 |
| stdlib/native | `strings`/`bufio` 라인 스캔으로 커버리지·압축 충분 | use | stdlib only |
| existing dependency | orchestra/promptlayer 이미 present, 추가 불필요 | reuse | 신규 dep 0 |
| new dependency or new abstraction | 신규는 `DocCoverage` struct, `min_providers` 필드, `--allow-degraded` 플래그, 압축 함수뿐 — 기존 구조의 additive 확장 | accepted | 새 dep/추상화 미도입 |
| minimum sufficient verification | 커버리지 정수 오라클, 압축 구조 보존, 승격 게이트, override, validate 경고, 하위호환 JSON, adapter parity | required checks | 오라클 테스트 세트 |

## Semantic Invariant Inventory

| ID | source clause | invariant type | affected outputs | acceptance IDs |
|----|---------------|----------------|------------------|----------------|
| INV-001 | "per-doc injected/total line counts + coverage%" | numeric formula | review.md Observation Coverage row | AC-RINT-COV-1 |
| INV-002 | "우선 전량 주입 시도(토큰 예산 내)" | state transition | injected excerpt | AC-RINT-COV-1 |
| INV-003 | "tail-critical 섹션을 우선 보존" | ordering/selection | injected excerpt | AC-RINT-STRUCT-3 |
| INV-004 | "절단된 채 리뷰되면 unqualified PASS 금지" | gate/threshold | verdict reason, spec status | AC-RINT-TRUNC-2 |
| INV-005 | "응답 provider 수 < 구성 수이면 승격 차단" | gate/threshold | verdict reason, spec status | AC-RINT-QUORUM-4 |
| INV-006 | "완전 관측 + 정족수 충족에서만 approved" | compound gate | spec status field | AC-RINT-TRUNC-2, AC-RINT-QUORUM-4 |
| INV-007 | "--allow-degraded 명시 override" | audit record | review.md override line | AC-RINT-OVERRIDE-5 |
| INV-008 | "validate --strict가 캡 초과를 경고" | gate/threshold | validate stderr, exit code | AC-RINT-AUTHOR-6 |
| INV-009 | "content SoT → 4 표면 parity green" | parity | adapter parity percent | AC-RINT-PARITY-7 |
| INV-010 | "review-findings.json 하위호환 유지" | schema compat | LoadFindings result | AC-RINT-COMPAT-8 |

## Feature Coverage Map

| Outcome slice | Covered by | Status |
|---------------|------------|--------|
| 커버리지 계측 | Primary SPEC T1,T2 | covered |
| 전량 우선 + 구조 보존 압축 | Primary SPEC T2,T3 | covered |
| 절단 fail-closed 승격 차단 | Primary SPEC T4,T6 | covered |
| 정족수 승격 차단 | Primary SPEC T5,T6 | covered |
| override 감사 경로 | Primary SPEC T7 | covered |
| authoring 캡 경고 | Primary SPEC T8 | covered |
| 플랫폼 parity | Primary SPEC T9 | covered |
| 하위호환 스키마 | Primary SPEC T1 | covered |

## Completion Debt

| Item | Blocks | Required resolution |
|------|--------|---------------------|
| None | - | - |

## Evolution Ideas

These are optional improvements and do not block sync completion.

| Idea | Why not required now | Promotion trigger |
|------|----------------------|-------------------|
| 프로바이더별 문단 단위 semantic 압축(라인 수 대신 토큰) | Outcome Lock을 막지 않음; 라인 기반으로 충분 | User explicitly requests it |
| 커버리지 히스토리 시계열 리포트 | 단일 리뷰 무결성과 무관 | User explicitly requests it |
| aux doc 읽기 사이즈 프리체크(예: >32MB 즉시 partial 처리) | 상속 약점(구 trimToLines 시절과 동일), 로컬 신뢰모델서 자기-DoS 수준 (Phase4 sec M-1) | User explicitly requests it |
| 사이드카/review.md 원자적 rename 쓰기 | 부분 쓰기 시 다음 로드가 fail-closed라 보안 무해, 가용성만 영향 (Phase4 sec L-2) | User explicitly requests it |
| tail-critical 섹션이 잔여 예산 초과 시 동작 문서화 | coverage%가 정직히 partial 기록되어 게이트가 잡음 (Phase4 rev #1) | User explicitly requests it |
| applyObservationIntegrity 라운드 간 멱등성 주석 | 재계산 결과 동일·성능 무해 (Phase4 rev #2) | User explicitly requests it |

## Sibling SPEC Decision

| Decision | Reason | Sibling SPEC IDs |
|----------|--------|------------------|
| none | Primary SPEC이 절단·정족수 두 면을 공유 기계로 함께 닫음 | None |

## Reference Discipline

| Reference | Type | Verification |
|-----------|------|--------------|
| pkg/spec/prompt.go injectAuxDocs/trimToLines | existing | Read 확인 (line 118,149) |
| internal/cli/spec_review_runtime.go syncReviewedSpecStatus | existing | Read 확인 (line 29-47) |
| pkg/spec/provider_health.go DegradedLabel/ShouldLabelDegraded | existing | Read 확인 |
| pkg/config/schema_spec.go DocContextMaxLines | existing | Read 확인 (line 28) |
| pkg/spec/quality_preflight.go ValidateSpecSet | existing | Read 확인 (line 11) |
| pkg/spec/doc_coverage.go, pkg/spec/doc_compaction.go | [NEW] planned addition | 미생성, 구현 대상 |
| ReviewResult.DocCoverages, ReviewGateConf.MinProviders, --allow-degraded | [NEW] planned addition | 미생성, 구현 대상 |

## Reviewer Brief

- Intended scope: 리뷰 게이트의 관측 무결성(절단 + 정족수)을 fail-closed로 만드는 것. 기존 `SPEC-SPECREV-001` degraded 계측 위에 승격 게이트를 추가한다.
- Explicit non-goals: 전략 의미 변경, 프로바이더 add/remove, orchestra 재작업, 파괴적 스키마 변경, 기존 526 SPEC 재리뷰.
- Self-verified: Traceability Matrix, Semantic Invariant Inventory, oracle acceptance(정수/문자열/status 불변), existing/[NEW] reference 분리.
- Reviewer should focus on: correctness, convergence safety, regression risk(단일 프로바이더 로컬 리뷰 비파손), Completion Debt만. 새 제품 scope 확장 제안은 Evolution Ideas로 둔다.

## Self-Verify Summary

- Q-CORR-01 | status: PASS | attempt: 1 | files: research.md, spec.md | reason: 기존 참조 경로/함수를 Read로 확인함
- Q-CORR-04 | status: PASS | attempt: 1 | files: research.md | reason: existing 참조와 [NEW] planned addition을 Reference Discipline에서 분리함
- Q-COMP-02 | status: PASS | attempt: 2 | files: spec.md, acceptance.md | reason: validate --strict가 inline heading 충돌로 Traceability rows 0을 보고하여, REQ-RINT-STRUCT-03의 `## ` 접두 인용을 제거해 REQ↔Task↔AC↔INV 매핑이 인식됨
- Q-COMP-05 | status: PASS | attempt: 2 | files: research.md, acceptance.md | reason: structural-only 신호 감지를 해소하려 acceptance oracle notes에 concrete expected value(예상 값)·numeric tolerance를 명시, INV-001~010이 Must oracle acceptance로 추적됨
- Q-COMP-06 | status: PASS | attempt: 2 | files: spec.md, research.md | reason: Traceability Matrix rows가 인식된 뒤 Reviewer Brief와 함께 리뷰 범위를 제한함
- PREFLIGHT | status: PASS | attempt: 2 | files: all | reason: auto spec validate --strict exit 0
- Q-COMP-07 | status: PASS | attempt: 1 | files: research.md | reason: Completion Debt(None)와 Evolution Ideas(optional)를 분리, ID 미승격
- Q-SEC-01 | status: PASS | attempt: 1 | files: research.md | reason: BS/ledger 및 provider output을 untrusted evidence로 취급, source clause를 요약 인용

## Revision 1 closure

| Finding | Category | How closed | File:line |
|---------|----------|------------|-----------|
| F-001 | completeness | full injection을 기본으로, 압축은 total-budget 초과 시 fallback으로 재정의(FULL-02/TRUNC-04 재서술); AC-RINT-COV-1 트리거를 budget-exceeded fixture로 바꾸고 default 예산 100% 경로를 명시; 설계결정에서 캡상향 기각을 budget-default-full로 조정하고 doc_context_max_lines를 압축 fallback threshold로 하위호환 재해석 | spec.md:32,44 · acceptance.md:8 · research.md:34,37 |
| F-002 | correctness | AC-RINT-QUORUM-4 Given에 exclude_failed_from_denom true 명시 → denom=1, 1/1=1.0 ≥ 0.67로 1/3 PASS가 재현 가능(DEVICE-SETUP 실제 config, MergeVerdictsWithDenomMode 검증) | acceptance.md:34 |
| F-003 | completeness | REQ-RINT-COV-01에 DocCoverages를 review-findings.json 내 additive optional 필드로 명시, 구 sidecar는 empty coverage로 로드(REQ-RINT-COMPAT-10 불변) | spec.md:27 |
