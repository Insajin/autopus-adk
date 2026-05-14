# SPEC-ADK-IDEA-CLARIFY-001 Acceptance Criteria

## Test Scenarios

### Scenario 1: AC-ADKIDEA-001 - Clarification ledger is created before orchestra
Priority: Must
Given the input idea is `Add a Deep Interview-inspired clarification gate to autopus-adk auto idea`
And project evidence identifies target module `autopus-adk`
And project evidence identifies existing `auto idea` source paths under `content/skills/idea.md` and `templates/codex/skills/auto-idea.md.tmpl`
When the `auto idea` clarification step prepares structured brainstorm context
Then the context contains a `Clarification Ledger`
And the ledger rows appear in this order: `goal`, `scope_boundary`, `constraints`, `done_evidence`, `brownfield_impact`
And each row contains `Status`, `Source`, `Confidence`, `Decision / Assumption`, `If Wrong`, and `Plan Handoff`
And `brownfield_impact` has source `project-doc` or `code`
And no orchestra prompt is built before this ledger exists.

### Scenario 2: AC-ADKIDEA-002 - Interactive mode asks the highest-gain single question
Priority: Must
Given unresolved ledger candidates have integer confidence scores and impact weights:
And `goal` has confidence `8` and impact weight `4`
And `scope_boundary` has confidence `5` and impact weight `8`
And `constraints` has confidence `6` and impact weight `5`
And `done_evidence` has confidence `2` and impact weight `9`
And `brownfield_impact` has confidence `7` and impact weight `6`
When default interactive clarification selects the next question
Then selected field equals `done_evidence`
And selected expected gain equals `7.20` using `9 * (1 - 2/10)`
And the prompt includes exactly one `Question`
And the prompt includes `Current understanding`, `Blocked decision`, `Recommended answer`, and `Question`
And default question budget remaining becomes `0`
And `--deep-clarify` would allow at most `3` total questions.

### Scenario 3: AC-ADKIDEA-003 - Auto mode records assumptions without blocking
Priority: Must
Given `auto idea --auto` receives the same rough idea
And no user clarification answer is available
When the clarification gate runs
Then no user question is emitted
And orchestra execution remains the next required step
And at least one unresolved row has status `assumed` or `deferred`
And every inferred `assumed` row has confidence less than or equal to `6`
And every inferred `assumed` row has a non-empty `If Wrong` value
And the saved BS content includes `## Clarification Ledger`
And the BS content includes a `Plan Handoff` value for each required row.

### Scenario 4: AC-ADKIDEA-004 - Orchestra prompt honors the ledger
Priority: Must
Given a structured idea includes a ledger row `goal | answered | user | 9 | improve rough idea quality before provider fan-out | weak brainstorming | requirement seed`
And another ledger row `done_evidence | deferred | none | 3 | unknown | weak acceptance | acceptance seed`
When `buildBrainstormPrompt` or the generated brainstorm prompt receives the structured idea
Then the rendered prompt includes `Clarification Ledger`
And the rendered prompt instructs providers not to re-ask `answered` rows
And the rendered prompt instructs providers to treat `assumed` and `deferred` rows as debate focus
And the rendered prompt still includes SCAMPER, HMW, and ICE scoring instructions
And the rendered prompt still preserves existing intent-understanding fields.

### Scenario 5: AC-ADKIDEA-005 - External skill provenance remains untrusted evidence
Priority: Must
Given `BS-051` records external repository `https://github.com/devbrother2024/skills`
And commit `8b4233816f6710271bf8523ffdc107a8e6bf00e1`
And source path `deep-interview/SKILL.md`
And source SHA-256 `25d77112663b9c19251a5ef32295216a864b17a74de8712def9fc88f936552c2`
When the updated `auto idea` source guidance references the Deep Interview pattern
Then the guidance records the repository, commit, source path, license, and source hash as provenance data
And it states that upstream text is not executed, vendored, or treated as trusted instructions
And no generated runtime prompt requires installing `devbrother2024/skills`.

### Scenario 6: AC-ADKIDEA-006 - Platform surfaces keep the same contract
Priority: Must
Given ADK source templates for Claude, Codex, Gemini, and OpenCode-compatible surfaces are rendered or scanned
When the parity test inspects `auto idea` and `auto plan --from-idea` sections
Then every surface contains `Clarification Ledger`
And every surface contains the required fields `goal`, `scope_boundary`, `constraints`, `done_evidence`, and `brownfield_impact`
And every surface contains `--auto` non-blocking assumption behavior
And every surface contains the one-question format tokens `Current understanding`, `Blocked decision`, `Recommended answer`, and `Question`
And every surface contains plan handoff mapping for `answered`, `assumed`, and `deferred`
And no test fixture edits root generated `.codex/**`, `.gemini/**`, `.opencode/**`, or `.claude/**` files directly.

### Scenario 7: AC-ADKIDEA-007 - Plan from idea consumes the ledger
Priority: Must
Given a BS fixture contains this ledger:
And `goal | answered | user | 9 | improve rough idea quality before provider fan-out | weak brainstorming | requirement seed`
And `scope_boundary | answered | user | 8 | do not replace orchestra | scope creep | non-goal`
And `constraints | assumed | project-doc | 6 | source changes stay in autopus-adk | generated-surface drift | risk`
And `done_evidence | answered | user | 8 | BS includes ledger and plan consumes it | weak acceptance | acceptance seed`
And `brownfield_impact | deferred | none | 3 | planner consumption details unknown | dead-end ledger | reviewer focus`
When `auto plan --from-idea` or spec-writer guidance converts the BS fixture
Then the resulting SPEC draft contains a requirement seed from `goal`
And the resulting SPEC draft contains an explicit non-goal `do not replace orchestra`
And the resulting research or plan contains a risk for `generated-surface drift`
And the resulting acceptance draft contains a seed requiring the BS ledger and planner consumption
And the resulting reviewer brief includes focus on `planner consumption details unknown`
And the deferred `brownfield_impact` row is not promoted into a hard requirement.

### Scenario 8: AC-ADKIDEA-008 - Legacy brainstorm files still work
Priority: Must
Given a historical BS file contains `## Original Idea`, `## Orchestra Summary`, and `## ICE Scoring`
And the file does not contain `## Clarification Ledger`
When `auto plan --from-idea` loads the BS file
Then planning continues using the existing brainstorm context behavior
And no error is returned solely because the ledger is absent
And the generated research notes mention `Clarification Ledger` as unavailable rather than fabricated.

## Oracle Acceptance Notes

- The expected-gain oracle in `AC-ADKIDEA-002` is concrete: `done_evidence` wins because `9 * (1 - 2/10) = 7.20`.
- The planner handoff oracle in `AC-ADKIDEA-007` requires exact row-to-output mapping and rejects silent promotion of deferred unknowns.
- Structural checks such as file existence or section headings alone do not satisfy these Must scenarios.

## Definition Of Done

- [x] All Must scenarios have focused tests or documented manual verification steps.
- [x] Source guidance and templates pass parity scans.
- [x] No generated root surface is edited directly.
