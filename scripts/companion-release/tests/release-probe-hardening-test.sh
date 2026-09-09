#!/usr/bin/env bash
set -euo pipefail
umask 077

# Probe-lane hardening (SPEC-OMP-007 T0, REQ-PROBE-001). A probe run must retain
# records and publish nothing. Four properties cannot be re-checked afterwards:
# the failure path stops and verifies the canary UID process set before any
# export, no publication call is reachable while a probe is configured, a
# completed runtime frame alone never certifies a record set as full, and a
# cleanup that failed leaves the operator's retained directory untouched. The
# export helper is built and exercised for real.

tests_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
script_dir=$(cd -- "$tests_dir/.." && pwd)
repo=$(cd -- "$script_dir/../.." && pwd)
prep="$script_dir/prepare-release.sh"
runtime_lib="$script_dir/prepare-release-runtime-lib.sh"
probe_lib="$script_dir/prepare-release-probe-lib.sh"
wrapper="$repo/scripts/release-tools/release-prep.sh"
pin="$repo/scripts/release-tools/advance-omp-pin.sh"

fail() { printf 'release probe hardening test: %s\n' "$1" >&2; exit 1; }
# The retained directory is validated as a canonical absolute path.
temp=$(cd -- "$(mktemp -d "${TMPDIR:-/tmp}/release-probe-hardening.XXXXXX")" && pwd -P)
dispatch_nonce=$(printf '%s' "$$:$RANDOM:$(date -u '+%s')" | shasum -a 256 | awk '{print substr($1,1,32)}')
probe_root="/private/tmp/autopus-adk-release-prep-${dispatch_nonce}-final"
trap 'rm -rf -- "$temp" "$probe_root"' EXIT
runner_uid=$(/usr/bin/id -u); runner_gid=$(/usr/bin/id -g)
temp_dir="$temp/prep-temp"; operation='apply'
install -d -m 0700 "$temp_dir" "$temp/retained" "$temp/other"
# shellcheck source=/dev/null
source "$probe_lib"

probe_configure
[[ "$probe_enabled" -eq 0 && -z "$probe_retained_dir" ]] || fail 'probe enabled itself without the operator variable'
OMP_CONTEXT_PROBE_DIR="$temp/retained" probe_configure 2>/dev/null
[[ "$probe_enabled" -eq 1 && "$probe_retained_dir" == "$temp/retained" ]] ||
  fail 'operator opt-in did not configure the retained directory'
# Staging is this run's own 0700 directory inside the release temporary tree,
# so nothing survives the tree unless retention republishes it.
[[ "$probe_staging_dir" == "$temp_dir/probe-staging" && -d "$probe_staging_dir" ]] ||
  fail "opt-in did not stage inside the release temporary tree: $probe_staging_dir"
[[ "$(/usr/bin/stat -f '%u:%Lp' "$probe_staging_dir")" == "${runner_uid}:700" ]] ||
  fail 'staging directory is not runner-owned 0700'
# The helper opens every component with O_NOFOLLOW, so a retained root named
# through a symlinked ancestor must be handed over resolved.
install -d -m 0700 "$temp/real-parent" "$temp/real-parent/records"
ln -s "$temp/real-parent" "$temp/linked-parent"
OMP_CONTEXT_PROBE_DIR="$temp/linked-parent/records" probe_configure 2>/dev/null
[[ "$probe_retained_dir" == "$temp/real-parent/records" ]] ||
  fail "retained directory kept a symlinked ancestor: $probe_retained_dir"
OMP_CONTEXT_PROBE_DIR="$temp/retained" probe_configure 2>/dev/null

