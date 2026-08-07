#!/usr/bin/env bash
set -euo pipefail
umask 077

fail() { printf 'release coordinate publish: %s\n' "$1" >&2; exit 1; }
usage() {
  printf '%s\n' 'usage: publish-release-coordinates.sh REPOSITORY ENVIRONMENT RELEASE_TAG SOURCE_COMMIT SOURCE_TREE STATIC_POLICY_FILE EVIDENCE_TAG_OBJECT EVIDENCE_COMMIT EVIDENCE_TREE REPORT_SHA256 ATTESTATION_SHA256 fresh:COMMIT|retained:COMMIT|reconcile TAG_SIGNING_KEY' >&2
  exit 64
}
[[ $# -eq 13 ]] || usage
readonly repository=$1 environment_name=$2 release_tag=$3 source_commit=$4 source_tree=$5
readonly static_policy_file=$6 evidence_tag_object=$7 evidence_commit=$8 evidence_tree=$9
shift 9
readonly report_sha256=$1 attestation_sha256=$2 prep_lock_argument=$3 tag_signing_key=$4
prep_lock_commit='' transaction_kind=''
case "$prep_lock_argument" in
  reconcile) transaction_kind='reconcile' ;;
  fresh:*) transaction_kind='fresh'; prep_lock_commit=${prep_lock_argument#fresh:} ;;
  retained:*) transaction_kind='retained'; prep_lock_commit=${prep_lock_argument#retained:} ;;
  *) usage ;;
esac
readonly prep_lock_commit transaction_kind
readonly evidence_tag="omp-context-evidence-${release_tag}"
readonly release_ref="refs/tags/${release_tag}"
readonly evidence_ref="refs/tags/${evidence_tag}"
readonly prep_lock_ref="refs/heads/${evidence_tag}-source"
readonly hex40='^[0-9a-f]{40}$' hex64='^[0-9a-f]{64}$'

[[ "$repository" == 'Insajin/autopus-adk' ]] || fail 'repository is not production authority'
[[ "$environment_name" == 'adk-companion-release' ]] || fail 'environment is not protected release authority'
[[ "$release_tag" == 'v0.50.101' ]] || fail 'release tag is not exact A22'
for value in "$source_commit" "$source_tree" "$evidence_tag_object" "$evidence_commit" "$evidence_tree"; do
  [[ "$value" =~ $hex40 ]] || fail 'Git coordinate is malformed'
done
[[ "$transaction_kind" == 'reconcile' || "$prep_lock_commit" =~ $hex40 ]] || fail 'prep lock coordinate is malformed'
for value in "$report_sha256" "$attestation_sha256"; do
  [[ "$value" =~ $hex64 ]] || fail 'evidence digest is malformed'
done
[[ -f "$static_policy_file" && ! -L "$static_policy_file" ]] || fail 'static policy file is unsafe'
IFS= read -r static_policy_b64 <"$static_policy_file" || fail 'read static policy'
[[ "$static_policy_b64" =~ ^[A-Za-z0-9_-]+$ && ${#static_policy_b64} -le 21846 ]] || fail 'static policy is malformed'
[[ "$(wc -l <"$static_policy_file" | tr -d ' ')" == '1' ]] || fail 'static policy file is not canonical'
[[ -f "$tag_signing_key" && ! -L "$tag_signing_key" ]] || fail 'release tag signing key is unsafe'
for tool in awk gh git jq mktemp shasum ssh-keygen stat uname; do command -v "$tool" >/dev/null || fail "$tool is unavailable"; done
tag_signing_key_owner_mode=''
case "$(uname -s)" in
  Darwin) tag_signing_key_owner_mode=$(/usr/bin/stat -f '%u:%Lp' "$tag_signing_key") ;;
  Linux) tag_signing_key_owner_mode=$(stat -c '%u:%a' "$tag_signing_key") ;;
  *) fail 'release tag signing key platform is unsupported' ;;
esac
readonly tag_signing_key_owner_mode
[[ "$tag_signing_key_owner_mode" == "$(id -u):600" ]] || fail 'release tag signing key ownership or mode is unsafe'

repo_root=$(git rev-parse --show-toplevel)
[[ "$(pwd -P)" == "$repo_root" ]] || fail 'publisher must run at the repository root'
readonly repo_root
readonly tag_public_key_file="$repo_root/scripts/companion-release/release-tag-signing-2026-q3.pub"
readonly tag_fingerprint_file="$repo_root/scripts/companion-release/release-tag-signing-2026-q3.fingerprint"
[[ -f "$tag_public_key_file" && ! -L "$tag_public_key_file" &&
   -f "$tag_fingerprint_file" && ! -L "$tag_fingerprint_file" ]] ||
  fail 'pinned release tag signer identity is unavailable'
