#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
# shellcheck source=ute_codex_canary_testlib.sh
source "$SCRIPT_DIR/ute_codex_canary_testlib.sh"
# shellcheck source=ute_codex_canary_auth_testlib.sh
source "$SCRIPT_DIR/ute_codex_canary_auth_testlib.sh"

TMP_ROOT=$(mktemp -d "${TMPDIR:-/tmp}/ute-codex-canary-test.XXXXXX")
trap 'rm -rf "$TMP_ROOT"' EXIT
REPO_ROOT=$(cd "$SCRIPT_DIR/.." && pwd -P)
HARNESS= CANONICAL_OUTPUT= HARNESS_INSTALL_ROOT=
COUNT_FILE="$TMP_ROOT/invocations"
FAKE_AUTO="$TMP_ROOT/auto"
FAKE_CODEX="$TMP_ROOT/codex"
SNAPSHOT="$TMP_ROOT/snapshot.git"

make_fake_auto "$FAKE_AUTO"
make_fake_codex "$FAKE_CODEX"
export PATH="$TMP_ROOT:$PATH"
git clone -q --bare "$REPO_ROOT" "$SNAPSHOT"

run_preflight_test() {
  make_harness_install preflight
  : > "$COUNT_FILE"
  FAKE_AUTO_COUNT_FILE="$COUNT_FILE" "$HARNESS" preflight --repo "$SNAPSHOT" >/dev/null
  assert_eq "0" "$(invocation_count "$COUNT_FILE")" "preflight invoked auto"
}

run_primary_test() {
  local state="$TMP_ROOT/state-primary" output
  make_harness_install primary
  output=$CANONICAL_OUTPUT
  mkdir -p "$state"
  : > "$COUNT_FILE"
  FAKE_AUTO_COUNT_FILE="$COUNT_FILE" UTE_CODEX_CANARY_EXECUTE=YES \
    "$HARNESS" primary --repo "$SNAPSHOT" --auto "$FAKE_AUTO" \
    --state "$state" --output "$output" >"$TMP_ROOT/primary.progress"
  assert_eq "58" "$(invocation_count "$COUNT_FILE")" "primary call count"
  assert_primary_ledger "$output/gpt-primary-call-ledger-v1.json"
  verify_progress "$TMP_ROOT/primary.progress" 58
  assert_no_raw_retention "$state" "$output"
  PRIMARY_LEDGER="$output/gpt-primary-call-ledger-v1.json"
  PRIMARY_HARNESS=$HARNESS
  PRIMARY_OUTPUT=$output
  PRIMARY_INSTALL_ROOT=$HARNESS_INSTALL_ROOT
}