install -d -m 0700 "$temp_dir/inside"
install -d -m 0770 "$temp/group-writable"
install -d -m 0500 "$temp/unwritable"
install -d -m 0700 "$probe_root" "$probe_root/tmp" "$probe_root/tmp/probe" "$probe_root/retained"
ln -s "$temp/retained" "$temp/retained-link"
reject_directory() {
  local directory=$1 reason=$2
  if (OMP_CONTEXT_PROBE_DIR="$directory" probe_configure) >/dev/null 2>&1; then
    fail "retained directory accepted $reason"
  fi
}
reject_directory 'retained' 'a relative path'
reject_directory "$temp/retained-link" 'a symlinked directory'
reject_directory "$temp/absent" 'a missing directory'
reject_directory "$temp/group-writable" 'a group-writable directory'
# Retention runs as the runner, so a directory it could not create in has to be
# refused before the canary spends 40 live provider calls. Root bypasses the
# permission entirely, so the case only means anything for an ordinary runner.
[[ "$runner_uid" == '0' ]] ||
  reject_directory "$temp/unwritable" 'a directory the runner cannot write'
reject_directory "$temp_dir/inside" 'a directory inside the release temporary tree'
reject_directory "$probe_root/retained" 'a directory inside the disposable isolation root'
if (operation='preflight'; OMP_CONTEXT_PROBE_DIR="$temp/retained" probe_configure) >/dev/null 2>&1; then
  fail 'probe mode was accepted for --preflight'
fi

# Outcome classification. A full probe is the runtime's completion frame plus a
# record set the exporter validated as the whole schedule: the frame alone says
# the run reached its end, not that every record survived validation.
printf '%s\n' '{"type":"call","sequence":1}' \
  '{"type":"error","error_code":"probe_completed","error_stage":"probe"}' >"$temp/full.jsonl"
printf '%s\n' '{"type":"call","sequence":1}' \
  '{"type":"error","error_code":"runtime_readback_failed","error_stage":"call"}' >"$temp/partial.jsonl"
printf '%s\n' '{"type":"call","sequence":1}' >"$temp/silent.jsonl"
outcome_is() {
  local expected=$1 reason=$2; shift 2
  [[ "$(probe_outcome "$@")" == "$expected" ]] || fail "$reason"
}
outcome_is 'full probe_completed' 'a completed probe over the whole schedule was not full' "$temp/full.jsonl" true 40 18 0
outcome_is 'partial missing_records' 'a set the exporter would not call complete read as full' "$temp/full.jsonl" false 40 18 0
outcome_is 'partial missing_records' 'a short call record set read as full' "$temp/full.jsonl" true 39 18 0
outcome_is 'partial missing_records' 'a short compaction record set read as full' "$temp/full.jsonl" true 40 17 0
outcome_is 'partial missing_records' 'a set holding a rejected record read as full' "$temp/full.jsonl" true 40 18 1
outcome_is 'partial missing_records' 'an unparseable summary read as full' "$temp/full.jsonl" true unknown unknown unknown
outcome_is 'partial runtime_readback_failed' 'an interrupted probe was not partial' "$temp/partial.jsonl" true 40 18 0
outcome_is 'partial unparseable' 'a transcript without an error frame read as completed' "$temp/silent.jsonl" true 40 18 0

# Guard order: the helper must not run while the canary UID may still be alive.
marker="$temp/helper-invoked"
probe_export_tool="$temp/stub-export"
cat >"$probe_export_tool" <<STUB
#!/usr/bin/env bash
printf 'invoked\n' >>'$marker'
printf '{"accepted":1,"rejected":0,"call_records":1,"compaction_records":0,"complete":false}\n'
STUB
chmod 0700 "$probe_export_tool"
release_canary_uid=$runner_uid
live_canary_started=1
if probe_export_records "$probe_root/tmp" "$temp/full.jsonl" final >/dev/null 2>&1; then
  fail 'export ran before the live canary UID was stopped'
fi
[[ ! -e "$marker" ]] || fail 'export helper was invoked before UID cleanup'
live_canary_started=0
release_canary_uid_is_process_free() { return 1; }
if probe_export_records "$probe_root/tmp" "$temp/full.jsonl" final >/dev/null 2>&1; then
  fail 'export ran while the canary UID still owned processes'
fi
[[ ! -e "$marker" ]] || fail 'export helper was invoked without a process-free UID'
release_canary_uid_is_process_free() { return 0; }
if probe_export_records "$temp/other" "$temp/full.jsonl" final >/dev/null 2>&1; then
  fail 'export accepted a source root outside the isolation root'
