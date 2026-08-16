#!/usr/bin/env bash
# Internal function library sourced only by prepare-release.sh.

remove_isolation_root() {
  local record=$1 root expected_root_identity expected_parent_identity
  IFS='|' read -r root expected_root_identity expected_parent_identity <<<"$record"
  if [[ ! -e "$root" && ! -L "$root" ]]; then return 0; fi
  [[ ! -L "$root" &&
     "$(/usr/bin/stat -f '%u:%Lp' "$root")" == '0:755' &&
     "$(/usr/bin/stat -f '%d:%i' "$root")" == "$expected_root_identity" &&
     "$(/usr/bin/stat -f '%d:%i' /private/tmp)" == "$expected_parent_identity" ]] ||
    return 1
  /usr/bin/sudo -n /bin/rm -rf -- "$root" || return 1
  [[ ! -e "$root" && ! -L "$root" ]]
}
cleanup() {
  local status=$1 remote_status=0 remote_source='' record cleanup_failed=0
  if [[ -n "${sudo_keepalive_pid:-}" ]]; then
    kill "$sudo_keepalive_pid" >/dev/null 2>&1 || true
    wait "$sudo_keepalive_pid" >/dev/null 2>&1 || true
    sudo_keepalive_pid=''
  fi
  for record in "${isolation_roots[@]-}"; do
    [[ -n "$record" ]] || continue
    if ! remove_isolation_root "$record"; then
      printf 'companion release prep: isolated canary cleanup failed for %s\n' "${record%%|*}" >&2
      cleanup_failed=1
    fi
  done
  if ! /usr/bin/sudo -k; then
    printf 'companion release prep: sudo authorization invalidation failed\n' >&2
    cleanup_failed=1
  fi
  if [[ -n "$evidence_source_commit" && "$retain_prep_lock" -eq 0 ]]; then
    remote_source=$(GIT_TERMINAL_PROMPT=0 GIT_HTTP_LOW_SPEED_LIMIT=1 GIT_HTTP_LOW_SPEED_TIME=10 \
      git ls-remote --exit-code --refs origin "$evidence_source_ref" 2>/dev/null) || remote_status=$?
    if [[ "$remote_status" -eq 0 && "$remote_source" == "$evidence_source_commit"$'\t'"$evidence_source_ref" ]]; then
      GIT_TERMINAL_PROMPT=0 GIT_HTTP_LOW_SPEED_LIMIT=1 GIT_HTTP_LOW_SPEED_TIME=10 \
        git push --force-with-lease="${evidence_source_ref}:${evidence_source_commit}" \
        origin ":${evidence_source_ref}" >/dev/null 2>&1 ||
        printf 'companion release prep: warning: owned prep lock cleanup failed\n' >&2
    elif [[ "$remote_status" -ne 2 ]]; then
      printf 'companion release prep: warning: prep lock ownership could not be inspected\n' >&2
    fi
  fi
  rm -rf -- "$temp_dir"
  if [[ "$status" -eq 0 && "$cleanup_failed" -ne 0 ]]; then status=1; fi
  exit "$status"
}
matched_variable() {
  local name=$1 repository_value environment_count environment_value
  repository_value=$(gh variable get "$name" --repo "$repository") ||
    fail "repository variable ${name} is unavailable"
  environment_count=$(jq --arg name "$name" '[.[] | select(.name == $name)] | length' \
    <<<"$environment_variables")
  case "$environment_count" in
    0) ;;
    1)
      environment_value=$(jq -r --arg name "$name" '.[] | select(.name == $name) | .value' \
        <<<"$environment_variables")
      [[ "$repository_value" == "$environment_value" ]] ||
        fail "repository/environment variable ${name} differs"
      ;;
    *) fail "environment variable ${name} is ambiguous" ;;
  esac
  [[ -n "$repository_value" ]] || fail "repository variable ${name} is unavailable"
  printf '%s' "$repository_value"
}

