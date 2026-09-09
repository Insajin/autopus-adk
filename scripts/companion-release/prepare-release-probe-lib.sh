#!/usr/bin/env bash
# Probe-only record retention (SPEC-OMP-007 T0), sourced only by prepare-release.sh.
#
# A probe run measures nothing and publishes nothing. It exists to keep the
# metadata records the canary wrote inside its own isolated TMPDIR, which the
# canary UID owns and can therefore replace at any component. Nothing here
# trusts those bytes: the export helper walks a no-follow descriptor chain from
# the source root, fstats the descriptor it read, and republishes only records
# it could validate into a new runner-owned file. Opt-in is the operator
# variable OMP_CONTEXT_PROBE_DIR, which names the retained directory and never
# reaches the canary environment.
#
# Retention is staged. The export that runs while the isolation root still
# exists publishes into a runner-owned staging directory inside the release
# temporary tree, so a run that cannot dismantle the canary account, its
# isolation root or its sudo authorization leaves nothing behind at all: the
# staged records die with the temporary tree. Only a cleanup that completed
# every one of those steps runs the same descriptor-safe helper a second time,
# out of the staging directory and into the operator's retained directory,
# where the file is created exclusively and never overwritten. The second pass
# re-validates the schema rather than copying bytes, so the retained file is
# re-serialized from records the exporter accepted twice.

probe_enabled=0
probe_retained_dir=''
probe_staging_dir=''
probe_export_tool=''
probe_pending_name=''
probe_pending_accepted=''
probe_pending_outcome=''
probe_pending_reason=''
# The cohort is 20 task pairs and 40 provider calls with one compaction attempt
# per optimized call after the first, so a probe that recorded the whole
# schedule retains exactly this many records of each kind. The helper decides
# completeness on the schedule itself; these counts only sharpen the receipt.
probe_expected_calls=40
probe_expected_compactions=18

probe_directory_is_trusted() {
  local directory=$1 owner mode
  [[ "$directory" == /* && "$directory" != */ && "$directory" != *//* &&
     "$directory" != *'/../'* && "$directory" != */.. &&
     "$directory" =~ ^/[A-Za-z0-9._/-]{1,192}$ ]] || return 1
  [[ "$directory" != "$temp_dir" && "$directory" != "$temp_dir"/* ]] || return 1
  [[ "$directory" != '/private/tmp/autopus-adk-release-prep-'* ]] || return 1
  [[ -d "$directory" && ! -L "$directory" ]] || return 1
  # The retaining export runs as the runner, so a directory the runner cannot
  # create in is a configuration error worth refusing before the canary runs
  # rather than after 40 live provider calls have already been spent.
  [[ -w "$directory" ]] || return 1
  owner=$(/usr/bin/stat -f '%u' "$directory") || return 1
  mode=$(/usr/bin/stat -f '%Lp' "$directory") || return 1
  [[ "$owner" == "$runner_uid" || "$owner" == '0' ]] || return 1
  [[ "$mode" =~ ^[0-7]{3,4}$ ]] || return 1
  (( (8#$mode & 0022) == 0 ))
}

# The staging directory is created by this run inside its own 0700 temporary
# tree. It is re-checked before each export because the helper walks it with
# O_NOFOLLOW and, on the retaining pass, reads its records as the runner.
probe_staging_is_trusted() {
  local owner mode
  [[ "$probe_staging_dir" == /* && "$probe_staging_dir" != */ && "$probe_staging_dir" != *//* &&
     "$probe_staging_dir" != *'/../'* && "$probe_staging_dir" != */.. &&
     "$probe_staging_dir" =~ ^/[A-Za-z0-9._/-]{1,192}$ ]] || return 1
  # Staging is the runner's own tree, never anything the canary UID could
  # reach: the isolation root is the one place a probe writes as another user.
  [[ "$probe_staging_dir" != '/private/tmp/autopus-adk-release-prep-'* ]] || return 1
  [[ -d "$probe_staging_dir" && ! -L "$probe_staging_dir" ]] || return 1
  owner=$(/usr/bin/stat -f '%u' "$probe_staging_dir") || return 1
  mode=$(/usr/bin/stat -f '%Lp' "$probe_staging_dir") || return 1
  [[ "$owner" == "$runner_uid" && "$mode" == '700' ]]
}

