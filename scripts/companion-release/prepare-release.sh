#!/usr/bin/env bash
set -euo pipefail
umask 077

fail() { printf 'companion release prep: %s\n' "$1" >&2; exit 1; }
usage() {
  cat >&2 <<'USAGE'
usage: prepare-release.sh --tag-signing-key PATH [--rotation-document PATH --rotation-signature PATH] [--apply]
USAGE
  exit 64
}
readonly repository='Insajin/autopus-adk'
readonly environment_name='adk-companion-release'
readonly release_tag='v0.50.109'
readonly release_ref="refs/tags/${release_tag}"
readonly bridge_lock_ref='refs/heads/release-bridge-v0.50.109-prep-lock'
tag_signing_key='' supplied_rotation_document='' supplied_rotation_signature='' apply=0
while [[ $# -gt 0 ]]; do
  case "$1" in
    --tag-signing-key) [[ $# -ge 2 ]] || usage; tag_signing_key=$2; shift 2 ;;
    --rotation-document) [[ $# -ge 2 ]] || usage; supplied_rotation_document=$2; shift 2 ;;
    --rotation-signature) [[ $# -ge 2 ]] || usage; supplied_rotation_signature=$2; shift 2 ;;
    --apply) apply=1; shift ;;
    *) usage ;;
  esac
done
[[ -n "$tag_signing_key" ]] || usage
if [[ -n "$supplied_rotation_document" || -n "$supplied_rotation_signature" ]]; then
  [[ -n "$supplied_rotation_document" && -n "$supplied_rotation_signature" ]] ||
    fail 'rotation document and signature must be supplied together'
fi
[[ "$(uname -s)" == 'Darwin' && "$(uname -m)" == 'arm64' ]] ||
  fail 'release prep requires Darwin arm64'
for tool in awk cmp gh git go install jq mktemp shasum ssh-keygen stat uname; do
  command -v "$tool" >/dev/null || fail "$tool is unavailable"
done
[[ -f "$tag_signing_key" && ! -L "$tag_signing_key" ]] || fail 'release tag signing key is unsafe'
tag_signing_key=$(cd -- "$(dirname -- "$tag_signing_key")" && pwd)/$(basename -- "$tag_signing_key")
[[ "$(/usr/bin/stat -f '%u:%Lp' "$tag_signing_key")" == "$(id -u):600" ]] ||
  fail 'release tag signing key ownership or mode is unsafe'
for supplied in "$supplied_rotation_document" "$supplied_rotation_signature"; do
  [[ -z "$supplied" || (-f "$supplied" && ! -L "$supplied") ]] ||
    fail 'supplied rotation sidecar input is unsafe'
done

repo_root=$(git rev-parse --show-toplevel)
[[ "$(pwd -P)" == "$repo_root" ]] || fail 'release prep must run at the repository root'
readonly repo_root
assert_source_identity() {
  [[ -z "$(git status --porcelain)" ]] || fail 'source worktree is not clean'
  [[ "$(git rev-parse --verify 'HEAD^{commit}')" == "$source_commit" ]] ||
    fail 'source commit changed during release prep'
  [[ "$(git rev-parse --verify 'HEAD^{tree}')" == "$source_tree" ]] ||
    fail 'source tree changed during release prep'
  [[ "$(git remote get-url origin)" =~ ^(https://github\.com/|git@github\.com:)(Insajin|insajin)/autopus-adk(\.git)?$ ]] ||
    fail 'origin is not the production repository'
}
[[ -z "$(git status --porcelain)" ]] || fail 'source worktree is not clean'
source_commit=$(git rev-parse --verify 'HEAD^{commit}')
source_tree=$(git rev-parse --verify 'HEAD^{tree}')
readonly source_commit source_tree
[[ "$source_commit" =~ ^[0-9a-f]{40}$ && "$source_tree" =~ ^[0-9a-f]{40}$ ]] ||
  fail 'source coordinates are malformed'
assert_source_identity
git fetch --no-tags origin main
[[ "$(git rev-parse --verify origin/main)" == "$source_commit" ]] || fail 'source is not exact origin/main'
[[ "$(gh api "repos/${repository}" --jq .default_branch)" == 'main' ]] || fail 'default branch differs'
assert_source_identity

bootstrap_cleanup() { rm -rf -- "$temp_dir"; }
temp_dir=$(mktemp -d "${TMPDIR:-/tmp}/companion-release-bridge.XXXXXX")
readonly temp_dir
chmod 0700 "$temp_dir"
trap bootstrap_cleanup EXIT
staged_tag_signing_key="$temp_dir/release-tag-signing-key"
/usr/bin/install -m 0600 "$tag_signing_key" "$staged_tag_signing_key"
tag_signing_key=$staged_tag_signing_key
runtime_lib="$temp_dir/prepare-release-runtime-lib.sh"
runtime_blob=$(git rev-parse --verify \
  "${source_commit}:scripts/companion-release/prepare-release-runtime-lib.sh") ||
  fail 'release prep runtime helper is absent from the exact source'
git cat-file blob "$runtime_blob" >"$runtime_lib"
chmod 0400 "$runtime_lib"
[[ "$(git hash-object "$runtime_lib")" == "$runtime_blob" ]] ||
  fail 'staged release prep runtime helper differs from the exact source'
# shellcheck source=/dev/null
source "$runtime_lib"
bridge_lock_commit='' retain_prep_lock=0 prep_lock_mode='fresh'
trap 'cleanup $?' EXIT

environment_variables=$(gh variable list --repo "$repository" --env "$environment_name" --json name,value) ||
  fail 'protected environment variables are unavailable'
readonly environment_variables
jq -e 'type == "array" and all(.[]; (.name | type) == "string" and (.value | type) == "string") and
  (([.[].name] | length) == ([.[].name] | unique | length))' <<<"$environment_variables" >/dev/null ||
  fail 'protected environment variable inventory is malformed'
channel_key_id=$(matched_variable AUTOPUS_ADK_CHANNEL_KEY_ID)
channel_public_key=$(matched_variable AUTOPUS_ADK_CHANNEL_PUBLIC_KEY)
[[ "$channel_key_id" =~ ^[A-Za-z0-9_.-]+$ && "$channel_public_key" =~ ^[A-Za-z0-9+/]+={0,2}$ ]] ||
  fail 'current ADK channel authority is malformed'
readonly channel_key_id channel_public_key

authority_dir="$temp_dir/key-rotation-authority"
authority_receipt=$(scripts/companion-release/materialize-key-rotation-authority.sh "$authority_dir") ||
  fail 'immutable key-rotation authority is unavailable'
jq -e '.authority_commit | test("^[0-9a-f]{40}$")' <<<"$authority_receipt" >/dev/null ||
  fail 'immutable key-rotation authority receipt is malformed'
rotation_verifier="$authority_dir/verify-rotation.sh"
rotation_dir="$temp_dir/verified-rotation"
rotation_args=("$rotation_verifier" "$source_commit" "$source_tree" "$rotation_dir")
if [[ -n "$supplied_rotation_document" ]]; then
  rotation_args+=("$supplied_rotation_document" "$supplied_rotation_signature")
fi
rotation_receipt=$(env -i PATH="$PATH" HOME="${HOME:-/}" TMPDIR="${TMPDIR:-/tmp}" \
  AUTOPUS_ADK_CHANNEL_KEY_ID="$channel_key_id" \
  AUTOPUS_ADK_CHANNEL_PUBLIC_KEY="$channel_public_key" \
  scripts/companion-release/verify-key-rotation-sidecar.sh "${rotation_args[@]}") ||
  fail 'independently signed key-rotation sidecar is invalid'
rotation_ref_commit=$(jq -er '.rotation_ref_commit | select(test("^[0-9a-f]{40}$"))' <<<"$rotation_receipt")
rotation_document_sha256=$(jq -er '.rotation_document_sha256 | select(test("^[0-9a-f]{64}$"))' <<<"$rotation_receipt")
rotation_document="$rotation_dir/adk-key-rotation-v1.json"
rotation_signature="$rotation_dir/adk-key-rotation-v1.sig"
readonly rotation_ref_commit rotation_document_sha256 rotation_document rotation_signature rotation_verifier
scripts/companion-release/verify-release-tag-ruleset.sh ||
  fail 'exact v0.50.109 tag authority ruleset is unavailable'

r2_public="$repo_root/scripts/companion-release/release-tag-signing-2026-q3-r2.pub"
r2_fingerprint="$repo_root/scripts/companion-release/release-tag-signing-2026-q3-r2.fingerprint"
[[ -f "$r2_public" && ! -L "$r2_public" && -f "$r2_fingerprint" && ! -L "$r2_fingerprint" ]] ||
  fail 'R2 release tag signer pins are unavailable'
derived_public="$temp_dir/release-tag-signing.pub"
ssh-keygen -y -f "$tag_signing_key" >"$derived_public"
expected_public=$(awk 'NF >= 2 { print $1 " " $2; exit }' "$r2_public")
derived_public_value=$(awk 'NF >= 2 { print $1 " " $2; exit }' "$derived_public")
expected_fingerprint=$(<"$r2_fingerprint")
[[ -n "$expected_public" && "$derived_public_value" == "$expected_public" ]] ||
  fail 'release tag private key differs from R2 public pin'
[[ "$expected_fingerprint" == 'SHA256:7FISPXCi8p7cFEdh4Fcyyp8RPQbXYZwmo3Mxi5+YjrQ' &&
   "$(ssh-keygen -lf "$derived_public" -E sha256 | awk '{print $2}')" == "$expected_fingerprint" ]] ||
  fail 'release tag private key differs from R2 fingerprint'

candidate="$temp_dir/auto-darwin-arm64-canonical-full-bridge"
build_bridge_candidate "$candidate"
candidate_sha256=$(shasum -a 256 "$candidate" | awk '{print $1}')
readonly candidate_sha256
[[ "$candidate_sha256" =~ ^[0-9a-f]{64}$ ]] || fail 'bridge candidate digest is malformed'
bridge_manifest="$temp_dir/omp-context-bridge-release.v1.json"
produce_bridge_manifest "$bridge_manifest"
readonly bridge_manifest
verify_homebrew_tap_pins
assert_source_identity

release_remote=$(git ls-remote --refs origin "$release_ref") || fail 'cannot inspect release tag'
lock_remote=$(git ls-remote --refs origin "$bridge_lock_ref") || fail 'cannot inspect bridge prep lock'
retained_lock_commit=''
if [[ -n "$lock_remote" ]]; then
  retained_lock_commit=${lock_remote%%$'\t'*}
  [[ "$lock_remote" == "$retained_lock_commit"$'\t'"$bridge_lock_ref" &&
     "$retained_lock_commit" =~ ^[0-9a-f]{40}$ ]] || fail 'bridge prep lock is malformed'
fi
release_present=0; [[ -n "$release_remote" ]] && release_present=1
release_count=$(gh api --paginate --slurp "repos/${repository}/releases?per_page=100" |
  jq '[.[][] | select(.tag_name == "v0.50.109")] | length') || fail 'cannot inspect GitHub Release state'
[[ "$release_count" == '0' || "$release_count" == '1' ]] || fail 'GitHub Release state is ambiguous'
[[ "$release_count" == '0' || "$release_present" -eq 1 || -n "$retained_lock_commit" ]] ||
  fail 'GitHub Release exists without its source tag or retained bridge lock'
if [[ "$apply" -eq 0 ]]; then
  jq -cn --arg release_tag "$release_tag" --arg release_mode 'canonical-full-bridge' \
    --arg source_commit "$source_commit" --arg source_tree "$source_tree" \
    --arg candidate_sha256 "$candidate_sha256" --arg rotation_ref_commit "$rotation_ref_commit" \
    --arg rotation_document_sha256 "$rotation_document_sha256" \
    --argjson release_tag_present "$release_present" \
    --argjson prep_lock_present "$([[ -n "$retained_lock_commit" ]] && printf 1 || printf 0)" \
    '{mode:"preflight",release_mode:$release_mode,release_tag:$release_tag,
      source_commit:$source_commit,source_tree:$source_tree,
      candidate_artifact_sha256:$candidate_sha256,
      rotation_ref_commit:$rotation_ref_commit,
      rotation_document_sha256:$rotation_document_sha256,
      release_tag_present:($release_tag_present == 1),
      prep_lock_present:($prep_lock_present == 1),remote_mutations:0}'
  exit 0
fi

git fetch --no-tags origin main
[[ "$(git rev-parse --verify origin/main)" == "$source_commit" ]] ||
  fail 'origin/main advanced during release prep'
assert_source_identity
if [[ "$release_present" -eq 1 ]]; then
  publish_bridge_coordinates reconcile
else
  ensure_bridge_prep_lock "$bridge_manifest"
  publish_bridge_coordinates "$bridge_lock_commit"
fi
