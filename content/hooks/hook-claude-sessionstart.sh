#!/bin/sh
# hook-claude-sessionstart.sh — Claude Code SessionStart hook for autopus.
# Writes a ready-signal file so the orchestrator can detect that the interactive
# provider session is live via file IPC instead of screen scraping the prompt
# (SPEC-ORCH-022). No-op unless AUTOPUS_SESSION_ID is set by the orchestrator.
# POSIX shell compatible. Does not read or require stdin.
set -e

SESSION_ID="${AUTOPUS_SESSION_ID:-}"
if [ -z "$SESSION_ID" ]; then
  exit 0
fi

# Validate session ID to prevent path traversal (alphanumeric, hyphen, underscore only).
case "$SESSION_ID" in
  *[!a-zA-Z0-9_-]*) exit 0 ;;
esac

SESSION_DIR="${AUTOPUS_SESSION_DIR:-/tmp/autopus/${SESSION_ID}}"
if [ ! -d "$SESSION_DIR" ]; then
  exit 0
fi

# Round-scoped ready file name. Defaults to round 0 when AUTOPUS_ROUND is unset,
# matching RoundSignalName(provider, 0, "ready") on the Go side.
case "${AUTOPUS_ROUND:-}" in *[!0-9]*) AUTOPUS_ROUND="" ;; esac
ROUND="${AUTOPUS_ROUND:-0}"
READY_FILE="${SESSION_DIR}/claude-round${ROUND}-ready"

: > "${READY_FILE}"
chmod 600 "${READY_FILE}" 2>/dev/null || true
exit 0
