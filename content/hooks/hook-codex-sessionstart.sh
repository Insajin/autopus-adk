#!/bin/sh
# hook-codex-sessionstart.sh — codex session-start hook for autopus.
# Writes a ready-signal file so the orchestrator detects session readiness via
# file IPC instead of screen scraping (SPEC-ORCH-022). No-op unless
# AUTOPUS_SESSION_ID is set. POSIX shell; does not require stdin.
set -e

SESSION_ID="${AUTOPUS_SESSION_ID:-}"
if [ -z "$SESSION_ID" ]; then
  exit 0
fi
case "$SESSION_ID" in
  *[!a-zA-Z0-9_-]*) exit 0 ;;
esac
SESSION_DIR="${AUTOPUS_SESSION_DIR:-/tmp/autopus/${SESSION_ID}}"
if [ ! -d "$SESSION_DIR" ]; then
  exit 0
fi
case "${AUTOPUS_ROUND:-}" in *[!0-9]*) AUTOPUS_ROUND="" ;; esac
ROUND="${AUTOPUS_ROUND:-0}"
READY_FILE="${SESSION_DIR}/codex-round${ROUND}-ready"
: > "${READY_FILE}"
chmod 600 "${READY_FILE}" 2>/dev/null || true
exit 0