fi
[[ ! -e "$marker" ]] || fail 'export helper was invoked for an unsafe source root'

# The real helper: validated records are staged, unsafe sources are not, and
# nothing at all reaches the operator's directory yet.
probe_export_tool="$temp/probeexport"
(cd -- "$repo" && go build -trimpath -o "$probe_export_tool" ./scripts/companion-release/probeexport) ||
  fail 'probe export helper does not build'
source_file="$probe_root/tmp/probe/probe.jsonl"
call_record='{"kind":"call","sequence":1,"variant":"A","session_sequence":1,"session_segment":1,"usage_identity_delta":0,"elapsed_ms":12}'
compaction_record='{"kind":"compaction","sequence":2,"variant":"B","session_sequence":2,"session_segment":1,"usage_identity_delta":0,"elapsed_ms":34,"outcome":"completed","method":"snapcompact","refusal":"none","attempt_coverage":"unknown"}'
tampered_record='{"kind":"compaction","sequence":3,"variant":"B","session_sequence":3,"session_segment":1,"usage_identity_delta":0,"elapsed_ms":34,"outcome":"completed","method":"snapcompact","refusal":"none","attempt_coverage":"unknown","content_types":{"Opaque-Item!":1}}'
# Restores the untampered canary-side source and stages it under a fresh nonce.
stage_records() {
  local nonce=$1
  rm -rf -- "$probe_root/tmp/probe"
  install -d -m 0700 "$probe_root/tmp/probe"
  printf '%s\n' "$call_record" "$compaction_record" >"$source_file"
  chmod 0600 "$source_file"
  dispatch_nonce=$nonce
  probe_export_records "$probe_root/tmp" "$temp/full.jsonl" final 2>"$temp/stage-receipt" ||
    fail "probe export refused a valid record set for ${nonce}: $(<"$temp/stage-receipt")"
  stage_summary=$(<"$temp/stage-receipt")
  [[ -f "$probe_staging_dir/probe-${nonce}.jsonl" ]] || fail "validated records were not staged for $nonce"
  [[ "$probe_pending_name" == "probe-${nonce}.jsonl" ]] || fail "staging left no pending state for $nonce"
}
stage_records "$dispatch_nonce"
staged="$probe_staging_dir/probe-${dispatch_nonce}.jsonl"
retained="$temp/retained/probe-${dispatch_nonce}.jsonl"
# This fixture is a valid but truncated record set. It is staged, and a
# completed runtime frame does not let it read as a completed observation.
[[ "$stage_summary" == *'outcome=partial reason=missing_records export_status=0 probe_records_accepted=2 probe_records_rejected=0 probe_call_records=1 probe_compaction_records=1 complete=false'* ]] ||
  fail "probe summary is not the body-free staged receipt: $stage_summary"
[[ "$stage_summary" != *retained=* ]] || fail 'the staging receipt already claimed a retained file'
[[ ! -L "$staged" && "$(/usr/bin/stat -f '%u:%Lp' "$staged")" == "${runner_uid}:600" ]] ||
  fail 'staged probe records are not a runner-owned 0600 regular file'
[[ ! -e "$retained" ]] || fail 'records reached the retained directory before cleanup completed'
if probe_export_records "$probe_root/tmp" "$temp/full.jsonl" final >/dev/null 2>&1; then
  fail 'a second export overwrote the staged probe file'
fi

# An interrupted probe still stages what it has, and says so.
partial_nonce=$(printf '%s' "partial:$dispatch_nonce" | shasum -a 256 | awk '{print substr($1,1,32)}')
partial_summary=$(dispatch_nonce=$partial_nonce \
  probe_export_records "$probe_root/tmp" "$temp/partial.jsonl" final 2>&1) ||
  fail "interrupted probe export refused a valid record set: $partial_summary"
[[ "$partial_summary" == *'outcome=partial reason=runtime_readback_failed export_status=0 probe_records_accepted=2 probe_records_rejected=0'* ]] ||
  fail "interrupted probe receipt differs: $partial_summary"
