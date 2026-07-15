#!/usr/bin/env bash
set -euo pipefail
umask 077
readonly A0_REPOSITORY='Insajin/autopus-adk'
readonly A0_TAG='v0.50.69'
readonly A0_VERSION='0.50.69'
readonly A1_TAG='v0.50.70'
readonly A1_VERSION='0.50.70'
readonly A0_COMMIT_SHA=''
readonly A0_RECEIPT_SHA256=''
readonly A0_SIGNATURE_SHA256=''
readonly A0_RECORD_SHA256=''
readonly A0_PUBLIC_KEY_SHA256=''
readonly A0_CHECKSUMS_SHA256=''
readonly A0_AMD64_MANIFEST_SHA256=''
readonly A0_ARM64_MANIFEST_SHA256=''
readonly BUNDLE_NAME='adk-companion-public-key-receipt.bundle'
readonly RECEIPT_NAME='public-key-receipt.json'
readonly SIGNATURE_NAME='public-key-receipt.sig'
readonly MANIFEST_NAME='adk-companion-manifest.json'
readonly ARTIFACT_NAME='auto'
readonly CHECKSUMS_NAME='checksums.txt'
readonly EVIDENCE_SOURCE='immutable A0 GitHub release'
readonly LOCAL_EVIDENCE_ERROR='fixture_or_local_evidence_forbidden'
fail() {
  printf 'companion release lineage: %s: %s\n' "$1" "$2" >&2
  exit 1
}
require_environment() { local name="$1"; [[ -n "${!name-}" ]] || fail prior_evidence_unverifiable "missing ${name}"; }
sha256_file() {
  local output digest
  output=$(shasum -a 256 "$1") || return 1
  digest="${output%%[[:space:]]*}"
  [[ "$digest" =~ ^[0-9a-f]{64}$ ]] || return 1
  printf 'sha256:%s' "$digest"
}
nonzero_hex() { [[ "$1" =~ ^[0-9a-f]{$2}$ && -n "${1//0/}" ]]; }
require_environment GITHUB_REF_NAME
COMPANION_VERSION="${GITHUB_REF_NAME#v}"
if [[ "$GITHUB_REF_NAME" == 'v0.50.69' && "$COMPANION_VERSION" == '0.50.69' ]]; then
  release_phase='A0'
  printf 'companion release lineage: %s bootstrap accepted for %s@%s\n' \
    "$release_phase" "$A0_REPOSITORY" "$A0_TAG"
  exit 0
elif [[ "$GITHUB_REF_NAME" == "$A1_TAG" && "$COMPANION_VERSION" == "$A1_VERSION" ]]; then
  release_phase='A1'
else
  fail prior_release_identity_mismatch 'release is outside the frozen A0/A1 policy'
fi
nonzero_hex "$A0_COMMIT_SHA" 40 \
  || fail prior_evidence_unverifiable \
    "A0 commit pin is not provisioned; ${LOCAL_EVIDENCE_ERROR}"
for pin in \
  "$A0_RECEIPT_SHA256" "$A0_SIGNATURE_SHA256" \
  "$A0_RECORD_SHA256" "$A0_PUBLIC_KEY_SHA256" "$A0_CHECKSUMS_SHA256" \
  "$A0_AMD64_MANIFEST_SHA256" "$A0_ARM64_MANIFEST_SHA256"
do
  nonzero_hex "$pin" 64 \
    || fail prior_evidence_unverifiable 'A0 receipt trust pins are not provisioned'
done
for name in \
  GITHUB_TOKEN COMPANION_SIGNER COMPANION_RECEIPT_VERIFIER \
  COMPANION_SIGNING_KEY_FILE COMPANION_KEY_ID COMPANION_HANDOFF \
  COMPANION_ROLLBACK_FLOOR COMPANION_PUBLIC_KEY_RECEIPT_ISSUED_AT \
  COMPANION_PUBLIC_KEY_RECEIPT_EXPIRES_AT
do
  require_environment "$name"
done
for tool in gh jq tar cmp shasum xxd awk grep find wc; do
  command -v "$tool" >/dev/null \
    || fail prior_evidence_unverifiable "required tool ${tool} is unavailable"
done
[[ -f "$COMPANION_SIGNER" && ! -L "$COMPANION_SIGNER" && -x "$COMPANION_SIGNER" ]] \
  || fail prior_evidence_unverifiable 'companion signer is invalid'
