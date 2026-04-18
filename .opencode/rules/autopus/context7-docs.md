# Context7 Documentation Auto-Fetch

IMPORTANT: Before any technology/library/framework-related work, fetch the latest documentation via Context7 MCP. If Context7 MCP is unavailable, returns no match, or the docs query fails, fall back to targeted web search. Subagents cannot call MCP tools — the main session MUST fetch and inject docs into subagent prompts.

## When to Fetch

- `/auto go` pipeline: Phase 1.8 (Doc Fetch) — systematic fetch before implementation
- `/auto fix` debugging: when error involves an external library
- General technology work: when SPEC, code imports, or user description references an external library/framework

## Technology Detection

Scan the following sources for library/framework names:

1. **SPEC requirements** — library names in requirement text (e.g., "cobra", "fiber", "react")
2. **plan.md task descriptions** — tool/framework references in task details
3. **File imports** — `import` statements in affected packages (e.g., `github.com/spf13/cobra`)
4. **User descriptions** — explicit library/framework mentions in the request
5. **Error messages** — library-specific error patterns in `/auto fix` contexts

### Detection Heuristics

- Go: parse `import` blocks, match against `go.mod` dependencies
- Node.js: parse `import`/`require` statements, match against `package.json` dependencies
- Python: parse `import`/`from` statements, match against `requirements.txt`/`pyproject.toml`
- Skip standard library modules (e.g., `fmt`, `os`, `path` in Go; `fs`, `http` in Node.js)

## Fetch Procedure

For each detected technology (up to **5 per pipeline run**):

```
Step 1: resolve-library-id
  → Call `Context7 MCP resolve-library-id tool (fallback: web search)` with the library name
  → If no match found: go to Step 3 (web fallback)
  → If match found: extract the library ID

Step 2: query-docs
  → Call `Context7 MCP query-docs tool (fallback: web search)` with the resolved library ID
  → Specify topic relevant to the task context (e.g., API usage, configuration, migration)
  → If the query returns empty or error: go to Step 3 (web fallback)
  → Cache the result for this pipeline run

Step 3: web-fallback
  → Use the session web search capability with a task-specific query
  → Prefer official docs, release notes, migration guides, and API references
  → Cache the result for this pipeline run and mark it as a web fallback source
```

## Prompt Injection Format

Inject fetched documentation into subagent prompts as follows:

```
## Reference Documentation

The following documentation was fetched from Context7 for libraries used in this task.

### {Library Name} (via Context7)
{trimmed documentation content — max ~2000 tokens per library}

### {Library Name 2} (via Context7)
{trimmed documentation content}
```

### Adaptive Token Budget

Token budget adjusts based on the number of detected libraries:

| Libraries Detected | Per Library | Total Budget | Rationale |
|--------------------|-------------|--------------|-----------|
| 1 | ~5000 tokens | ~5000 | Single dependency — deep context available |
| 2 | ~3000 tokens | ~6000 | Moderate depth per library |
| 3 | ~2500 tokens | ~7500 | Balanced across libraries |
| 4-5 | ~2000 tokens | ~8000-10000 | Breadth over depth |

**Hard cap**: total injected docs MUST NOT exceed **10000 tokens** regardless of library count.

**Trimming priority** (what to keep when trimming):
1. API signatures and type definitions (highest priority)
2. Configuration examples and common patterns
3. Version-specific breaking changes and migration notes
4. Error handling patterns
5. Tutorials and introductory content (lowest — trim first)

### Task-Specific Topic Query

WHEN injecting docs into individual executor/tester prompts, THE SYSTEM SHALL tailor the `topic` parameter to the specific task:

```
Phase 1.8 (pipeline-level):
  → query-docs(libraryId, topic="API overview and core patterns")
  → Cache as "base docs"

Phase 2 (per-executor, optional refinement):
  → If task description mentions a specific API area (e.g., "routing", "middleware", "testing"),
    query-docs(libraryId, topic="{task-specific area}")
  → Merge with base docs, dedup, apply per-library token limit
  → This is OPTIONAL — only when the task clearly needs a different facet of the same library
```

Per-executor refinement queries count toward the pipeline's 5-library limit. A refinement for the same library consumes 1 additional query slot.

## Caching

- **Base docs** are fetched once per pipeline in Phase 1.8
- **Refinement docs** (per-executor) are cached per `{library-id}:{topic}` key
- All agents in the same pipeline share the cached results
- Cache is discarded when the pipeline completes
- If a refinement query returns the same content as base docs, skip injection to save tokens

## Limits

| Limit | Value | Rationale |
|-------|-------|-----------|
| Max libraries per pipeline | 5 | Token cost control |
| Max per-library tokens | ~2000-5000 (adaptive) | Scales with library count |
| Max total injected tokens | 10000 (hard cap) | Prevent prompt bloat |
| Max refinement queries | 3 per pipeline | Avoid excessive MCP calls |

## Error Handling

- `resolve-library-id` returns no match → log and continue with web fallback
- `query-docs` returns empty or error → log and continue with web fallback
- MCP server unavailable → log warning and continue with web fallback
- Web search also fails → log and skip, do NOT block the pipeline
- Never retry MCP calls more than once per library

## Anti-Patterns

- Do NOT let subagents call MCP tools directly — they cannot access them
- Do NOT fetch documentation for standard library modules
- Do NOT fetch more than 5 libraries per pipeline run
- Do NOT inject full documentation without trimming — always respect token budgets
- Do NOT skip directly to web search when Context7 MCP is available and applicable
- Do NOT block the pipeline on Context7 failures — documentation is supplementary, not critical

## Ref

SPEC-CTX7-001