[[ -f "$probe_staging_dir/probe-${partial_nonce}.jsonl" ]] || fail 'interrupted probe staged nothing'

# Cleanup did not complete: the staged bytes go with the temporary tree and the
# operator's directory stays untouched.
probe_finalize_records 1 2>/dev/null || fail 'finalization reported an error after a failed cleanup'
[[ ! -e "$retained" ]] || fail 'a failed cleanup still retained probe records'
[[ -z "$probe_pending_name" ]] || fail 'a discarded retention left pending state behind'

# Cleanup completed: the same descriptor-safe helper republishes the staged
# records into a new operator-side file it created exclusively.
retain_nonce=$(printf '%s' "retain:$dispatch_nonce" | shasum -a 256 | awk '{print substr($1,1,32)}')
stage_records "$retain_nonce"
retained_records="$temp/retained/probe-${retain_nonce}.jsonl"
probe_finalize_records 0 2>/dev/null || fail 'finalization refused to retain after a completed cleanup'
[[ -f "$retained_records" && ! -L "$retained_records" ]] || fail 'a completed cleanup retained nothing'
[[ "$(/usr/bin/stat -f '%u:%Lp' "$retained_records")" == "${runner_uid}:600" ]] ||
  fail 'retained probe records are not runner-owned 0600'
[[ "$(wc -l <"$retained_records" | tr -d ' ')" == '2' ]] ||
  fail 'retained record count differs from the staged count'
[[ "$(/usr/bin/stat -f '%l' "$retained_records")" == '1' &&
   "$(/usr/bin/stat -f '%i' "$retained_records")" != "$(/usr/bin/stat -f '%i' "$probe_staging_dir/probe-${retain_nonce}.jsonl")" ]] ||
  fail 'retention published the staged inode instead of re-exporting the records'
[[ -z "$probe_pending_name" ]] || fail 'a completed retention left pending state behind'
probe_finalize_records 0 2>/dev/null || fail 'a second finalization reported an error'
[[ "$(wc -l <"$retained_records" | tr -d ' ')" == '2' ]] || fail 'a second finalization rewrote the retained file'

# An out-of-range identifier is tampered data: counted, refused, unpublished.
tampered_nonce=$(printf '%s' "tampered:$dispatch_nonce" | shasum -a 256 | awk '{print substr($1,1,32)}')
printf '%s\n' "$call_record" "$compaction_record" "$tampered_record" >"$source_file"
tampered_summary=$(dispatch_nonce=$tampered_nonce \
  probe_export_records "$probe_root/tmp" "$temp/full.jsonl" final 2>&1) && tampered_status=0 || tampered_status=$?
[[ "$tampered_status" -ne 0 ]] || fail 'out-of-range identifier was exported as accepted'
[[ "$tampered_summary" == *'probe_records_rejected=1'* ]] || fail "rejected count is missing: $tampered_summary"
[[ ! -e "$probe_staging_dir/probe-${tampered_nonce}.jsonl" ]] ||
  fail 'refused probe records were staged'
[[ ! -e "$temp/retained/probe-${tampered_nonce}.jsonl" ]] ||
  fail 'refused probe records created a retained file'

