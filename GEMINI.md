<!-- AUTOPUS:BEGIN -->
# Autopus-ADK Harness

> 이 섹션은 Autopus-ADK에 의해 자동 생성됩니다. 수동으로 편집하지 마세요.

- **프로젝트**: autopus-adk
- **모드**: full

## 스킬 디렉터리

- Legacy Gemini-compatible: .gemini/skills/
- Antigravity workspace plugin: .agents/plugins/autopus/
- Antigravity workspace hooks: .agents/hooks.json

## Core Guidelines

### Subagent Delegation

IMPORTANT: Use subagents for complex tasks that modify 3+ files, span multiple domains, or exceed 200 lines of new code. Define clear scope, provide full context, review output before integrating.

### File Size Limit

IMPORTANT: No source code file may exceed 300 lines. Target under 200 lines. Split source code by type, concern, or layer when approaching the limit. SPEC Markdown files under .autopus/specs/** are documentation and exempt from the 300-line source code limit. Excluded: generated files (*_generated.go, *.pb.go), documentation (*.md), and config files (*.yaml, *.json).

### Code Review

During review, verify:
- No source code file exceeds 300 lines (REQUIRED)
- SPEC Markdown files under .autopus/specs/** are not split or rejected for line count alone
- Complex changes use subagent delegation (SUGGESTED)

## Rules

@.agents/plugins/autopus/rules/lore-commit.md
@.agents/plugins/autopus/rules/file-size-limit.md
@.agents/plugins/autopus/rules/subagent-delegation.md
@.agents/plugins/autopus/rules/language-policy.md
@.agents/plugins/autopus/rules/techstack-freshness.md

<!-- AUTOPUS:END -->
