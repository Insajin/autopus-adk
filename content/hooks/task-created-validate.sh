#!/usr/bin/env bash
# TaskCreated hook: validates title length and action verb allow-list.
# Exit 0 = pass (or warn-only fallthrough); Exit 2 = block (enforce mode).
set -euo pipefail

MODE="${AUTOPUS_TASKCREATED_FLAG_MODE:-${TASKCREATED_MODE:-${AUTOPUS_TASKCREATED_DEFAULT_MODE:-warn}}}"
LOG_FILE="${AUTOPUS_TASK_AUDIT_LOG:-.autopus/task-audit.log}"
VERBS="add update remove fix refactor rename move rewrite implement create delete clean check verify document migrate split merge extract simplify optimize"

case "$MODE" in
  warn|enforce) ;;
  *) MODE="warn" ;;
esac

if ! command -v jq &>/dev/null; then
  _ts=$(date -u +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || date +"%Y-%m-%dT%H:%M:%SZ")
  mkdir -p "$(dirname "$LOG_FILE")"
  printf '{"timestamp":"%s","outcome":"warn","mode":"%s","reason":"jq_missing","title":"","spec_id":null,"task_id":null}\n' \
    "$_ts" "$MODE" >> "$LOG_FILE"
  echo "[WARN] jq not found — task-created-validate skipped (fail-open)" >&2
  exit 0
fi

# Read stdin with 5-second per-line timeout (portable; avoids GNU timeout dependency)
RAW=""
while IFS= read -r -t 5 line || { [[ -n "$line" ]] && true; }; do
  RAW+="$line"
done
[[ -z "$RAW" ]] && RAW="{}"

TITLE=$(echo "$RAW" | jq -r '.task_subject // .title // ""' 2>/dev/null || true)
DESC=$(echo "$RAW" | jq -r '.task_description // .description // ""' 2>/dev/null || true)

# Extract SPEC-ID from title + description
SPEC_ID=""
COMBINED="$TITLE $DESC"
[[ "$COMBINED" =~ (SPEC-[A-Z][A-Z0-9]*-[0-9][0-9]*) ]] && SPEC_ID="${BASH_REMATCH[1]}"

# Append JSON Lines audit log entry
append_log() {
  local outcome="$1" reason="${2:-null}"
  local ts
  ts=$(date -u +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || date +"%Y-%m-%dT%H:%M:%SZ")
  local spec_json="null"
  [[ -n "$SPEC_ID" ]] && spec_json="\"$SPEC_ID\""
  local reason_json="null"
  [[ "$reason" != "null" ]] && reason_json="\"$reason\""
  local title_esc
  title_esc=$(printf '%s' "$TITLE" | sed 's/\\/\\\\/g; s/"/\\"/g')
  mkdir -p "$(dirname "$LOG_FILE")"
  printf '{"timestamp":"%s","outcome":"%s","mode":"%s","reason":%s,"title":"%s","spec_id":%s,"task_id":null}\n' \
    "$ts" "$outcome" "$MODE" "$reason_json" "$title_esc" "$spec_json" >> "$LOG_FILE"
}

# Determine failure reason (first match wins)
FAIL=""
[[ -z "$TITLE" ]] && FAIL="invalid_payload"
[[ -z "$FAIL" && ${#TITLE} -lt 5 ]] && FAIL="title_too_short"
if [[ -z "$FAIL" ]]; then
  FIRST="${TITLE%% *}"
  FIRST_LC=$(echo "$FIRST" | tr '[:upper:]' '[:lower:]')
  HIT=0
  for v in $VERBS; do
    [[ "$FIRST_LC" == "$v" ]] && HIT=1 && break
  done
  [[ $HIT -eq 0 ]] && FAIL="action_verb_not_allowed"
fi

# Pass
if [[ -z "$FAIL" ]]; then
  append_log "pass"
  exit 0
fi

# Fail — enforce blocks, warn logs and passes
if [[ "$MODE" == "enforce" ]]; then
  case "$FAIL" in
    title_too_short) echo "TaskCreated validation failed: title must be >=5 chars, got: \"$TITLE\"" >&2 ;;
    action_verb_not_allowed) echo "TaskCreated validation failed: action verb not in allow-list: \"${TITLE%% *}\"" >&2 ;;
    invalid_payload) echo "TaskCreated validation failed: invalid_payload (could not parse title from stdin JSON)" >&2 ;;
  esac
  append_log "fail" "$FAIL"
  exit 2
fi

append_log "warn" "$FAIL"
exit 0
