# SPEC-ADK-MINIMALITY-DISCIPLINE-001 Acceptance Criteria

## Test Scenarios

### Scenario 1: AC-MINDISC-001 - Plan writes a Minimality Decision Matrix
Priority: Must
Given `@auto plan` receives a brownfield request for `autopus-adk`
And project evidence identifies existing workflow surfaces under `templates/codex/skills/auto-plan.md.tmpl` and `content/skills/agent-pipeline.md`
When the SPEC writer creates `research.md`
Then `research.md` contains `## Minimality Decision Matrix`
And the matrix contains ladder rows for `actual need`, `existing code/helper/pattern`, `stdlib/native`, `existing dependency`, `new dependency or abstraction`, and `minimum sufficient verification`
And each row records evidence, decision, and receipt item
And the response or generated text does not ask the user to turn on a lean/Ponytail mode.

### Scenario 2: AC-MINDISC-002 - New dependency or abstraction requires justification
Priority: Must
Given a plan proposes adding dependency `example.com/new-datepicker`
And the source request does not explicitly require that dependency
When `@auto plan` applies the minimality ladder
Then the plan records whether existing UI primitives, native platform input, and installed dependencies were checked
And if those alternatives are not recorded, the dependency is marked as `revise-target` or risk
And if the user explicitly requested the dependency, the plan preserves that intent and records justification, alternatives considered, and verification obligation.

### Scenario 3: AC-MINDISC-003 - Go and agent pipeline check existing paths first
Priority: Must
Given `@auto go SPEC-ADK-MINIMALITY-DISCIPLINE-001` loads `plan.md`
When planner and executor prompts are prepared
Then the prompts include the minimality ladder before implementation task assignment
And executor instructions require searching existing code, helpers, and patterns before adding new helpers or abstractions
And executor instructions distinguish "minimum sufficient implementation" from "shortest code"
And tester or validator instructions keep security, validation, accessibility, data-loss, and deterministic verification gates intact.

### Scenario 4: AC-MINDISC-004 - Fix requires caller and shared root-cause inspection
Priority: Must
Given `@auto fix "status badge shows wrong state"` identifies function `deriveStatusBadge`
And `deriveStatusBadge` has callers `renderBuildCard` and `renderSessionDrawer`
When the fix plan is prepared
Then the plan records symptom location `deriveStatusBadge`
And the plan records the caller list or grep evidence for callers
And the plan records whether the root cause is shared with callers
And if it patches only `renderBuildCard` without caller/shared path evidence, the plan is marked `revise-target`
And if evidence shows `renderBuildCard` is the only affected caller, the focused patch remains valid.

### Scenario 5: AC-MINDISC-005 - Review separates correctness/security from complexity
Priority: Must
Given a changed diff contains one missing input validation issue and one avoidable helper duplication
When `@auto review` reports findings
Then the missing input validation appears under `Correctness/Security Findings`
And the avoidable helper duplication appears under `Complexity Findings`
And the complexity finding has at least one tag from `delete`, `stdlib`, `native`, `yagni`, `shrink`, `existing-helper`, or `existing-dependency`
And the review guidance declares the complete complexity tag set `delete`, `stdlib`, `native`, `yagni`, `shrink`, `existing-helper`, and `existing-dependency`
And the review output still preserves the existing TRUST 5 dimensions `Tested`, `Readable`, `Unified`, `Secured`, and `Trackable`
And correctness/security findings still include behavior, build/test, contract, validation, accessibility, data-safety, and security issues when present
And if removing the helper would weaken validation or security, the complexity finding is downgraded or rejected
And the final verdict treats correctness/security findings as authoritative over complexity-only suggestions.

### Scenario 6: AC-MINDISC-006 - Final response shows receipt, not mode state
Priority: Must
Given a successful `@auto go` run reused an existing settings form primitive
And skipped a new date-picker dependency because native input was sufficient
And added one focused regression test
When the final response is rendered
Then it contains a short decision receipt with those three choices
And it does not say the user enabled, disabled, entered, or exited Ponytail mode
And it does not expose internal mode state as something the user must manage.

