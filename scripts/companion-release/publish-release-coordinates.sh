#!/usr/bin/env bash
set -euo pipefail
umask 077

fail() { printf 'release coordinate publish: %s\n' "$1" >&2; exit 1; }
usage() {
  printf '%s\n' 'usage: publish-release-coordinates.sh REPOSITORY ENVIRONMENT RELEASE_TAG SOURCE_COMMIT SOURCE_TREE CANDIDATE_SHA256 ROTATION_DOCUMENT ROTATION_SIGNATURE ROTATION_REF_COMMIT ROTATION_DOCUMENT_SHA256 BRIDGE_MANIFEST ROTATION_VERIFIER fresh:COMMIT|retained:COMMIT|reconcile TAG_SIGNING_KEY' >&2
  exit 64
}
[[ $# -eq 14 ]] || usage
readonly repository=$1 environment_name=$2 release_tag=$3 source_commit=$4 source_tree=$5
readonly candidate_sha256=$6 rotation_document=$7 rotation_signature=$8 rotation_ref_commit=$9
shift 9
readonly rotation_document_sha256=$1 bridge_manifest=$2 rotation_verifier=$3
readonly prep_lock_argument=$4 tag_signing_key=$5
readonly release_mode='canonical-full-bridge'
readonly rotation_ref='refs/heads/release-key-rotation-v0.50.109'
readonly promotion_key_id='omp-context-promotion-2026-q3-k3'
readonly promotion_public_sha256='2a9b41dec1330f65937d9b25b20967cb29fd9209c722ce5fe1a9afd6ca45b937'
readonly release_ref="refs/tags/${release_tag}"
readonly prep_lock_ref='refs/heads/release-bridge-v0.50.109-prep-lock'
readonly hex40='^[0-9a-f]{40}$' hex64='^[0-9a-f]{64}$'
prep_lock_commit='' transaction_kind=''
case "$prep_lock_argument" in
  reconcile) transaction_kind='reconcile' ;;
  fresh:*) transaction_kind='fresh'; prep_lock_commit=${prep_lock_argument#fresh:} ;;
  retained:*) transaction_kind='retained'; prep_lock_commit=${prep_lock_argument#retained:} ;;
  *) usage ;;
esac
readonly prep_lock_commit transaction_kind

[[ "$repository" == 'Insajin/autopus-adk' ]] || fail 'repository is not production authority'
[[ "$environment_name" == 'adk-companion-release' ]] || fail 'environment is not protected release authority'
[[ "$release_tag" == 'v0.50.109' ]] || fail 'release tag is not exact A22 bridge'
for value in "$source_commit" "$source_tree" "$rotation_ref_commit"; do
  [[ "$value" =~ $hex40 ]] || fail 'Git coordinate is malformed'
done
for value in "$candidate_sha256" "$rotation_document_sha256" "$promotion_public_sha256"; do
  [[ "$value" =~ $hex64 ]] || fail 'bridge digest is malformed'
done
[[ "$transaction_kind" == 'reconcile' || "$prep_lock_commit" =~ $hex40 ]] ||
  fail 'bridge prep lock coordinate is malformed'
for path in "$rotation_document" "$rotation_signature" "$bridge_manifest" "$rotation_verifier" "$tag_signing_key"; do
  [[ -f "$path" && ! -L "$path" ]] || fail 'bridge authority input is unsafe'
done
[[ -x "$rotation_verifier" ]] || fail 'rotation verifier is not executable'
for tool in awk cmp gh git jq mktemp shasum ssh-keygen stat uname; do
  command -v "$tool" >/dev/null || fail "$tool is unavailable"
done
case "$(uname -s)" in
  Darwin) key_owner_mode=$(/usr/bin/stat -f '%u:%Lp' "$tag_signing_key") ;;
  Linux) key_owner_mode=$(stat -c '%u:%a' "$tag_signing_key") ;;
  *) fail 'release tag signing platform is unsupported' ;;
esac
[[ "$key_owner_mode" == "$(id -u):600" ]] ||
  fail 'release tag signing key ownership or mode is unsafe'
