#!/usr/bin/env bash
set -euo pipefail
umask 077
readonly A0_REPOSITORY='Insajin/autopus-adk' A0_TAG='v0.50.69' A0_VERSION='0.50.69'
readonly A1_REPOSITORY='Insajin/autopus-adk' A1_TAG='v0.50.70' A1_VERSION='0.50.70'
readonly A2_REPOSITORY='Insajin/autopus-adk' A2_TAG='v0.50.71' A2_VERSION='0.50.71'
readonly A3_TAG='v0.50.72' A3_VERSION='0.50.72'
readonly A3_REPOSITORY='Insajin/autopus-adk' A4_TAG='v0.50.73' A4_VERSION='0.50.73'
readonly A4_REPOSITORY='Insajin/autopus-adk' A5_TAG='v0.50.74' A5_VERSION='0.50.74'
readonly A5_REPOSITORY='Insajin/autopus-adk' A6_TAG='v0.50.77' A6_VERSION='0.50.77'
readonly A6_REPOSITORY='Insajin/autopus-adk' A7_TAG='v0.50.78' A7_VERSION='0.50.78'
readonly A7_REPOSITORY='Insajin/autopus-adk' A8_TAG='v0.50.79' A8_VERSION='0.50.79'
readonly A8_REPOSITORY='Insajin/autopus-adk' A9_TAG='v0.50.80' A9_VERSION='0.50.80'
readonly A9_REPOSITORY='Insajin/autopus-adk' A10_TAG='v0.50.81' A10_VERSION='0.50.81'
readonly BUNDLE_NAME='adk-companion-public-key-receipt.bundle' RECEIPT_NAME='public-key-receipt.json'
readonly SIGNATURE_NAME='public-key-receipt.sig' MANIFEST_NAME='adk-companion-manifest.json'
readonly MANIFEST_SIGNATURE_NAME='adk-companion-manifest.sig'
readonly ARTIFACT_NAME='auto' CHECKSUMS_NAME='checksums.txt'
readonly A0_EVIDENCE_SOURCE='immutable A0 GitHub release'
readonly LOCAL_EVIDENCE_ERROR='fixture_or_local_evidence_forbidden'
fail() { printf 'companion release lineage: %s: %s\n' "$1" "$2" >&2; exit 1; }
script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd) || fail prior_evidence_unverifiable 'cannot resolve lineage verifier directory'
pins_helper="$script_dir/verify-public-key-lineage-pins.sh"
[[ -f "$pins_helper" && ! -L "$pins_helper" ]] || fail prior_evidence_unverifiable 'lineage pin source is invalid'
# shellcheck source=verify-public-key-lineage-pins.sh
source "$pins_helper"
require_environment() { local name="$1"; [[ -n "${!name-}" ]] || fail prior_evidence_unverifiable "missing ${name}"; }
require_environment GITHUB_REF_NAME
COMPANION_VERSION="${GITHUB_REF_NAME#v}"
prior_tree=''
if [[ "$GITHUB_REF_NAME" == 'v0.50.69' && "$COMPANION_VERSION" == '0.50.69' ]]; then
  release_phase='A0'
  printf 'companion release lineage: %s bootstrap accepted for %s@%s\n' "$release_phase" "$A0_REPOSITORY" "$A0_TAG"
  exit 0
elif [[ "$GITHUB_REF_NAME" == "$A1_TAG" && "$COMPANION_VERSION" == "$A1_VERSION" ]]; then
  release_phase='A1' prior_phase='A0' prior_repository="$A0_REPOSITORY" prior_evidence_source="$A0_EVIDENCE_SOURCE"
  prior_tag="$A0_TAG" prior_version="$A0_VERSION" prior_commit="$A0_COMMIT_SHA"
  prior_tag_object='' prior_checksums="$A0_CHECKSUMS_SHA256" prior_amd64_archive='' prior_arm64_archive=''
  prior_amd64_manifest="$A0_AMD64_MANIFEST_SHA256" prior_arm64_manifest="$A0_ARM64_MANIFEST_SHA256"
