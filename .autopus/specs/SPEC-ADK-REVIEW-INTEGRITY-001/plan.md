# SPEC-ADK-REVIEW-INTEGRITY-001 구현 계획

## Implementation Strategy

기존 `pkg/spec` 리뷰 파이프라인을 확장하여 관측 무결성을 fail-closed로 만든다. 새 의존성은 추가하지 않고, `SPEC-SPECREV-001`이 남긴 `ProviderStatus`/`DegradedLabel`/`MergeVerdictsWithDenomMode`와 `ReviewResult`/`syncReviewedSpecStatus`/`ValidateSpecSet`를 재사용한다. TDD로 각 오라클 테스트를 먼저 작성한다.

핵심 방향:
- 절단 봉합은 "먼저 전량 주입 시도 → 토큰 예산 초과 시에만 구조 보존 압축 → 커버리지 계측 → 100% 미만이면 degraded" 순서로 처리한다. head-only `trimToLines`는 섹션 인지 압축으로 대체한다.
- degraded 사유(`partial_doc_context`, `provider_quorum`)를 `ReviewResult`에 실어 `review.md`에 렌더하고, 승격 게이트가 이 사유를 읽어 auto-promotion을 차단한다.
- 정족수 기본값은 구성 프로바이더 과반(`len/2 + 1`)으로, 단일 프로바이더 로컬 리뷰(min=1)는 그대로 통과한다.
- `--allow-degraded`만이 degraded 승격을 허용하며 override 사유를 감사 라인으로 남긴다.
- 300줄 소스 한도 때문에 커버리지와 압축은 신규 파일로 분리한다.

## Visual Planning Brief

리뷰 1회 데이터 흐름 (sequence):

```
auto spec review <ID> [--allow-degraded]
  -> spec_review.go: load gate cfg (min_providers, doc cap), parse override flag
  -> spec_review_loop.go
       -> buildSpecReviewProviderPrompt
            -> prompt.go injectAuxDocs
                 -> for each aux doc: try full inject within token budget
                    -> if over budget: doc_compaction.go (preserve tail-critical sections)
                    -> doc_coverage.go: record injected/total/percent/complete
       -> orchestra fan-out -> per-provider VERDICT
       -> BuildProviderStatuses + successful count
       -> quorum check: successful >= DefaultMinProviders(configured, cfg)
       -> assemble ReviewResult{Verdict, DocCoverages, DegradedReasons}
       -> PersistReview (review.md: verdict + Observation Coverage + Provider Health)
  -> spec_review_runtime.go syncReviewedSpecStatus
       -> gate: PASS && no active findings && fully_observed && quorum_met
                 || (PASS && no findings && allow_degraded override)
       -> promote to approved OR leave status unchanged (+ audit line)
```

승격 판정 (decision):

```
observed_ok  = all(dc.complete for dc in DocCoverages)
quorum_ok    = successful_providers >= min_providers
promote      = verdict==PASS && !activeFindings && !shipped
               && (observed_ok && quorum_ok || allow_degraded)
```

## Tasks

- [ ] T1: `[NEW] pkg/spec/doc_coverage.go` — `DocCoverage{Name,Injected,Total,Percent,Complete}`, `ComputeCoverage`, `RenderObservationCoverage`; `ReviewResult`에 `DocCoverages []DocCoverage` 및 `DegradedReasons []string` 추가. `review-findings.json` 하위호환(추가 필드 optional) 회귀 테스트 포함 (REQ-RINT-COV-01, REQ-RINT-COMPAT-10).
- [ ] T2: `pkg/spec/prompt.go` — `injectAuxDocs`가 기본 전량 주입하고 adaptive TOTAL 예산 초과 시에만 압축 fallback을 쓰도록 변경, 문서별 `DocCoverage` 반환 (REQ-RINT-FULL-02, REQ-RINT-COV-01).
- [ ] T3: `[NEW] pkg/spec/doc_compaction.go` — 섹션 인지 압축으로 tail-critical 섹션 보존, `trimToLines` head-only 대체 (REQ-RINT-STRUCT-03).
- [ ] T4: `pkg/spec/review_persist.go` — verdict 라인에 degraded 사유 병기, `## Observation Coverage` 섹션 및 override 감사 라인 렌더 (REQ-RINT-TRUNC-04, REQ-RINT-OVERRIDE-07).
- [ ] T5: `pkg/spec/provider_health.go` + `pkg/config/schema_spec.go` + `pkg/config/defaults.go` — `MinProviders` 설정 필드와 `DefaultMinProviders`(과반) helper, `MeetsProviderQuorum` (REQ-RINT-QUORUM-05).
- [ ] T6: `internal/cli/spec_review_loop.go` + `internal/cli/spec_review_runtime.go` — 커버리지·정족수 집계로 `DegradedReasons` 세팅, `syncReviewedSpecStatus`에 관측 무결성 게이트 추가 (REQ-RINT-TRUNC-04, REQ-RINT-QUORUM-05, REQ-RINT-PROMO-06).
- [ ] T7: `internal/cli/spec_review.go` — `--allow-degraded` 플래그 추가, 게이트로 전달 (REQ-RINT-OVERRIDE-07).
- [ ] T8: `pkg/spec/quality_preflight.go` — strict preflight에 캡 초과 warning 추가 (REQ-RINT-AUTHOR-08).
- [ ] T9: `content/skills/spec-review.md` + `content/rules/spec-quality.md` — degraded/coverage fail-closed 의미 기술, `pkg/adapter` parity 유지 (REQ-RINT-PARITY-09).
- [ ] T10: 통합 오라클 테스트 — 캡 초과 fixture로 무자격 PASS 불가·커버리지 기록·degraded 승격 차단·override 승격을 end-to-end로 검증.

## Feature Completion Scope

Primary SPEC이 Outcome Lock을 단독으로 닫는다. 절단 봉합(A)과 정족수 봉합(B)은 같은 관측 무결성 속성의 두 면이며 커버리지·degraded 사유·승격 게이트 기계를 공유하므로 분리하면 scaffold-only 반쪽이 된다(계측만 있고 게이트 없음, 또는 게이트만 있고 관측 근거 없음). 따라서 sibling SPEC 없음. 승인된 sibling 의존성 없음. Completion Debt 없음 — 모든 mandatory 요구사항이 T1~T10과 Must acceptance로 닫힌다.