[[ "$(git rev-parse --verify 'HEAD^{commit}')" == "$source_commit" ]] || fail 'HEAD differs from source commit'
[[ "$(git rev-parse --verify 'HEAD^{tree}')" == "$source_tree" ]] || fail 'HEAD differs from source tree'
remote_evidence=$(git ls-remote --refs origin "$evidence_ref") || fail 'cannot inspect remote evidence tag'
[[ "$remote_evidence" == "$evidence_tag_object"$'\t'"$evidence_ref" ]] || fail 'remote evidence tag differs'
git fetch --no-tags origin "$evidence_ref" || fail 'cannot fetch remote evidence tag'
[[ "$(git cat-file -t "$evidence_tag_object")" == 'tag' ]] || fail 'fetched evidence tag is not annotated'
[[ "$(git rev-parse --verify "${evidence_tag_object}^{commit}")" == "$evidence_commit" ]] || fail 'fetched evidence commit differs'
[[ "$(git rev-parse --verify "${evidence_tag_object}^{tree}")" == "$evidence_tree" ]] || fail 'fetched evidence tree differs'

bootstrap_cleanup() { rm -rf -- "$temp_dir"; }
readonly temp_dir=$(mktemp -d "${TMPDIR:-/tmp}/release-coordinate-publish.XXXXXX")
chmod 0700 "$temp_dir"
trap bootstrap_cleanup EXIT
readonly repository_snapshot="$temp_dir/repository-variables.json"
readonly environment_snapshot="$temp_dir/environment-variables.json"
readonly policy_snapshot="$temp_dir/deployment-policies.json"
readonly derived_public_key="$temp_dir/release-tag-signing.pub"
readonly allowed_signers="$temp_dir/release-tag.allowed-signers"
readonly verification_ref="refs/autopus-release-prep/verify-${release_tag}"
snapshots_ready=0 coordinates_started=0 committed=0 rollback_enabled=1 rollback_failed=0
created_release_tag=0 created_policy_id='' policy_creation_ambiguous=0 mode='publish'
reservation_id='' reservation_created=0 reservation_preexisting=0 reservation_ambiguous=0

ssh-keygen -y -f "$tag_signing_key" >"$derived_public_key"
chmod 0600 "$derived_public_key"
expected_public_key=$(awk 'NF >= 2 { print $1 " " $2; exit }' "$tag_public_key_file")
expected_tag_signer_fingerprint=$(<"$tag_fingerprint_file")
derived_public_key_value=$(awk 'NF >= 2 { print $1 " " $2; exit }' "$derived_public_key")
[[ -n "$expected_public_key" && "$derived_public_key_value" == "$expected_public_key" ]] || fail 'release tag private key differs from pinned public key'
[[ "$expected_tag_signer_fingerprint" =~ ^SHA256:[A-Za-z0-9+/]{43}$ &&
   "$(ssh-keygen -lf "$derived_public_key" -E sha256 | awk '{print $2}')" == "$expected_tag_signer_fingerprint" ]] ||
  fail 'release tag signer fingerprint differs'
printf 'autopus-adk-release-tag %s\n' "$derived_public_key_value" >"$allowed_signers"
chmod 0600 "$allowed_signers"
tag_git_config=(
  GIT_CONFIG_COUNT=5
  GIT_CONFIG_KEY_0=gpg.format GIT_CONFIG_VALUE_0=ssh
  GIT_CONFIG_KEY_1=user.signingkey GIT_CONFIG_VALUE_1="$tag_signing_key"
  GIT_CONFIG_KEY_2=gpg.ssh.allowedSignersFile GIT_CONFIG_VALUE_2="$allowed_signers"
  GIT_CONFIG_KEY_3=user.name GIT_CONFIG_VALUE_3='Joseph'
  GIT_CONFIG_KEY_4=user.email GIT_CONFIG_VALUE_4='joseph@Josephui-MacBookPro.local'
)
static_policy_sha256=$(printf '%s' "$static_policy_b64" | shasum -a 256 | awk '{print $1}')
[[ "$static_policy_sha256" =~ $hex64 ]] || fail 'static policy digest is malformed'
readonly reservation_name="$release_tag"
reservation_body=$(jq -cn --arg schema 'autopus.adk_release_reservation.v1' \
  --arg release_tag "$release_tag" --arg source_commit "$source_commit" --arg source_tree "$source_tree" \
  --arg evidence_tag_object "$evidence_tag_object" --arg evidence_commit "$evidence_commit" \
  --arg evidence_tree "$evidence_tree" --arg report_sha256 "$report_sha256" \
  --arg attestation_sha256 "$attestation_sha256" --arg static_policy_sha256 "$static_policy_sha256" \
  '{schema_version:$schema,release_tag:$release_tag,source_commit:$source_commit,source_tree:$source_tree,evidence_tag_object:$evidence_tag_object,evidence_commit:$evidence_commit,evidence_tree:$evidence_tree,report_sha256:$report_sha256,attestation_sha256:$attestation_sha256,static_policy_sha256:$static_policy_sha256}')