repo_root=$(git rev-parse --show-toplevel)
[[ "$(pwd -P)" == "$repo_root" ]] || fail 'publisher must run at repository root'
readonly repo_root
[[ "$(git rev-parse --verify 'HEAD^{commit}')" == "$source_commit" ]] || fail 'HEAD differs from source commit'
[[ "$(git rev-parse --verify 'HEAD^{tree}')" == "$source_tree" ]] || fail 'HEAD differs from source tree'
[[ -z "$(git status --porcelain)" ]] || fail 'source worktree is not clean'

bootstrap_cleanup() { rm -rf -- "$temp_dir"; }
temp_dir=$(mktemp -d "${TMPDIR:-/tmp}/release-coordinate-bridge.XXXXXX")
readonly temp_dir
chmod 0700 "$temp_dir"
trap bootstrap_cleanup EXIT
git fetch --no-tags origin main
[[ "$(git rev-parse --verify origin/main)" == "$source_commit" ]] ||
  fail 'source is not exact origin/main'
environment_variables=$(gh variable list --repo "$repository" --env "$environment_name" --json name,value) ||
  fail 'protected environment variables are unavailable'
matched_current_variable() {
  local name=$1 repository_value environment_count environment_value
  repository_value=$(gh variable get "$name" --repo "$repository") || return 1
  environment_count=$(jq --arg name "$name" '[.[] | select(.name == $name)] | length' \
    <<<"$environment_variables") || return 1
  if [[ "$environment_count" == '1' ]]; then
    environment_value=$(jq -r --arg name "$name" '.[] | select(.name == $name) | .value' \
      <<<"$environment_variables") || return 1
    [[ "$repository_value" == "$environment_value" ]] || return 1
  elif [[ "$environment_count" != '0' ]]; then
    return 1
  fi
  [[ -n "$repository_value" ]] || return 1
  printf '%s' "$repository_value"
}
channel_key_id=$(matched_current_variable AUTOPUS_ADK_CHANNEL_KEY_ID) ||
  fail 'current ADK channel key ID is unavailable'
channel_public_key=$(matched_current_variable AUTOPUS_ADK_CHANNEL_PUBLIC_KEY) ||
  fail 'current ADK channel public key is unavailable'
verified_rotation_dir="$temp_dir/verified-rotation"
rotation_receipt=$(env -i PATH="$PATH" HOME="${HOME:-/}" TMPDIR="${TMPDIR:-/tmp}" \
  AUTOPUS_ADK_CHANNEL_KEY_ID="$channel_key_id" \
  AUTOPUS_ADK_CHANNEL_PUBLIC_KEY="$channel_public_key" \
  scripts/companion-release/verify-key-rotation-sidecar.sh \
  "$rotation_verifier" "$source_commit" "$source_tree" "$verified_rotation_dir" \
  "$rotation_document" "$rotation_signature") || fail 'signed rotation sidecar cannot authorize R2'
[[ "$(jq -er '.rotation_ref_commit' <<<"$rotation_receipt")" == "$rotation_ref_commit" &&
   "$(jq -er '.rotation_document_sha256' <<<"$rotation_receipt")" == "$rotation_document_sha256" ]] ||
  fail 'verified rotation sidecar coordinates differ'

expected_manifest="$temp_dir/omp-context-bridge-release.v1.json"
env -i PATH="$PATH" HOME="${HOME:-/}" TMPDIR="${TMPDIR:-/tmp}" \
  COMPANION_RELEASE_TAG="$release_tag" COMPANION_SOURCE_COMMIT="$source_commit" \
  COMPANION_SOURCE_TREE="$source_tree" \
  OMP_CONTEXT_CANDIDATE_ARTIFACT_SHA256="$candidate_sha256" \
  ADK_KEY_ROTATION_DOCUMENT_SHA256="$rotation_document_sha256" \
  ADK_KEY_ROTATION_REF_COMMIT="$rotation_ref_commit" \
  scripts/companion-release/produce-omp-context-bridge-manifest.sh "$expected_manifest"
cmp -s "$expected_manifest" "$bridge_manifest" || fail 'bridge manifest differs from authorized coordinates'

r2_public="$repo_root/scripts/companion-release/release-tag-signing-2026-q3-r2.pub"
r2_fingerprint="$repo_root/scripts/companion-release/release-tag-signing-2026-q3-r2.fingerprint"
[[ -f "$r2_public" && ! -L "$r2_public" &&
   -f "$r2_fingerprint" && ! -L "$r2_fingerprint" ]] ||
  fail 'R2 release tag signer pins are missing or unsafe'