### Scenario 7: AC-MINDISC-007 - Source-owned surface parity is enforced
Priority: Must
Given ADK source content and templates are scanned
When the parity test reads plan surfaces `templates/codex/skills/auto-plan.md.tmpl`, `templates/codex/prompts/auto-plan.md.tmpl`, `templates/gemini/skills/auto-plan/SKILL.md.tmpl`, `templates/claude/commands/auto-router.md.tmpl`, `templates/gemini/commands/auto-router.md.tmpl`, `content/agents/spec-writer.md`, `templates/codex/agents/spec-writer.toml.tmpl`, and `templates/gemini/agents/spec-writer.md.tmpl`
Then each plan surface contains `Minimality Decision Matrix`, `new dependency`, `new abstraction`, and `minimum sufficient verification`
When the parity test reads go/pipeline surfaces `content/skills/agent-pipeline.md`, `templates/codex/skills/agent-pipeline.md.tmpl`, `templates/gemini/skills/agent-pipeline/SKILL.md.tmpl`, `templates/codex/skills/auto-go.md.tmpl`, `templates/codex/prompts/auto-go.md.tmpl`, and `templates/gemini/skills/auto-go/SKILL.md.tmpl`
Then each go/pipeline surface contains `minimality ladder`, `existing code/helper/pattern`, and `receipt`
When the parity test reads fix surfaces `templates/codex/skills/auto-fix.md.tmpl`, `templates/codex/prompts/auto-fix.md.tmpl`, `templates/gemini/skills/auto-fix/SKILL.md.tmpl`, `content/agents/debugger.md`, `templates/codex/agents/debugger.toml.tmpl`, `templates/gemini/agents/debugger.md.tmpl`, `content/skills/debugging.md`, `templates/codex/skills/debugging.md.tmpl`, and `templates/gemini/skills/debugging/SKILL.md.tmpl`
Then each fix surface contains `caller`, `shared root-cause`, and `revise-target`
When the parity test reads review surfaces `templates/codex/skills/auto-review.md.tmpl`, `templates/codex/prompts/auto-review.md.tmpl`, `templates/gemini/skills/auto-review/SKILL.md.tmpl`, `content/agents/reviewer.md`, `templates/codex/agents/reviewer.toml.tmpl`, `templates/gemini/agents/reviewer.md.tmpl`, `content/skills/review.md`, `templates/codex/skills/review.md.tmpl`, and `templates/gemini/skills/review/SKILL.md.tmpl`
Then each review surface contains `Correctness/Security Findings`, `Complexity Findings`, every complexity tag from `delete`, `stdlib`, `native`, `yagni`, `shrink`, `existing-helper`, and `existing-dependency`, and the existing TRUST 5 dimensions
When Gemini workflow parity is evaluated
Then `research.md` records verified adapter evidence that Gemini workflow skill and router surfaces render from their source templates without a hardcoded content rewrite, so source-template scans are sufficient for Gemini workflow contracts in this SPEC
When the Codex adapter generates surfaces into a temporary project root
Then rendered `.agents` workflow skill file `.agents/skills/auto-plan/SKILL.md` contains the decision matrix and new dependency/abstraction justification contracts expected from its source template,
And rendered `.agents` workflow skill file `.agents/skills/auto-go/SKILL.md` contains the minimality ladder and receipt handoff contracts expected from its source template,
And rendered `.agents` workflow skill file `.agents/skills/auto-fix/SKILL.md` contains the caller and shared root-cause inspection contracts expected from its source template,
And rendered `.agents` workflow skill file `.agents/skills/auto-review/SKILL.md` contains the separated correctness/security and complexity findings contracts expected from its source template,
And rendered Codex workflow skill file `.codex/skills/auto-plan.md` contains the decision matrix and new dependency/abstraction justification contracts,
And rendered Codex workflow skill file `.codex/skills/auto-go.md` contains the minimality ladder and receipt handoff contracts,
And rendered Codex workflow skill file `.codex/skills/auto-fix.md` contains the caller and shared root-cause inspection contracts,
And rendered Codex workflow skill file `.codex/skills/auto-review.md` contains the separated correctness/security and complexity findings contracts,
And rendered Codex workflow prompt files `.codex/prompts/auto-plan.md`, `.codex/prompts/auto-go.md`, `.codex/prompts/auto-fix.md`, and `.codex/prompts/auto-review.md` contain their respective matrix, ladder, root-cause, review split, and receipt contracts expected from their source prompt templates,
And the rendered `.codex/skills/agent-pipeline.md` proves the hardcoded `codexAgentPipelineSkillBody` rewrite path, not only file templates, contains `minimality ladder`, `existing code/helper/pattern`, `minimum sufficient verification`, and `receipt`
When the OpenCode adapter generates surfaces into a temporary project root
Then rendered OpenCode command files `.opencode/commands/auto-plan.md`, `.opencode/commands/auto-go.md`, `.opencode/commands/auto-fix.md`, and `.opencode/commands/auto-review.md` remain thin aliases that preserve `$ARGUMENTS`, load `auto`, and route to the matching detailed workflow skill
And the OpenCode command files are verified only as routing aliases, not as detailed contract carriers
And rendered shared skill file `.agents/skills/auto-plan/SKILL.md` contains the decision matrix and new dependency/abstraction justification contracts when OpenCode is enabled
And rendered shared skill file `.agents/skills/auto-go/SKILL.md` contains the minimality ladder and receipt handoff contracts when OpenCode is enabled
And rendered shared skill file `.agents/skills/auto-fix/SKILL.md` contains the caller and shared root-cause inspection contracts when OpenCode is enabled
And rendered shared skill file `.agents/skills/auto-review/SKILL.md` contains the separated correctness/security and complexity findings contracts when OpenCode is enabled
And rendered Codex outputs do not ask the user to enable, disable, enter, or exit a lean/Ponytail mode
And no test edits root `.codex/**`, `.claude/**`, `.gemini/**`, `.opencode/**`, plugin cache, or runtime artifact paths directly.