readonly static_policy_sha256 reservation_body

readonly transaction_lib="$temp_dir/release-coordinate-transaction-lib.sh"
transaction_lib_blob=$(git rev-parse --verify \
  "${source_commit}:scripts/companion-release/release-coordinate-transaction-lib.sh") ||
  fail 'release coordinate transaction helper is absent from the exact source'
[[ "$(git cat-file -t "$transaction_lib_blob")" == 'blob' ]] ||
  fail 'release coordinate transaction helper is not a source blob'
git cat-file blob "$transaction_lib_blob" >"$transaction_lib"
chmod 0400 "$transaction_lib"
[[ "$(git hash-object "$transaction_lib")" == "$transaction_lib_blob" ]] ||
  fail 'staged release coordinate transaction helper differs from the exact source'
exec 9<"$transaction_lib"
rm -f -- "$transaction_lib"
# shellcheck source=/dev/null
source /dev/fd/9
exec 9<&-
trap cleanup EXIT

names=(ADK_COMPANION_APPROVED_SOURCE_COMMIT ADK_COMPANION_APPROVED_SOURCE_TREE \
  OMP_CONTEXT_EVIDENCE_TAG_OBJECT_SHA OMP_CONTEXT_EVIDENCE_COMMIT_SHA \
  OMP_CONTEXT_EVIDENCE_TREE_SHA OMP_CONTEXT_EVIDENCE_REPORT_SHA256 \
  OMP_CONTEXT_EVIDENCE_ATTESTATION_SHA256 OMP_CONTEXT_STATIC_POLICY_B64)
values=("$source_commit" "$source_tree" "$evidence_tag_object" "$evidence_commit" \
  "$evidence_tree" "$report_sha256" "$attestation_sha256" "$static_policy_b64")

remote_release='' remote_release_status=0
remote_release=$(git ls-remote --refs origin "$release_ref") || remote_release_status=$?
if [[ "$remote_release_status" -ne 0 ]]; then
  rollback_enabled=0
  fail 'cannot inspect remote release tag'
fi
if [[ -n "$remote_release" ]]; then
  if [[ "$transaction_kind" != 'reconcile' ]]; then
    rollback_enabled=0
    fail 'release tag appeared while a prep transaction was requested'
  fi
  mode='reconcile'
  [[ -z "$(git ls-remote --refs origin "$prep_lock_ref")" ]] || fail 'committed release still has a prep lock'
  verify_remote_release || fail 'remote release tag identity or signature is invalid'
  verify_coordinates || fail 'committed release coordinates are not converged'
  verify_owned_release_record || fail 'committed GitHub Release is not operator-owned'
  committed=1
  emit_receipt reconciled
  exit 0
fi
[[ "$prep_lock_commit" =~ $hex40 ]] || fail 'publishing requires an exact prep lock commit'
[[ "$(git ls-remote --refs origin "$prep_lock_ref")" == "$prep_lock_commit"$'\t'"$prep_lock_ref" ]] || fail 'release-prep compare-and-swap lock is not owned'
release_lookup_status=0
if verify_owned_draft_reservation; then
  if [[ "$transaction_kind" != 'retained' ]]; then
    reservation_ambiguous=1
    fail 'unexpected owned draft exists for a fresh transaction'
  fi
  reservation_preexisting=1
else
  release_lookup_status=$?
  if [[ "$release_lookup_status" -ne 2 ]]; then
    reservation_ambiguous=1
    fail 'GitHub Release state is unavailable, duplicated, or unowned'
  fi
  if ! jq -e 'length == 0' <<<"$draft_name_matches" >/dev/null; then
    reservation_ambiguous=1
    fail 'another draft collides with the GoReleaser reservation name'
  fi
fi
gh api "repos/${repository}" | jq -e '.permissions.admin == true' >/dev/null || fail 'repository administration permission is unavailable'
environment_json=$(gh api "repos/${repository}/environments/${environment_name}")
jq -e '.deployment_branch_policy.custom_branch_policies == true and .deployment_branch_policy.protected_branches == false' <<<"$environment_json" >/dev/null || fail 'environment deployment policy mode is unsafe'

if git show-ref --verify --quiet "$release_ref"; then
  env "${tag_git_config[@]}" git verify-tag "$release_ref" >/dev/null || fail 'stale local release tag signature is invalid'
  [[ "$(git rev-parse --verify "${release_ref}^{commit}")" == "$source_commit" ]] || fail 'stale local release tag targets another commit'
  git update-ref -d "$release_ref"