[[ -f "$COMPANION_RECEIPT_VERIFIER" && ! -L "$COMPANION_RECEIPT_VERIFIER" &&
   -x "$COMPANION_RECEIPT_VERIFIER" ]] \
  || fail prior_evidence_unverifiable 'companion receipt verifier is invalid'
[[ -f "$COMPANION_SIGNING_KEY_FILE" && ! -L "$COMPANION_SIGNING_KEY_FILE" ]] \
  || fail prior_evidence_unverifiable 'companion signing key file is invalid'
temp_dir=''
cleanup() {
  local status=$?
  local cleanup_status=0
  if [[ -n "$temp_dir" ]]; then
    if rm -rf -- "$temp_dir"; then :; else cleanup_status=$?; fi
  fi
  if [[ "$cleanup_status" != '0' ]]; then
    printf 'companion release lineage: cleanup failed\n' >&2
    return "$cleanup_status"
  fi
  return "$status"
}
trap cleanup EXIT
trap 'exit 1' HUP INT TERM
temp_dir=$(mktemp -d "${TMPDIR:-/tmp}/adk-public-key-lineage.XXXXXX") \
  || fail prior_evidence_unverifiable 'cannot allocate verification workspace'
release_json="$temp_dir/a0-release.json"
tag_ref_json="$temp_dir/a0-tag-ref.json"
commit_json="$temp_dir/a0-commit.json"
if ! env -i PATH="$PATH" HOME="${HOME-}" GITHUB_TOKEN="$GITHUB_TOKEN" \
  gh api "repos/$A0_REPOSITORY/releases/tags/$A0_TAG" >"$release_json"; then
  fail prior_evidence_absent "cannot obtain ${EVIDENCE_SOURCE} metadata"
fi
if ! env -i PATH="$PATH" HOME="${HOME-}" GITHUB_TOKEN="$GITHUB_TOKEN" \
  gh api "repos/$A0_REPOSITORY/git/ref/tags/$A0_TAG" >"$tag_ref_json"; then
  fail prior_evidence_absent 'cannot obtain immutable A0 tag metadata'
fi
if ! env -i PATH="$PATH" HOME="${HOME-}" GITHUB_TOKEN="$GITHUB_TOKEN" \
  gh api "repos/$A0_REPOSITORY/commits/$A0_COMMIT_SHA" >"$commit_json"; then
  fail prior_evidence_absent 'cannot obtain immutable A0 commit metadata'
fi

tag_name=$(jq -er '.tag_name | select(type == "string")' "$release_json") \
  || fail prior_evidence_malformed 'release tag_name is malformed'
target_commitish=$(jq -er '.target_commitish | select(type == "string")' "$release_json") \
  || fail prior_evidence_malformed 'release target_commitish is malformed'
jq -e '.draft == false and .prerelease == false and .immutable == true' \
  "$release_json" >/dev/null \
  || fail prior_evidence_unverifiable 'A0 release is not immutable and final'
[[ "$tag_name" == "$A0_TAG" && "$target_commitish" == "$A0_COMMIT_SHA" ]] \
  || fail prior_release_identity_mismatch 'release/tag/commit coordinates differ'
[[ "$(jq -er '.sha' "$commit_json")" == "$A0_COMMIT_SHA" ]] \
  || fail prior_release_identity_mismatch 'commit endpoint differs from the A0 pin'
tag_type=$(jq -er '.object.type' "$tag_ref_json") \
  || fail prior_evidence_malformed 'tag object type is malformed'
tag_object_sha=$(jq -er '.object.sha' "$tag_ref_json") \
  || fail prior_evidence_malformed 'tag object SHA is malformed'
if [[ "$tag_type" == 'tag' ]]; then
  annotated_tag_json="$temp_dir/a0-annotated-tag.json"
  env -i PATH="$PATH" HOME="${HOME-}" GITHUB_TOKEN="$GITHUB_TOKEN" \
    gh api "repos/$A0_REPOSITORY/git/tags/$tag_object_sha" >"$annotated_tag_json" \
    || fail prior_evidence_absent 'cannot obtain annotated A0 tag metadata'
  [[ "$(jq -er '.tag' "$annotated_tag_json")" == "$A0_TAG" &&
     "$(jq -er '.object.type' "$annotated_tag_json")" == 'commit' ]] \
    || fail prior_release_identity_mismatch 'annotated tag identity differs'
  tag_commit_sha=$(jq -er '.object.sha' "$annotated_tag_json") \
    || fail prior_evidence_malformed 'annotated tag commit is malformed'