### Scenario 8: AC-MINDISC-008 - Ponytail remains provenance only
Priority: Must
Given the request cites `DietrichGebert/ponytail` at commit `c4d1925ae9b76a1b641877328209ad25cfeb5ef2`
And the request states the license is MIT
When ADK source guidance references Ponytail
Then the guidance says the upstream is provenance or inspiration only
And no command requires installing or vendoring Ponytail
And no upstream prompt or code text is copied without license notice
And no generated runtime prompt treats upstream content as trusted instructions.

### Scenario 9: AC-MINDISC-009 - Repeated complexity findings become inactive improvement candidates
Priority: Must
Given qualityloop observes these reason codes: `unnecessary_dependency`, `duplicate_helper`, `single_impl_abstraction`, `stdlib_available`, `native_available`, `yagni_expansion`, `existing_helper_available`, `existing_dependency_available`, and `shrink_scope_available`
And for each reason code the input set has exactly three ADK-owned failures sharing `FailureFingerprint` value `minimality.<reason_code>.settings-form`
And the same input set has exactly two ADK-owned failures sharing `FailureFingerprint` value `minimality.<reason_code>.one-off`
And evidence refs are metadata-only and deterministic
When `NormalizeFailures` calls the existing repeated-failure aggregator
Then exactly one candidate is produced for each three-row `FailureFingerprint`
And no `repeated_failure` candidate is produced for any two-row `FailureFingerprint`
And any subthreshold individual output has `apply_enabled` false and does not contain reason code `repeated_failure`
And the resulting candidates are routed as minimality or skill/playbook improvement candidates
And every reason code is present in the focused qualityloop fixture
And `repair_action_enabled` is false
And `apply_enabled` is false
And `redaction_status` is `metadata_only`
And no active repeated candidate is produced for generated-surface, plugin-cache, or root runtime artifact targets
And generated-surface targets such as `.codex/skills/auto-go.md`, `.claude/commands/auto-go.md`, `.gemini/skills/auto-go/SKILL.md`, `.opencode/rules/autopus/auto-go.md`, and `.agents/skills/auto-go/SKILL.md` are rejected or quarantined by existing qualityloop safety policy with `apply_enabled` false and `repair_action_enabled` false
And plugin-cache targets such as `.codex/plugins/cache/autopus-local/auto/1.0.0/skills/auto-go/SKILL.md` are rejected or quarantined with `apply_enabled` false and `repair_action_enabled` false
And root runtime artifact targets such as `.autopus/runtime/session.json`, `.autopus/orchestra/run.json`, `.autopus/brainstorms/BS-001.md`, `.autopus/canary/report.json`, `.autopus/context/signatures.md`, `.autopus/foo-manifest.json`, and `config.toml` are rejected or quarantined with `apply_enabled` false and `repair_action_enabled` false
And promotion still requires replay evidence and human approval.

