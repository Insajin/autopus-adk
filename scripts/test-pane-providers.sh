#!/usr/bin/env bash
# test-pane-providers.sh — 프로바이더별 interactive pane 개념검증 스크립트
# Usage: ./scripts/test-pane-providers.sh [provider]
# Provider: claude | gemini | opencode | all (default: all)

set -euo pipefail

PROVIDER="${1:-all}"
PROMPT="Say hello and nothing else. One sentence max."
WAIT_LAUNCH=15    # CLI 시작 대기 (초)
WAIT_RESPONSE=30  # 응답 대기 (초)
POLL_INTERVAL=2   # 폴링 간격 (초)

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m'

log() { echo -e "${YELLOW}[$(date +%H:%M:%S)]${NC} $1"; }
ok()  { echo -e "${GREEN}[OK]${NC} $1"; }
fail(){ echo -e "${RED}[FAIL]${NC} $1"; }

get_launch_cmd() {
  case "$1" in
    claude)   echo "claude --model opus --effort max --dangerously-skip-permissions" ;;
    gemini)   echo "gemini -m gemini-3.1-pro-preview" ;;
    opencode) echo "opencode -m openai/gpt-5.4" ;;
  esac
}

get_prompt_pattern() {
  case "$1" in
    claude)   echo "❯" ;;
    gemini)   echo "Type your" ;;
    opencode) echo "Ask anything" ;;
  esac
}

cleanup_panes() {
  for pane in "${PANES[@]:-}"; do
    cmux close-surface --surface "$pane" 2>/dev/null || true
  done
}
trap cleanup_panes EXIT

PANES=()

test_provider() {
  local name="$1"
  local cmd
  cmd=$(get_launch_cmd "$name")
  local pattern
  pattern=$(get_prompt_pattern "$name")

  echo ""
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  log "Testing: $name"
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

  # Step 1: Create pane
  log "Step 1: Creating pane..."
  local pane_out
  pane_out=$(cmux new-split right 2>&1)
  local pane
  pane=$(echo "$pane_out" | grep -o 'surface:[0-9]*')
  PANES+=("$pane")
  ok "Pane created: $pane"

  # Step 2: Wait for shell ready
  log "Step 2: Waiting 2s for shell..."
  sleep 2

  # Step 3: Launch CLI
  log "Step 3: Launching: $cmd"
  cmux send --surface "$pane" "$cmd"
  sleep 0.5
  cmux send --surface "$pane" $'\n'
  ok "Launch command sent"

  # Step 4: Wait for CLI prompt
  log "Step 4: Waiting for CLI prompt (pattern: '$pattern', max ${WAIT_LAUNCH}s)..."
  local elapsed=0
  local ready=false
  while [ $elapsed -lt $WAIT_LAUNCH ]; do
    sleep $POLL_INTERVAL
    elapsed=$((elapsed + POLL_INTERVAL))
    local screen
    screen=$(cmux read-screen --surface "$pane" 2>/dev/null || echo "")
    # Strip ANSI for matching
    local clean
    clean=$(echo "$screen" | sed 's/\x1b\[[0-9;]*[a-zA-Z]//g')
    if echo "$clean" | grep -q "$pattern"; then
      ok "CLI prompt detected after ${elapsed}s"
      ready=true
      break
    fi
    echo -n "  . (${elapsed}s) "
  done
  echo ""

  if [ "$ready" = false ]; then
    fail "CLI prompt NOT detected after ${WAIT_LAUNCH}s"
    log "Current screen content:"
    cmux read-screen --surface "$pane" 2>/dev/null | tail -5
    echo ""
    return 1
  fi

  # Step 5: Send prompt via set-buffer/paste-buffer
  log "Step 5: Sending prompt: '$PROMPT'"
  local buf_name="test-$$-$name"
  cmux set-buffer --name "$buf_name" "$PROMPT"
  cmux paste-buffer --name "$buf_name" --surface "$pane"
  sleep 0.5
  cmux send --surface "$pane" $'\n'
  ok "Prompt sent via buffer"

  # Step 6: Wait for response (poll for prompt reappearance)
  log "Step 6: Waiting for response (max ${WAIT_RESPONSE}s)..."
  elapsed=0
  local responded=false
  # Capture baseline immediately after sending
  sleep 3  # Give AI a head start
  while [ $elapsed -lt $WAIT_RESPONSE ]; do
    sleep $POLL_INTERVAL
    elapsed=$((elapsed + POLL_INTERVAL))
    local screen
    screen=$(cmux read-screen --surface "$pane" 2>/dev/null || echo "")
    local clean
    clean=$(echo "$screen" | sed 's/\x1b\[[0-9;]*[a-zA-Z]//g')
    # Check if prompt reappeared (means response complete)
    if echo "$clean" | grep -q "$pattern"; then
      # Verify it's not just the prompt we sent to
      local line_count
      line_count=$(echo "$clean" | grep -c "$pattern" || true)
      if [ "$line_count" -ge 1 ] && [ $elapsed -ge 5 ]; then
        ok "Response complete after ${elapsed}s"
        responded=true
        break
      fi
    fi
    echo -n "  . (${elapsed}s) "
  done
  echo ""

  # Step 7: Read final screen
  log "Step 7: Final screen capture"
  echo "--- BEGIN SCREEN ---"
  cmux read-screen --surface "$pane" --scrollback 2>/dev/null | tail -20
  echo ""
  echo "--- END SCREEN ---"

  if [ "$responded" = true ]; then
    ok "$name: PASS ✓"
  else
    fail "$name: Response not detected (timeout)"
  fi

  # Cleanup this pane
  cmux close-surface --surface "$pane" 2>/dev/null || true
  # Remove from array
  PANES=("${PANES[@]/$pane/}")

  echo ""
  return 0
}

echo "🐙 Interactive Pane Provider Test"
echo "================================="
echo "Prompt: $PROMPT"
echo ""

RESULTS=()

if [ "$PROVIDER" = "all" ]; then
  for p in opencode gemini claude; do
    if test_provider "$p"; then
      RESULTS+=("$p: ✓")
    else
      RESULTS+=("$p: ✗")
    fi
  done
else
  if test_provider "$PROVIDER"; then
    RESULTS+=("$PROVIDER: ✓")
  else
    RESULTS+=("$PROVIDER: ✗")
  fi
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Summary:"
for r in "${RESULTS[@]}"; do
  echo "  $r"
done
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
