<!-- AUTOPUS:BEGIN -->
# Autopus-ADK Harness

> 이 섹션은 Autopus-ADK에 의해 자동 생성됩니다. 수동으로 편집하지 마세요.

- **프로젝트**: autopus-adk
- **모드**: full

## 스킬 디렉터리

- Gemini Skills: .gemini/skills/
- Cross-platform: .agents/skills/

## Core Guidelines

### Subagent Delegation

IMPORTANT: Use subagents for complex tasks that modify 3+ files, span multiple domains, or exceed 200 lines of new code. Define clear scope, provide full context, review output before integrating.

### File Size Limit

IMPORTANT: No source code file may exceed 300 lines. Target under 200 lines. Split by type, concern, or layer when approaching the limit. Excluded: generated files (*_generated.go, *.pb.go), documentation (*.md), and config files (*.yaml, *.json).

### Code Review

During review, verify:
- No file exceeds 300 lines (REQUIRED)
- Complex changes use subagent delegation (SUGGESTED)

## Rules

@.gemini/rules/autopus/lore-commit.md
@.gemini/rules/autopus/file-size-limit.md
@.gemini/rules/autopus/subagent-delegation.md
@.gemini/rules/autopus/language-policy.md

<!-- AUTOPUS:END -->
