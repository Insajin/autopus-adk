---
name: auto-setup
description: 프로젝트 컨텍스트 생성 — 코드베이스를 분석하고 ARCHITECTURE.md 및 .autopus/project 문서를 생성합니다
category: workflow
---

# auto-setup — 프로젝트 컨텍스트 생성 스킬

## Workspace Folder Boundary

ADK는 workspace folder profile의 소유자가 아니라 workspace 또는 repo checkout 안에 설치되는 harness/execution layer입니다. `auto setup`은 루트 역할과 추적 정책을 문서화하지만, canonical Knowledge Hub의 folder identity, promotion, admission 정책을 ADK generated surface로 대체하지 않습니다.

Generated/runtime/harness surfaces are excluded from canonical Knowledge Hub indexing unless they are explicitly human-managed project/spec documents:
- Excluded: `.autopus/runtime/**`, `.autopus/qa/{runs,cache,gui,feedback,evidence,releases}/**` raw artifacts, `.autopus/context/signatures.md`, `.autopus/*-manifest.json`, `.autopus/plugins/**`, `.autopus/orchestra/**`, `.autopus/brainstorms/**`, `.autopus/design/{imports,verify}/**`, `.autopus/canary/**`, `.codex/**`, `.claude/**`, `.gemini/**`, `.opencode/**`, `.agents/{skills,plugins,commands}/**`, `.agents/hooks.json`, `.symphony/artifacts/**`, `config.toml`
- Indexable local docs: `.autopus/project/**`, `.autopus/specs/**`, `.autopus/vault/**`, `README.md`, `docs/**`
- Candidate/projection only: `.autopus/inbox/**` requires promotion; sanitized `.autopus/learnings/pipeline.jsonl` rows remain ADK Decision/Quality Index projection evidence.

`auto setup` 결과 문서에는 ADK가 harness layer라는 경계, generated/runtime surface의 source-of-truth repo, root tracking keep/drop policy, canonical Knowledge Hub에서 제외되는 경로, QA/Journey Pack 대상 리포와 `auto qa init --project-dir <repo>` 명령을 명시해야 합니다.

## OpenCode Invocation

Use `/auto setup` 또는 `/auto setup --auto`로 실행합니다. OpenCode에서도 먼저 `explorer` agent로 코드베이스를 분석하고, 그 결과를 바탕으로 문서를 갱신합니다.

필수 산출물:
- `ARCHITECTURE.md`
- `.autopus/project/product.md`
- `.autopus/project/structure.md`
- `.autopus/project/tech.md`
- `.autopus/project/workspace.md`
- `.autopus/project/scenarios.md`
- `.autopus/project/canary.md`

`workspace.md`에는 root repo role, nested git repo boundaries, generated/runtime paths, tracking/commit policy, source-of-truth repo, QA/Journey Pack target repos, and `auto qa init --project-dir <repo>` command routing을 기록합니다. meta workspace라면 root `.autopus/qa/**`를 제품 Journey Pack 위치로 쓰지 말고 runtime/generated evidence로 분리합니다.
