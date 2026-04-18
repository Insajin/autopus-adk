---
description: "보안 감사 — OWASP Top 10 관점에서 변경 범위를 점검합니다"
---

# auto-secure — Security Audit

## Autopus Branding

When handling this workflow, start the response with the canonical banner from `templates/shared/branding-formats.md.tmpl`:

```text
🐙 Autopus ─────────────────────────
```

End the completed response with `🐙`.


## 설명

보안 감사 — OWASP Top 10 관점에서 변경 범위를 점검합니다

## 실행 원칙

- Codex에서는 `spawn_agent(...)` 기반 subagent-first 로 진행합니다.
- 대상 path가 있으면 해당 범위를, 없으면 현재 프로젝트 루트를 분석합니다.

## 권장 서브에이전트 호출

```python
spawn_agent(
    agent_type="security-auditor",
    message="Audit the requested scope using OWASP Top 10 categories. Focus on exploitable risks, missing tests, and secrets exposure.",
)
```