run_rollback_test() {
  local state="$TMP_ROOT/state-rollback" output
  local receipt="$TMP_ROOT/applied-rollback.json" before after hash evidence
  HARNESS=$PRIMARY_HARNESS; output=$PRIMARY_OUTPUT
  mkdir -p "$state"
  evidence="$REPO_ROOT/.autopus/specs/SPEC-ADK-ULTRA-EFFICIENCY-001/evidence"
  write_applied_receipt "$receipt" "$PRIMARY_LEDGER" "$evidence"
  hash="sha256:$(shasum -a 256 "$receipt" | awk '{print $1}')"
  before=$(invocation_count "$COUNT_FILE")
  FAKE_AUTO_COUNT_FILE="$COUNT_FILE" UTE_CODEX_CANARY_EXECUTE=YES \
    "$HARNESS" rollback --repo "$SNAPSHOT" --auto "$FAKE_AUTO" \
    --state "$state" --output "$output" --primary-ledger "$PRIMARY_LEDGER" \
    --rollback-receipt "$receipt" --rollback-receipt-hash "$hash" >"$TMP_ROOT/rollback.progress"
  after=$(invocation_count "$COUNT_FILE")
  assert_eq "5" "$((after - before))" "rollback call count"
  assert_rollback_ledger "$output/gpt-rollback-call-ledger-v1.json"
  verify_progress "$TMP_ROOT/rollback.progress" 5
  assert_no_raw_retention "$state" "$output"

  local second_state="$TMP_ROOT/state-rollback-second" second_before
  mkdir -p "$second_state"; second_before=$(invocation_count "$COUNT_FILE")
  if FAKE_AUTO_COUNT_FILE="$COUNT_FILE" UTE_CODEX_CANARY_EXECUTE=YES \
    "$HARNESS" rollback --repo "$SNAPSHOT" --auto "$FAKE_AUTO" --state "$second_state" --output "$output" \
    --primary-ledger "$PRIMARY_LEDGER" --rollback-receipt "$receipt" --rollback-receipt-hash "$hash" >/dev/null 2>&1; then
    fail "rollback replay authorization was reused"
  fi
  assert_eq "$second_before" "$(invocation_count "$COUNT_FILE")" "second rollback invoked auto"

  mkdir -p "$TMP_ROOT/state-refuse" "$TMP_ROOT/output-refuse"
  if FAKE_AUTO_COUNT_FILE="$COUNT_FILE" UTE_CODEX_CANARY_EXECUTE=YES \
    "$HARNESS" rollback --repo "$SNAPSHOT" --auto "$FAKE_AUTO" \
    --state "$TMP_ROOT/state-refuse" --output "$TMP_ROOT/output-refuse" \
    --primary-ledger "$PRIMARY_LEDGER" >/dev/null 2>&1; then
    fail "rollback without applied receipt was accepted"
  fi
  assert_eq "$after" "$(invocation_count "$COUNT_FILE")" "rollback refusal invoked auto"
}

run_audit_go_contract_test() {
  local helper="$TMP_ROOT/audit-check.go"
  cat > "$helper" <<'EOF'
package main
import (
  "fmt"
  "github.com/insajin/autopus-adk/pkg/experiment"
)
func main() {
  policy := "sha256:1640281825b184f9ffbb92dc36a9afac27a5b55a9ff9d2632aadfa2dcce9430b"
  rows := []struct{ id, hash string; bucket int }{
    {"001","sha256:fac11ca2f98a23ea83b28bcc61255edde85e81c0f711658b256c725922f4e303",91},
    {"004","sha256:08ca0cde64b78772f7d298eddac967e48d8efc379b6dbe6fdf0f99f691a01b1e",64},
    {"005","sha256:e414021cbf4da33e47794324f90a14987fcfe3cffe5805db8aacdacbcb96adfc",13},
    {"011","sha256:61e9a1c04c26ce21225fdf41432de7042d517e1715bf8369dff87caf3c665773",22},
    {"012","sha256:e49a068e2a90bb2b709cc448661b97355d94573b27cb9e23d14a6ede4d554603",94},
  }
  selected := ""
  for _, row := range rows {
    got, err := experiment.SelectFullDepthAudit(row.hash, policy, 20)
    if err != nil || got.Bucket != row.bucket { panic(fmt.Sprintf("audit mismatch %s", row.id)) }
    if got.Selected { if selected != "" { panic("multiple audits") }; selected = row.id }
  }
  if selected != "005" { panic("task 005 not uniquely selected") }
}
EOF
  (cd "$REPO_ROOT" && go run "$helper") >/dev/null
}

