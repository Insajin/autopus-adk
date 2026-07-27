# SPEC-CONTEXT-ENGINEERING-EVOLUTION-001 Acceptance

## Test Scenarios

### S1: Full delivery remains authoritative
Priority: Must
Given a verified workflow context
When context command, worker delivery, and binding gates run
Then expected schema remains `autopus.context_delivery.v1`
And complete bodies, hashes, architecture inclusion, and verification are
unchanged
And no v2 field appears in v1 JSON

### S2: Shadow plan reports deterministic deltas
Priority: Must
Given verified delivery and a fresh memory projection
When `auto workflow context-plan` runs
Then expected schema is `autopus.context_plan.v2`
And `shadow_only=true`, active mode is `full`, candidate mode is `jit`
And token delta equals full minus candidate estimate
And for full `10,000` and candidate `2,000`, expected delta is `8,000` and
reduction is `80.0%`
And selected refs/hashes come only from verified v1 metadata

### S3: Shadow failure is body-free and non-gating
Priority: Must
Given a missing, stale, or corrupt projection
When a plan is built
Then expected status is `unavailable`
And candidate metrics are null with a reason
And full delivery still succeeds
And output has no raw query, prompt, body, secret, or absolute project path

### S4: Receipt parser owns the exact contract
Priority: Must
Given one versioned worker receipt marker
When provider-neutral parsing runs
Then expected body has exactly five canonical fields
And optional condensed evidence is a sibling envelope field, ref/hash based
And the entire marked JSON is at most 2,000 tokens by
`promptlayer.EstimateTokens`
And short valid receipts are accepted without padding

### S5: Pipeline consumes only explicit valid receipts
Priority: Must
Given valid, absent, and malformed markers
When the pipeline evidence boundary runs
Then a valid receipt is appended exactly once
And markerless output preserves prior behavior with an empty list
And malformed, duplicate, trailing, or oversized marked output fails
And orchestra debate output remains outside this parser

### S6: Claude and Gemini honor opt-in compilation
Priority: Must
Given full and split compiler configurations
When Claude and Gemini/Antigravity scratch roots are generated or updated
Then full mode emits the existing compatible set
And split retains core/workflow and selected bundles
And excluded long-tail skills are absent from native/mirror roots
And returning to full restores the compatible set
And unrelated files inside the platform skill parent and files outside the
platform-owned generated roots remain byte-identical

### S7: Gemini tool projection is native and honest
Priority: Must
Given source agents declare known and unknown tools
When Gemini templates are generated
Then known tools map to native `tools` names
And unsupported or recursive tools are omitted
And body/skills remain unchanged
And Codex guidance is not labeled native enforcement

### S8: Canonical examples stay small
Priority: Must
Given effective pipeline/receipt guidance
When examples are inspected
Then they cover raw-body replay, malformed receipt evidence, and unsupported
tool enforcement
And example count is at most three
And no example contains a secret or absolute path

## Edge Cases

### Edge Case 1: v2 passed to v1 binding
Priority: Must
Given a valid v2 plan
When used as a v1 context manifest
Then expected binding rejects it

### Edge Case 2: No selection labels
Priority: Must
Given no expected refs
When hits are calculated
Then plan status remains `planned`
And selection-hit status is `unavailable` with hit rate JSON null
And token delta and reduction are still calculated

### Edge Case 3: Duplicate receipt marker
Priority: Must
Given two receipt markers
When parsing runs
Then parsing fails and no receipt is appended

### Edge Case 4: Unsupported Gemini capability
Priority: Must
Given an unverified extension or recursive tool
When Gemini agent generation runs
Then it is omitted from the native allowlist
And diagnostics do not claim enforcement
And generated settings, policy, sandbox, and approval configuration are
byte-identical

## Oracle Acceptance Notes

- Plan tests compare exact JSON schema, pointer/null semantics, deterministic
  refs, hashes, and arithmetic without inspecting raw bodies.
- Receipt tests use adversarial marker, unknown-field, unsafe-ref, and size
  fixtures plus persisted pipeline JSON.
- Adapter tests generate scratch roots and compare full/split paths and Gemini
  native/mirror tool/skill semantics.
- Existing v1, Codex, OpenCode, and template tests are compatibility oracles.

## Definition of Done

- [x] S1-S8 and Edge Case 1-4 PASS
- [x] focused/race/vet/build/SPEC/architecture/diff gates PASS
- [x] reviewer/security findings resolved
- [x] Completion Debt explicit