# Reads the operator opt-in. A configured probe forbids every publication path
# for the whole run, so it is resolved before the first candidate is built.
probe_configure() {
  local directory=${OMP_CONTEXT_PROBE_DIR-} physical=''
  [[ -n "$directory" ]] || return 0
  [[ "$operation" == 'apply' ]] || fail 'probe mode requires --apply'
  [[ "$directory" == /* && -d "$directory" && ! -L "$directory" ]] ||
    fail 'retained probe directory must be an existing absolute directory and not a symlink'
  # The helper opens every path component with O_NOFOLLOW, so a retained root
  # reached through a symlinked ancestor is unusable. Resolve it here rather
  # than discovering it in the helper after the canary has already run.
  physical=$(cd -- "$directory" && pwd -P) || fail 'retained probe directory cannot be resolved'
  probe_directory_is_trusted "$physical" ||
    fail 'retained probe directory must be operator-owned and runner-writable, not group or world writable, outside the release temporary tree'
  probe_retained_dir=$physical
  probe_export_tool="$temp_dir/probeexport"
  install -d -m 0700 "$temp_dir/probe-staging" || fail 'probe staging directory cannot be created'
  probe_staging_dir=$(cd -- "$temp_dir/probe-staging" && pwd -P) ||
    fail 'probe staging directory cannot be resolved'
  probe_staging_is_trusted || fail 'probe staging directory is unsafe'
  probe_enabled=1
  printf 'companion release prep: probe mode; records are staged under %s and retained under %s only once cleanup has completed; no report, evidence, prep lock, tag or coordinate is published\n' \
    "$probe_staging_dir" "$probe_retained_dir" >&2
}

# The tag signing authority check proves the release key by creating a real
# signed tag object in a temporary clone. A probe publishes no tag and must not
# exercise the signing key at all, so the check belongs to the normal lane
# only; every other verification the normal lane runs still runs.
probe_verify_tag_signing_authority() {
  if [[ "$probe_enabled" -eq 1 ]]; then
    printf 'companion release prep: probe mode; the release tag signing authority check and its temporary signed tag are skipped\n' >&2
    return 0
  fi
  verify_tag_signing_authority
}

probe_build_export_tool() {
  [[ "$probe_enabled" -eq 1 ]] || return 0
  env GOENV=off GOTOOLCHAIN="$expected_go_toolchain" go build -trimpath \
    -o "$probe_export_tool" ./scripts/companion-release/probeexport
  [[ -f "$probe_export_tool" && ! -L "$probe_export_tool" && -x "$probe_export_tool" ]] ||
    fail 'probe export helper is unusable'
}

# One invocation of the export helper. Its stdout is the body-free summary,
# which it prints on both the accepting and the refusing path, and its exit
# status is returned unchanged.
probe_run_export() {
  local source_root=$1 source_relative=$2 source_uid=$3 destination_root=$4 name=$5
  local elevate=()
  [[ -f "$probe_export_tool" && ! -L "$probe_export_tool" && -x "$probe_export_tool" ]] || return 1
  # A canary-owned source is a 0600 file under a 0700 directory, so reading its
  # descriptor needs elevation. The runner's own staged file never does, and
  # the source permissions are never loosened either way.
  if [[ "$source_uid" != "$runner_uid" && "$EUID" -ne 0 ]]; then elevate=(/usr/bin/sudo -n); fi
  ${elevate[@]+"${elevate[@]}"} "$probe_export_tool" --source-root "$source_root" \
    --source-relative "$source_relative" --destination-root "$destination_root" \
    --destination-name "$name" --source-uid "$source_uid" \
    --owner-uid "$runner_uid" --owner-gid "$runner_gid"
}

# The body-free counts of one summary, as tab-separated accepted, rejected,
# call records, compaction records and completeness. Every field is validated
# by the caller, so a helper that printed nothing or printed something
# unparseable reads as unknown rather than as a completed observation.
probe_summary_fields() {
  local summary=$1 fields=''
  fields=$(jq -r '[(.accepted | tostring), (.rejected | tostring), (.call_records | tostring),
    (.compaction_records | tostring), (if .complete == true then "true" else "false" end)] | @tsv' \
    <<<"$summary" 2>/dev/null) || fields=''
  [[ "$fields" == *$'\t'*$'\t'*$'\t'*$'\t'* ]] || fields=$'unknown\tunknown\tunknown\tunknown\tfalse'
  printf '%s' "$fields"
}

# A full probe is the runtime's completion frame plus a record set the exporter
# itself validated as the whole schedule. The frame alone only says the run
# reached its end, not that every record survived validation, so a short or
# rejected record set is a partial observation whatever the frame says.
probe_outcome() {
  local output=$1 complete=$2 calls=$3 compactions=$4 rejected=$5 code=''
  code=$(jq -s -r '[.[] | select(.type? == "error")] |
    if length == 0 then "" else (.[-1].error_code? // "") end' "$output" 2>/dev/null) || code=''
  [[ "$code" =~ ^[a-z_]{1,64}$ ]] || code=unparseable
  [[ "$code" == 'probe_completed' ]] || { printf 'partial %s\n' "$code"; return 0; }
  if [[ "$complete" == 'true' && "$calls" == "$probe_expected_calls" &&
        "$compactions" == "$probe_expected_compactions" && "$rejected" == '0' ]]; then
    printf 'full probe_completed\n'
  else
    printf 'partial missing_records\n'
  fi
}

# Runs after the canary UID process set is stopped and verified absent, and
# before the account record, the isolation root or the failure receipt. It
# publishes into the staging directory only: nothing reaches the operator's
# retained directory until cleanup has completed. Export failure stages
# nothing and is not survivable: the caller aborts.
probe_export_records() {
  local source_root=$1 output=$2 label=$3 summary='' export_status=0 name=''
  local accepted rejected calls compactions complete outcome reason
  [[ "$probe_enabled" -eq 1 ]] || return 0
  probe_directory_is_trusted "$probe_retained_dir" || return 1
  probe_staging_is_trusted || return 1
  [[ "$source_root" == '/private/tmp/autopus-adk-release-prep-'*'/tmp' ]] || return 1
  [[ "${dispatch_nonce:-}" =~ ^[0-9a-f]{32}$ ]] || return 1
  [[ "${release_canary_uid:-}" =~ ^[0-9]+$ && "${runner_uid:-}" =~ ^[0-9]+$ &&
     "${runner_gid:-}" =~ ^[0-9]+$ ]] || return 1
  [[ "${live_canary_started:-1}" -eq 0 ]] || return 1
  release_canary_uid_is_process_free "$release_canary_uid" || return 1
  name="probe-${dispatch_nonce}.jsonl"
  summary=$(probe_run_export "$source_root" probe/probe.jsonl "$release_canary_uid" \
    "$probe_staging_dir" "$name") || export_status=$?
  IFS=$'\t' read -r accepted rejected calls compactions complete <<<"$(probe_summary_fields "$summary")"
  [[ "$accepted" =~ ^[0-9]+$ ]] || accepted='unknown'
  [[ "$rejected" =~ ^[0-9]+$ ]] || rejected='unknown'
  [[ "$calls" =~ ^[0-9]+$ ]] || calls='unknown'
  [[ "$compactions" =~ ^[0-9]+$ ]] || compactions='unknown'
  IFS=' ' read -r outcome reason <<<"$(probe_outcome "$output" "$complete" "$calls" "$compactions" "$rejected")"
  printf 'companion release prep: %s probe summary: outcome=%s reason=%s export_status=%s probe_records_accepted=%s probe_records_rejected=%s probe_call_records=%s probe_compaction_records=%s complete=%s staged=%s/%s\n' \
    "$label" "$outcome" "$reason" "$export_status" "$accepted" "$rejected" "$calls" "$compactions" \
    "$complete" "$probe_staging_dir" "$name" >&2
  [[ "$export_status" -eq 0 && "$accepted" != 'unknown' && "$rejected" != 'unknown' ]] || return 1
  probe_pending_name=$name
  probe_pending_accepted=$accepted
  probe_pending_outcome=$outcome
  probe_pending_reason=$reason
}

# Commits or abandons the staged records, from cleanup and only from cleanup.
# The staged bytes live inside the release temporary tree that cleanup is about
# to remove, so a cleanup that left the canary account, its isolation root or a
# sudo authorization behind retains nothing: the staged records are dropped
# with the tree and no file is ever created under the operator's directory.
probe_finalize_records() {
  local cleanup_failed=$1 summary='' export_status=0 accepted=''
  [[ "$probe_enabled" -eq 1 && -n "$probe_pending_name" ]] || return 0
  if [[ "$cleanup_failed" -ne 0 ]]; then
    probe_pending_name=''
    printf 'companion release prep: probe records are discarded with the release temporary tree; cleanup did not complete and nothing is retained\n' >&2
    return 0
  fi
  if ! probe_directory_is_trusted "$probe_retained_dir" || ! probe_staging_is_trusted; then
    printf 'companion release prep: probe record retention refused; the retained or staging directory is no longer trusted\n' >&2
    return 1
  fi
  summary=$(probe_run_export "$probe_staging_dir" "$probe_pending_name" "$runner_uid" \
    "$probe_retained_dir" "$probe_pending_name") || export_status=$?
  IFS=$'\t' read -r accepted _ _ _ _ <<<"$(probe_summary_fields "$summary")"
  if [[ "$export_status" -ne 0 || "$accepted" != "$probe_pending_accepted" ]]; then
    printf 'companion release prep: probe record retention failed: export_status=%s probe_records_accepted=%s expected=%s\n' \
      "$export_status" "$accepted" "$probe_pending_accepted" >&2
    return 1
  fi
  printf 'companion release prep: probe records retained: outcome=%s reason=%s probe_records_accepted=%s retained=%s/%s\n' \
    "$probe_pending_outcome" "$probe_pending_reason" "$accepted" "$probe_retained_dir" "$probe_pending_name" >&2
  probe_pending_name=''
}

# The last guard: a probe that somehow produced a clean canary must still not
# reach report validation, policy derivation, signing, locks or coordinates.
probe_terminate() {
  [[ "$probe_enabled" -eq 1 ]] || return 0
  printf 'companion release prep: probe run complete; report validation, signing, prep lock, tag and coordinate publication are disabled\n' >&2
  exit 1
}
