# SPEC-ADK-GENERATED-HYGIENE-001 리서치

## 현재 관찰

2026-06-20 기준 `autopus-adk` worktree는 staged source changes, unstaged generated/runtime drift, tracked-but-ignored 후보가 섞여 있다. 이 상태에서 generated surface를 바로 삭제하면 어떤 파일이 source-of-truth이고 어떤 파일이 dogfood output인지 review하기 어렵다.

관찰된 dirty path family:

- staged source changes: `internal/cli/check*.go`, `pkg/workflow/drift_gate.go`, `pkg/content/**`, `pkg/adapter/codex/**`, `templates/**`, human docs.
- unstaged generated/runtime changes: `.autopus/*-manifest.json`, `.autopus/plugins/**`, `.codex/**`, `.claude/**`, `.gemini/**`, `.opencode/**`, `config.toml`.
- tracked-but-ignored risk: `.gitignore` 정책과 이미 tracked된 generated surface가 불일치할 가능성이 있다.

Planner read-only inventory found 200 tracked-but-ignored paths. The largest families were `.opencode/**` (44), `.codex/**` (44), `.autopus/specs/**` review artifacts (39), `.gemini/**` (35), `.autopus/plugins/**` (18), and `.autopus/brainstorms/**` (12). This inventory must be reviewed by family before any `git rm --cached` command.

## Inventory Commands

```bash
git status --short --branch
git diff --name-status --cached
git diff --name-status
git ls-files -c -i --exclude-standard
git ls-files | rg '^(.codex/|.claude/|.gemini/|.opencode/|.agents/plugins/|.autopus/(.*-manifest.json|plugins/|txns/|brainstorms/|orchestra/|runtime/)|config.toml$)'
```

`git ls-files -c -i --exclude-standard`는 tracked-but-ignored drift를 보여준다. cleanup 전에 반드시 실행해야 하며, ignored 정책이 source files를 실수로 포함하는지 확인해야 한다.

Do not blindly run:

```bash
git rm --cached $(git ls-files -c -i --exclude-standard)
```

The current ignore patterns are not fully root-anchored. For example, `.gemini/` can match nested source fixtures such as `pkg/adapter/gemini/.gemini/settings.json`. Cleanup must split root dogfood generated output from nested fixture/source paths.

## Slice 1 Anchoring Inventory Update

2026-06-20 slice 1 anchored root platform dogfood patterns and added `.autopus/txns/` to the repo `.gitignore` policy. After the change:

- `pkg/adapter/gemini/.gemini/settings.json` is no longer matched by `git check-ignore --no-index`.
- root `.gemini/settings.json` remains ignored by `/.gemini/`.
- nested file fixtures such as `pkg/example/.claude.json`, `pkg/example/.mcp.json`, and `pkg/example/config.toml` are no longer matched by root-only file patterns.
- root `.claude.json`, `.mcp.json`, and `config.toml` remain ignored by `/.claude.json`, `/.mcp.json`, and `/config.toml`.
- root `.autopus/txns/**` remains ignored by `.autopus/txns/`.
- `git ls-files -c -i --exclude-standard` decreased from 200 to 199 by removing the nested Gemini source fixture from the tracked-but-ignored inventory.

Current tracked-but-ignored inventory after anchoring:

| Family | Count | Proposed cleanup slice |
|--------|-------|------------------------|
| `.opencode/**` | 44 | platform generated surface slice, after regenerate/diff evidence |
| `.codex/**` | 44 | platform generated surface slice, after regenerate/diff evidence |
| `.autopus/specs/**/review.md` | 38 | SPEC review evidence slice, separate from canonical SPEC docs |
| `.gemini/**` | 35 | platform generated surface slice, after regenerate/diff evidence |
| `.autopus/plugins/**` | 18 | plugin generated surface slice, after plugin source mapping evidence |
| `.autopus/brainstorms/**` | 12 | runtime-only slice, first untrack candidate |
| `.autopus/*-manifest.json` | 4 | generated manifest slice |
| `.autopus/specs/**/review-findings.json` | 1 | SPEC review evidence slice |
| `.agents/plugins/marketplace.json` | 1 | generated plugin marketplace slice |
| `.claude/skills/autopus/idea.md` | 1 | platform generated surface slice |
| `config.toml` | 1 | local config slice |

