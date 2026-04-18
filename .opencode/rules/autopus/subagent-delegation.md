---
name: subagent-delegation
description: Guidelines for delegating complex tasks to subagents
category: workflow
---

# Subagent Delegation

IMPORTANT: Use subagents for complex tasks. Do NOT implement large features in a single pass.

## When to Delegate

- Task modifies 3 or more files
- Task spans multiple domains (backend + frontend, etc.)
- Task requires specialized expertise (security, performance, database)
- Implementation will exceed 200 lines of new code

## How to Delegate

1. Define a clear, scoped task with expected output
2. Include all necessary context in the prompt
3. Review subagent output before integrating
4. Use parallel delegation for independent subtasks

## Anti-Patterns

- Do NOT delegate trivial tasks (typo fixes, single-line changes)
- Do NOT chain more than 3 subagents sequentially
- Do NOT delegate without sufficient context
