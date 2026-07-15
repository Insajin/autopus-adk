#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
# shellcheck source=ute_gpt_evidence_testlib.sh
source "$SCRIPT_DIR/ute_gpt_evidence_testlib.sh"
# shellcheck source=ute_codex_canary_testlib.sh
source "$SCRIPT_DIR/ute_codex_canary_testlib.sh"

ROOT=$(cd "$SCRIPT_DIR/.." && pwd -P)
EVIDENCE_HARNESS="$SCRIPT_DIR/ute_gpt_evidence.sh"
TMP_ROOT=$(mktemp -d "${TMPDIR:-/tmp}/ute-gpt-evidence-test.XXXXXX")
trap 'rm -rf "$TMP_ROOT"' EXIT
REAL_AUTO="$TMP_ROOT/auto-real"
FAKE_AUTO="$TMP_ROOT/auto-fake"
FAKE_CODEX="$TMP_ROOT/codex"
COUNT_FILE="$TMP_ROOT/provider-invocations"
REPO="$TMP_ROOT/snapshot.git"
HOME_DIR="$TMP_ROOT/home"
mkdir -p "$HOME_DIR/.codex"
printf '%s\n' 'sentinel = "unchanged"' > "$HOME_DIR/.codex/config.toml"
USER_CONFIG_BEFORE=$(shasum -a 256 "$HOME_DIR/.codex/config.toml" | awk '{print $1}')

(cd "$ROOT" && go build -o "$REAL_AUTO" ./cmd/auto)
make_fake_auto "$FAKE_AUTO"
make_fake_codex "$FAKE_CODEX"
export PATH="$TMP_ROOT:$PATH"
git clone -q --bare "$ROOT" "$REPO"

REPO_ROOT=$ROOT
HARNESS= CANONICAL_OUTPUT= HARNESS_INSTALL_ROOT=
make_harness_install gpt-evidence-primary
CANARY=$HARNESS
CANARY_INSTALL_ROOT=$HARNESS_INSTALL_ROOT
FAKE_OUTPUT=$CANONICAL_OUTPUT
HARNESS=$EVIDENCE_HARNESS
FAKE_STATE="$TMP_ROOT/fake-state"
mkdir -p "$FAKE_STATE"
: > "$COUNT_FILE"
FAKE_AUTO_COUNT_FILE="$COUNT_FILE" UTE_CODEX_CANARY_EXECUTE=YES \
  "$CANARY" primary --repo "$REPO" --auto "$FAKE_AUTO" --state "$FAKE_STATE" --output "$FAKE_OUTPUT" >/dev/null
[[ "$(invocation_count "$COUNT_FILE")" == 58 ]] || gpt_fail "fake primary did not complete 58 calls"
AUTH_POLICY=$(shasum -a 256 "$FAKE_OUTPUT/gpt-codex-policy-v1.json" | awk '{print $1}')
AUTH_CLAIM="$CANARY_INSTALL_ROOT/.autopus/runtime/ute-authorizations/$AUTH_POLICY/reservation.json"
[[ "$AUTH_CLAIM" == "$TMP_ROOT/"* && -f "$AUTH_CLAIM" ]] || gpt_fail "authorization claim escaped temp install"
PRIMARY="$TMP_ROOT/gpt-primary-call-ledger-v1.json"
gpt_normalize_fake_ledger "$FAKE_OUTPUT/gpt-primary-call-ledger-v1.json" "$PRIMARY" "$REAL_AUTO"

BLOCK_CODEX="$TMP_ROOT/codex"
printf '#!/usr/bin/env bash\nprintf "called\\n" >> %q\nexit 99\n' "$COUNT_FILE" > "$BLOCK_CODEX"
chmod 755 "$BLOCK_CODEX"
: > "$COUNT_FILE"
OUTPUT="$TMP_ROOT/evidence-output"
mkdir "$OUTPUT"
HOME="$HOME_DIR" "$HARNESS" evaluate --primary-ledger "$PRIMARY" --repo "$REPO" --auto "$REAL_AUTO" --output "$OUTPUT"
[[ "$(invocation_count "$COUNT_FILE")" == 0 ]] || gpt_fail "builder invoked provider binary"
[[ "$USER_CONFIG_BEFORE" == "$(shasum -a 256 "$HOME_DIR/.codex/config.toml" | awk '{print $1}')" ]] || \
  gpt_fail "user config changed"
gpt_assert_named_outputs "$OUTPUT"
gpt_assert_success_contracts "$OUTPUT"

TAMPER_DIR="$TMP_ROOT/tampers"
mkdir "$TAMPER_DIR"
tamper_ledger() {
  local name=$1 filter=$2 target
  target="$TAMPER_DIR/$name.json"
  jq "$filter" "$PRIMARY" > "$target"
  gpt_write_sidecar "$target"
  printf '%s\n' "$target"
}