elif [[ "$GITHUB_REF_NAME" == "$A2_TAG" && "$COMPANION_VERSION" == "$A2_VERSION" ]]; then
  release_phase='A2' prior_phase='A1' prior_repository="$A1_REPOSITORY" prior_evidence_source='immutable A1 GitHub release'
  prior_tag="$A1_TAG" prior_version="$A1_VERSION" prior_commit="$A1_COMMIT_SHA"
  prior_tag_object="$A1_TAG_OBJECT_SHA" prior_checksums="$A1_CHECKSUMS_SHA256" prior_amd64_archive="$A1_AMD64_ARCHIVE_SHA256" prior_arm64_archive="$A1_ARM64_ARCHIVE_SHA256"
  prior_amd64_manifest="$A1_AMD64_MANIFEST_SHA256" prior_arm64_manifest="$A1_ARM64_MANIFEST_SHA256"
elif [[ "$GITHUB_REF_NAME" == "$A3_TAG" && "$COMPANION_VERSION" == "$A3_VERSION" ]]; then
  release_phase='A3' prior_phase='A2' prior_repository="$A2_REPOSITORY" prior_evidence_source='immutable A2 GitHub release'
  prior_tag="$A2_TAG" prior_version="$A2_VERSION" prior_commit="$A2_COMMIT_SHA"
  prior_tag_object="$A2_TAG_OBJECT_SHA" prior_checksums="$A2_CHECKSUMS_SHA256" prior_amd64_archive="$A2_AMD64_ARCHIVE_SHA256" prior_arm64_archive="$A2_ARM64_ARCHIVE_SHA256"
  prior_amd64_manifest="$A2_AMD64_MANIFEST_SHA256" prior_arm64_manifest="$A2_ARM64_MANIFEST_SHA256"
elif [[ "$GITHUB_REF_NAME" == "$A4_TAG" && "$COMPANION_VERSION" == "$A4_VERSION" ]]; then
  release_phase='A4' prior_phase='A3' prior_repository="$A3_REPOSITORY" prior_evidence_source='immutable A3 GitHub release' prior_tag="$A3_TAG" prior_version="$A3_VERSION" prior_commit="$A3_COMMIT_SHA"
  prior_tag_object="$A3_TAG_OBJECT_SHA" prior_checksums="$A3_CHECKSUMS_SHA256" prior_amd64_archive="$A3_AMD64_ARCHIVE_SHA256" prior_arm64_archive="$A3_ARM64_ARCHIVE_SHA256" prior_amd64_manifest="$A3_AMD64_MANIFEST_SHA256" prior_arm64_manifest="$A3_ARM64_MANIFEST_SHA256"
elif [[ "$GITHUB_REF_NAME" == "$A5_TAG" && "$COMPANION_VERSION" == "$A5_VERSION" ]]; then
  release_phase='A5' prior_phase='A4' prior_repository="$A4_REPOSITORY" prior_evidence_source='immutable A4 GitHub release' prior_tag="$A4_TAG" prior_version="$A4_VERSION" prior_commit="$A4_COMMIT_SHA"
  prior_tag_object="$A4_TAG_OBJECT_SHA" prior_checksums="$A4_CHECKSUMS_SHA256" prior_amd64_archive="$A4_AMD64_ARCHIVE_SHA256" prior_arm64_archive="$A4_ARM64_ARCHIVE_SHA256" prior_amd64_manifest="$A4_AMD64_MANIFEST_SHA256" prior_arm64_manifest="$A4_ARM64_MANIFEST_SHA256"
elif [[ "$GITHUB_REF_NAME" == "$A6_TAG" && "$COMPANION_VERSION" == "$A6_VERSION" ]]; then
  release_phase='A6' prior_phase='A5' prior_repository="$A5_REPOSITORY" prior_evidence_source='immutable A5 GitHub release' prior_tag="$A5_TAG" prior_version="$A5_VERSION" prior_commit="$A5_COMMIT_SHA"
  prior_tag_object="$A5_TAG_OBJECT_SHA" prior_checksums="$A5_CHECKSUMS_SHA256" prior_amd64_archive="$A5_AMD64_ARCHIVE_SHA256" prior_arm64_archive="$A5_ARM64_ARCHIVE_SHA256" prior_amd64_manifest="$A5_AMD64_MANIFEST_SHA256" prior_arm64_manifest="$A5_ARM64_MANIFEST_SHA256"
