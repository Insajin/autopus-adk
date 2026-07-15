#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
# shellcheck source=ute_gpt_diag_canary_testlib.sh
source "$SCRIPT_DIR/ute_gpt_diag_canary_testlib.sh"

TMP_ROOT=$(mktemp -d "${TMPDIR:-/tmp}/ute-gpt-diag-test.XXXXXX")
trap 'rm -rf "$TMP_ROOT"' EXIT
REPO_ROOT=$(cd "$SCRIPT_DIR/.." && pwd -P)
COUNT_FILE="$TMP_ROOT/invocations"
FAKE_AUTO="$TMP_ROOT/auto"
FAKE_CODEX="$TMP_ROOT/codex"
SNAPSHOT="$TMP_ROOT/snapshot.git"
HARNESS= CANONICAL_OUTPUT= INSTALL_ROOT=

make_diag_fake_auto "$FAKE_AUTO"
make_diag_fake_codex "$FAKE_CODEX"
export PATH="$TMP_ROOT:$PATH"
git clone -q --bare "$REPO_ROOT" "$SNAPSHOT"

run_preflight() {
  make_diag_install preflight
  : > "$COUNT_FILE"
  FAKE_DIAG_COUNT_FILE="$COUNT_FILE" "$HARNESS" preflight --repo "$SNAPSHOT" >/dev/null
  diag_assert_eq 0 "$(diag_count "$COUNT_FILE")" "preflight invoked auto"
}

run_static_tamper() {
  local config
  make_diag_install static-tamper
  config="$CANONICAL_OUTPUT/gpt-diagnostic-config-v1.json"
  printf '\n' >> "$config"
  : > "$COUNT_FILE"
  if FAKE_DIAG_COUNT_FILE="$COUNT_FILE" "$HARNESS" preflight --repo "$SNAPSHOT" >/dev/null 2>&1; then
    diag_fail "tampered diagnostic config accepted"
  fi
  diag_assert_eq 0 "$(diag_count "$COUNT_FILE")" "static tamper invoked auto"
}

run_complete() {
  local state="$TMP_ROOT/state-complete"
  make_diag_install complete; mkdir "$state"; : > "$COUNT_FILE"
  FAKE_DIAG_COUNT_FILE="$COUNT_FILE" UTE_GPT_DIAG_EXECUTE=YES \
    "$HARNESS" run --repo "$SNAPSHOT" --auto "$FAKE_AUTO" --state "$state" \
    --output "$CANONICAL_OUTPUT" > "$TMP_ROOT/progress"
  diag_assert_eq 10 "$(diag_count "$COUNT_FILE")" "diagnostic call count"
  diag_assert_ledger "$CANONICAL_OUTPUT/gpt-diagnostic-call-ledger-v1.json"
  diag_assert_progress "$TMP_ROOT/progress" 10
  diag_assert_no_raw "$state" "$CANONICAL_OUTPUT"
  [[ -f "$INSTALL_ROOT/.autopus/runtime/ute-diagnostic-authorizations/920e6370cebb84739872233cd4a0eeb88295bf816b19b6d43cfac99591a1dc20/reservation.json" ]] ||
    diag_fail "diagnostic authorization claim missing"
  [[ ! -e "$INSTALL_ROOT/.autopus/runtime/ute-authorizations/920e6370cebb84739872233cd4a0eeb88295bf816b19b6d43cfac99591a1dc20" ]] ||
    diag_fail "diagnostic authorization shared primary namespace"
  COMPLETE_HARNESS=$HARNESS; COMPLETE_OUTPUT=$CANONICAL_OUTPUT; COMPLETE_INSTALL=$INSTALL_ROOT
}

