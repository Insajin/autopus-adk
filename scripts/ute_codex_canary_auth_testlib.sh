#!/usr/bin/env bash

run_primary_reuse_tests() {
  local state="$TMP_ROOT/state-primary-reuse" arbitrary_state="$TMP_ROOT/state-primary-arbitrary"
  local arbitrary_output="$TMP_ROOT/output-primary-arbitrary" before
  HARNESS=$PRIMARY_HARNESS
  mkdir -p "$state" "$arbitrary_state" "$arbitrary_output"
  before=$(invocation_count "$COUNT_FILE")
  if FAKE_AUTO_COUNT_FILE="$COUNT_FILE" UTE_CODEX_CANARY_EXECUTE=YES \
    "$HARNESS" primary --repo "$SNAPSHOT" --auto "$FAKE_AUTO" --state "$state" \
    --output "$PRIMARY_OUTPUT" >/dev/null 2>&1; then
    fail "completed primary authorization was reused"
  fi
  assert_eq "$before" "$(invocation_count "$COUNT_FILE")" "completed primary reuse invoked auto"
  if FAKE_AUTO_COUNT_FILE="$COUNT_FILE" UTE_CODEX_CANARY_EXECUTE=YES \
    "$HARNESS" primary --repo "$SNAPSHOT" --auto "$FAKE_AUTO" --state "$arbitrary_state" \
    --output "$arbitrary_output" >/dev/null 2>&1; then
    fail "noncanonical fresh output was accepted"
  fi
  assert_eq "$before" "$(invocation_count "$COUNT_FILE")" "noncanonical output invoked auto"
}

run_rollback_tamper_tests() {
  local evidence="$REPO_ROOT/.autopus/specs/SPEC-ADK-ULTRA-EFFICIENCY-001/evidence"
  local tampered="$TMP_ROOT/tampered-primary.json" receipt="$TMP_ROOT/tampered-primary-receipt.json"
  local state="$TMP_ROOT/state-primary-tamper" output hash before
  HARNESS=$PRIMARY_HARNESS; output=$PRIMARY_OUTPUT
  jq '.calls[0].arm = "B"' "$PRIMARY_LEDGER" > "$tampered"
  write_named_sidecar "$tampered"
  write_applied_receipt "$receipt" "$tampered" "$evidence"
  hash="sha256:$(shasum -a 256 "$receipt" | awk '{print $1}')"
  mkdir -p "$state"; before=$(invocation_count "$COUNT_FILE")
  if FAKE_AUTO_COUNT_FILE="$COUNT_FILE" UTE_CODEX_CANARY_EXECUTE=YES \
    "$HARNESS" rollback --repo "$SNAPSHOT" --auto "$FAKE_AUTO" --state "$state" --output "$output" \
    --primary-ledger "$tampered" --rollback-receipt "$receipt" --rollback-receipt-hash "$hash" >/dev/null 2>&1; then
    fail "tampered primary ledger was accepted"
  fi
  assert_eq "$before" "$(invocation_count "$COUNT_FILE")" "tampered ledger invoked auto"

  state="$TMP_ROOT/state-receipt-tamper"; receipt="$TMP_ROOT/tampered-applied-receipt.json"
  write_applied_receipt "$receipt" "$PRIMARY_LEDGER" "$evidence"
  jq '.atomic_replace = false' "$receipt" > "$receipt.tmp"; mv "$receipt.tmp" "$receipt"
  hash="sha256:$(shasum -a 256 "$receipt" | awk '{print $1}')"
  mkdir -p "$state"
  if FAKE_AUTO_COUNT_FILE="$COUNT_FILE" UTE_CODEX_CANARY_EXECUTE=YES \
    "$HARNESS" rollback --repo "$SNAPSHOT" --auto "$FAKE_AUTO" --state "$state" --output "$output" \
    --primary-ledger "$PRIMARY_LEDGER" --rollback-receipt "$receipt" --rollback-receipt-hash "$hash" >/dev/null 2>&1; then
    fail "tampered applied rollback receipt was accepted"
  fi
  assert_eq "$before" "$(invocation_count "$COUNT_FILE")" "tampered receipt invoked auto"
}

run_parallel_reservation_test() {
  local state1="$TMP_ROOT/state-parallel-1" state2="$TMP_ROOT/state-parallel-2" output pid status i
  make_harness_install parallel; output=$CANONICAL_OUTPUT
  mkdir -p "$state1" "$state2"; : > "$COUNT_FILE"
  FAKE_AUTO_COUNT_FILE="$COUNT_FILE" FAKE_AUTO_FAIL_AT=1 FAKE_AUTO_DELAY_SECONDS=2 UTE_CODEX_CANARY_EXECUTE=YES \
    "$HARNESS" primary --repo "$SNAPSHOT" --auto "$FAKE_AUTO" --state "$state1" --output "$output" >/dev/null 2>&1 &
  pid=$!
  for i in $(seq 1 200); do
    [[ "$(invocation_count "$COUNT_FILE")" == 1 ]] && break
    kill -0 "$pid" 2>/dev/null || break
    sleep 0.05
  done
  assert_eq "1" "$(invocation_count "$COUNT_FILE")" "first parallel call did not start"
  if FAKE_AUTO_COUNT_FILE="$COUNT_FILE" UTE_CODEX_CANARY_EXECUTE=YES \
    "$HARNESS" primary --repo "$SNAPSHOT" --auto "$FAKE_AUTO" --state "$state2" --output "$output" >/dev/null 2>&1; then
    fail "concurrent primary reservation was accepted"
  fi
  assert_eq "1" "$(invocation_count "$COUNT_FILE")" "concurrent launch made an extra call"
  set +e; wait "$pid"; status=$?; set -e
  [[ "$status" != 0 ]] || fail "parallel failure fixture unexpectedly passed"
  assert_eq "1" "$(invocation_count "$COUNT_FILE")" "parallel reservation retried"
}
