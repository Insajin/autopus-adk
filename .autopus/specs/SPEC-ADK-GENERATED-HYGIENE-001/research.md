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