TAMPER=$(tamper_ledger missing-security '(.calls[] | select(.role == "security-auditor") | .role) = "reviewer"' | tail -n1)
gpt_expect_builder_failure "$HARNESS" "$TAMPER" "$REPO" "$REAL_AUTO" "$TMP_ROOT/out-missing-security"
TAMPER=$(tamper_ledger effort-spoof '.calls[0].effort = "max"' | tail -n1)
gpt_expect_builder_failure "$HARNESS" "$TAMPER" "$REPO" "$REAL_AUTO" "$TMP_ROOT/out-effort"
TAMPER=$(tamper_ledger duplicate-call-id '.calls[1].result.call_id = .calls[0].result.call_id | .calls[1].usage.call_id = .calls[0].usage.call_id' | tail -n1)
gpt_expect_builder_failure "$HARNESS" "$TAMPER" "$REPO" "$REAL_AUTO" "$TMP_ROOT/out-duplicate"
ln -s "$PRIMARY" "$TAMPER_DIR/primary-symlink.json"
gpt_expect_builder_failure "$HARNESS" "$TAMPER_DIR/primary-symlink.json" "$REPO" "$REAL_AUTO" "$TMP_ROOT/out-symlink"

INCOMPLETE="$TMP_ROOT/incomplete-quality.json"
jq '.outcomes = .outcomes[:-1] | .row_count = 6' "$OUTPUT/gpt-quality-ledger-v1.json" > "$INCOMPLETE"
if bash -c 'source "$1"; validate_quality_ledger "$2" "$3"' _ \
  "$SCRIPT_DIR/ute_gpt_evidence_build.sh" "$INCOMPLETE" "$OUTPUT/gpt-security-receipts-v1.json" >/dev/null 2>&1; then
  gpt_fail "incomplete quality ledger was accepted"
fi

COPY_ROOT="$TMP_ROOT/copied-source"
mkdir -p "$COPY_ROOT/scripts" "$COPY_ROOT/.autopus/specs/SPEC-ADK-ULTRA-EFFICIENCY-001"
cp "$SCRIPT_DIR"/ute_gpt_evidence*.sh "$SCRIPT_DIR"/ute_codex_canary_{lib,schedule,receipt}.sh "$COPY_ROOT/scripts/"
cp -R "$ROOT/.autopus/specs/SPEC-ADK-ULTRA-EFFICIENCY-001/evidence" \
  "$COPY_ROOT/.autopus/specs/SPEC-ADK-ULTRA-EFFICIENCY-001/evidence"
jq '.tasks[0].oracle.hash = ("sha256:" + ("0" * 64))' \
  "$COPY_ROOT/.autopus/specs/SPEC-ADK-ULTRA-EFFICIENCY-001/evidence/corpus-v1.json" > "$TMP_ROOT/corpus-tampered"
mv "$TMP_ROOT/corpus-tampered" "$COPY_ROOT/.autopus/specs/SPEC-ADK-ULTRA-EFFICIENCY-001/evidence/corpus-v1.json"
gpt_expect_builder_failure "$COPY_ROOT/scripts/ute_gpt_evidence.sh" "$PRIMARY" "$REPO" "$REAL_AUTO" "$TMP_ROOT/out-oracle"

BAD_AUTO="$TMP_ROOT/auto-bad-evaluator"
sed 's/^+//' > "$BAD_AUTO" <<'BAD'
+#!/usr/bin/env bash
+set -euo pipefail
+if [[ ${1:-} == version && ${2:-} == --short ]]; then printf '%s\n' '0.50.99-test'; exit 0; fi
+if [[ ${1:-} == telemetry && ${2:-} == efficiency ]]; then printf '%s\n' '{}'; exit 0; fi
+exit 90
BAD
chmod 755 "$BAD_AUTO"
BAD_LEDGER="$TAMPER_DIR/bad-evaluator-ledger.json"
gpt_normalize_fake_ledger "$PRIMARY" "$BAD_LEDGER" "$BAD_AUTO"
gpt_expect_builder_failure "$HARNESS" "$BAD_LEDGER" "$REPO" "$BAD_AUTO" "$TMP_ROOT/out-bad-evaluator"

MUTATE_LEDGER="$TAMPER_DIR/toctou-ledger.json"
MUTATE_AUTO="$TMP_ROOT/auto-mutating-evaluator"
MUTATE_MARKER="$TMP_ROOT/ledger-mutated"
printf '#!/usr/bin/env bash\nset -euo pipefail\nif [[ ${1:-} == version && ${2:-} == --short ]]; then printf "%%s\\n" 0.50.99-test; exit 0; fi\n%q "$@"\nif [[ ! -e %q ]]; then printf "\\n" >> %q; : > %q; fi\n' \
  "$REAL_AUTO" "$MUTATE_MARKER" "$MUTATE_LEDGER" "$MUTATE_MARKER" > "$MUTATE_AUTO"
chmod 755 "$MUTATE_AUTO"
gpt_normalize_fake_ledger "$PRIMARY" "$MUTATE_LEDGER" "$MUTATE_AUTO"
gpt_expect_builder_failure "$HARNESS" "$MUTATE_LEDGER" "$REPO" "$MUTATE_AUTO" "$TMP_ROOT/out-toctou"

if rg -uuu -l 'FAKE-RAW-PROVIDER-BODY|UTE-RAW-PROMPT-|"(prompt|raw_output|stdout|stderr|session_id|environment|cwd)"' "$OUTPUT" >/dev/null 2>&1; then
  gpt_fail "raw/provider body retained in evidence"
fi

printf '%s\n' 'ute gpt evidence hermetic tests: PASS'