elif [[ "$GITHUB_REF_NAME" == "$A7_TAG" && "$COMPANION_VERSION" == "$A7_VERSION" ]]; then
  release_phase='A7' prior_phase='A6' prior_repository="$A6_REPOSITORY" prior_evidence_source='immutable A6 GitHub release' prior_tag="$A6_TAG" prior_version="$A6_VERSION" prior_commit="$A6_COMMIT_SHA"
  prior_tag_object="$A6_TAG_OBJECT_SHA" prior_checksums="$A6_CHECKSUMS_SHA256" prior_amd64_archive="$A6_AMD64_ARCHIVE_SHA256" prior_arm64_archive="$A6_ARM64_ARCHIVE_SHA256" prior_amd64_manifest="$A6_AMD64_MANIFEST_SHA256" prior_arm64_manifest="$A6_ARM64_MANIFEST_SHA256"
elif [[ "$GITHUB_REF_NAME" == "$A8_TAG" && "$COMPANION_VERSION" == "$A8_VERSION" ]]; then
  release_phase='A8' prior_phase='A7' prior_repository="$A7_REPOSITORY" prior_evidence_source='immutable A7 GitHub release' prior_tag="$A7_TAG" prior_version="$A7_VERSION" prior_commit="$A7_COMMIT_SHA" prior_tree="$A7_TREE_SHA"
  prior_tag_object="$A7_TAG_OBJECT_SHA" prior_checksums="$A7_CHECKSUMS_SHA256" prior_amd64_archive="$A7_AMD64_ARCHIVE_SHA256" prior_arm64_archive="$A7_ARM64_ARCHIVE_SHA256" prior_amd64_manifest="$A7_AMD64_MANIFEST_SHA256" prior_arm64_manifest="$A7_ARM64_MANIFEST_SHA256"
elif [[ "$GITHUB_REF_NAME" == "$A9_TAG" && "$COMPANION_VERSION" == "$A9_VERSION" ]]; then
  release_phase='A9' prior_phase='A8' prior_repository="$A8_REPOSITORY" prior_evidence_source='immutable A8 GitHub release' prior_tag="$A8_TAG" prior_version="$A8_VERSION" prior_commit="$A8_COMMIT_SHA" prior_tree="$A8_TREE_SHA"
  prior_tag_object="$A8_TAG_OBJECT_SHA" prior_checksums="$A8_CHECKSUMS_SHA256" prior_amd64_archive="$A8_AMD64_ARCHIVE_SHA256" prior_arm64_archive="$A8_ARM64_ARCHIVE_SHA256" prior_amd64_manifest="$A8_AMD64_MANIFEST_SHA256" prior_arm64_manifest="$A8_ARM64_MANIFEST_SHA256"
elif [[ "$GITHUB_REF_NAME" == "$A10_TAG" && "$COMPANION_VERSION" == "$A10_VERSION" ]]; then
  release_phase='A10' prior_phase='A9' prior_repository="$A9_REPOSITORY" prior_evidence_source='immutable A9 GitHub release' prior_tag="$A9_TAG" prior_version="$A9_VERSION" prior_commit="$A9_COMMIT_SHA" prior_tree="$A9_TREE_SHA"
  prior_tag_object="$A9_TAG_OBJECT_SHA" prior_checksums="$A9_CHECKSUMS_SHA256" prior_amd64_archive="$A9_AMD64_ARCHIVE_SHA256" prior_arm64_archive="$A9_ARM64_ARCHIVE_SHA256" prior_amd64_manifest="$A9_AMD64_MANIFEST_SHA256" prior_arm64_manifest="$A9_ARM64_MANIFEST_SHA256"
else
  fail prior_release_identity_mismatch 'release is outside the frozen A0/A1/A2/A3/A4/A5/A6/A7/A8/A9/A10 policy'
fi
archive_helper="$script_dir/verify-public-key-lineage-archive.sh"
[[ -f "$archive_helper" && ! -L "$archive_helper" ]] || fail prior_evidence_unverifiable 'lineage archive verifier is invalid'
# shellcheck source=verify-public-key-lineage-archive.sh
source "$archive_helper"
nonzero_hex "$prior_commit" 40 || fail prior_evidence_unverifiable "${prior_phase} commit pin is not provisioned; ${LOCAL_EVIDENCE_ERROR}"
for pin in "$A0_RECEIPT_SHA256" "$A0_SIGNATURE_SHA256" "$A0_RECORD_SHA256" \
  "$A0_PUBLIC_KEY_SHA256" "$prior_checksums" "$prior_amd64_manifest" "$prior_arm64_manifest"; do
  nonzero_hex "$pin" 64 || fail prior_evidence_unverifiable 'prior release trust pins are not provisioned'