Dry-run untrack counts:

```bash
git rm -r --cached --dry-run -- .autopus/brainstorms        # 12
git rm -r --cached --dry-run -- .autopus/plugins            # 18
git rm -r --cached --dry-run -- .codex                      # 44
git rm -r --cached --dry-run -- .gemini                     # 35
git rm -r --cached --dry-run -- .opencode                   # 44
git rm --cached --dry-run -- .claude/skills/autopus/idea.md .agents/plugins/marketplace.json config.toml .autopus/claude-code-manifest.json .autopus/codex-manifest.json .autopus/gemini-cli-manifest.json .autopus/opencode-manifest.json # 7
git rm --cached --dry-run -- ':(glob).autopus/specs/*/review.md' ':(glob).autopus/specs/*/review-findings.json' # 39
```

Recommended next cleanup order:

1. Runtime-only: `.autopus/brainstorms/**`.
2. Local/generated config and manifests: `config.toml`, `.autopus/*-manifest.json`, `.agents/plugins/marketplace.json`.
3. SPEC review evidence: `.autopus/specs/**/review.md`, `.autopus/specs/**/review-findings.json`.
4. Plugin generated surface: `.autopus/plugins/**`.
5. Platform generated surface: `.claude/**`, `.codex/**`, `.gemini/**`, `.opencode/**`, each with regenerate/diff evidence.

## Slice 2 Runtime Brainstorms Untrack

2026-06-20 slice 2 applied the first path-family untrack for runtime-only brainstorm artifacts.

Command:

```bash
git rm -r --cached -- .autopus/brainstorms
```

Result:

- 12 tracked brainstorm files were removed from the Git index only.
- Working tree files still exist locally under `.autopus/brainstorms/`.
- `git ls-files .autopus/brainstorms` returns no tracked paths.
- `git ls-files -c -i --exclude-standard` decreased from 199 to 187.
- `git ls-files -c -i --exclude-standard | rg '^\.autopus/brainstorms/'` returns no paths.
- `auto check --hygiene --arch --quiet --staged` treats delete-only generated/runtime cleanup as allowed while still blocking staged add/modify generated/runtime drift.

Untracked path family:

```text
.autopus/brainstorms/BS-001.md
.autopus/brainstorms/BS-002.md
.autopus/brainstorms/BS-003.md
.autopus/brainstorms/BS-004.md
.autopus/brainstorms/BS-005.md
.autopus/brainstorms/BS-006.md
.autopus/brainstorms/BS-009.md
.autopus/brainstorms/BS-010.md
.autopus/brainstorms/BS-011.md
.autopus/brainstorms/BS-012.md
.autopus/brainstorms/BS-020.md
.autopus/brainstorms/BS-026.md
```

Next cleanup family remains: local/generated config and manifests (`config.toml`, `.autopus/*-manifest.json`, `.agents/plugins/marketplace.json`).

## Slice 3 Dry-Run: Local Config, Manifests, Marketplace

2026-06-20 slice 3 dry-run reviewed the next tracked-but-ignored path family before any index mutation.

Pre-slice status:

```bash
git status --short --branch
```

Observed state:

- Branch: `main...origin/main [ahead 4]`
- Existing unstaged generated/runtime drift remains in `.autopus/*-manifest.json`, `.autopus/plugins/**`, `.claude/**`, `.codex/**`, `.gemini/**`, `.opencode/**`, and `config.toml`.
- No staged changes were present before this slice.
- `.autopus/brainstorms/**` remains untracked from the index; `git ls-files .autopus/brainstorms` returns no paths.

Tracked-but-ignored inventory:

```bash
git ls-files -c -i --exclude-standard | wc -l
```

Result: `187`

Family count before slice 3:

| Family | Count | Slice decision |
|--------|-------|----------------|
| `.opencode/**` | 44 | defer to platform generated surface slice |
| `.codex/**` | 44 | defer to platform generated surface slice |
| `.autopus/specs/**/review.md` | 38 | defer to SPEC review evidence slice |
| `.gemini/**` | 35 | defer to platform generated surface slice |
| `.autopus/plugins/**` | 18 | defer to plugin generated surface slice |
| `.autopus/*-manifest.json` | 4 | include in slice 3 |
| `config.toml` | 1 | include in slice 3 |
| `.claude/**` | 1 | defer to platform generated surface slice |
| `.autopus/specs/**/review-findings.json` | 1 | defer to SPEC review evidence slice |
| `.agents/plugins/marketplace.json` | 1 | include in slice 3 |

Dry-run command:

```bash
git rm --cached --dry-run -- \
  config.toml \
  .agents/plugins/marketplace.json \
  .autopus/claude-code-manifest.json \
  .autopus/codex-manifest.json \
  .autopus/gemini-cli-manifest.json \
  .autopus/opencode-manifest.json
```

Dry-run output:

```text
rm '.agents/plugins/marketplace.json'
rm '.autopus/claude-code-manifest.json'
rm '.autopus/codex-manifest.json'
rm '.autopus/gemini-cli-manifest.json'
rm '.autopus/opencode-manifest.json'
rm 'config.toml'
```

Path family dry-run split:

```bash
git rm --cached --dry-run -- config.toml
git rm --cached --dry-run -- .agents/plugins/marketplace.json
git rm --cached --dry-run -- .autopus/claude-code-manifest.json .autopus/codex-manifest.json .autopus/gemini-cli-manifest.json .autopus/opencode-manifest.json
```

Results:

- `config.toml`: 1 path, local runtime config, ignored by `/.gitignore` pattern `/config.toml`.
- `.agents/plugins/marketplace.json`: 1 path, generated plugin marketplace, ignored by `.gitignore` pattern `.agents/plugins/`.
- `.autopus/*-manifest.json`: 4 paths, generated platform manifests, ignored by `.gitignore` pattern `.autopus/*-manifest.json`.

Ignore evidence:

```bash
git check-ignore -v --no-index -- config.toml .agents/plugins/marketplace.json .autopus/claude-code-manifest.json .autopus/codex-manifest.json .autopus/gemini-cli-manifest.json .autopus/opencode-manifest.json
```

Output:

```text
.gitignore:47:/config.toml	config.toml
.gitignore:42:.agents/plugins/	.agents/plugins/marketplace.json
.gitignore:11:.autopus/*-manifest.json	.autopus/claude-code-manifest.json
.gitignore:11:.autopus/*-manifest.json	.autopus/codex-manifest.json
.gitignore:11:.autopus/*-manifest.json	.autopus/gemini-cli-manifest.json
.gitignore:11:.autopus/*-manifest.json	.autopus/opencode-manifest.json
```

Slice judgment:

- Proceed with three small index-only untrack operations: `config.toml`, `.autopus/*-manifest.json`, then `.agents/plugins/marketplace.json`.
- Do not edit generated/runtime file contents. This slice only removes tracked ignored local/generated files from the Git index.
- `config.toml` is already absent from the working tree (`D config.toml`), so `git rm --cached` records the same intended deletion in the index without touching a source file.
- Keep `.autopus/plugins/**`, `.codex/**`, `.claude/**`, `.gemini/**`, `.opencode/**`, and SPEC review artifacts out of this slice.
- Expected tracked-but-ignored count after this slice: `181`.

Read-only planner review agreed with the split and recommended treating `config.toml` as its own small sub-slice because the file was already absent in the working tree. The manifest files and plugin marketplace metadata can be grouped as generated registry metadata, with evidence split by sub-family.

## Slice 3 Applied: Local Config, Manifests, Marketplace

2026-06-20 slice 3 applied exact-path index-only untrack operations after the dry-run evidence above.

Commands:

```bash
git rm --cached -- config.toml
git rm --cached -- .autopus/claude-code-manifest.json .autopus/codex-manifest.json .autopus/gemini-cli-manifest.json .autopus/opencode-manifest.json
git rm --cached -- .agents/plugins/marketplace.json
```