# Unsafe source. Each case carries records the helper would otherwise accept, so
# only the path or the owner can be the reason to refuse.
export_refuses() {
  local nonce=$1 reason=$2 output status=0
  output=$(dispatch_nonce=$nonce probe_export_records "$probe_root/tmp" "$temp/full.jsonl" final 2>&1) ||
    status=$?
  [[ "$status" -ne 0 ]] || fail "probe export accepted $reason"
  [[ ! -e "$probe_staging_dir/probe-${nonce}.jsonl" ]] || fail "probe export staged a file for $reason"
  [[ ! -e "$temp/retained/probe-${nonce}.jsonl" ]] || fail "probe export retained a file for $reason"
  printf '%s' "$output"
}
install -d -m 0700 "$temp/attacker"
printf '%s\n' "$call_record" "$compaction_record" >"$temp/attacker/probe.jsonl"
chmod 0600 "$temp/attacker/probe.jsonl"
rm -rf -- "$probe_root/tmp/probe"
ln -s "$temp/attacker" "$probe_root/tmp/probe"
symlink_nonce=$(printf '%s' "symlink:$dispatch_nonce" | shasum -a 256 | awk '{print substr($1,1,32)}')
export_refuses "$symlink_nonce" 'a symlinked source directory' >/dev/null
# A protected file behind the swapped component must never reach the receipt.
printf 'sentinel-protected-content\n' >"$temp/sentinel.jsonl"
chmod 0600 "$temp/sentinel.jsonl"
cp "$temp/sentinel.jsonl" "$temp/attacker/probe.jsonl"
rm -rf -- "$probe_root/tmp/probe"
ln -s "$temp/attacker" "$probe_root/tmp/probe"
sentinel_nonce=$(printf '%s' "sentinel:$dispatch_nonce" | shasum -a 256 | awk '{print substr($1,1,32)}')
unsafe_output=$(export_refuses "$sentinel_nonce" 'a swapped protected source')
[[ "$unsafe_output" != *'sentinel-protected-content'* ]] || fail 'probe export echoed protected bytes'

# Retention is committed by cleanup and only by a cleanup that dismantled every
# canary trace. The sudo invalidation and the two account steps are replaced so
# no test invokes sudo; the git and gh surfaces are captured so a probe-lane
# cleanup that reached a remote mutation would be visible.
# shellcheck source=/dev/null
source "$runtime_lib"
mutations="$temp/mutations"
: >"$mutations"
run_cleanup() {
  local status=$1 gate=$2
  (
    sudo_keepalive_pid=''; evidence_source_commit=''; retain_prep_lock=0
    isolation_roots=("$probe_root|0:0|0:0")
    release_canary_user=''
    cleanup_live_canary_uid() { [[ "$gate" != 'uid' ]]; }
    remove_release_canary_account() { [[ "$gate" != 'account' ]]; }
    remove_isolation_root() { [[ "$gate" != 'root' ]]; }
    invalidate_sudo_authorization() { [[ "$gate" != 'sudo' ]]; }
    git() { printf 'git %s\n' "$*" >>"$mutations"; return 1; }
    gh() { printf 'gh %s\n' "$*" >>"$mutations"; return 1; }
    cleanup "$status"
  ) >/dev/null 2>&1
}
# Stages a record set, runs one cleanup, and reports the retained path it would
# have published in cleanup_retained.
cleanup_case() {
  local gate=$1 exit_status=0
  local nonce
  nonce=$(printf '%s' "cleanup:$gate:$dispatch_nonce" | shasum -a 256 | awk '{print substr($1,1,32)}')
  temp_dir="$temp/prep-$gate"
  install -d -m 0700 "$temp_dir"
  OMP_CONTEXT_PROBE_DIR="$temp/retained" probe_configure 2>/dev/null
  probe_export_tool="$temp/probeexport"
  stage_records "$nonce"
  # A probe always exits nonzero, so retention must survive that status.
  run_cleanup 1 "$gate" || exit_status=$?
  [[ "$exit_status" -eq 1 ]] || fail "cleanup for gate ${gate:-none} exited $exit_status"
  [[ ! -e "$temp_dir" ]] || fail "cleanup left the release temporary tree behind for gate ${gate:-none}"
  cleanup_retained="$temp/retained/probe-${nonce}.jsonl"
}
for failed_gate in uid account root sudo; do
  cleanup_case "$failed_gate"
  [[ ! -e "$cleanup_retained" ]] ||
    fail "a cleanup that failed the $failed_gate step still retained probe records"
done
cleanup_case ''
completed_file=$cleanup_retained
[[ -f "$completed_file" && ! -L "$completed_file" ]] ||
  fail 'a completed cleanup did not retain the staged probe records'