build_candidate() {
  local policy=$1 output=$2
  env COMPANION_RELEASE_TAG="$release_tag" GITHUB_SHA="$source_commit" COMPANION_SOURCE_TREE="$source_tree" OMP_CONTEXT_STATIC_POLICY_B64="$policy" \
    scripts/companion-release/build-omp-context-candidate.sh "$output"
}
extract_project() {
  local destination=$1
  install -d -m 0700 "$destination"
  git archive "$source_commit" | tar -x -C "$destination"
}
capture_canary_progress() {
  local output=$1 label=$2
  [[ "$label" == 'final' ]] || return 1
  [[ "$output" == /* && ! -e "$output" && ! -L "$output" ]] || return 1
  : >"$output" || return 1
  LC_ALL=C /usr/bin/awk -v output="$output" -v label="$label" '
    {
      print $0 >> output
      close(output)
      printf "companion release prep: %s production canary progress (%d/42 records)\n", label, NR > "/dev/stderr"
      fflush("/dev/stderr")
    }
  '
}
canary_failure_receipt() {
  local label=$1 canary_status=$2 output=$3 transcript_records failure_fields
  local failure_code failure_stage failed_sequence
  if [[ ! "$canary_status" =~ ^[1-9][0-9]*$ ]] || (( canary_status > 255 )); then
    canary_status=1
  fi
  transcript_records=$(wc -l <"$output" | tr -d ' ')
  [[ "$transcript_records" =~ ^[0-9]+$ ]] || transcript_records=unknown
  failure_fields=$(jq -s -r \
    '[.[] | select(.type? == "error")] | if length == 0 then
       ["unclassified", "unknown", "0"] else
       [(.[-1].error_code? // "unclassified"), (.[-1].error_stage? // "unknown"),
        ((.[-1].failed_sequence? // 0) | tostring)] end | @tsv' "$output" 2>/dev/null) ||
    failure_fields=$'unparseable\tunknown\t0'
  IFS=$'\t' read -r failure_code failure_stage failed_sequence <<<"$failure_fields"
  [[ "$failure_code" =~ ^[A-Za-z0-9_.:-]{1,128}$ ]] || failure_code=unparseable
  [[ "$failure_stage" =~ ^[a-z]{1,32}$ ]] || failure_stage=unknown
  [[ "$failed_sequence" =~ ^[0-9]+$ ]] || failed_sequence=0
  printf 'companion release prep: %s production canary execution failed: exit=%s transcript_records=%s/42 error_code=%s error_stage=%s failed_sequence=%s\n' \
    "$label" "$canary_status" "$transcript_records" "$failure_code" "$failure_stage" "$failed_sequence" >&2
  return "$canary_status"
}
run_canary() {
  local candidate=$1 project=$2 output=$3 label=$4
  local sandbox_args=() root isolated_project isolated_home isolated_tmp
  local isolated_candidate isolated_omp isolated_credential credential_staging
  local root_identity record candidate_sha isolated_candidate_sha
  local canary_status started_at
  root="/private/tmp/autopus-adk-release-prep-${dispatch_nonce}-final"
  [[ "$label" == 'final' &&
     "$root" =~ ^/private/tmp/autopus-adk-release-prep-[0-9a-f]{32}-final$ &&
     ! -e "$root" && ! -L "$root" ]] || fail 'isolated canary root is unsafe'
  /usr/bin/sudo -n /usr/bin/install -d -m 0755 -o root -g wheel "$root"
  root_identity=$(/usr/bin/stat -f '%d:%i' "$root")
  record="${root}|${root_identity}|${private_tmp_identity}"
  isolation_roots+=("$record")
  isolated_project="$root/project"
  isolated_home="$root/home"
  isolated_tmp="$root/tmp"
  isolated_candidate="$root/auto"
  isolated_omp="$root/omp-darwin-arm64"
  isolated_credential="$root/provider-credential"
  credential_staging="$temp_dir/${label}-provider-credential"
  /usr/bin/sudo -n /usr/bin/install -d -m 0700 -o nobody -g nobody \
    "$isolated_project" "$isolated_home" "$isolated_tmp"
  /usr/bin/sudo -n /bin/cp -R "$project/." "$isolated_project/"
  /usr/bin/sudo -n /usr/sbin/chown -R nobody:nobody "$isolated_project"
  /usr/bin/sudo -n /usr/bin/install -m 0555 -o root -g wheel "$candidate" "$isolated_candidate"
  /usr/bin/sudo -n /usr/bin/install -m 0555 -o root -g wheel "$omp_executable" "$isolated_omp"
  (umask 077; printf '%s\n' "${!credential_locator}" >"$credential_staging")
  /usr/bin/sudo -n /usr/bin/install -m 0440 -o root -g nobody \
    "$credential_staging" "$isolated_credential"
  rm -f -- "$credential_staging"
  candidate_sha=$(shasum -a 256 "$candidate" | awk '{print $1}')
  isolated_candidate_sha=$(shasum -a 256 "$isolated_candidate" | awk '{print $1}')
  [[ "$candidate_sha" == "$isolated_candidate_sha" &&
     "$(shasum -a 256 "$isolated_omp" | awk '{print $1}')" == "$expected_omp_sha256" ]] ||
    fail 'root-owned canary executable bytes differ'
  sandbox_args=(--omp "$isolated_omp")
  if [[ "$inherit_parent_sandbox" -eq 1 ]]; then sandbox_args+=(--inherit-parent-sandbox); fi
  kill -0 "$sudo_keepalive_pid" >/dev/null 2>&1 || fail 'sudo keepalive stopped before production canary'
  printf 'companion release prep: %s production canary started (40 sequential provider calls)\n' \
    "$label" >&2
  started_at=$SECONDS
  if /usr/bin/sudo -n -u nobody /usr/bin/env -i PATH='/usr/bin:/bin:/usr/sbin:/sbin' \
    HOME="$isolated_home" TMPDIR="$isolated_tmp" LC_ALL=C /bin/sh -c '
      credential_locator=$1
      credential_file=$2
      project=$3
      candidate=$4
      shift 4
      IFS= read -r credential <"$credential_file" || exit 1
      export "$credential_locator=$credential"
      cd "$project" || exit 1
      exec "$candidate" "$@"
    ' sh "$credential_locator" "$isolated_credential" "$isolated_project" \
    "$isolated_candidate" workflow context-runtime observe-session \
    --explicit-live --input-jsonl - --output - --format jsonl --project-dir "$isolated_project" --spec-id "$spec_id" \
    --provider "$provider" --model "$model" --model-context-window "$model_context_window" --endpoint "$endpoint" \
    --credential-locator "$credential_locator" --producer-repository "$repository" --producer-workflow-ref "$producer_workflow_ref" \
    --producer-run-id "$producer_run_id" --producer-run-attempt 1 --candidate-repository "$repository" \
    --policy-id omp-context-active-v1 --oracle-policy-digest "$oracle_policy_digest" --target-git-commit "$source_commit" \
    "${sandbox_args[@]}" <"$input_jsonl" |
    capture_canary_progress "$output" "$label"; then
    :
  else
    canary_status=$?
    canary_failure_receipt "$label" "$canary_status" "$output"
  fi
  kill -0 "$sudo_keepalive_pid" >/dev/null 2>&1 || fail 'sudo keepalive stopped during production canary'
  OMP_CONTEXT_RELEASE_CANARY_ROOT="$root" OMP_CONTEXT_RELEASE_CANARY_EXECUTABLE="$isolated_omp" \
    "$execsmoke" --artifact "$isolated_candidate" --expected-version "${release_tag#v}" \
    --architecture arm64 --timeout 30s
  printf 'companion release prep: %s production canary completed (42/42 records, %ss)\n' \
    "$label" "$((SECONDS - started_at))" >&2
  rm -rf -- "$project"
  /usr/bin/sudo -n /usr/sbin/chown -R "${runner_uid}:${runner_gid}" "$isolated_project"
  /usr/bin/sudo -n /bin/mv "$isolated_project" "$project"
  remove_isolation_root "$record" || fail 'isolated canary root cleanup failed'
  assert_source_identity
}
validate_canary() {
  local project=$1 output=$2 candidate=$3 candidate_sha
  local report="$project/.autopus/runtime/omp-context/promotion-report-v1.json"
  [[ -f "$report" && ! -L "$report" ]] || fail 'production report is absent'
  candidate_sha=$(shasum -a 256 "$candidate" | awk '{print $1}')
  jq -s -e '([.[] | select(.type == "handshake")] | length) == 1 and ([.[] | select(.type == "call")] | length) == 40 and ([.[] | select(.type == "call") | .task_id_digest] | unique | length) == 20 and ([.[] | select(.type == "shutdown")] | length) == 1 and .[-1].cleanup_verified == true and .[-1].calls_completed == 40' "$output" >/dev/null || fail 'production canary transcript is invalid'
  jq -e --arg repository "$repository" --arg revision "$source_commit" --arg tree "$source_tree" --arg artifact "sha256:${candidate_sha}" --arg version "${release_tag#v}" \
    '.candidate.repository == $repository and .candidate.revision == $revision and .candidate.tree_sha == $tree and .candidate.artifact_sha256 == $artifact and .runtime.auto_version == $version and (.gates | length) == 14 and all(.gates[]; .status == "passed")' "$report" >/dev/null || fail 'production report coordinates or gates differ'
}
derive_policy() {
  local report=$1 output=$2
  "$policy_tool" companion-manifest omp-context-static-policy --report "$report" --target darwin-arm64 \
    --release-lineage-key-id "$lineage_key_id" --release-lineage-handoff "$lineage_handoff" --minimum-rollback-floor "$rollback_floor" >"$output"
  chmod 0600 "$output"
}
create_prep_lock() {
  local report=$1 report_name='omp-context-promotion-report.v1.json' report_blob evidence_index evidence_tree
  report_blob=$(git hash-object -w -- "$report")
  evidence_index="$temp_dir/evidence.index"
  rm -f -- "$evidence_index"
  GIT_INDEX_FILE="$evidence_index" git read-tree --empty
  GIT_INDEX_FILE="$evidence_index" git update-index --add --cacheinfo "100644,$report_blob,$report_name"
  evidence_tree=$(GIT_INDEX_FILE="$evidence_index" git write-tree)
  evidence_source_commit=$(printf 'OMP context release-prep source %s\n' "$release_tag" | \
    git -c user.name='Joseph' -c user.email='joseph@Josephui-MacBookPro.local' commit-tree "$evidence_tree")
  [[ "$(git rev-list --parents -n 1 "$evidence_source_commit")" == "$evidence_source_commit" ]] || fail 'prep lock commit is not an orphan'
  git push --force-with-lease="${evidence_source_ref}:" origin "$evidence_source_commit:$evidence_source_ref"
  [[ "$(git ls-remote --refs origin "$evidence_source_ref")" == "$evidence_source_commit"$'\t'"$evidence_source_ref" ]] || fail 'prep lock differs after acquisition'
}
use_retained_prep_lock() {
  local report=$1 verified_lock
  [[ -n "$retained_lock_commit" ]] || fail 'retained prep lock is unavailable'
  verified_lock=$(scripts/companion-release/verify-release-prep-lock.sh \
    "$evidence_source_ref" "$retained_lock_commit" "$report") ||
    fail 'retained prep lock verification failed'
  [[ "$verified_lock" == "$retained_lock_commit" ]] ||
    fail 'retained prep lock verifier returned another commit'
  evidence_source_commit=$retained_lock_commit
  prep_lock_mode='retained'
  retain_prep_lock=1
}
ensure_prep_lock() {
  local report=$1
  if [[ -n "$retained_lock_commit" ]]; then
    use_retained_prep_lock "$report"
  else
    create_prep_lock "$report"
  fi
}
publish_coordinates() {
  local lock_argument=$1 coordinates status
  if [[ "$lock_argument" != 'reconcile' ]]; then
    lock_argument="${prep_lock_mode}:${lock_argument}"
  fi
  if coordinates=$(scripts/companion-release/publish-release-coordinates.sh \
    "$repository" "$environment_name" "$release_tag" "$source_commit" "$source_tree" "$static_policy_file" \
    "$evidence_tag_object" "$evidence_commit" "$evidence_tree" "$report_sha256" "$attestation_sha256" \
    "$lock_argument" "$tag_signing_key"); then
    evidence_source_commit=''
    printf '%s\n' "$coordinates"
    return 0
  else
    status=$?
    if [[ "$status" -eq 75 ]]; then retain_prep_lock=1; fi
    exit "$status"
  fi
}

# One implementation, two callers: prep runs this before the canary and the
# release workflow runs it before GoReleaser, so a stale pin can never reach the
# point where an immutable release already exists.
verify_homebrew_tap_pins() {
  scripts/companion-release/verify-homebrew-tap-pins.sh ||
    fail 'Homebrew tap predecessor pins do not match the live tap'
}
