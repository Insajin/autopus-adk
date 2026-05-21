# auto-setup — 프로젝트 컨텍스트 생성 스킬

## Workspace Folder Boundary

ADK는 workspace folder profile의 소유자가 아니라 workspace 또는 repo checkout 안에 설치되는 harness/execution layer입니다. `auto setup`은 루트 역할과 추적 정책을 문서화하지만, canonical Knowledge Hub의 folder identity, promotion, admission 정책을 ADK generated surface로 대체하지 않습니다.

Generated/runtime/harness surfaces are excluded from canonical Knowledge Hub indexing unless they are explicitly human-managed project/spec documents:
- Excluded: `.autopus/runtime/**`, `.autopus/qa/**` raw artifacts, `.autopus/context/signatures.md`, `.autopus/*-manifest.json`, `.autopus/plugins/**`, `.autopus/orchestra/**`, `.autopus/brainstorms/**`, `.codex/**`, `.claude/**`, `.gemini/**`, `.opencode/**`, `.agents/plugins/**`, `.symphony/artifacts/**`, `config.toml`
- Indexable local docs: `.autopus/project/**`, `.autopus/specs/**`, `.autopus/vault/**`, `README.md`, `docs/**`
- Candidate/projection only: `.autopus/inbox/**` requires promotion; sanitized `.autopus/learnings/pipeline.jsonl` rows remain ADK Decision/Quality Index projection evidence.

`auto setup` 결과 문서에는 ADK가 harness layer라는 경계, generated/runtime surface의 source-of-truth repo, root tracking keep/drop policy, 그리고 canonical Knowledge Hub에서 제외되는 경로를 명시해야 합니다.