Result:

- `config.toml`: removed from the Git index. The working tree file was already absent before this slice, so this records the intended end of local runtime config tracking rather than preserving a local ignored copy.
- `.autopus/*-manifest.json`: 4 generated platform manifest files removed from the Git index only; local generated files remain present and ignored.
- `.agents/plugins/marketplace.json`: generated plugin marketplace metadata removed from the Git index only; local generated file remains present and ignored.

Post-slice inventory:

```bash
git ls-files -- config.toml .agents/plugins/marketplace.json .autopus/claude-code-manifest.json .autopus/codex-manifest.json .autopus/gemini-cli-manifest.json .autopus/opencode-manifest.json
```

Result: no tracked paths.

```bash
git ls-files -c -i --exclude-standard | wc -l
```

Result: `181`

```bash
git ls-files -c -i --exclude-standard | rg '^(config\.toml$|\.agents/plugins/marketplace\.json$|\.autopus/.*-manifest\.json$)' || true
```

Result: no remaining tracked-but-ignored paths from this slice.

Local file presence check:

```text
marketplace_exists
.autopus/claude-code-manifest.json exists
.autopus/codex-manifest.json exists
.autopus/gemini-cli-manifest.json exists
.autopus/opencode-manifest.json exists
config_missing
```

Regenerate/diff verification:

```bash
go run ./cmd/auto update --local --dry-run --yes
```

Result: exit 0. The command uses the local source tree and preview mode, so it computes managed output without writing generated files.

Relevant preview output:

```text
[codex] extended skills: 46 compatible, 1 incompatible
[gemini] extended skills: 46 compatible, 1 incompatible
Preview: auto update
- [generated_surface] retain .agents/plugins/marketplace.json (codex) — managed checksum is unchanged and would be retained
- [runtime_state] update .autopus/antigravity-cli-manifest.json (antigravity-cli) — manifest diff would record 4 emit, 198 retain, 0 prune actions
- [runtime_state] update .autopus/claude-code-manifest.json (claude-code) — manifest diff would record 4 emit, 93 retain, 0 prune actions
- [runtime_state] update .autopus/codex-manifest.json (codex) — manifest diff would record 7 emit, 104 retain, 10 prune actions
- [runtime_state] update .autopus/opencode-manifest.json (opencode) — manifest diff would record 3 emit, 117 retain, 0 prune actions
```

Interpretation:

- `.agents/plugins/marketplace.json` remains deterministically regenerable from the Codex plugin marketplace output.
- Current `autopus.yaml` platforms are `claude-code`, `codex`, `antigravity-cli`, and `opencode`. The current regenerate target for Gemini-family output is `.autopus/antigravity-cli-manifest.json`; the tracked `.autopus/gemini-cli-manifest.json` removed in this slice is legacy generated metadata already outside the current platform regenerate path.
- The preview records manifest diff actions for the current generated manifest family without mutating the working tree.
- `config.toml` is local runtime config, not a deterministic platform manifest target. It was already missing before the slice and remains absent after the index cleanup.

No-write check after preview:

```bash
git diff --cached --name-status
```

Result remains limited to the intended 7 staged paths:

```text
D	.agents/plugins/marketplace.json
D	.autopus/claude-code-manifest.json
D	.autopus/codex-manifest.json
D	.autopus/gemini-cli-manifest.json
D	.autopus/opencode-manifest.json
M	.autopus/specs/SPEC-ADK-GENERATED-HYGIENE-001/research.md
D	config.toml
```

Next cleanup family remains: SPEC review evidence (`.autopus/specs/**/review.md`, `.autopus/specs/**/review-findings.json`) or plugin generated surface (`.autopus/plugins/**`), depending on desired blast radius.

## Path Family Mapping

