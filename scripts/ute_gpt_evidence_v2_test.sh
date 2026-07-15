#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
ROOT=$(cd "$SCRIPT_DIR/.." && pwd -P)
STATIC="$ROOT/.autopus/specs/SPEC-ADK-ULTRA-EFFICIENCY-001/evidence"
EVALUATOR="$SCRIPT_DIR/ute_gpt_evidence.sh"
FINALIZER="$SCRIPT_DIR/ute_gpt_full_evaluation_finalize.sh"
# shellcheck source=ute_gpt_evidence_v2_testlib.sh
source "$SCRIPT_DIR/ute_gpt_evidence_v2_testlib.sh"
# shellcheck source=ute_gpt_evidence_testlib.sh
source "$SCRIPT_DIR/ute_gpt_evidence_testlib.sh"
# shellcheck source=ute_codex_canary_testlib.sh
source "$SCRIPT_DIR/ute_codex_canary_testlib.sh"
missing=()
[[ -x "$FINALIZER" ]] || missing+=("ute_gpt_full_evaluation_finalize.sh")
for base in gpt-codex-policy-v2 gpt-codex-config-v2 gpt-verdict-schema-v2 \
  gpt-canary-schedule-v2 gpt-snapshot-manifest-v2 gpt-prior-evidence-manifest-v2 \
  gpt-full-evaluation-identity-v2 gpt-canary-preflight-v2; do
  [[ -f "$STATIC/$base.json" && -f "$STATIC/$base.sha256" ]] || missing+=("$base")
