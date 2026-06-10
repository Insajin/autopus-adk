#!/bin/sh
# hook-claude-stop.sh — Claude Code Stop hook for autopus result collection.
# Reads hook JSON from stdin, extracts last_assistant_message,
# writes result.json and done signal to the session directory.
# POSIX shell compatible. No jq dependency — uses python3 for JSON.
set -e

SESSION_ID="${AUTOPUS_SESSION_ID:-}"
if [ -z "$SESSION_ID" ]; then
  exit 0
fi

# Validate session ID to prevent path traversal (alphanumeric, hyphen, underscore only)
case "$SESSION_ID" in
  *[!a-zA-Z0-9_-]*) exit 0 ;;
esac

SESSION_DIR="${AUTOPUS_SESSION_DIR:-/tmp/autopus/${SESSION_ID}}"
if [ ! -d "$SESSION_DIR" ]; then
  exit 0
fi

# Determine round-scoped file names when AUTOPUS_ROUND is set (integer-only).
case "${AUTOPUS_ROUND:-}" in *[!0-9]*) AUTOPUS_ROUND="" ;; esac
if [ -n "$AUTOPUS_ROUND" ]; then
  RESULT_FILE="${SESSION_DIR}/claude-round${AUTOPUS_ROUND}-result.json"
  DONE_FILE="${SESSION_DIR}/claude-round${AUTOPUS_ROUND}-done"
else
  RESULT_FILE="${SESSION_DIR}/claude-result.json"
  DONE_FILE="${SESSION_DIR}/claude-done"
fi

# Read hook JSON from stdin and extract last_assistant_message via python3.
# Input is passed via stdin (not argv) to avoid shell injection.
python3 -c "
import json, sys
data = json.load(sys.stdin)
msg = data.get('last_assistant_message', '')
if not msg:
    sys.exit(0)
result = {'output': msg, 'exit_code': 0}
with open(sys.argv[1], 'w') as f:
    json.dump(result, f)
" "${RESULT_FILE}"

chmod 600 "${RESULT_FILE}"

# Write done signal (empty file).
: > "${DONE_FILE}"

# Send cmux completion signal for SignalDetector (SPEC-SURFCOMP-001 R8).
if command -v cmux >/dev/null 2>&1; then
  if [ -n "$AUTOPUS_ROUND" ] && [ "$AUTOPUS_ROUND" -gt 1 ] 2>/dev/null; then
    cmux wait-for -S "done-claude-round${AUTOPUS_ROUND}" 2>/dev/null || true
  else
    cmux wait-for -S "done-claude" 2>/dev/null || true
  fi
fi

# --- Bidirectional IPC: Ready signal + Input watch loop (SPEC-ORCH-017) ---
# Only activate for round-scoped sessions.
if [ -n "$AUTOPUS_ROUND" ]; then
  NEXT_ROUND=$((AUTOPUS_ROUND + 1))
  READY_FILE="${SESSION_DIR}/claude-round${NEXT_ROUND}-ready"
  INPUT_FILE="${SESSION_DIR}/claude-round${NEXT_ROUND}-input.json"
  ABORT_FILE="${SESSION_DIR}/claude-round${NEXT_ROUND}-abort"

  # Signal ready for next round input.
  : > "${READY_FILE}"

  # Poll for input file (200ms intervals, 120s timeout = 600 iterations).
  # @AX:NOTE [AUTO] magic constants 200ms/600 iterations — must match Go-side fileIPCReadyTimeout budget
  WAIT_COUNT=0
  MAX_WAIT=600
  while [ "$WAIT_COUNT" -lt "$MAX_WAIT" ]; do
    if [ -f "$ABORT_FILE" ]; then
      rm -f "${READY_FILE}" "${ABORT_FILE}"
      exit 0
    fi
    if [ -f "$INPUT_FILE" ]; then
      PROMPT=$(python3 -c "
import json, sys
with open(sys.argv[1]) as f:
    data = json.load(f)
print(data.get('prompt', ''))
" "${INPUT_FILE}") || PROMPT=""
      rm -f "${INPUT_FILE}" "${READY_FILE}"
      if [ -n "$PROMPT" ]; then
        printf '%s' "$PROMPT"
      fi
      exit 0
    fi
    python3 -c "import time; time.sleep(0.2)" || sleep 1
    WAIT_COUNT=$((WAIT_COUNT + 1))
  done

  # Timeout — clean up ready signal and exit normally.
  rm -f "${READY_FILE}"
fi