run_complete_reuse() {
  local state="$TMP_ROOT/state-complete-reuse" arbitrary_state="$TMP_ROOT/state-complete-arbitrary"
  local arbitrary_output="$TMP_ROOT/output-complete-arbitrary" before
  HARNESS=$COMPLETE_HARNESS; mkdir "$state" "$arbitrary_state" "$arbitrary_output"
  before=$(diag_count "$COUNT_FILE")
  if FAKE_DIAG_COUNT_FILE="$COUNT_FILE" UTE_GPT_DIAG_EXECUTE=YES \
    "$HARNESS" run --repo "$SNAPSHOT" --auto "$FAKE_AUTO" --state "$state" --output "$COMPLETE_OUTPUT" >/dev/null 2>&1; then
    diag_fail "complete authorization reused"
  fi
  diag_assert_eq "$before" "$(diag_count "$COUNT_FILE")" "complete reuse invoked auto"
  if FAKE_DIAG_COUNT_FILE="$COUNT_FILE" UTE_GPT_DIAG_EXECUTE=YES \
    "$HARNESS" run --repo "$SNAPSHOT" --auto "$FAKE_AUTO" --state "$arbitrary_state" --output "$arbitrary_output" >/dev/null 2>&1; then
    diag_fail "noncanonical output accepted"
  fi
  diag_assert_eq "$before" "$(diag_count "$COUNT_FILE")" "noncanonical output invoked auto"
}

run_partial_reuse_and_reconcile() {
  local state="$TMP_ROOT/state-partial" output partial before claim moved next
  make_diag_install partial; output=$CANONICAL_OUTPUT; mkdir "$state"; : > "$COUNT_FILE"
  if FAKE_DIAG_COUNT_FILE="$COUNT_FILE" FAKE_DIAG_PROCESS_FAIL_AT=4 UTE_GPT_DIAG_EXECUTE=YES \
    "$HARNESS" run --repo "$SNAPSHOT" --auto "$FAKE_AUTO" --state "$state" --output "$output" >/dev/null 2>&1; then
    diag_fail "operational failure accepted"
  fi
  diag_assert_eq 4 "$(diag_count "$COUNT_FILE")" "partial retry count"
  partial="$output/gpt-diagnostic-call-ledger-v1.partial-fail.json"
  jq -e '.completed == false and .diagnostic_only == true and .promotion_eligible == false and .attempted_calls == 4' "$partial" >/dev/null
  before=$(diag_count "$COUNT_FILE"); next="$TMP_ROOT/state-partial-reuse"; mkdir "$next"
  if FAKE_DIAG_COUNT_FILE="$COUNT_FILE" UTE_GPT_DIAG_EXECUTE=YES \
    "$HARNESS" run --repo "$SNAPSHOT" --auto "$FAKE_AUTO" --state "$next" --output "$output" >/dev/null 2>&1; then
    diag_fail "partial authorization reused"
  fi
  diag_assert_eq "$before" "$(diag_count "$COUNT_FILE")" "partial reuse invoked auto"

  claim="$INSTALL_ROOT/.autopus/runtime/ute-diagnostic-authorizations/920e6370cebb84739872233cd4a0eeb88295bf816b19b6d43cfac99591a1dc20"
  rm -rf "$claim"; next="$TMP_ROOT/state-partial-reconcile"; mkdir "$next"
  if FAKE_DIAG_COUNT_FILE="$COUNT_FILE" UTE_GPT_DIAG_EXECUTE=YES \
    "$HARNESS" run --repo "$SNAPSHOT" --auto "$FAKE_AUTO" --state "$next" --output "$output" >/dev/null 2>&1; then
    diag_fail "legacy partial accepted"
  fi
  jq -e '.kind == "diagnostic_consumption_reconciliation" and .source_ledger_valid == true and .observed_calls == 4' "$claim/reservation.json" >/dev/null
  moved="$TMP_ROOT/moved-partial"; mkdir "$moved"; mv "$partial" "${partial%.json}.sha256" "$moved/"
  next="$TMP_ROOT/state-partial-moved"; mkdir "$next"
  if FAKE_DIAG_COUNT_FILE="$COUNT_FILE" UTE_GPT_DIAG_EXECUTE=YES \
    "$HARNESS" run --repo "$SNAPSHOT" --auto "$FAKE_AUTO" --state "$next" --output "$output" >/dev/null 2>&1; then
    diag_fail "moved partial released authorization"
  fi
  diag_assert_eq "$before" "$(diag_count "$COUNT_FILE")" "moved partial invoked auto"
}