run_fail_closed_test() {
  local state="$TMP_ROOT/state-fail" output
  make_harness_install fail
  output=$CANONICAL_OUTPUT
  mkdir -p "$state"
  : > "$COUNT_FILE"
  if FAKE_AUTO_COUNT_FILE="$COUNT_FILE" FAKE_AUTO_FAIL_AT=7 UTE_CODEX_CANARY_EXECUTE=YES \
    "$HARNESS" primary --repo "$SNAPSHOT" --auto "$FAKE_AUTO" \
    --state "$state" --output "$output" >/dev/null 2>&1; then
    fail "injected failure was accepted"
  fi
  assert_eq "7" "$(invocation_count "$COUNT_FILE")" "circuit breaker retried"
  local partial="$output/gpt-primary-call-ledger-v1.partial-fail.json"
  jq -e '.promotion_eligible == false and .completed == false and .attempted_calls == 7' "$partial" >/dev/null
  assert_no_raw_retention "$state" "$output"

  local before restart_state arbitrary_state arbitrary_output claim moved
  before=$(invocation_count "$COUNT_FILE")
  restart_state="$TMP_ROOT/state-fail-restart"; mkdir -p "$restart_state"
  if FAKE_AUTO_COUNT_FILE="$COUNT_FILE" UTE_CODEX_CANARY_EXECUTE=YES \
    "$HARNESS" primary --repo "$SNAPSHOT" --auto "$FAKE_AUTO" --state "$restart_state" --output "$output" >/dev/null 2>&1; then
    fail "partial primary authorization was reused"
  fi
  assert_eq "$before" "$(invocation_count "$COUNT_FILE")" "partial restart invoked auto"

  arbitrary_state="$TMP_ROOT/state-fail-arbitrary"; arbitrary_output="$TMP_ROOT/output-fail-arbitrary"
  mkdir -p "$arbitrary_state" "$arbitrary_output"
  if FAKE_AUTO_COUNT_FILE="$COUNT_FILE" UTE_CODEX_CANARY_EXECUTE=YES \
    "$HARNESS" primary --repo "$SNAPSHOT" --auto "$FAKE_AUTO" --state "$arbitrary_state" --output "$arbitrary_output" >/dev/null 2>&1; then
    fail "partial authorization escaped through fresh output"
  fi
  assert_eq "$before" "$(invocation_count "$COUNT_FILE")" "fresh output after partial invoked auto"

  claim="$HARNESS_INSTALL_ROOT/.autopus/runtime/ute-authorizations/1640281825b184f9ffbb92dc36a9afac27a5b55a9ff9d2632aadfa2dcce9430b"
  [[ -f "$claim/reservation.json" ]] || fail "durable authorization claim missing"
  rm -rf "$claim"
  restart_state="$TMP_ROOT/state-fail-reconcile"; mkdir -p "$restart_state"
  if FAKE_AUTO_COUNT_FILE="$COUNT_FILE" UTE_CODEX_CANARY_EXECUTE=YES \
    "$HARNESS" primary --repo "$SNAPSHOT" --auto "$FAKE_AUTO" --state "$restart_state" --output "$output" >/dev/null 2>&1; then
    fail "legacy partial reconciliation was accepted"
  fi
  jq -e '.kind == "legacy_primary_consumption_reconciliation" and .source_ledger_valid == true and
    .observed_calls == 7 and .state == "CONSUMED_ON_RECONCILIATION"' "$claim/reservation.json" >/dev/null
  moved="$TMP_ROOT/moved-partial"; mkdir -p "$moved"
  mv "$partial" "$moved/"; mv "${partial%.json}.sha256" "$moved/"
  restart_state="$TMP_ROOT/state-fail-after-move"; mkdir -p "$restart_state"
  if FAKE_AUTO_COUNT_FILE="$COUNT_FILE" UTE_CODEX_CANARY_EXECUTE=YES \
    "$HARNESS" primary --repo "$SNAPSHOT" --auto "$FAKE_AUTO" --state "$restart_state" --output "$output" >/dev/null 2>&1; then
    fail "moved partial released reconciled authorization"
  fi
  assert_eq "$before" "$(invocation_count "$COUNT_FILE")" "moved partial restart invoked auto"
}

