# Project Identity

IMPORTANT: Do NOT confuse the user's project (the product being built) with the `.autopus/` directory (ADK harness config for development tooling).

## Two Distinct Layers

| Layer | What it is | Location | Contains |
|-------|-----------|----------|----------|
| **The Product** | The user's actual project — the software being built | Source code directories (`src/`, `pkg/`, `cmd/`, `app/`, etc.) | Application code, business logic, APIs, tests |
| **`.autopus/` directory** | ADK harness config — development tooling | `.autopus/` at root or in each submodule | SPECs, brainstorms, profiles, project context docs |

## Rules

- `.autopus/specs/`, `.autopus/brainstorms/`, `.autopus/project/` are **development artifacts** managed by the ADK harness, not product features
- When asked "what does this project do?", describe the **product** (what the source code builds and delivers), not the ADK harness structure
- When comparing with other projects, compare **product capabilities**, not SPEC file structures or ADK config
- `product.md` describes what the product does; `.autopus/` structure describes how we develop it
- The actual product source code lives in source directories and submodules, not in `.autopus/`

## How to Identify the Product

1. Read `ARCHITECTURE.md` and `.autopus/project/product.md` for product description
2. Look at source code directories (`src/`, `pkg/`, `cmd/`, `internal/`, `app/`) for actual functionality
3. Check `go.mod`, `package.json`, or equivalent for project identity
4. Do NOT infer product features from `.autopus/` directory contents
