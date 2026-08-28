#!/usr/bin/env bash
# Internal function library sourced only by prepare-release.sh.

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

cleanup() {
  local status=$1 remote_status=0 remote_lock=''
  if [[ -n "${bridge_lock_commit:-}" && "${retain_prep_lock:-0}" -eq 0 ]]; then
    remote_lock=$(git ls-remote --exit-code --refs origin "$bridge_lock_ref" 2>/dev/null) ||
      remote_status=$?
    if [[ "$remote_status" -eq 0 &&
       "$remote_lock" == "$bridge_lock_commit"$'\t'"$bridge_lock_ref" ]]; then
      git push --force-with-lease="${bridge_lock_ref}:${bridge_lock_commit}" \
        origin ":${bridge_lock_ref}" >/dev/null 2>&1 ||
        printf 'companion release prep: warning: owned bridge prep lock cleanup failed\n' >&2
    elif [[ "$remote_status" -ne 2 ]]; then
      printf 'companion release prep: warning: bridge prep lock ownership is ambiguous\n' >&2
    fi
  fi
  rm -rf -- "$temp_dir"
  exit "$status"
}

build_bridge_candidate() {
  local output=$1
  env -i PATH="$PATH" HOME="${HOME:-/}" TMPDIR="${TMPDIR:-/tmp}" \
    COMPANION_RELEASE_TAG="$release_tag" GITHUB_SHA="$source_commit" \
    COMPANION_SOURCE_TREE="$source_tree" \
    scripts/companion-release/build-omp-context-candidate.sh "$output"
}

produce_bridge_manifest() {
  local output=$1
  env -i PATH="$PATH" HOME="${HOME:-/}" TMPDIR="${TMPDIR:-/tmp}" \
    COMPANION_RELEASE_TAG="$release_tag" \
    COMPANION_SOURCE_COMMIT="$source_commit" COMPANION_SOURCE_TREE="$source_tree" \
    OMP_CONTEXT_CANDIDATE_ARTIFACT_SHA256="$candidate_sha256" \
    ADK_KEY_ROTATION_DOCUMENT_SHA256="$rotation_document_sha256" \
    ADK_KEY_ROTATION_REF_COMMIT="$rotation_ref_commit" \
    scripts/companion-release/produce-omp-context-bridge-manifest.sh "$output"
}

create_bridge_prep_lock() {
  local manifest=$1 manifest_blob lock_index lock_tree remote_lock
  manifest_blob=$(git hash-object -w -- "$manifest")
  lock_index="$temp_dir/bridge-lock.index"
  rm -f -- "$lock_index"
  GIT_INDEX_FILE="$lock_index" git read-tree --empty
  GIT_INDEX_FILE="$lock_index" git update-index --add --cacheinfo \
    "100644,$manifest_blob,omp-context-bridge-release.v1.json"
  lock_tree=$(GIT_INDEX_FILE="$lock_index" git write-tree)
  bridge_lock_commit=$(printf 'canonical-full bridge release prep %s\n' "$release_tag" | \
    git -c user.name='Joseph' -c user.email='joseph@Josephui-MacBookPro.local' \
      commit-tree "$lock_tree")
  [[ "$(git rev-list --parents -n 1 "$bridge_lock_commit")" == "$bridge_lock_commit" ]] ||
    fail 'bridge prep lock commit is not an orphan'
  git push --force-with-lease="${bridge_lock_ref}:" origin \
    "$bridge_lock_commit:$bridge_lock_ref"
  remote_lock=$(git ls-remote --refs origin "$bridge_lock_ref")
  [[ "$remote_lock" == "$bridge_lock_commit"$'\t'"$bridge_lock_ref" ]] ||
    fail 'bridge prep lock differs after acquisition'
}

use_retained_bridge_prep_lock() {
  local manifest=$1 verified_lock
  [[ -n "$retained_lock_commit" ]] || fail 'retained bridge prep lock is unavailable'
  verified_lock=$(scripts/companion-release/verify-release-prep-lock.sh \
    "$bridge_lock_ref" "$retained_lock_commit" "$manifest") ||
    fail 'retained bridge prep lock verification failed'
  [[ "$verified_lock" == "$retained_lock_commit" ]] ||
    fail 'retained bridge prep lock verifier returned another commit'
  bridge_lock_commit=$retained_lock_commit
  prep_lock_mode='retained'
  retain_prep_lock=1
}

ensure_bridge_prep_lock() {
  local manifest=$1
  if [[ -n "$retained_lock_commit" ]]; then
    use_retained_bridge_prep_lock "$manifest"
  else
    create_bridge_prep_lock "$manifest"
  fi
}

publish_bridge_coordinates() {
  local lock_argument=$1 coordinates status
  if [[ "$lock_argument" != 'reconcile' ]]; then
    lock_argument="${prep_lock_mode}:${lock_argument}"
  fi
  if coordinates=$(scripts/companion-release/publish-release-coordinates.sh \
    "$repository" "$environment_name" "$release_tag" "$source_commit" "$source_tree" \
    "$candidate_sha256" "$rotation_document" "$rotation_signature" \
    "$rotation_ref_commit" "$rotation_document_sha256" "$bridge_manifest" \
    "$rotation_verifier" "$lock_argument" "$tag_signing_key"); then
    bridge_lock_commit=''
    printf '%s\n' "$coordinates"
    return 0
  else
    status=$?
    if [[ "$status" -eq 75 ]]; then retain_prep_lock=1; fi
    exit "$status"
  fi
}

verify_homebrew_tap_pins() {
  scripts/companion-release/verify-homebrew-tap-pins.sh ||
    fail 'Homebrew tap predecessor pins do not match the live tap'
}