done
if [[ "$release_phase" == 'A2' || "$release_phase" == 'A3' || "$release_phase" == 'A4' || "$release_phase" == 'A5' || "$release_phase" == 'A6' || "$release_phase" == 'A7' || "$release_phase" == 'A8' || "$release_phase" == 'A9' || "$release_phase" == 'A10' ]]; then
  nonzero_hex "$prior_tag_object" 40 || fail prior_evidence_unverifiable "${prior_phase} annotated tag pin is not provisioned"
  for pin in "$prior_amd64_archive" "$prior_arm64_archive"; do
    nonzero_hex "$pin" 64 || fail prior_evidence_unverifiable "${prior_phase} archive pins are not provisioned"
  done
fi
if [[ -n "$prior_tree" ]]; then
  nonzero_hex "$prior_tree" 40 \
    || fail prior_evidence_unverifiable "${prior_phase} source tree pin is not provisioned"
fi
for name in GITHUB_TOKEN COMPANION_SIGNER COMPANION_RECEIPT_VERIFIER \
  COMPANION_SIGNING_KEY_FILE COMPANION_KEY_ID COMPANION_HANDOFF COMPANION_ROLLBACK_FLOOR \
  COMPANION_PUBLIC_KEY_RECEIPT_ISSUED_AT COMPANION_PUBLIC_KEY_RECEIPT_EXPIRES_AT; do
  require_environment "$name"
done
case "${COMPANION_LINEAGE_MANIFEST_VERIFICATION_REQUIRED-0}" in
  0) ;;
  1) require_environment COMPANION_MANIFEST_VERIFIER ;;
  *) fail prior_evidence_unverifiable 'COMPANION_LINEAGE_MANIFEST_VERIFICATION_REQUIRED must be 0 or 1' ;;
esac
for tool in gh jq tar cmp shasum xxd awk grep find wc; do
  command -v "$tool" >/dev/null || fail prior_evidence_unverifiable "required tool ${tool} is unavailable"
done
[[ -f "$COMPANION_SIGNER" && ! -L "$COMPANION_SIGNER" && -x "$COMPANION_SIGNER" ]] || fail prior_evidence_unverifiable 'companion signer is invalid'
[[ -f "$COMPANION_RECEIPT_VERIFIER" && ! -L "$COMPANION_RECEIPT_VERIFIER" &&
   -x "$COMPANION_RECEIPT_VERIFIER" ]] \
  || fail prior_evidence_unverifiable 'companion receipt verifier is invalid'
if [[ -n "${COMPANION_MANIFEST_VERIFIER-}" ]]; then
  [[ -f "$COMPANION_MANIFEST_VERIFIER" && ! -L "$COMPANION_MANIFEST_VERIFIER" &&
     -x "$COMPANION_MANIFEST_VERIFIER" ]] \
    || fail prior_evidence_unverifiable 'companion manifest verifier is invalid'
fi
[[ -f "$COMPANION_SIGNING_KEY_FILE" && ! -L "$COMPANION_SIGNING_KEY_FILE" ]] \
  || fail prior_evidence_unverifiable 'companion signing key file is invalid'
temp_dir=''
cleanup() {
  local status=$? cleanup_status=0
  [[ -z "$temp_dir" ]] || rm -rf -- "$temp_dir" || cleanup_status=$?
  if [[ "$cleanup_status" != '0' ]]; then
    printf 'companion release lineage: cleanup failed\n' >&2
    return "$cleanup_status"
  fi
  return "$status"
}
trap cleanup EXIT; trap 'exit 1' HUP INT TERM
temp_dir=$(mktemp -d "${TMPDIR:-/tmp}/adk-public-key-lineage.XXXXXX") \
  || fail prior_evidence_unverifiable 'cannot allocate verification workspace'
release_json="$temp_dir/prior-release.json" tag_ref_json="$temp_dir/prior-tag-ref.json" commit_json="$temp_dir/prior-commit.json"
env -i PATH="$PATH" HOME="${HOME-}" GITHUB_TOKEN="$GITHUB_TOKEN" \
  gh api "repos/$prior_repository/releases/tags/$prior_tag" >"$release_json" \
  || fail prior_evidence_absent "cannot obtain ${prior_evidence_source} metadata"
env -i PATH="$PATH" HOME="${HOME-}" GITHUB_TOKEN="$GITHUB_TOKEN" \
  gh api "repos/$prior_repository/git/ref/tags/$prior_tag" >"$tag_ref_json" \
  || fail prior_evidence_absent "cannot obtain immutable ${prior_phase} tag metadata"