run_runtime_identity_test() {
  local state="$TMP_ROOT/state-identity" output
  make_harness_install identity
  output=$CANONICAL_OUTPUT
  mkdir -p "$state"
  : > "$COUNT_FILE"
  if FAKE_AUTO_COUNT_FILE="$COUNT_FILE" FAKE_CODEX_BAD_VERSION=YES UTE_CODEX_CANARY_EXECUTE=YES \
    "$HARNESS" primary --repo "$SNAPSHOT" --auto "$FAKE_AUTO" \
    --state "$state" --output "$output" >/dev/null 2>&1; then
    fail "wrong codex version was accepted"
  fi
  assert_eq "0" "$(invocation_count "$COUNT_FILE")" "identity failure invoked agent run"
  jq -e '.completed == false and .evaluation_eligible == false and .promotion_eligible == false and
    .failure_code == "runtime_identity" and .attempted_calls == 0' \
    "$output/gpt-primary-call-ledger-v1.partial-fail.json" >/dev/null
}

run_retention_circuit_test() {
  local state="$TMP_ROOT/state-retention" output
  make_harness_install retention
  output=$CANONICAL_OUTPUT
  mkdir -p "$state"
  : > "$COUNT_FILE"
  if FAKE_AUTO_COUNT_FILE="$COUNT_FILE" FAKE_AUTO_LEAK_AT=3 UTE_CODEX_CANARY_EXECUTE=YES \
    "$HARNESS" primary --repo "$SNAPSHOT" --auto "$FAKE_AUTO" \
    --state "$state" --output "$output" >/dev/null 2>&1; then
    fail "retained-field leak was accepted"
  fi
  assert_eq "3" "$(invocation_count "$COUNT_FILE")" "retention circuit breaker retried"
  local partial="$output/gpt-primary-call-ledger-v1.partial-fail.json"
  jq -e '.completed == false and .evaluation_eligible == false and .promotion_eligible == false and
    .failure_code == "retained_field_scan" and .attempted_calls == 3' "$partial" >/dev/null
  assert_no_raw_retention "$state" "$output"
}

run_static_tamper_tests() {
  local evidence="$REPO_ROOT/.autopus/specs/SPEC-ADK-ULTRA-EFFICIENCY-001/evidence"
  local copy="$TMP_ROOT/evidence-copy"
  cp -R "$evidence" "$copy"

  printf '0%.0s' {1..64} > "$copy/gpt-codex-config-v1.sha256"
  if bash -c 'source "$1"; validate_static_evidence "$2"' _ \
    "$SCRIPT_DIR/ute_codex_canary_lib.sh" "$copy" >/dev/null 2>&1; then
    fail "tampered sidecar was accepted"
  fi

  rm -rf "$copy"; cp -R "$evidence" "$copy"
  jq '.tasks[0].pair_order = "BA"' "$copy/gpt-canary-cohort-v1.json" > "$copy/cohort.tmp"
  mv "$copy/cohort.tmp" "$copy/gpt-canary-cohort-v1.json"
  write_named_sidecar "$copy/gpt-canary-cohort-v1.json"
  if bash -c 'source "$1"; validate_static_evidence "$2"' _ \
    "$SCRIPT_DIR/ute_codex_canary_lib.sh" "$copy" >/dev/null 2>&1; then
    fail "tampered cohort order was accepted"
  fi

  rm -rf "$copy"; cp -R "$evidence" "$copy"
  jq '.authorization.provider_call_cap = 57' "$copy/gpt-codex-policy-v1.json" > "$copy/policy.tmp"
  mv "$copy/policy.tmp" "$copy/gpt-codex-policy-v1.json"
  write_named_sidecar "$copy/gpt-codex-policy-v1.json"
  if bash -c 'source "$1"; validate_static_evidence "$2"' _ \
    "$SCRIPT_DIR/ute_codex_canary_lib.sh" "$copy" >/dev/null 2>&1; then
    fail "tampered authorization cap was accepted"
  fi
}

run_preflight_test
run_audit_go_contract_test
run_primary_test
run_primary_reuse_tests
run_rollback_tamper_tests
run_rollback_test
run_fail_closed_test
run_parallel_reservation_test
run_runtime_identity_test
run_retention_circuit_test
run_static_tamper_tests
printf '%s\n' "ute codex canary hermetic tests: PASS"