[[ "$(/usr/bin/stat -f '%u:%Lp' "$completed_file")" == "${runner_uid}:600" ]] ||
  fail 'cleanup retained probe records that are not runner-owned 0600'
[[ "$(wc -l <"$completed_file" | tr -d ' ')" == '2' ]] || fail 'cleanup retained a different record count'
[[ ! -s "$mutations" ]] || fail "probe-lane cleanup reached a remote surface: $(<"$mutations")"

# A probe must never exercise the release tag signing key, and the normal lane
# must still prove its authority before anything is published.
signing_marker="$temp/signing-invoked"
verify_tag_signing_authority() { printf 'invoked\n' >>"$signing_marker"; }
probe_enabled=1
probe_verify_tag_signing_authority 2>/dev/null
[[ ! -e "$signing_marker" ]] || fail 'a probe run reached the release tag signing authority check'
probe_enabled=0
probe_verify_tag_signing_authority 2>/dev/null
[[ -f "$signing_marker" ]] || fail 'the normal lane stopped verifying release tag signing authority'
probe_enabled=1

# Pin admission. A probe may proceed against a version refused on performance
# and only that: an unverified major is refused, and the candidate's own
# identity and protocol verification still runs.
pin_work="$temp/pin"
install -d -m 0700 "$pin_work/scripts/companion-release" "$pin_work/scripts/release-tools" "$pin_work/bin"
cp "$pin" "$pin_work/scripts/release-tools/advance-omp-pin.sh"
pin_declaration="$pin_work/scripts/companion-release/prepare-release.sh"
printf '%s\n' "[[ \"\$(\"\$staged_omp\" --version)\" == 'omp/17.2.7' ]] || fail 'version'" \
  "readonly expected_omp_sha256='$(printf '0123456789abcdef%.0s' 1 2 3 4)'" >"$pin_declaration"
printf '#!/usr/bin/env bash\nexit 1\n' >"$pin_work/bin/gh"
chmod 0700 "$pin_work/bin/gh"
pin_declaration_digest=$(shasum -a 256 "$pin_declaration" | awk '{print $1}')
run_pin() {
  local output status=0
  output=$(cd -- "$pin_work" && env PATH="$pin_work/bin:$PATH" \
    bash scripts/release-tools/advance-omp-pin.sh "$@" 2>&1) || status=$?
  [[ "$status" -ne 0 ]] || fail "advance-omp-pin.sh $* unexpectedly succeeded"
  printf '%s' "$output"
}
refused_output=$(run_pin 18.1.13)
[[ "$refused_output" == *'reduces context below the floor'* ]] || fail 'the 18.1.13 refusal changed'
[[ "$refused_output" != *'cannot download'* ]] || fail 'a refused version reached the candidate asset'
measure_output=$(run_pin 18.1.13 --measure)
[[ "$measure_output" == *'reduces context below the floor'* ]] ||
  fail '--measure stopped refusing a version measured below the floor'
probe_output=$(run_pin 18.1.13 --probe)
[[ "$probe_output" == *'no measurement, verdict, evidence, tag or coordinate'* ]] ||
  fail 'the probe admission does not disclaim measurement'
[[ "$probe_output" == *'cannot download'* ]] ||
  fail 'probe admission skipped candidate identity verification'
unverified_output=$(run_pin 19.0.0 --probe)
[[ "$unverified_output" == *'effective compaction chain is unverified'* ]] ||
  fail 'a probe was admitted for a major with no verified compaction chain'
[[ "$unverified_output" != *'cannot download'* ]] || fail 'an unverified major reached the candidate asset'
[[ "$(shasum -a 256 "$pin_declaration" | awk '{print $1}')" == "$pin_declaration_digest" ]] ||
  fail 'a probe moved the pin without verifying the candidate'

(cd -- "$repo" && go run ./cmd/source-lines --max 300 "$probe_lib" "$prep" "$runtime_lib" "$pin" \
  "$wrapper" "$tests_dir/release-probe-hardening-test.sh" >/dev/null) ||
  fail 'probe lane sources exceed 300 code lines'
printf 'release probe hardening test: PASS\n'
