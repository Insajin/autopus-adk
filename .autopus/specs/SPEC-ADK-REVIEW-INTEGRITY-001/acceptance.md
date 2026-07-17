# SPEC-ADK-REVIEW-INTEGRITY-001 수락 기준

## Test Scenarios

### Scenario 1: AC-RINT-COV-1 - Per-document coverage is computed and recorded
Priority: Must
Given a spec directory whose `research.md` has exactly 250 content lines and whose `plan.md` has exactly 150 content lines
And a deliberately small total context budget fixture that forces `research.md` to compress to 200 injected lines while `plan.md` still fits whole
When the review prompt and its coverage records are built
Then the coverage record for `research.md` reports injected 200, total 250, percent 80, and complete false
And the coverage record for `plan.md` reports injected 150, total 150, percent 100, and complete true
And under the default generous budget both documents inject at 100 percent coverage with complete true
And `review.md` contains a `## Observation Coverage` section listing both rows.

### Scenario 2: AC-RINT-TRUNC-2 - Partial-context PASS does not promote status
Priority: Must
Given a SPEC whose status is `draft` and whose `research.md` is 250 lines against a deliberately small total context budget that forces compaction
And every configured provider returns the literal verdict `VERDICT: PASS` with zero findings
When `auto spec review` runs without `--allow-degraded`
Then the `review.md` verdict line contains the degraded reason `partial_doc_context`
And the `spec.md` status field remains `draft` and is not rewritten to `approved`.

### Scenario 3: AC-RINT-STRUCT-3 - Tail-critical sections survive compaction
Priority: Must
Given a `research.md` whose first 240 lines are filler and whose final lines contain `## Self-Verify Summary` with the line `Q-COMP-05 | status: PASS`
And an injection budget smaller than the document total
When the auxiliary document is compacted for the review prompt
Then the injected excerpt contains the substring `## Self-Verify Summary`
And the injected excerpt contains the substring `Q-COMP-05`
And the injected excerpt drops filler head lines rather than the tail sections.

### Scenario 4: AC-RINT-QUORUM-4 - Sub-quorum PASS does not promote status
Priority: Must
Given three configured providers with `exclude_failed_from_denom` true where two time out and one returns `VERDICT: PASS` with zero findings
And the effective minimum quorum resolves to 2
When `auto spec review` runs without `--allow-degraded`
Then the successful provider count is 1 which is below the quorum of 2
And the `review.md` verdict line contains the degraded reason `provider_quorum`
And the `spec.md` status field is not rewritten to `approved`.

### Scenario 5: AC-RINT-OVERRIDE-5 - Explicit override promotes and is audited
Priority: Must
Given the same degraded sub-quorum PASS review as AC-RINT-QUORUM-4 on a `draft` SPEC
When `auto spec review --allow-degraded` runs
Then the `spec.md` status field is rewritten to `approved`
And `review.md` contains an audit line recording the override reason `allow-degraded`.

### Scenario 6: AC-RINT-AUTHOR-6 - Strict validate warns on over-cap authoring
Priority: Should
Given a spec directory whose `research.md` has 429 lines against a 200-line review injection cap
When `auto spec validate <dir> --strict` runs
Then stderr contains a warning naming `research.md`, its line count 429, and the cap 200
And the command exit code is 0.

### Scenario 7: AC-RINT-PARITY-7 - Guidance renders to every platform surface
Priority: Must
Given the reviewer guidance changes authored in the `content/` source of truth
When the `pkg/adapter` parity tests run across claude-code, codex, antigravity-cli, and opencode
Then the skills and rules parity percentage is 100
And no platform surface is missing the updated spec-review guidance.

### Scenario 8: AC-RINT-COMPAT-8 - Old findings sidecar still loads
Priority: Must
Given a `review-findings.json` fixture written by the prior schema with three findings and no coverage fields
When `LoadFindings` reads the directory
Then it returns three findings without error
And the absent coverage fields are treated as optional additive data.

## Oracle Acceptance Notes

- INV-001 coverage 공식: `percent = floor(injected * 100 / total)`, `complete = injected >= total`. AC-RINT-COV-1은 250줄/200주입 → 80, 150줄/150주입 → 100의 concrete row를 검증한다(구조 존재만이 아니라 정수값).
- INV-004/INV-005 게이트: AC-RINT-TRUNC-2, AC-RINT-QUORUM-4는 status 필드의 before=`draft`, after=`draft` 불변을 오라클로 확인한다(승격 없음).
- INV-003 구조 보존: AC-RINT-STRUCT-3은 head가 잘려도 tail 문자열 `## Self-Verify Summary`/`Q-COMP-05`가 남는지 substring 오라클로 확인한다.
- 정족수 기본값: `DefaultMinProviders(n) = n/2 + 1` (정수 나눗셈). n=3 → 2, n=1 → 1, n=2 → 2. 단일 프로바이더 로컬 리뷰는 min=1로 통과한다.
- 각 Must scenario는 concrete expected value(예상 값: percent 80/100, status draft/approved, substring 매치)와 필요 시 numeric tolerance를 명시하는 oracle-first 형식이며, section heading 존재나 exit code 0 같은 structural 신호만으로 Must를 닫지 않는다.