env -i PATH="$PATH" HOME="${HOME-}" GITHUB_TOKEN="$GITHUB_TOKEN" \
  gh api "repos/$prior_repository/commits/$prior_commit" >"$commit_json" \
  || fail prior_evidence_absent "cannot obtain immutable ${prior_phase} commit metadata"
tag_name=$(jq -er '.tag_name | select(type == "string")' "$release_json") \
  || fail prior_evidence_malformed 'release tag_name is malformed'
target_commitish=$(jq -er '.target_commitish | select(type == "string")' "$release_json") \
  || fail prior_evidence_malformed 'release target_commitish is malformed'
jq -e '.draft == false and .prerelease == false and .immutable == true' \
  "$release_json" >/dev/null \
  || fail prior_evidence_unverifiable "${prior_phase} release is not immutable and final"
[[ "$tag_name" == "$prior_tag" && "$target_commitish" == "$prior_commit" ]] \
  || fail prior_release_identity_mismatch 'release/tag/commit coordinates differ'
[[ "$(jq -er '.sha' "$commit_json")" == "$prior_commit" ]] \
  || fail prior_release_identity_mismatch "commit endpoint differs from the ${prior_phase} pin"
if [[ -n "$prior_tree" ]]; then
  [[ "$(jq -er '.commit.tree.sha' "$commit_json")" == "$prior_tree" ]] \
    || fail prior_release_identity_mismatch "${prior_phase} source tree differs from its pin"
fi
tag_type=$(jq -er '.object.type' "$tag_ref_json") \
  || fail prior_evidence_malformed 'tag object type is malformed'
tag_object_sha=$(jq -er '.object.sha' "$tag_ref_json") \
  || fail prior_evidence_malformed 'tag object SHA is malformed'
if [[ "$tag_type" == 'tag' ]]; then
  [[ -z "$prior_tag_object" || "$tag_object_sha" == "$prior_tag_object" ]] \
    || fail prior_release_identity_mismatch "${prior_phase} annotated tag object differs from its pin"
  annotated_tag_json="$temp_dir/prior-annotated-tag.json"
  env -i PATH="$PATH" HOME="${HOME-}" GITHUB_TOKEN="$GITHUB_TOKEN" \
    gh api "repos/$prior_repository/git/tags/$tag_object_sha" >"$annotated_tag_json" \
    || fail prior_evidence_absent "cannot obtain annotated ${prior_phase} tag metadata"
  [[ "$(jq -er '.tag' "$annotated_tag_json")" == "$prior_tag" && \
     "$(jq -er '.object.type' "$annotated_tag_json")" == 'commit' ]] \
    || fail prior_release_identity_mismatch 'annotated tag identity differs'
  tag_commit_sha=$(jq -er '.object.sha' "$annotated_tag_json") \
    || fail prior_evidence_malformed 'annotated tag commit is malformed'
elif [[ "$tag_type" == 'commit' ]]; then
  [[ -z "$prior_tag_object" ]] \
    || fail prior_release_identity_mismatch "${prior_phase} tag must be the pinned annotated object"
  tag_commit_sha="$tag_object_sha"
else
  fail prior_evidence_malformed "${prior_phase} tag does not resolve to a commit"
fi
[[ "$tag_commit_sha" == "$prior_commit" ]] \
  || fail prior_release_identity_mismatch "${prior_phase} tag commit differs from the source pin"
readonly PRIOR_AMD64_ASSET="autopus-adk_${prior_version}_darwin_amd64.tar.gz"
readonly PRIOR_ARM64_ASSET="autopus-adk_${prior_version}_darwin_arm64.tar.gz"
jq -e --arg bundle "$BUNDLE_NAME" --arg receipt "$RECEIPT_NAME" --arg signature "$SIGNATURE_NAME" \
  '[.assets[].name | select(. == $bundle or . == $receipt or . == $signature)] | length == 0' \
  "$release_json" >/dev/null \
  || fail prior_evidence_malformed 'independent receipt assets are forbidden'