done
if [[ ${#missing[@]} -gt 0 ]]; then
  printf 'RED: missing v2 evaluator/finalizer fixture: %s\n' "${missing[*]}" >&2
  exit 1
fi
IDENTITY_HASH=$(v2_sha "$STATIC/gpt-full-evaluation-identity-v2.json")
v2_assert_sidecar "$STATIC/gpt-full-evaluation-identity-v2.json"
TMP_ROOT=$(mktemp -d "${TMPDIR:-/tmp}/ute-gpt-evidence-v2-test.XXXXXX")
trap 'rm -rf "$TMP_ROOT"' EXIT
REAL_AUTO="$TMP_ROOT/auto-real"
FAKE_AUTO="$TMP_ROOT/auto-fake"
FAKE_CODEX="$TMP_ROOT/codex"
COUNT_FILE="$TMP_ROOT/provider-invocations"
REPO="$TMP_ROOT/snapshot.git"
HOME_DIR="$TMP_ROOT/home"
mkdir -p "$HOME_DIR/.codex"
printf '%s\n' 'sentinel = "unchanged"' > "$HOME_DIR/.codex/config.toml"
printf '%s\n' '{"sentinel":"unchanged"}' > "$HOME_DIR/.codex/model_index.json"
CONFIG_BEFORE=$(shasum -a 256 "$HOME_DIR/.codex/config.toml" | awk '{print $1}')
INDEX_BEFORE=$(shasum -a 256 "$HOME_DIR/.codex/model_index.json" | awk '{print $1}')
cp "$ROOT/.autopus/runtime/auto-ute-transport-diagnosis-v8" "$REAL_AUTO"
make_fake_auto "$FAKE_AUTO"
make_fake_codex "$FAKE_CODEX"
export PATH="$TMP_ROOT:$PATH"
git clone -q --bare "$ROOT" "$REPO"
REPO_ROOT=$ROOT
HARNESS= CANONICAL_OUTPUT= HARNESS_INSTALL_ROOT=
make_harness_install v2-evaluator-source
CANARY=$HARNESS
CANARY_OUTPUT=$CANONICAL_OUTPUT
FAKE_STATE="$TMP_ROOT/fake-state"
mkdir -p "$FAKE_STATE"
: > "$COUNT_FILE"
FAKE_AUTO_COUNT_FILE="$COUNT_FILE" UTE_CODEX_CANARY_EXECUTE=YES \
  "$CANARY" primary --repo "$REPO" --auto "$FAKE_AUTO" --state "$FAKE_STATE" \
  --output "$CANARY_OUTPUT" >/dev/null
[[ "$(invocation_count "$COUNT_FILE")" == 58 ]] || v2_fail "fake primary fixture did not reach 58"
V1_PRIMARY="$CANARY_OUTPUT/gpt-primary-call-ledger-v1.json"
PRIMARY="$TMP_ROOT/gpt-primary-call-ledger-v2.json"
gpt_normalize_fake_ledger "$V1_PRIMARY" "$TMP_ROOT/normalized-v1.json" "$REAL_AUTO"
PRIMARY_RESERVATION="$TMP_ROOT/primary-reservation-v2.json"
v2_make_reservation "$PRIMARY_RESERVATION" "$STATIC" "$REAL_AUTO"
v2_make_primary "$TMP_ROOT/normalized-v1.json" "$PRIMARY" "$STATIC" "$(v2_sha "$PRIMARY_RESERVATION")"
AUTH=$IDENTITY_HASH
printf '#!/usr/bin/env bash\nprintf "called\\n" >> %q\nexit 99\n' "$COUNT_FILE" > "$FAKE_CODEX"
chmod 755 "$FAKE_CODEX"
: > "$COUNT_FILE"
EVALUATED="$TMP_ROOT/evaluated"
mkdir "$EVALUATED"
HOME="$HOME_DIR" "$EVALUATOR" evaluate --admission-generation v2 --primary-ledger "$PRIMARY" \
  --repo "$REPO" --auto "$REAL_AUTO" --output "$EVALUATED" >/dev/null
[[ "$(invocation_count "$COUNT_FILE")" == 0 ]] || v2_fail "evaluator invoked provider/Codex"
v2_assert_evaluation "$EVALUATED" "$AUTH"
CHAIN_INSTALL="$TMP_ROOT/chain-install"
v2_make_chain_install "$CHAIN_INSTALL" "$STATIC"
CHAIN_SENTINEL="$TMP_ROOT/evaluator-source-sentinel"
printf '\ntouch %q\n' "$CHAIN_SENTINEL" >> "$CHAIN_INSTALL/scripts/ute_codex_canary_prompt.sh"
v2_expect_evaluator_failure "$CHAIN_INSTALL/scripts/ute_gpt_evidence.sh" "$PRIMARY" "$REPO" "$REAL_AUTO" \
  "$TMP_ROOT/out-chain-tamper"
[[ ! -e "$CHAIN_SENTINEL" ]] || v2_fail "evaluator sourced helper before full-chain validation"
FINALIZER_CHAIN="$TMP_ROOT/finalizer-chain"; v2_make_chain_install "$FINALIZER_CHAIN" "$STATIC"
FINALIZER_SENTINEL="$TMP_ROOT/finalizer-source-sentinel"
printf '\ntouch %q\n' "$FINALIZER_SENTINEL" >> "$FINALIZER_CHAIN/scripts/ute_gpt_full_evaluation_finalize_lib.sh"
if "$FINALIZER_CHAIN/scripts/ute_gpt_full_evaluation_finalize.sh" finalize --evidence-dir \
  "$FINALIZER_CHAIN/.autopus/specs/SPEC-ADK-ULTRA-EFFICIENCY-001/evidence" --primary-ledger /invalid --auto "$REAL_AUTO" \
  >/dev/null 2>&1; then v2_fail "tampered finalizer chain succeeded"; fi
[[ ! -e "$FINALIZER_SENTINEL" ]] || v2_fail "finalizer sourced helper before full-chain validation"
V1_REJECT="$TMP_ROOT/out-v1"; v2_expect_evaluator_failure "$EVALUATOR" "$TMP_ROOT/normalized-v1.json" "$REPO" "$REAL_AUTO" "$V1_REJECT"
PARTIAL="$TMP_ROOT/gpt-primary-call-ledger-v2.partial-fail.json"
jq '.completed=false | .evaluation_eligible=false | .attempted_calls=57 | .successful_calls=57 |
  .observed_calls=57 | .calls=.calls[:57] | .failure_code="primary_incomplete" | .circuit_breaker="OPEN"' \
  "$PRIMARY" > "$PARTIAL"; v2_sidecar "$PARTIAL"
v2_expect_evaluator_failure "$EVALUATOR" "$PARTIAL" "$REPO" "$REAL_AUTO" "$TMP_ROOT/out-partial"
WRONG="$TMP_ROOT/primary-wrong-generation.json"
jq '.admission_generation="v1"' "$PRIMARY" > "$WRONG"; v2_sidecar "$WRONG"
v2_expect_evaluator_failure "$EVALUATOR" "$WRONG" "$REPO" "$REAL_AUTO" "$TMP_ROOT/out-wrong-generation"
LEGACY_IDS="$TMP_ROOT/primary-converted-v1-ids.json"
jq --slurpfile old "$TMP_ROOT/normalized-v1.json" '.calls |= map(. as $c | ($old[0].calls[] | select(.sequence == $c.sequence)) as $o | .result.call_id=$o.result.call_id | .usage.call_id=$o.usage.call_id | .result.run_id=$o.result.run_id | .usage.run_id=$o.usage.run_id)' "$PRIMARY" > "$LEGACY_IDS"; v2_sidecar "$LEGACY_IDS"
v2_expect_evaluator_failure "$EVALUATOR" "$LEGACY_IDS" "$REPO" "$REAL_AUTO" "$TMP_ROOT/out-converted-v1-ids"
TAMPER="$TMP_ROOT/primary-tampered.json"
jq '.authorization_identity_sha256=("sha256:" + ("0" * 64))' "$PRIMARY" > "$TAMPER"; v2_sidecar "$TAMPER"
v2_expect_evaluator_failure "$EVALUATOR" "$TAMPER" "$REPO" "$REAL_AUTO" "$TMP_ROOT/out-tampered"
MISSING_SECURITY="$TMP_ROOT/primary-missing-security.json"
jq '(.calls[] | select(.role == "security-auditor") | .role) = "reviewer"' "$PRIMARY" > "$MISSING_SECURITY"; v2_sidecar "$MISSING_SECURITY"
v2_expect_evaluator_failure "$EVALUATOR" "$MISSING_SECURITY" "$REPO" "$REAL_AUTO" "$TMP_ROOT/out-missing-security"
RAW="$TMP_ROOT/primary-raw.json"
jq '.raw_output="FAKE-RAW-PROVIDER-BODY"' "$PRIMARY" > "$RAW"; v2_sidecar "$RAW"
v2_expect_evaluator_failure "$EVALUATOR" "$RAW" "$REPO" "$REAL_AUTO" "$TMP_ROOT/out-raw"
WRONG_AUTO_SENTINEL="$TMP_ROOT/wrong-auto-executed"
WRONG_AUTO="$TMP_ROOT/auto-wrong"
printf '#!/usr/bin/env bash\ntouch %q\nprintf "wrong-auto\\n"\n' "$WRONG_AUTO_SENTINEL" > "$WRONG_AUTO"
chmod 755 "$WRONG_AUTO"
v2_expect_evaluator_failure "$EVALUATOR" "$PRIMARY" "$REPO" "$WRONG_AUTO" "$TMP_ROOT/out-wrong-auto"
[[ ! -e "$WRONG_AUTO_SENTINEL" ]] || v2_fail "wrong Auto executed before frozen hash validation"
ARITH_SENTINEL="$TMP_ROOT/arithmetic-injection-executed"
ARITHMETIC="$TMP_ROOT/primary-arithmetic-injection.json"
ARITH_PAYLOAD="\$(touch $ARITH_SENTINEL)"
jq --arg payload "$ARITH_PAYLOAD" '.observed_calls=$payload' "$PRIMARY" > "$ARITHMETIC"; v2_sidecar "$ARITHMETIC"
v2_expect_evaluator_failure "$EVALUATOR" "$ARITHMETIC" "$REPO" "$REAL_AUTO" "$TMP_ROOT/out-arithmetic"
[[ ! -e "$ARITH_SENTINEL" ]] || v2_fail "evaluator executed ledger arithmetic payload"
MISSING_REPLAY=$(v2_case_dir missing-replay)
v2_copy_canonical "$MISSING_REPLAY" "$STATIC" "$EVALUATED" "$REAL_AUTO"
cp "$PRIMARY" "${PRIMARY%.json}.sha256" "$MISSING_REPLAY/"
if HOME="$HOME_DIR" "$FINALIZER" finalize --evidence-dir "$MISSING_REPLAY" \
  --primary-ledger "$MISSING_REPLAY/$(basename "$PRIMARY")" --auto "$REAL_AUTO" >/dev/null 2>&1; then
  v2_fail "finalizer succeeded before replay"
fi
v2_assert_failure_terminal "$MISSING_REPLAY" rollback_replay_missing
WRONG_AUTO_CASE=$(v2_case_dir wrong-auto)
v2_copy_canonical "$WRONG_AUTO_CASE" "$STATIC" "$EVALUATED" "$REAL_AUTO"
cp "$PRIMARY" "${PRIMARY%.json}.sha256" "$WRONG_AUTO_CASE/"
if "$FINALIZER" finalize --evidence-dir "$WRONG_AUTO_CASE" \
  --primary-ledger "$WRONG_AUTO_CASE/$(basename "$PRIMARY")" --auto "$WRONG_AUTO" >/dev/null 2>&1; then
  v2_fail "wrong Auto reached finalizer success"
fi
v2_assert_failure_terminal "$WRONG_AUTO_CASE" evidence_link_mismatch null
jq -e '.authorization_identity_sha256 == null' "$WRONG_AUTO_CASE/gpt-full-evaluation-terminal-outcome-v2.json" >/dev/null ||
  v2_fail "unvalidated authorization identity was fabricated"
[[ ! -e "$WRONG_AUTO_SENTINEL" ]] || v2_fail "finalizer executed wrong Auto before frozen hash validation"
BAD_AUTH_CASE=$(v2_case_dir invalid-authorization)
v2_copy_canonical "$BAD_AUTH_CASE" "$STATIC" "$EVALUATED" "$REAL_AUTO"
cp "$PRIMARY" "${PRIMARY%.json}.sha256" "$BAD_AUTH_CASE/"
jq '.decision="REJECTED"' "$BAD_AUTH_CASE/gpt-full-evaluation-authorization-v2.json" > "$BAD_AUTH_CASE/auth.tmp"
mv "$BAD_AUTH_CASE/auth.tmp" "$BAD_AUTH_CASE/gpt-full-evaluation-authorization-v2.json"
v2_sidecar "$BAD_AUTH_CASE/gpt-full-evaluation-authorization-v2.json"
if "$FINALIZER" finalize --evidence-dir "$BAD_AUTH_CASE" --primary-ledger "$BAD_AUTH_CASE/$(basename "$PRIMARY")" \
  --auto "$REAL_AUTO" >/dev/null 2>&1; then v2_fail "invalid authorization reached finalizer success"; fi
v2_assert_failure_terminal "$BAD_AUTH_CASE" evidence_link_mismatch null
jq -e '.authorization_identity_sha256 == null' "$BAD_AUTH_CASE/gpt-full-evaluation-terminal-outcome-v2.json" >/dev/null ||
  v2_fail "invalid authorization produced a trusted identity"
ARITHMETIC_CASE=$(v2_case_dir arithmetic-injection)
v2_copy_canonical "$ARITHMETIC_CASE" "$STATIC" "$EVALUATED" "$REAL_AUTO"
cp "$ARITHMETIC" "${ARITHMETIC%.json}.sha256" "$ARITHMETIC_CASE/"
if "$FINALIZER" finalize --evidence-dir "$ARITHMETIC_CASE" \
  --primary-ledger "$ARITHMETIC_CASE/$(basename "$ARITHMETIC")" --auto "$REAL_AUTO" >/dev/null 2>&1; then
  v2_fail "arithmetic injection ledger reached success"
fi
v2_assert_failure_terminal "$ARITHMETIC_CASE" primary_incomplete
[[ ! -e "$ARITH_SENTINEL" ]] || v2_fail "finalizer executed ledger arithmetic payload"
SUCCESS=$(v2_case_dir success)
v2_copy_canonical "$SUCCESS" "$STATIC" "$EVALUATED" "$REAL_AUTO"
cp "$PRIMARY" "${PRIMARY%.json}.sha256" "$SUCCESS/"
ROLLBACK="$SUCCESS/gpt-rollback-call-ledger-v2.json"
v2_make_rollback "$SUCCESS/$(basename "$PRIMARY")" "$SUCCESS/gpt-applied-rollback-v2.json" "$ROLLBACK"
HOME="$HOME_DIR" "$FINALIZER" finalize --evidence-dir "$SUCCESS" \
  --primary-ledger "$SUCCESS/$(basename "$PRIMARY")" --rollback-ledger "$ROLLBACK" --auto "$REAL_AUTO" >/dev/null
v2_assert_success_terminal "$SUCCESS" "$AUTH"
TERMINAL_HASH=$(v2_sha "$SUCCESS/gpt-full-evaluation-terminal-outcome-v2.json")
if HOME="$HOME_DIR" "$FINALIZER" finalize --evidence-dir "$SUCCESS" \
  --primary-ledger "$SUCCESS/$(basename "$PRIMARY")" --rollback-ledger "$ROLLBACK" --auto "$REAL_AUTO" >/dev/null 2>&1; then
  v2_fail "terminal authorization was reused"
fi
[[ "$TERMINAL_HASH" == "$(v2_sha "$SUCCESS/gpt-full-evaluation-terminal-outcome-v2.json")" ]] || \
  v2_fail "terminal changed on reuse"
MISSING_RESERVATION_CASE=$(v2_case_dir missing-reservation)
v2_copy_canonical "$MISSING_RESERVATION_CASE" "$STATIC" "$EVALUATED" "$REAL_AUTO"
cp "$PRIMARY" "${PRIMARY%.json}.sha256" "$MISSING_RESERVATION_CASE/"
MISSING_RESERVATION_REPLAY="$MISSING_RESERVATION_CASE/gpt-rollback-call-ledger-v2.json"
v2_make_rollback "$MISSING_RESERVATION_CASE/$(basename "$PRIMARY")" \
  "$MISSING_RESERVATION_CASE/gpt-applied-rollback-v2.json" "$MISSING_RESERVATION_REPLAY"
rm "$MISSING_RESERVATION_CASE/gpt-full-evaluation-reservation-v2.json" \
  "$MISSING_RESERVATION_CASE/gpt-full-evaluation-reservation-v2.sha256"
if HOME="$HOME_DIR" "$FINALIZER" finalize --evidence-dir "$MISSING_RESERVATION_CASE" \
  --primary-ledger "$MISSING_RESERVATION_CASE/$(basename "$PRIMARY")" \
  --rollback-ledger "$MISSING_RESERVATION_REPLAY" --auto "$REAL_AUTO" >/dev/null 2>&1; then
  v2_fail "missing reservation reached success"
fi
v2_assert_failure_terminal "$MISSING_RESERVATION_CASE" reservation_missing
TAMPERED_RESERVATION_CASE=$(v2_case_dir tampered-reservation)
v2_copy_canonical "$TAMPERED_RESERVATION_CASE" "$STATIC" "$EVALUATED" "$REAL_AUTO"
cp "$PRIMARY" "${PRIMARY%.json}.sha256" "$TAMPERED_RESERVATION_CASE/"
TAMPERED_RESERVATION="$TAMPERED_RESERVATION_CASE/gpt-full-evaluation-reservation-v2.json"
jq '.authorization_identity_sha256=("sha256:" + ("0" * 64))' "$TAMPERED_RESERVATION" > "$TAMPERED_RESERVATION.tmp"
mv "$TAMPERED_RESERVATION.tmp" "$TAMPERED_RESERVATION"; v2_sidecar "$TAMPERED_RESERVATION"
TAMPERED_RESERVATION_REPLAY="$TAMPERED_RESERVATION_CASE/gpt-rollback-call-ledger-v2.json"
v2_make_rollback "$TAMPERED_RESERVATION_CASE/$(basename "$PRIMARY")" \
  "$TAMPERED_RESERVATION_CASE/gpt-applied-rollback-v2.json" "$TAMPERED_RESERVATION_REPLAY"
if HOME="$HOME_DIR" "$FINALIZER" finalize --evidence-dir "$TAMPERED_RESERVATION_CASE" \
  --primary-ledger "$TAMPERED_RESERVATION_CASE/$(basename "$PRIMARY")" \
  --rollback-ledger "$TAMPERED_RESERVATION_REPLAY" --auto "$REAL_AUTO" >/dev/null 2>&1; then
  v2_fail "tampered reservation reached success"
fi
v2_assert_failure_terminal "$TAMPERED_RESERVATION_CASE" reservation_invalid null

PARTIAL_CASE=$(v2_case_dir partial-primary)
v2_copy_canonical "$PARTIAL_CASE" "$STATIC" "" "$REAL_AUTO"
cp "$PARTIAL" "${PARTIAL%.json}.sha256" "$PARTIAL_CASE/"
if HOME="$HOME_DIR" "$FINALIZER" finalize --evidence-dir "$PARTIAL_CASE" \
  --primary-ledger "$PARTIAL_CASE/$(basename "$PARTIAL")" --auto "$REAL_AUTO" >/dev/null 2>&1; then
  v2_fail "partial primary reached success"
fi
v2_assert_failure_terminal "$PARTIAL_CASE" primary_incomplete
[[ -z "$(find "$PARTIAL_CASE" -maxdepth 1 -type f \( -name 'gpt-efficiency-*-v2.json' -o \
  -name 'gpt-security-receipts-v2.json' -o -name 'gpt-quality-ledger-v2.json' -o \
  -name 'gpt-rollout-*-v2.json' -o -name 'gpt-rollback-call-ledger-v2.json' -o \
  -name 'gpt-applied-rollback-v2.json' \) -print -quit)" ]] || \
  v2_fail "partial primary caused fabricated evaluator/replay artifacts"

PARTIAL_REPLAY_CASE=$(v2_case_dir partial-replay)
v2_copy_canonical "$PARTIAL_REPLAY_CASE" "$STATIC" "$EVALUATED" "$REAL_AUTO"
cp "$PRIMARY" "${PRIMARY%.json}.sha256" "$PARTIAL_REPLAY_CASE/"
PARTIAL_REPLAY="$PARTIAL_REPLAY_CASE/gpt-rollback-call-ledger-v2.partial-fail.json"
v2_make_rollback "$PARTIAL_REPLAY_CASE/$(basename "$PRIMARY")" \
  "$PARTIAL_REPLAY_CASE/gpt-applied-rollback-v2.json" "$PARTIAL_REPLAY" 4 false
if HOME="$HOME_DIR" "$FINALIZER" finalize --evidence-dir "$PARTIAL_REPLAY_CASE" \
  --primary-ledger "$PARTIAL_REPLAY_CASE/$(basename "$PRIMARY")" --rollback-ledger "$PARTIAL_REPLAY" \
  --auto "$REAL_AUTO" >/dev/null 2>&1; then
  v2_fail "partial replay reached success"
fi
v2_assert_failure_terminal "$PARTIAL_REPLAY_CASE" rollback_replay_incomplete

EXTRA_CASE=$(v2_case_dir extra-call)
v2_copy_canonical "$EXTRA_CASE" "$STATIC" "$EVALUATED" "$REAL_AUTO"
cp "$PRIMARY" "${PRIMARY%.json}.sha256" "$EXTRA_CASE/"
EXTRA="$EXTRA_CASE/gpt-rollback-call-ledger-v2.json"
v2_make_rollback "$EXTRA_CASE/$(basename "$PRIMARY")" "$EXTRA_CASE/gpt-applied-rollback-v2.json" "$EXTRA" 6 true
if HOME="$HOME_DIR" "$FINALIZER" finalize --evidence-dir "$EXTRA_CASE" \
  --primary-ledger "$EXTRA_CASE/$(basename "$PRIMARY")" --rollback-ledger "$EXTRA" --auto "$REAL_AUTO" >/dev/null 2>&1; then
  v2_fail "64th provider call was accepted"
fi
v2_assert_failure_terminal "$EXTRA_CASE" call_cap_exceeded

BAD_LINK_CASE=$(v2_case_dir bad-link)
v2_copy_canonical "$BAD_LINK_CASE" "$STATIC" "$EVALUATED" "$REAL_AUTO"
cp "$PRIMARY" "${PRIMARY%.json}.sha256" "$BAD_LINK_CASE/"
BAD_LINK="$BAD_LINK_CASE/gpt-rollback-call-ledger-v2.json"
v2_make_rollback "$BAD_LINK_CASE/$(basename "$PRIMARY")" "$BAD_LINK_CASE/gpt-applied-rollback-v2.json" "$BAD_LINK"
jq '.primary_ledger_sha256=("sha256:" + ("0" * 64))' "$BAD_LINK" > "$BAD_LINK.tmp"
mv "$BAD_LINK.tmp" "$BAD_LINK"; v2_sidecar "$BAD_LINK"
if HOME="$HOME_DIR" "$FINALIZER" finalize --evidence-dir "$BAD_LINK_CASE" \
  --primary-ledger "$BAD_LINK_CASE/$(basename "$PRIMARY")" --rollback-ledger "$BAD_LINK" --auto "$REAL_AUTO" >/dev/null 2>&1; then
  v2_fail "wrong primary/replay link was accepted"
fi
v2_assert_failure_terminal "$BAD_LINK_CASE" evidence_link_mismatch

BAD_READBACK_CASE=$(v2_case_dir bad-readback)
v2_copy_canonical "$BAD_READBACK_CASE" "$STATIC" "$EVALUATED" "$REAL_AUTO"
cp "$PRIMARY" "${PRIMARY%.json}.sha256" "$BAD_READBACK_CASE/"
jq '.state_readback="compact_ultra"' "$BAD_READBACK_CASE/gpt-applied-rollback-v2.json" > "$BAD_READBACK_CASE/readback.tmp"
mv "$BAD_READBACK_CASE/readback.tmp" "$BAD_READBACK_CASE/gpt-applied-rollback-v2.json"
v2_sidecar "$BAD_READBACK_CASE/gpt-applied-rollback-v2.json"
BAD_READBACK="$BAD_READBACK_CASE/gpt-rollback-call-ledger-v2.json"
v2_make_rollback "$BAD_READBACK_CASE/$(basename "$PRIMARY")" \
  "$BAD_READBACK_CASE/gpt-applied-rollback-v2.json" "$BAD_READBACK"
if HOME="$HOME_DIR" "$FINALIZER" finalize --evidence-dir "$BAD_READBACK_CASE" \
  --primary-ledger "$BAD_READBACK_CASE/$(basename "$PRIMARY")" --rollback-ledger "$BAD_READBACK" --auto "$REAL_AUTO" >/dev/null 2>&1; then
  v2_fail "invalid rollback readback was accepted"
fi
v2_assert_failure_terminal "$BAD_READBACK_CASE" rollback_readback_invalid

MISSING_QUALITY_CASE=$(v2_case_dir missing-quality)
v2_copy_canonical "$MISSING_QUALITY_CASE" "$STATIC" "$EVALUATED" "$REAL_AUTO"
cp "$PRIMARY" "${PRIMARY%.json}.sha256" "$MISSING_QUALITY_CASE/"
rm "$MISSING_QUALITY_CASE/gpt-quality-ledger-v2.json" "$MISSING_QUALITY_CASE/gpt-quality-ledger-v2.sha256"
MISSING_QUALITY_REPLAY="$MISSING_QUALITY_CASE/gpt-rollback-call-ledger-v2.json"
v2_make_rollback "$MISSING_QUALITY_CASE/$(basename "$PRIMARY")" \
  "$MISSING_QUALITY_CASE/gpt-applied-rollback-v2.json" "$MISSING_QUALITY_REPLAY"
if HOME="$HOME_DIR" "$FINALIZER" finalize --evidence-dir "$MISSING_QUALITY_CASE" \
  --primary-ledger "$MISSING_QUALITY_CASE/$(basename "$PRIMARY")" --rollback-ledger "$MISSING_QUALITY_REPLAY" \
  --auto "$REAL_AUTO" >/dev/null 2>&1; then
  v2_fail "missing quality ledger was accepted"
fi
v2_assert_failure_terminal "$MISSING_QUALITY_CASE" quality_evidence_incomplete
v2_run_tamper_case wrong-high-task gpt-rollout-high-input-v2.json '.rollout.experiment_id += ":tampered"'
v2_run_tamper_case wrong-critical-task gpt-rollout-critical-input-v2.json '.rollout.experiment_id += ":tampered"'
v2_run_tamper_case replaced-result gpt-efficiency-result-v2.json '.comparison.median_paired_raw_reduction_pct=31'
v2_run_tamper_case security-link gpt-security-receipts-v2.json '.receipts[0].security_call.verdict="FAIL"'
v2_run_tamper_case quality-link gpt-quality-ledger-v2.json '.outcomes[0].baseline_security_receipt_hash=("sha256:"+("0"*64))'
v2_run_tamper_case aggregate-link gpt-rollout-receipts-v2.json '.high.input_sha256=("sha256:"+("0"*64))'
v2_run_tamper_case nested-tamper gpt-rollback-reservation-v2.json '.authorization_identity_sha256=("sha256:"+("0"*64))' rollback_reservation_invalid
v2_run_tamper_case nested-missing gpt-rollback-reservation-v2.json MISSING rollback_reservation_missing

[[ "$(invocation_count "$COUNT_FILE")" == 0 ]] || v2_fail "provider/Codex sentinel was invoked"
[[ "$CONFIG_BEFORE" == "$(shasum -a 256 "$HOME_DIR/.codex/config.toml" | awk '{print $1}')" ]] || v2_fail "user config changed"
[[ "$INDEX_BEFORE" == "$(shasum -a 256 "$HOME_DIR/.codex/model_index.json" | awk '{print $1}')" ]] || v2_fail "user index changed"
if rg -uuu -l 'FAKE-RAW-PROVIDER-BODY|UTE-RAW-PROMPT-|"(prompt|raw_output|stdout|stderr|session_id|environment|cwd)"' \
  "$EVALUATED" "$SUCCESS" >/dev/null 2>&1; then
  v2_fail "raw/provider body retained in v2 evidence"
fi

printf '%s\n' 'ute gpt evidence v2 hermetic tests: PASS'