| Generated family | Canonical source owner | Cleanup stance |
|------------------|------------------------|----------------|
| `.codex/agents/**` | `content/agents/**`, `templates/codex/agents/**`, `pkg/adapter/codex/**` | generated output |
| `.codex/skills/**` | `content/skills/**`, `templates/codex/skills/**`, `pkg/adapter/codex/**`, `pkg/content/**` | generated output |
| `.codex/rules/**` | `content/rules/**`, `templates/codex/rules/**`, `pkg/adapter/codex/**` | generated output |
| `.codex/prompts/**` | `templates/codex/prompts/**` | generated output |
| `.claude/**` | `content/**`, `templates/claude/**`, `pkg/adapter/claude/**`, `pkg/content/**` | generated output |
| `.gemini/**` | `content/**`, `templates/gemini/**`, `pkg/adapter/gemini/**`, `pkg/content/**` | generated output |
| `.opencode/**` | `content/**`, `templates/opencode/**`, `pkg/adapter/opencode/**`, `pkg/content/**` | generated output |
| `.autopus/*-manifest.json` | setup/render pipeline output | generated output |
| `.autopus/plugins/**` | plugin/templates/content output | generated output |
| `.autopus/context/signatures.md` | setup context scan output | generated output |
| `.autopus/txns/**` | runtime transaction journal | never commit |
| `.autopus/brainstorms/**` | runtime brainstorm output | never commit |
| `.autopus/orchestra/**` | runtime orchestra output | never commit |
| `.autopus/runtime/**` | runtime projection/cache | never commit |
| `.autopus/project/**` | human project knowledge | keep tracked |
| `.autopus/specs/**` | human SPEC knowledge | keep tracked |
| `.autopus/learnings/pipeline.jsonl` | sanitized learning store | keep only redacted entries |
| `config.toml` | local runtime config | generated/local, do not commit |
| `.autopus/qa/journeys/*.yaml` | reviewed Journey Pack candidate | policy decision required |
| `.autopus/specs/**/review.md` | generated review evidence | separate from canonical SPEC docs |
| `.autopus/specs/**/review-findings.json` | generated review evidence | separate from canonical SPEC docs |
| `pkg/**/.gemini/**` fixtures | adapter/source test fixtures | ambiguous, do not bulk-untrack |

## Design Notes

### D1: Observability Before Deletion

Doctor/status panels should make the hygiene state visible before CI and cleanup start blocking broad categories. This is especially important in `autopus-adk` because dogfood generated output may currently be tracked for historical reasons.

### D2: Runtime Artifacts Have No Source Exception

Runtime files are not deterministic generated platform surface. A source/template change cannot justify committing `.autopus/txns/**` or `.autopus/runtime/**`, so these families should always be blocked or reported as high-severity cleanup targets.

### D3: Broad Source Prefix Is Too Permissive

`content/**` changing should not automatically permit every generated path. The hygiene gate needs path-family owner mapping, such as `.codex/agents/**` matching `content/agents/**` or `templates/codex/agents/**`, not unrelated workflow content.

### D4: Dogfood Generated Surface Needs Regenerate Evidence

Before untracking platform generated files, each family needs a command that proves local generated output can be rebuilt from source. Candidate verification:

```bash
go run ./cmd/auto update --dir . --yes --preview
go run ./cmd/auto workflow render --dry-run
go test ./pkg/content ./pkg/adapter/... ./internal/cli -count=1
```

Exact commands may change as setup/update surfaces are stabilized; the cleanup plan should record the command used for each path family.

## Open Questions

- Should ADK dogfood generated output be entirely untracked, or should a small reviewed bootstrap subset remain tracked for self-hosting?
- Should `AGENTS.md`, `CLAUDE.md`, `GEMINI.md`, and `opencode.json` remain tracked as reviewed bootstrap config even when generated by setup/update?
- Should `auto check --hygiene` expose a JSON mode so CI can publish structured path-family summaries?
- Which command is the canonical regenerate check for all platform surfaces: `auto update --preview`, content generator tests, or a new explicit dry-run command?
- Should `.autopus/learnings/pipeline.jsonl` remain curated knowledge, or become runtime-only learning output?
- Should `.autopus/qa/journeys/adk-go-fast.yaml` remain as a reviewed Journey Pack source file?