download_dir="$temp_dir/downloads"
install -m 0700 -d "$download_dir"
for asset in "$PRIOR_AMD64_ASSET" "$PRIOR_ARM64_ASSET" "$CHECKSUMS_NAME"; do
  asset_digest=$(jq -er --arg name "$asset" \
    '[.assets[] | select(.name == $name)] | select(length == 1) | .[0] |
     select(.state == "uploaded") | .digest |
     select(type == "string" and test("^sha256:[0-9a-f]{64}$"))' "$release_json") \
    || fail prior_evidence_malformed "exact asset metadata is missing for ${asset}"
  env -i PATH="$PATH" HOME="${HOME-}" GITHUB_TOKEN="$GITHUB_TOKEN" \
    gh release download "$prior_tag" --repo "$prior_repository" \
    --pattern "$asset" --dir "$download_dir" \
    || fail prior_evidence_absent "exact ${prior_phase} asset is absent: ${asset}"
  downloaded_asset="$download_dir/$asset"
  [[ -f "$downloaded_asset" && ! -L "$downloaded_asset" ]] \
    || fail prior_evidence_absent "downloaded ${prior_phase} asset is invalid: ${asset}"
  actual_asset_digest=$(sha256_file "$downloaded_asset")
  [[ "$actual_asset_digest" == "$asset_digest" ]] \
    || fail prior_evidence_unverifiable "server digest differs for ${asset}"
  case "$asset" in
    "$PRIOR_AMD64_ASSET") archive_pin="$prior_amd64_archive" ;;
    "$PRIOR_ARM64_ASSET") archive_pin="$prior_arm64_archive" ;;
    *) archive_pin='' ;;
  esac
  [[ -z "$archive_pin" || "$actual_asset_digest" == "sha256:$archive_pin" ]] \
    || fail prior_archive_digest_mismatch "${prior_phase} archive differs from its pin: ${asset}"
done
extract_bundle "$download_dir/$PRIOR_AMD64_ASSET" "$temp_dir/amd64" amd64 \
  "$prior_amd64_manifest"
extract_bundle "$download_dir/$PRIOR_ARM64_ASSET" "$temp_dir/arm64" arm64 \
  "$prior_arm64_manifest"
checksums="$download_dir/$CHECKSUMS_NAME"
[[ "$(sha256_file "$checksums")" == "sha256:$prior_checksums" ]] \
  || fail prior_checksums_bytes_mismatch "checksums.txt differs from its ${prior_phase} pin"
for asset in "$PRIOR_AMD64_ASSET" "$PRIOR_ARM64_ASSET"; do
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
  || fail prior_receipt_bytes_mismatch "${prior_phase} architecture receipt bytes differ"
cmp -- "$prior_signature" "$temp_dir/arm64/$SIGNATURE_NAME" \
  || fail prior_signature_bytes_mismatch "${prior_phase} architecture signature bytes differ"
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
  || fail prior_evidence_unverifiable "cannot produce current ${release_phase} receipt"
[[ "$(find "$current_bundle" -mindepth 1 -maxdepth 1 -print | wc -l | tr -d '[:space:]')" == '2' ]] \
  || fail prior_evidence_malformed "current ${release_phase} bundle is malformed"
current_receipt="$current_bundle/$RECEIPT_NAME"
current_signature="$current_bundle/$SIGNATURE_NAME"
cmp -- "$prior_receipt" "$current_receipt" \
  || fail prior_receipt_bytes_mismatch "${release_phase} does not republish exact ${prior_phase} receipt bytes"
cmp -- "$prior_signature" "$current_signature" \
  || fail prior_signature_bytes_mismatch "${release_phase} does not republish exact ${prior_phase} signature bytes"
current_receipt_sha256=$(sha256_file "$current_receipt")
current_signature_sha256=$(sha256_file "$current_signature")
current_record_sha256=$({
  printf 'autopus.public-key-receipt.a0-record.v1\0'
  printf '%s' "${current_receipt_sha256#sha256:}" | xxd -r -p
  printf '%s' "${current_signature_sha256#sha256:}" | xxd -r -p
} | shasum -a 256 | awk '{print "sha256:" $1}') \
  || fail prior_evidence_unverifiable 'cannot digest current key record'
[[ "$current_record_sha256" == "sha256:$A0_RECORD_SHA256" ]] \
  || fail prior_record_digest_mismatch 'current key record differs from its A0 pin'
[[ "$(jq -er '.public_key_sha256' "$current_receipt")" == "sha256:$A0_PUBLIC_KEY_SHA256" ]] \
  || fail prior_public_key_digest_mismatch 'current public key digest differs from its A0 pin'
printf 'companion release lineage: %s exact %s key record verified\n' "$release_phase" "$prior_phase"