derived_public="$temp_dir/release-tag-signing.pub"
allowed_signers="$temp_dir/release-tag.allowed-signers"
ssh-keygen -y -f "$tag_signing_key" >"$derived_public"
expected_public=$(awk 'NF >= 2 { print $1 " " $2; exit }' "$r2_public")
derived_public_value=$(awk 'NF >= 2 { print $1 " " $2; exit }' "$derived_public")
expected_fingerprint=$(<"$r2_fingerprint")
[[ "$derived_public_value" == "$expected_public" ]] || fail 'tag private key differs from R2 public pin'
[[ "$expected_fingerprint" == 'SHA256:7FISPXCi8p7cFEdh4Fcyyp8RPQbXYZwmo3Mxi5+YjrQ' &&
   "$(ssh-keygen -lf "$derived_public" -E sha256 | awk '{print $2}')" == "$expected_fingerprint" ]] ||
  fail 'tag private key differs from R2 fingerprint'
printf 'autopus-adk-release-tag %s\n' "$derived_public_value" >"$allowed_signers"
chmod 0600 "$derived_public" "$allowed_signers"
tag_git_config=(
  GIT_CONFIG_COUNT=5
  GIT_CONFIG_KEY_0=gpg.format GIT_CONFIG_VALUE_0=ssh
  GIT_CONFIG_KEY_1=user.signingkey GIT_CONFIG_VALUE_1="$tag_signing_key"
  GIT_CONFIG_KEY_2=gpg.ssh.allowedSignersFile GIT_CONFIG_VALUE_2="$allowed_signers"
  GIT_CONFIG_KEY_3=user.name GIT_CONFIG_VALUE_3='Joseph'
  GIT_CONFIG_KEY_4=user.email GIT_CONFIG_VALUE_4='joseph@Josephui-MacBookPro.local'
)
readonly reservation_name="$release_tag"
reservation_body=$(jq -cnS --arg schema 'autopus.adk_release_reservation.v2' \
  --arg release_mode "$release_mode" --arg release_tag "$release_tag" \
  --arg source_commit "$source_commit" --arg source_tree "$source_tree" \
  --arg candidate_sha256 "$candidate_sha256" --arg rotation_ref "$rotation_ref" \
  --arg rotation_ref_commit "$rotation_ref_commit" \
  --arg rotation_document_sha256 "$rotation_document_sha256" \
  --arg promotion_key_id "$promotion_key_id" \
  --arg promotion_public_sha256 "$promotion_public_sha256" \
  '{schema_version:$schema,release_mode:$release_mode,release_tag:$release_tag,
    source_commit:$source_commit,source_tree:$source_tree,
    candidate_artifact_sha256:$candidate_sha256,rotation_ref:$rotation_ref,
    rotation_ref_commit:$rotation_ref_commit,
    rotation_document_sha256:$rotation_document_sha256,
    promotion_key_id:$promotion_key_id,
    promotion_public_sha256:$promotion_public_sha256}')
readonly reservation_body

transaction_lib="$temp_dir/release-coordinate-transaction-lib.sh"
transaction_blob=$(git rev-parse --verify \
  "${source_commit}:scripts/companion-release/release-coordinate-transaction-lib.sh") ||
  fail 'release coordinate transaction helper is absent from exact source'
git cat-file blob "$transaction_blob" >"$transaction_lib"
chmod 0400 "$transaction_lib"
[[ "$(git hash-object "$transaction_lib")" == "$transaction_blob" ]] ||
  fail 'staged release coordinate transaction helper differs from exact source'
# shellcheck source=/dev/null
source "$transaction_lib"

names=(ADK_COMPANION_APPROVED_SOURCE_COMMIT ADK_COMPANION_APPROVED_SOURCE_TREE)
values=("$source_commit" "$source_tree")
obsolete_names=(OMP_CONTEXT_EVIDENCE_TAG_OBJECT_SHA OMP_CONTEXT_EVIDENCE_COMMIT_SHA \
  OMP_CONTEXT_EVIDENCE_TREE_SHA OMP_CONTEXT_EVIDENCE_REPORT_SHA256 \
  OMP_CONTEXT_EVIDENCE_ATTESTATION_SHA256 OMP_CONTEXT_STATIC_POLICY_B64)
