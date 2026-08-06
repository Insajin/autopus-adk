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
  if [[ -n "$evidence_source_commit" && "$retain_prep_lock" -eq 0 ]]; then
    remote_source=$(git ls-remote --exit-code --refs origin "$evidence_source_ref" 2>/dev/null) || remote_status=$?
    if [[ "$remote_status" -eq 0 && "$remote_source" == "$evidence_source_commit"$'\t'"$evidence_source_ref" ]]; then
      git push --force-with-lease="${evidence_source_ref}:${evidence_source_commit}" origin ":${evidence_source_ref}" >/dev/null 2>&1 ||
        printf 'companion release prep: warning: owned prep lock cleanup failed\n' >&2
    elif [[ "$remote_status" -ne 2 ]]; then
      printf 'companion release prep: warning: prep lock ownership could not be inspected\n' >&2
    fi
  fi
  for record in "${isolation_roots[@]-}"; do
    [[ -n "$record" ]] || continue
    if ! remove_isolation_root "$record"; then
      printf 'companion release prep: isolated canary cleanup failed for %s\n' "${record%%|*}" >&2
      cleanup_failed=1
    fi
  done
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
  env GITHUB_REF_NAME="$release_tag" GITHUB_SHA="$source_commit" COMPANION_SOURCE_TREE="$source_tree" OMP_CONTEXT_STATIC_POLICY_B64="$policy" \
    scripts/companion-release/build-omp-context-candidate.sh "$output"
}
extract_project() {
  local destination=$1
  install -d -m 0700 "$destination"
  git archive "$source_commit" | tar -x -C "$destination"
}
run_canary() {
  local candidate=$1 project=$2 output=$3 label=$4
  local sandbox_args=() root isolated_project isolated_home isolated_tmp
  local isolated_candidate isolated_omp isolated_credential credential_staging
  local root_identity record candidate_sha isolated_candidate_sha
  root="/private/tmp/autopus-adk-release-prep-${dispatch_nonce}-${label}"
  [[ "$root" =~ ^/private/tmp/autopus-adk-release-prep-[0-9a-f]{32}-(bootstrap|final)$ &&
     ! -e "$root" && ! -L "$root" ]] || fail 'isolated canary root is unsafe'
  /usr/bin/sudo -n /usr/bin/install -d -m 0755 -o root -g wheel "$root"
  root_identity=$(/usr/bin/stat -f '%d:%i' "$root")
  record="${root}|${root_identity}|${private_tmp_identity}"
  isolation_roots+=("$record")
  isolated_project="$root/project"
  isolated_home="$root/home"
  isolated_tmp="$root/tmp"
  isolated_candidate="$root/auto-darwin-arm64"
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
  if [[ "$inherit_parent_sandbox" -eq 1 ]]; then sandbox_args=(--inherit-parent-sandbox); fi
  /usr/bin/sudo -n -u nobody /usr/bin/env -i PATH='/usr/bin:/bin:/usr/sbin:/sbin' \
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
    --omp "$isolated_omp" "${sandbox_args[@]}" <"$input_jsonl" >"$output"
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
watch_dispatch() {
  local workflow=$1 title=$2 runs_json; shift 2
  gh workflow run "$workflow" --repo "$repository" --ref main "$@"
  local run_id=''
  for _ in $(seq 1 90); do
    runs_json=$(gh api \
      "repos/${repository}/actions/workflows/${workflow}/runs?event=workflow_dispatch&branch=main&per_page=100") ||
      fail "cannot inspect ${workflow} dispatches"
    run_id=$(jq -r --arg title "$title" --arg source "$source_commit" \
      --argjson actor "$operator_actor_id" \
      '[.workflow_runs[] | select(.actor.id == $actor and .display_title == $title and
        .head_sha == $source)] | sort_by(.id) | last | .id // empty' <<<"$runs_json")
    [[ "$run_id" =~ ^[0-9]+$ ]] && break
    sleep 2
  done
  [[ "$run_id" =~ ^[0-9]+$ ]] || fail "cannot identify trusted ${workflow} run by nonce"
  gh run watch "$run_id" --repo "$repository" --exit-status
}
dispatch_preflight() {
  watch_dispatch companion-release-preflight.yml "Companion release preflight ${release_tag} ${dispatch_nonce}" \
    -f release_tag="$release_tag" -f source_commit="$source_commit" -f source_tree="$source_tree" \
    -f static_policy_b64="$static_policy_b64" -f dispatch_nonce="$dispatch_nonce"
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
load_evidence() {
  local local_tag_object verified_dir
  if git show-ref --verify --quiet "$evidence_ref"; then
    local_tag_object=$(git rev-parse --verify "$evidence_ref")
    [[ "$evidence_remote" == "$local_tag_object"$'\t'"$evidence_ref" ]] || fail 'local and remote evidence tag differ'
  else
    git fetch origin "$evidence_ref:$evidence_ref"
  fi
  evidence_tag_object=$(git rev-parse --verify "$evidence_ref")
  evidence_commit=$(git rev-parse --verify "${evidence_ref}^{commit}")
  evidence_tree=$(git rev-parse --verify "${evidence_ref}^{tree}")
  report_sha256=$(git cat-file blob "${evidence_commit}:omp-context-promotion-report.v1.json" | shasum -a 256 | awk '{print $1}')
  attestation_sha256=$(git cat-file blob "${evidence_commit}:omp-context-promotion-attestation.v2.json" | shasum -a 256 | awk '{print $1}')
  verified_dir="$temp_dir/verified-evidence"
  rm -rf -- "$verified_dir"
  env OMP_CONTEXT_EVIDENCE_TAG_OBJECT_SHA="$evidence_tag_object" OMP_CONTEXT_EVIDENCE_COMMIT_SHA="$evidence_commit" \
    OMP_CONTEXT_EVIDENCE_TREE_SHA="$evidence_tree" OMP_CONTEXT_EVIDENCE_REPORT_SHA256="$report_sha256" \
    OMP_CONTEXT_EVIDENCE_ATTESTATION_SHA256="$attestation_sha256" \
    scripts/companion-release/verify-omp-context-evidence-tag.sh "$verified_dir"
  verified_report="$verified_dir/omp-context-promotion-report.v1.json"
  verified_attestation="$verified_dir/omp-context-promotion-attestation.v2.json"
}
verify_evidence() {
  "$verifier" --mode historical --report "$verified_report" --attestation "$verified_attestation" \
    --report-sha256 "$report_sha256" --attestation-sha256 "$attestation_sha256" \
    --candidate-repository "$repository" --candidate-revision "$source_commit" --candidate-tree "$source_tree" \
    --candidate-artifact-sha256 "$candidate_sha256" --static-policy-b64 "$static_policy_b64"
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