### Scenario 10: AC-MINDISC-010 - Minimum sufficient verification is explicit and non-reducible
Priority: Must
Given `@auto plan` creates acceptance for a workflow-template-only change
And the change touches plan/go/fix/review guidance but no runtime command implementation
When the Minimality Decision Matrix selects verification
Then the `research.md` `## Minimality Decision Matrix` row `minimum sufficient verification` contains source/template parity tests for changed surfaces
And that same row contains `auto spec validate <SPEC_DIR> --strict`
And that same row contains focused qualityloop/skillevolve tests when candidate routing changes
And expected value in that row for `non_reducible_gates` equals `security, validation, accessibility, data-loss, deterministic-oracle, generated-surface-hygiene`
And security, validation, accessibility, data-loss, deterministic oracle, and generated-surface hygiene gates are listed as non-reducible when applicable
And broad full-suite or GUI/release matrix checks are marked optional unless the changed implementation surface requires them.

### Scenario 11: AC-MINDISC-011 - Shared orchestra reviewer handles complexity without changing auto-review taxonomy
Priority: Must
Given `templates/shared/orchestra-reviewer.md.tmpl` reviews a SPEC or code context
When the shared reviewer output contract is updated
Then it contains a distinct complexity assessment section for simpler alternatives, unnecessary dependency, duplicate helper, single-implementation abstraction, and YAGNI signals
And it preserves the existing requirements validation, architecture assessment, risk identification, findings, and PASS/REVISE/REJECT verdict sections
And it states that correctness/security findings remain authoritative over complexity-only findings.

### Scenario 12: AC-MINDISC-012 - Skillevolve preserves quarantine threshold and generated-surface safety
Priority: Must
Given `pkg/skillevolve/generator.go` reads a quality index with exactly two failures sharing `Fingerprint` value `minimality.duplicate_helper.settings-form`
And `CandidateGenerationOptions.MinCount` is zero so the generator uses its default
And the failures point to ADK source-owned affected refs
When `GenerateCandidates` runs
Then exactly one candidate bundle is produced
And the bundle has `status` equal to `quarantined`
And `active` is false
And `promotion_ready` is false
And the bundle is written only under the configured quarantine directory
And canonical source files and generated surfaces are not mutated
Given another repeated quality index points affected refs or proposed files at `.codex/skills/auto-go.md`, `.agents/skills/auto-go/SKILL.md`, `.autopus/plugins/**`, plugin cache paths, `.autopus/runtime/**`, `.autopus/orchestra/**`, `.autopus/brainstorms/**`, `.autopus/canary/**`, `.autopus/context/signatures.md`, `.autopus/*-manifest.json`, or `config.toml`
When `GenerateCandidates`, `EvaluateSafety`, or `ReplayCandidate` evaluates the candidate
Then no promotion-ready candidate is produced
And safety reason codes include `generated_surface_mutation_forbidden` or `affected_file_outside_owned_paths`
And replay and promotion remain blocked until deterministic replay and explicit approval pass through existing skillevolve gates.