repository_snapshot="$temp_dir/repository-variables.json"
environment_snapshot="$temp_dir/environment-variables.json"
policy_snapshot="$temp_dir/deployment-policies.json"
snapshots_ready=0 coordinates_started=0 committed=0 rollback_enabled=1 rollback_failed=0
created_release_tag=0 created_policy_id='' policy_creation_ambiguous=0 mode='publish'
reservation_id='' reservation_created=0 reservation_preexisting=0 reservation_ambiguous=0
verification_ref="refs/autopus-release-prep/verify-${release_tag}"
trap cleanup EXIT

remote_release=$(git ls-remote --refs origin "$release_ref") || fail 'cannot inspect remote release tag'
if [[ -n "$remote_release" ]]; then
  [[ "$transaction_kind" == 'reconcile' ]] || { rollback_enabled=0; fail 'release tag appeared during prep'; }
  mode='reconcile'
  [[ -z "$(git ls-remote --refs origin "$prep_lock_ref")" ]] || fail 'committed release still has a prep lock'
  verify_remote_release || fail 'remote release tag identity or R2 signature is invalid'
  verify_coordinates || fail 'committed release coordinates are not converged'
  verify_owned_release_record || fail 'committed GitHub Release is not operator-owned'
  committed=1
  emit_receipt reconciled
  exit 0
fi
[[ "$prep_lock_commit" =~ $hex40 ]] || fail 'publishing requires exact bridge prep lock commit'
[[ "$(scripts/companion-release/verify-release-prep-lock.sh \
  "$prep_lock_ref" "$prep_lock_commit" "$bridge_manifest")" == "$prep_lock_commit" ]] ||
  fail 'bridge prep lock is not owned'
if verify_owned_draft_reservation; then
  [[ "$transaction_kind" == 'retained' ]] || { reservation_ambiguous=1; fail 'unexpected draft exists'; }
  reservation_preexisting=1
else
  status=$?
  [[ "$status" -eq 2 ]] || { reservation_ambiguous=1; fail 'release state is unavailable or unowned'; }
  jq -e 'length == 0' <<<"$draft_name_matches" >/dev/null ||
    { reservation_ambiguous=1; fail 'another draft collides with reservation'; }
fi
gh api "repos/${repository}" | jq -e '.permissions.admin == true' >/dev/null ||
  fail 'repository administration permission is unavailable'
environment_json=$(gh api "repos/${repository}/environments/${environment_name}")
jq -e '.deployment_branch_policy.custom_branch_policies == true and
  .deployment_branch_policy.protected_branches == false' <<<"$environment_json" >/dev/null ||
  fail 'environment deployment policy mode is unsafe'
if git show-ref --verify --quiet "$release_ref"; then
  env "${tag_git_config[@]}" git verify-tag "$release_ref" >/dev/null ||
    fail 'stale local release tag signature is invalid'
  [[ "$(git rev-parse --verify "${release_ref}^{commit}")" == "$source_commit" ]] ||
    fail 'stale local release tag targets another source'
  git update-ref -d "$release_ref"
fi
signing_probe="$temp_dir/signing-probe"
git clone --quiet --no-checkout --shared . "$signing_probe"
env "${tag_git_config[@]}" git -C "$signing_probe" tag -s release-signing-probe "$source_commit" \
  -m 'release signing probe'
env "${tag_git_config[@]}" git -C "$signing_probe" verify-tag refs/tags/release-signing-probe >/dev/null

scope_json --repo "$repository" >"$repository_snapshot"
scope_json --repo "$repository" --env "$environment_name" >"$environment_snapshot"
gh api "repos/${repository}/environments/${environment_name}/deployment-branch-policies" >"$policy_snapshot"
chmod 0600 "$repository_snapshot" "$environment_snapshot" "$policy_snapshot"
snapshots_ready=1; coordinates_started=1
for index in "${!names[@]}"; do
  gh variable set "${names[$index]}" --repo "$repository" --body "${values[$index]}"
  gh variable set "${names[$index]}" --repo "$repository" --env "$environment_name" --body "${values[$index]}"