fi
readonly signing_probe="$temp_dir/signing-probe"
git clone --quiet --no-checkout --shared . "$signing_probe"
env "${tag_git_config[@]}" git -C "$signing_probe" tag -s release-signing-probe "$source_commit" -m 'release signing probe'
env "${tag_git_config[@]}" git -C "$signing_probe" verify-tag refs/tags/release-signing-probe >/dev/null

scope_json --repo "$repository" >"$repository_snapshot"
scope_json --repo "$repository" --env "$environment_name" >"$environment_snapshot"
gh api "repos/${repository}/environments/${environment_name}/deployment-branch-policies" >"$policy_snapshot"
jq -e 'length <= 500 and ([.[].name] | length) == ([.[].name] | unique | length)' \
  "$repository_snapshot" >/dev/null || fail 'repository variable snapshot is incomplete or ambiguous'
jq -e 'length <= 100 and ([.[].name] | length) == ([.[].name] | unique | length)' \
  "$environment_snapshot" >/dev/null || fail 'environment variable snapshot is incomplete or ambiguous'
chmod 0600 "$repository_snapshot" "$environment_snapshot" "$policy_snapshot"
snapshots_ready=1
coordinates_started=1
for index in "${!names[@]}"; do
  gh variable set "${names[$index]}" --repo "$repository" --body "${values[$index]}"
  gh variable set "${names[$index]}" --repo "$repository" --env "$environment_name" --body "${values[$index]}"
done
policy_id=$(jq -r --arg tag "$release_tag" '.branch_policies[] | select(.type == "tag" and .name == $tag) | .id' "$policy_snapshot")
if [[ -z "$policy_id" ]]; then
  created_policy=''
  if ! created_policy=$(gh api --method POST \
    "repos/${repository}/environments/${environment_name}/deployment-branch-policies" \
    -f name="$release_tag" -f type=tag); then
    policy_creation_ambiguous=1
    fail 'exact deployment tag policy creation outcome is ambiguous'
  fi
  created_policy_id=$(jq -r --arg tag "$release_tag" \
    'select(.type == "tag" and .name == $tag) | .id' <<<"$created_policy")
  if [[ ! "$created_policy_id" =~ ^[0-9]+$ ]]; then
    policy_creation_ambiguous=1
    fail 'exact deployment tag policy response is invalid'
  fi
fi
verify_coordinates || fail 'release coordinates differ after write'
create_or_adopt_release_reservation ||
  fail 'cannot reserve the exact operator-owned draft release'
verify_owned_draft_reservation || fail 'owned draft release reservation changed before tag commit'
[[ "$(git ls-remote --refs origin "$prep_lock_ref")" == "$prep_lock_commit"$'\t'"$prep_lock_ref" ]] || fail 'release-prep lock was lost before tag commit'

env "${tag_git_config[@]}" git tag -s "$release_tag" "$source_commit" -m "${release_tag} - A22 companion release"
created_release_tag=1
[[ "$(git cat-file -t "$release_ref")" == 'tag' ]] || fail 'release tag is not annotated'
env "${tag_git_config[@]}" git verify-tag "$release_ref" >/dev/null
local_tag_object=$(git rev-parse --verify "$release_ref")
rollback_enabled=0
push_status=0
git push --atomic --force-with-lease="${prep_lock_ref}:${prep_lock_commit}" origin \
  "$release_ref:$release_ref" ":$prep_lock_ref" || push_status=$?
remote_state=''
for attempt in 1 2 3 4 5; do
  if remote_state=$(git ls-remote --refs origin "$release_ref" "$prep_lock_ref"); then break; fi
  remote_state=''; sleep "$attempt"
done
if [[ -n "$remote_state" || "$push_status" -eq 0 ]]; then
  observed_release=$(awk -v ref="$release_ref" '$2 == ref { print $0 }' <<<"$remote_state")
  observed_lock=$(awk -v ref="$prep_lock_ref" '$2 == ref { print $0 }' <<<"$remote_state")
  if [[ "$observed_release" == "$local_tag_object"$'\t'"$release_ref" && -z "$observed_lock" ]]; then
    committed=1
    release_tag_object=$local_tag_object
  elif [[ -z "$observed_release" && "$observed_lock" == "$prep_lock_commit"$'\t'"$prep_lock_ref" ]]; then
    rollback_enabled=1
    fail 'atomic release tag commit was rejected'
  else
    fail 'atomic release tag commit reached an inconsistent remote state'
  fi
else
  fail 'atomic release tag commit outcome cannot be inspected'
fi
verify_remote_release || fail 'committed release tag signature cannot be reverified'
verify_coordinates || fail 'committed release coordinates changed after tag push'
verify_owned_release_record || fail 'committed GitHub Release reservation changed after tag push'
[[ -z "$(git ls-remote --refs origin "$prep_lock_ref")" ]] || fail 'committed release retained its prep lock'
emit_receipt committed