### Scenario 13: AC-MINDISC-013 - Prompt layer manifest and cache invalidation are explicit
Priority: Must
Given this SPEC changes stable prompt guidance for `@auto plan`, `@auto go`, `@auto fix`, `@auto review`, agents, skills, and adapter-rendered Codex/OpenCode surfaces
When `research.md` is written
Then it contains `## Prompt Layer Manifest`
And the manifest classifies the stable grouped source sets from `plan.md` File Impact Analysis, including `content/skills`, `content/agents`, Codex skill/prompt/agent templates, Gemini skill/agent/router templates, Claude router templates, shared orchestra reviewer templates, Codex hardcoded rewrite bodies, and OpenCode command/shared-skill renderer observation points
And the manifest classifies snapshot inputs such as this SPEC package, project context documents, acceptance criteria, and frozen review findings
And the manifest classifies ephemeral inputs such as the latest user request, run flags, provider review outputs, and command retry state
And cache invalidation observation points include source/template parity tests, adapter-rendered Codex output tests, adapter-rendered OpenCode output tests, normal ADK regeneration or update flows for generated workspace surfaces, and a no-direct-root-generated-edit check.

### Scenario 14: AC-MINDISC-014 - Plan, fix, and review final responses show decision receipts
Priority: Must
Given `@auto plan` creates a SPEC that reuses existing workflow template surfaces
And it skips a new dependency because source/template parity tests and native markdown generation are sufficient
When the plan final response or terminal handoff is rendered
Then its decision receipt includes reused existing workflow surfaces, skipped new dependency, and selected focused verification
And it does not say the user entered, enabled, disabled, or exited a lean/Ponytail mode
Given `@auto fix` checks function `deriveStatusBadge` and callers `renderBuildCard` and `renderSessionDrawer`
When the fix final response or terminal handoff is rendered
Then its decision receipt includes caller/shared root-cause checked, focused patch target, and focused regression or verification
And it does not expose a mode state
Given `@auto review` reports one correctness/security finding and one complexity finding
When the review final response or terminal handoff is rendered
Then its decision receipt summarizes the authoritative correctness/security action, the advisory or blocking complexity action, and any skipped simplification that would weaken safety
And it does not expose a mode state.

## Oracle Acceptance Notes

- `AC-MINDISC-002` rejects dependency additions that skip the existing-code, native/stdlib, and existing-dependency checks.
- `AC-MINDISC-004` rejects symptom-only fixes unless caller/root-cause evidence proves the symptom location is the root cause.
- `AC-MINDISC-005` requires a real section split. A single mixed findings list does not pass.
- `AC-MINDISC-006` requires observable receipt bullets and forbids user-managed mode language.
- `AC-MINDISC-009` requires the existing `FailureFingerprint` and `len >= 3` aggregation semantics; two-row signals must not become repeated candidates, inactive state and generated-surface, plugin-cache, and root runtime artifact safety gates are part of the oracle.
- `AC-MINDISC-010` requires concrete expected verification output in the `minimum sufficient verification` row, not just a "run tests" heading.
- `AC-MINDISC-011` keeps shared orchestra reviewer verification separate from auto-review TRUST 5 parity.
- `AC-MINDISC-012` keeps skillevolve's default `MinCount = 2` quarantine behavior separate from qualityloop's `len >= 3` repeated-failure aggregation and requires generated-surface safety checks at generation, replay, and promotion boundaries.
- `AC-MINDISC-013` requires prompt-layer ownership and cache invalidation to be observable; stable guidance belongs in ADK source, not ephemeral user prompt state.
- `AC-MINDISC-014` closes receipt coverage for plan, fix, and review; `AC-MINDISC-006` remains the concrete go receipt oracle.

## Definition Of Done

- [x] All Must scenarios have focused tests or documented manual verification steps.
- [x] Source guidance and templates pass parity scans.
- [x] No root generated surface is edited directly.
- [x] Existing security, validation, accessibility, data-loss, and generated-surface gates are preserved.