done
for name in "${obsolete_names[@]}"; do
  if jq -e --arg name "$name" 'any(.[]; .name == $name)' "$repository_snapshot" >/dev/null; then
    gh variable delete "$name" --repo "$repository"
  fi
  if jq -e --arg name "$name" 'any(.[]; .name == $name)' "$environment_snapshot" >/dev/null; then
    gh variable delete "$name" --repo "$repository" --env "$environment_name"
  fi
done
policy_count=$(jq -r --arg tag "$release_tag" \
  '[.branch_policies[] | select(.type == "tag" and .name == $tag)] | length' \
  "$policy_snapshot")
case "$policy_count" in
  0)
    created_policy=$(gh api --method POST \
      "repos/${repository}/environments/${environment_name}/deployment-branch-policies" \
      -f name="$release_tag" -f type=tag) ||
      { policy_creation_ambiguous=1; fail 'tag policy creation is ambiguous'; }
    created_policy_id=$(jq -r --arg tag "$release_tag" \
      'select(.type == "tag" and .name == $tag) | .id' <<<"$created_policy")
    [[ "$created_policy_id" =~ ^[0-9]+$ ]] ||
      { policy_creation_ambiguous=1; fail 'tag policy response is invalid'; }
    ;;
  1) ;;
  *) fail 'release tag deployment policy is ambiguous' ;;
esac
verify_coordinates || fail 'bridge release coordinates differ after write'
create_or_adopt_release_reservation || fail 'cannot reserve exact operator-owned bridge draft'
verify_owned_draft_reservation || fail 'bridge draft changed before tag commit'
[[ "$(git ls-remote --refs origin "$rotation_ref")" ==
   "$rotation_ref_commit"$'\t'"$rotation_ref" ]] || fail 'rotation ref changed before tag commit'
[[ "$(git ls-remote --refs origin "$prep_lock_ref")" ==
   "$prep_lock_commit"$'\t'"$prep_lock_ref" ]] || fail 'bridge prep lock changed before tag commit'
scripts/companion-release/verify-release-tag-ruleset.sh || fail 'release tag authority ruleset changed'
git fetch --no-tags origin main
[[ "$(git rev-parse --verify origin/main)" == "$source_commit" ]] ||
  fail 'origin/main advanced before tag commit'
[[ "$(git rev-parse --verify 'HEAD^{tree}')" == "$source_tree" &&
   -z "$(git status --porcelain)" ]] || fail 'source changed before tag commit'

# The independently authorized R2 tag is the final external mutation.
env "${tag_git_config[@]}" git tag -s "$release_tag" "$source_commit" \
  -m "${release_tag} - A22 canonical-full bridge release"
created_release_tag=1
env "${tag_git_config[@]}" git verify-tag "$release_ref" >/dev/null
local_tag_object=$(git rev-parse --verify "$release_ref")
rollback_enabled=0
push_status=0
git push --atomic --force-with-lease="${prep_lock_ref}:${prep_lock_commit}" origin \
  "$release_ref:$release_ref" ":$prep_lock_ref" || push_status=$?
remote_state=$(git ls-remote --refs origin "$release_ref" "$prep_lock_ref") ||
  fail 'atomic tag outcome cannot be inspected'
observed_release=$(awk -v ref="$release_ref" '$2 == ref { print $0 }' <<<"$remote_state")
observed_lock=$(awk -v ref="$prep_lock_ref" '$2 == ref { print $0 }' <<<"$remote_state")
if [[ "$observed_release" == "$local_tag_object"$'\t'"$release_ref" && -z "$observed_lock" ]]; then
  committed=1; release_tag_object=$local_tag_object
elif [[ -z "$observed_release" && "$observed_lock" == "$prep_lock_commit"$'\t'"$prep_lock_ref" ]]; then
  rollback_enabled=1; fail 'atomic R2 tag commit was rejected'
else
  fail 'atomic R2 tag commit reached an inconsistent remote state'
fi
[[ "$push_status" -eq 0 || "$committed" -eq 1 ]] || fail 'R2 tag push failed'
verify_remote_release || fail 'committed R2 tag cannot be reverified'
verify_coordinates || fail 'committed bridge coordinates changed after tag push'
verify_owned_release_record || fail 'committed GitHub Release reservation changed after tag push'
emit_receipt committed