run_broken_output_target() {
  local state="$TMP_ROOT/state-broken-target" outside="$TMP_ROOT/outside-sidecar"
  make_diag_install broken-target; mkdir "$state"; : > "$COUNT_FILE"
  ln -s "$outside" "$CANONICAL_OUTPUT/gpt-diagnostic-call-ledger-v1.sha256"
  if FAKE_DIAG_COUNT_FILE="$COUNT_FILE" UTE_GPT_DIAG_EXECUTE=YES \
    "$HARNESS" run --repo "$SNAPSHOT" --auto "$FAKE_AUTO" --state "$state" \
    --output "$CANONICAL_OUTPUT" >/dev/null 2>&1; then
    diag_fail "broken output symlink accepted"
  fi
  diag_assert_eq 0 "$(diag_count "$COUNT_FILE")" "broken output target invoked auto"
  [[ ! -e "$outside" ]] || diag_fail "broken output symlink followed"
}

run_scope_and_concurrency() {
  local state="$TMP_ROOT/state-scope" output before s1 s2 pid status i
  make_diag_install scope; output=$CANONICAL_OUTPUT; mkdir "$state"; : > "$COUNT_FILE"
  if FAKE_DIAG_COUNT_FILE="$COUNT_FILE" FAKE_DIAG_BAD_SCOPE_AT=3 UTE_GPT_DIAG_EXECUTE=YES \
    "$HARNESS" run --repo "$SNAPSHOT" --auto "$FAKE_AUTO" --state "$state" --output "$output" >/dev/null 2>&1; then
    diag_fail "out-of-scope hash accepted"
  fi
  diag_assert_eq 3 "$(diag_count "$COUNT_FILE")" "scope failure retried"

  make_diag_install parallel; output=$CANONICAL_OUTPUT; s1="$TMP_ROOT/state-parallel-1"; s2="$TMP_ROOT/state-parallel-2"
  mkdir "$s1" "$s2"; : > "$COUNT_FILE"
  FAKE_DIAG_COUNT_FILE="$COUNT_FILE" FAKE_DIAG_PROCESS_FAIL_AT=1 FAKE_DIAG_DELAY=2 UTE_GPT_DIAG_EXECUTE=YES \
    "$HARNESS" run --repo "$SNAPSHOT" --auto "$FAKE_AUTO" --state "$s1" --output "$output" >/dev/null 2>&1 & pid=$!
  for i in $(seq 1 40); do [[ "$(diag_count "$COUNT_FILE")" == 1 ]] && break; sleep 0.05; done
  before=$(diag_count "$COUNT_FILE"); diag_assert_eq 1 "$before" "parallel first call"
  if FAKE_DIAG_COUNT_FILE="$COUNT_FILE" UTE_GPT_DIAG_EXECUTE=YES \
    "$HARNESS" run --repo "$SNAPSHOT" --auto "$FAKE_AUTO" --state "$s2" --output "$output" >/dev/null 2>&1; then
    diag_fail "concurrent authorization accepted"
  fi
  diag_assert_eq "$before" "$(diag_count "$COUNT_FILE")" "concurrent extra call"
  set +e; wait "$pid"; status=$?; set -e; [[ "$status" != 0 ]] || diag_fail "parallel fixture passed"
}

run_preflight
run_static_tamper
run_complete
run_complete_reuse
run_partial_reuse_and_reconcile
run_broken_output_target
run_scope_and_concurrency
printf '%s\n' 'ute gpt diagnostic canary hermetic tests: PASS'
