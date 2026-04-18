---
name: language-policy
description: Language policy for code comments, commit messages, and AI responses
category: workflow
platform: codex
---

# Language Policy

IMPORTANT: Follow the configured language settings strictly for all work in the project.

## Configuration

Language policy is configured per-project with three independent settings:

| Setting | Controls | Example |
|---------|----------|---------|
| `code_comments` | Code comments, docstrings, inline documentation | `en` (English) |
| `commit_messages` | Git commit messages | `ko` (Korean) |
| `ai_responses` | AI agent responses to the user | `ko` (Korean) |

## Rules

- **Code comments**: Write all code comments, docstrings, and inline documentation in the configured language
- **Commit messages**: Write all git commit messages in the configured language
- **AI responses**: Respond to the user in the configured language

## Scope

This policy applies to ALL agents in the system. Each agent MUST check the project's language configuration before producing output.

## Defaults

When no language policy is explicitly configured:
- Code comments: English (`en`)
- Commit messages: English (`en`)
- AI responses: English (`en`)