elif [[ "$tag_type" == 'commit' ]]; then
  tag_commit_sha="$tag_object_sha"
else
  fail prior_evidence_malformed 'A0 tag does not resolve to a commit'
fi
[[ "$tag_commit_sha" == "$A0_COMMIT_SHA" ]] \
  || fail prior_release_identity_mismatch 'A0 tag commit differs from the source pin'
readonly A0_AMD64_ASSET="autopus-adk_${A0_VERSION}_darwin_amd64.tar.gz"
readonly A0_ARM64_ASSET="autopus-adk_${A0_VERSION}_darwin_arm64.tar.gz"
jq -e --arg bundle "$BUNDLE_NAME" --arg receipt "$RECEIPT_NAME" --arg signature "$SIGNATURE_NAME" \
  '[.assets[].name | select(. == $bundle or . == $receipt or . == $signature)] | length == 0' \
  "$release_json" >/dev/null \
  || fail prior_evidence_malformed 'independent receipt assets are forbidden'
download_dir="$temp_dir/downloads"
install -m 0700 -d "$download_dir"
for asset in "$A0_AMD64_ASSET" "$A0_ARM64_ASSET" "$CHECKSUMS_NAME"; do
  asset_digest=$(jq -er --arg name "$asset" \
    '[.assets[] | select(.name == $name)] | select(length == 1) | .[0] |
     select(.state == "uploaded") | .digest |
     select(type == "string" and test("^sha256:[0-9a-f]{64}$"))' "$release_json") \
    || fail prior_evidence_malformed "exact asset metadata is missing for ${asset}"
  env -i PATH="$PATH" HOME="${HOME-}" GITHUB_TOKEN="$GITHUB_TOKEN" \
    gh release download "$A0_TAG" --repo "$A0_REPOSITORY" \
    --pattern "$asset" --dir "$download_dir" \
    || fail prior_evidence_absent "exact A0 asset is absent: ${asset}"
  downloaded_asset="$download_dir/$asset"
  [[ -f "$downloaded_asset" && ! -L "$downloaded_asset" ]] \
    || fail prior_evidence_absent "downloaded A0 asset is invalid: ${asset}"
  [[ "$(sha256_file "$downloaded_asset")" == "$asset_digest" ]] \
    || fail prior_evidence_unverifiable "server digest differs for ${asset}"
done
extract_bundle() {
  local archive="$1"
  local output_dir="$2"
  local architecture="$3"
  local manifest_pin="$4"
  local listing="$output_dir/archive-entries"
  install -m 0700 -d "$output_dir"
  tar -tzf "$archive" >"$listing" \
    || fail prior_evidence_malformed 'A0 archive cannot be listed'
  for entry in \
    "$ARTIFACT_NAME" "$MANIFEST_NAME" \
    "$BUNDLE_NAME/$RECEIPT_NAME" "$BUNDLE_NAME/$SIGNATURE_NAME"
  do
    [[ "$(grep -Fxc "$entry" "$listing")" == '1' ]] \
      || fail prior_evidence_absent "canonical archive entry is absent: ${entry}"
  done
  unexpected=$(awk -v prefix="$BUNDLE_NAME/" -v receipt="$BUNDLE_NAME/$RECEIPT_NAME" \
    -v signature="$BUNDLE_NAME/$SIGNATURE_NAME" \
    'index($0, prefix) == 1 && $0 != prefix && $0 != receipt && $0 != signature { print; exit }' \
    "$listing")
  [[ -z "$unexpected" ]] \
    || fail prior_evidence_malformed 'canonical receipt bundle has extra entries'
  tar -xOzf "$archive" "$BUNDLE_NAME/$RECEIPT_NAME" >"$output_dir/$RECEIPT_NAME" \
    || fail prior_evidence_absent 'cannot extract prior_receipt'
  tar -xOzf "$archive" "$BUNDLE_NAME/$SIGNATURE_NAME" >"$output_dir/$SIGNATURE_NAME" \
    || fail prior_evidence_absent 'cannot extract prior signature'
  tar -xOzf "$archive" "$MANIFEST_NAME" >"$output_dir/$MANIFEST_NAME" \
    || fail prior_evidence_absent 'cannot extract prior manifest'
  tar -xOzf "$archive" "$ARTIFACT_NAME" >"$output_dir/$ARTIFACT_NAME" \
    || fail prior_evidence_absent 'cannot extract prior artifact'
  chmod 0600 "$output_dir/$RECEIPT_NAME" "$output_dir/$SIGNATURE_NAME" \
    "$output_dir/$MANIFEST_NAME" "$output_dir/$ARTIFACT_NAME"
  manifest="$output_dir/$MANIFEST_NAME"
  manifest_version=$(jq -er '.version | select(type == "string")' "$manifest") \
    || fail prior_evidence_malformed 'prior manifest version is malformed'
  [[ "$manifest_version" == "$A0_VERSION" ]] \
    || fail prior_manifest_version_mismatch 'prior manifest version differs from A0'
  manifest_key_id=$(jq -er '.key_id | select(type == "string")' "$manifest") \
    || fail prior_evidence_malformed 'prior manifest key_id is malformed'
  receipt_key_id=$(jq -er '.key_id | select(type == "string")' "$output_dir/$RECEIPT_NAME") \
    || fail prior_evidence_malformed 'prior receipt key_id is malformed'
  [[ "$manifest_key_id" == "$COMPANION_KEY_ID" && "$receipt_key_id" == "$manifest_key_id" ]] \
    || fail prior_key_overlap_mismatch 'prior manifest and receipt key IDs do not overlap A1'
  jq -e --arg arch "$architecture" --arg handoff "$COMPANION_HANDOFF" \
    --arg floor "$COMPANION_ROLLBACK_FLOOR" \
    '.schema_version == "adk-companion-manifest.v1" and .platform == "darwin" and
     .architecture == $arch and .handoff == $handoff and (.rollback_floor | tostring) == $floor' \
    "$manifest" >/dev/null || fail prior_manifest_claim_mismatch 'prior manifest claims differ'
  claimed_artifact_digest=$(jq -er '.artifact_digest | select(type == "string")' "$manifest") \
    || fail prior_evidence_malformed 'prior manifest artifact digest is malformed'
  [[ "$claimed_artifact_digest" == "$(sha256_file "$output_dir/$ARTIFACT_NAME")" ]] \
    || fail prior_manifest_artifact_mismatch 'prior manifest does not bind its artifact'
  [[ "$(sha256_file "$manifest")" == "sha256:$manifest_pin" ]] \
    || fail prior_manifest_digest_mismatch 'prior manifest differs from its A0 pin'
}
extract_bundle "$download_dir/$A0_AMD64_ASSET" "$temp_dir/amd64" amd64 \
  "$A0_AMD64_MANIFEST_SHA256"
extract_bundle "$download_dir/$A0_ARM64_ASSET" "$temp_dir/arm64" arm64 \
  "$A0_ARM64_MANIFEST_SHA256"
checksums="$download_dir/$CHECKSUMS_NAME"
[[ "$(sha256_file "$checksums")" == "sha256:$A0_CHECKSUMS_SHA256" ]] \
  || fail prior_checksums_bytes_mismatch 'checksums.txt differs from its A0 pin'
for asset in "$A0_AMD64_ASSET" "$A0_ARM64_ASSET"; do
  checksum_line=$(grep -E "^[0-9a-f]{64}  ${asset}$" "$checksums") \
    || fail prior_checksums_malformed "checksum entry is absent for ${asset}"
  [[ "$(grep -Ec "^[0-9a-f]{64}  ${asset}$" "$checksums")" == '1' ]] \
    || fail prior_checksums_malformed "checksum entry is not unique for ${asset}"
  [[ "$(sha256_file "$download_dir/$asset")" == "sha256:${checksum_line%% *}" ]] \
    || fail prior_archive_checksum_mismatch "archive differs from checksums.txt: ${asset}"
done
prior_receipt="$temp_dir/amd64/$RECEIPT_NAME"
prior_signature="$temp_dir/amd64/$SIGNATURE_NAME"
cmp -- "$prior_receipt" "$temp_dir/arm64/$RECEIPT_NAME" \
  || fail prior_receipt_bytes_mismatch 'A0 architecture receipt bytes differ'
cmp -- "$prior_signature" "$temp_dir/arm64/$SIGNATURE_NAME" \
  || fail prior_signature_bytes_mismatch 'A0 architecture signature bytes differ'
receipt_sha256=$(sha256_file "$prior_receipt") \
  || fail prior_evidence_unverifiable 'cannot digest prior receipt'
signature_sha256=$(sha256_file "$prior_signature") \
  || fail prior_evidence_unverifiable 'cannot digest prior signature'
[[ "$receipt_sha256" == "sha256:$A0_RECEIPT_SHA256" ]] \
  || fail prior_receipt_bytes_mismatch 'prior receipt differs from its A0 pin'
[[ "$signature_sha256" == "sha256:$A0_SIGNATURE_SHA256" ]] \
  || fail prior_signature_bytes_mismatch 'prior signature differs from its A0 pin'
claimed_public_key_sha256=$(jq -er '.public_key_sha256' "$prior_receipt") \
  || fail prior_evidence_malformed 'prior public_key_sha256 claim is absent'
[[ "$claimed_public_key_sha256" == "sha256:$A0_PUBLIC_KEY_SHA256" ]] \
  || fail prior_public_key_digest_mismatch 'prior public key digest differs'
env -i PATH="$PATH" HOME="${HOME-}" \
  "$COMPANION_RECEIPT_VERIFIER" \
  --receipt "$prior_receipt" --signature "$prior_signature" \
  --signing-key "$COMPANION_SIGNING_KEY_FILE" \
  --key-id "$COMPANION_KEY_ID" \
  --issued-at "$COMPANION_PUBLIC_KEY_RECEIPT_ISSUED_AT" \
  --expires-at "$COMPANION_PUBLIC_KEY_RECEIPT_EXPIRES_AT" \
  --handoff "$COMPANION_HANDOFF" \
  --minimum-rollback-floor "$COMPANION_ROLLBACK_FLOOR" \
  --public-key-sha256 "sha256:$A0_PUBLIC_KEY_SHA256" \
  || fail prior_evidence_unverifiable 'prior receipt claims or signature are invalid'
record_sha256=$(
  {
    printf 'autopus.public-key-receipt.a0-record.v1\0'
    printf '%s' "${receipt_sha256#sha256:}" | xxd -r -p
    printf '%s' "${signature_sha256#sha256:}" | xxd -r -p
  } | shasum -a 256 | awk '{print "sha256:" $1}'
) || fail prior_evidence_unverifiable 'cannot digest prior key record'
[[ "$record_sha256" == "sha256:$A0_RECORD_SHA256" ]] \
  || fail prior_record_digest_mismatch 'prior key record differs from its A0 pin'
current_bundle="$temp_dir/current/$BUNDLE_NAME"
install -m 0700 -d "$temp_dir/current"
env -i PATH="$PATH" HOME="${HOME-}" TMPDIR="$temp_dir" \
  "$COMPANION_SIGNER" companion-manifest public-key-receipt \
  --key-file "$COMPANION_SIGNING_KEY_FILE" \
  --bundle-output "$current_bundle" \
  --key-id "$COMPANION_KEY_ID" \
  --issued-at "$COMPANION_PUBLIC_KEY_RECEIPT_ISSUED_AT" \
  --expires-at "$COMPANION_PUBLIC_KEY_RECEIPT_EXPIRES_AT" \
  --handoff "$COMPANION_HANDOFF" \
  --minimum-rollback-floor "$COMPANION_ROLLBACK_FLOOR" >/dev/null \
  || fail prior_evidence_unverifiable 'cannot produce current A1 receipt'
[[ "$(find "$current_bundle" -mindepth 1 -maxdepth 1 -print | wc -l | tr -d '[:space:]')" == '2' ]] \
  || fail prior_evidence_malformed 'current A1 bundle is malformed'
current_receipt="$current_bundle/$RECEIPT_NAME"
current_signature="$current_bundle/$SIGNATURE_NAME"
cmp -- "$prior_receipt" "$current_receipt" \
  || fail prior_receipt_bytes_mismatch 'A1 does not republish exact A0 receipt bytes'
cmp -- "$prior_signature" "$current_signature" \
  || fail prior_signature_bytes_mismatch 'A1 does not republish exact A0 signature bytes'
printf 'companion release lineage: %s exact A0 key record verified\n' "$release_phase"
