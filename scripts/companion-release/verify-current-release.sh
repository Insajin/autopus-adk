#!/usr/bin/env bash
set -euo pipefail
umask 077

fail() { printf 'current release evidence: %s\n' "$1" >&2; exit 1; }
readonly RELEASE_REPOSITORY='Insajin/autopus-adk'
readonly RELEASE_VERSION='0.50.109'
readonly RELEASE_TAG='v0.50.109'
readonly RELEASE_MODE='canonical-full-bridge'
readonly BRIDGE_COMPANION_KEY_ID='adk-release-2026-q3-b0'
readonly BRIDGE_COMPANION_HANDOFF='v1'
readonly BRIDGE_COMPANION_ROLLBACK_FLOOR='5069'
readonly BRIDGE_COMPANION_PUBLIC_KEY_SHA256='sha256:c387da9e9c43dbaa2605207a00635c84937ff397a8b6ed73414d2e66b89941a4'
readonly BRIDGE_MANIFEST_NAME='omp-context-bridge-release.v1.json'
readonly ROTATION_DOCUMENT_NAME='adk-key-rotation-v1.json'
readonly ROTATION_SIGNATURE_NAME='adk-key-rotation-v1.sig'
readonly LINEAGE_NAME='release-lineage-v1.json'
readonly LINEAGE_SIGNATURE_NAME='release-lineage-v1.sig'
readonly ARM64_ARCHIVE_NAME="autopus-adk_${RELEASE_VERSION}_darwin_arm64.tar.gz"
EXPECTED_ARCHIVES=(
  "autopus-adk_${RELEASE_VERSION}_darwin_amd64.tar.gz"
  "autopus-adk_${RELEASE_VERSION}_darwin_arm64.tar.gz"
  "autopus-adk_${RELEASE_VERSION}_linux_amd64.tar.gz"
  "autopus-adk_${RELEASE_VERSION}_linux_arm64.tar.gz"
  "autopus-adk_${RELEASE_VERSION}_windows_amd64.tar.gz"
  "autopus-adk_${RELEASE_VERSION}_windows_amd64.zip"
  "autopus-adk_${RELEASE_VERSION}_windows_arm64.tar.gz"
  "autopus-adk_${RELEASE_VERSION}_windows_arm64.zip"
)
readonly EXPECTED_ARCHIVES
EXPECTED_ASSETS=(
  "${EXPECTED_ARCHIVES[@]}"
  'checksums.txt'
  'checksums.txt.bundle'
  'checksums.txt.signatures'
  "$BRIDGE_MANIFEST_NAME"
  "$ROTATION_DOCUMENT_NAME"
  "$ROTATION_SIGNATURE_NAME"
  "$LINEAGE_NAME"
  "$LINEAGE_SIGNATURE_NAME"
)
readonly EXPECTED_ASSETS
[[ $# -eq 1 ]] || fail 'usage: verify-current-release.sh CHECKSUMS_OUTPUT'
readonly checksums_output=$1
[[ -n "${GITHUB_TOKEN:-}" ]] || fail 'missing GITHUB_TOKEN'
for name in COMPANION_SOURCE_COMMIT COMPANION_SOURCE_TREE; do
  [[ "${!name:-}" =~ ^[0-9a-f]{40}$ ]] || fail "${name} is missing or malformed"
done
[[ "${OMP_CONTEXT_CANDIDATE_ARTIFACT_SHA256:-}" =~ ^[0-9a-f]{64}$ ]] ||
  fail 'OMP_CONTEXT_CANDIDATE_ARTIFACT_SHA256 is missing or malformed'
if [[ -n "${ADK_KEY_ROTATION_REF_COMMIT:-}" ]]; then
  [[ "$ADK_KEY_ROTATION_REF_COMMIT" =~ ^[0-9a-f]{40}$ ]] ||
    fail 'ADK_KEY_ROTATION_REF_COMMIT is malformed'
fi
if [[ -n "${ADK_KEY_ROTATION_DOCUMENT_SHA256:-}" ]]; then
  [[ "$ADK_KEY_ROTATION_DOCUMENT_SHA256" =~ ^[0-9a-f]{64}$ ]] ||
    fail 'ADK_KEY_ROTATION_DOCUMENT_SHA256 is malformed'
fi
for name in OMP_CONTEXT_STATIC_POLICY_B64 OMP_CONTEXT_EVIDENCE_REPORT_SHA256 \
  OMP_CONTEXT_EVIDENCE_ATTESTATION_SHA256 OMP_CONTEXT_EVIDENCE_VERIFIER \
  OMP_CONTEXT_PROMOTION_REPORT_PATH OMP_CONTEXT_PROMOTION_ATTESTATION_PATH
do
  [[ "${!name+x}" != x ]] || fail "canonical-full bridge forbids ${name}"
done
[[ -f "${OMP_CONTEXT_LINEAGE_VERIFIER:-}" && ! -L "$OMP_CONTEXT_LINEAGE_VERIFIER" &&
   -x "$OMP_CONTEXT_LINEAGE_VERIFIER" ]] || fail 'OMP_CONTEXT_LINEAGE_VERIFIER is missing or unsafe'
[[ -f "${COMPANION_MANIFEST_VERIFIER:-}" && ! -L "$COMPANION_MANIFEST_VERIFIER" &&
   -x "$COMPANION_MANIFEST_VERIFIER" ]] || fail 'COMPANION_MANIFEST_VERIFIER is missing or unsafe'
[[ -f "${ADK_KEY_ROTATION_VERIFIER:-}" && ! -L "$ADK_KEY_ROTATION_VERIFIER" &&
   -x "$ADK_KEY_ROTATION_VERIFIER" ]] || fail 'ADK_KEY_ROTATION_VERIFIER is missing or unsafe'
for tool in cmp gh jq shasum tar; do command -v "$tool" >/dev/null || fail "$tool is unavailable"; done
script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd) || fail 'cannot resolve verifier directory'
signature_helper="$script_dir/verify-current-release-signatures.sh"
[[ -f "$signature_helper" && ! -L "$signature_helper" && -x "$signature_helper" ]] ||
  fail 'current release signature helper is missing or unsafe'
output_dir=$(dirname -- "$checksums_output")
[[ -d "$output_dir" && ! -L "$output_dir" ]] || fail 'checksums output parent is unsafe'
[[ ! -e "$checksums_output" && ! -L "$checksums_output" ]] || fail 'checksums output already exists'

temp_dir=$(mktemp -d "${TMPDIR:-/tmp}/adk-current-bridge.XXXXXX") ||
  fail 'cannot create release evidence directory'
cleanup() {
  local status=$?
  rm -rf -- "$temp_dir" || status=$?
  if [[ "$status" -ne 0 ]]; then rm -f -- "$checksums_output" || true; fi
  return "$status"
}
trap cleanup EXIT
release_json="$temp_dir/release.json"
checksums="$temp_dir/checksums.txt"
bundle="$temp_dir/checksums.txt.bundle"
envelope="$temp_dir/checksums.txt.signatures"
bridge_manifest="$temp_dir/$BRIDGE_MANIFEST_NAME"
rotation_document="$temp_dir/$ROTATION_DOCUMENT_NAME"
rotation_signature="$temp_dir/$ROTATION_SIGNATURE_NAME"
lineage="$temp_dir/$LINEAGE_NAME"
lineage_signature="$temp_dir/$LINEAGE_SIGNATURE_NAME"
arm64_archive="$temp_dir/$ARM64_ARCHIVE_NAME"

GH_TOKEN="$GITHUB_TOKEN" gh api -H 'Accept: application/vnd.github+json' \
  "repos/${RELEASE_REPOSITORY}/releases/tags/${RELEASE_TAG}" >"$release_json" ||
  fail 'cannot read exact A22 bridge release'
[[ -s "$release_json" && ! -L "$release_json" ]] || fail 'A22 release metadata is empty or unsafe'
expected_assets_json=$(printf '%s\n' "${EXPECTED_ASSETS[@]}" |
  jq -Rsc 'split("\n") | map(select(length > 0))') || fail 'cannot construct expected asset set'
jq -e --arg tag "$RELEASE_TAG" --arg commit "$COMPANION_SOURCE_COMMIT" \
  --argjson expected "$expected_assets_json" '
  type == "object" and .tag_name == $tag and .target_commitish == $commit and
  .draft == false and .prerelease == false and .immutable == true and
  (.assets | type) == "array" and (.assets | length) == ($expected | length) and
  ([.assets[].name] | sort) == ($expected | sort) and
  ([.assets[].name] | unique | length) == ($expected | length) and
  all(.assets[]; (.id | type) == "number" and .id > 0 and .state == "uploaded" and
    (.size | type) == "number" and .size > 0 and
    (.digest | type) == "string" and (.digest | test("^sha256:[0-9a-f]{64}$")))
' "$release_json" >/dev/null || fail 'A22 bridge release is not exact, immutable, and complete'

download_release_asset() {
  local asset_name=$1 destination=$2 metadata asset_id asset_size api_digest actual_size actual_digest
  metadata=$(jq -er --arg name "$asset_name" \
    '.assets[] | select(.name == $name) | [.id, .size, .digest] | @tsv' "$release_json") ||
    fail "${asset_name} metadata is unavailable"
  IFS=$'\t' read -r asset_id asset_size api_digest <<<"$metadata"
  [[ "$asset_id" =~ ^[1-9][0-9]*$ && "$asset_size" =~ ^[1-9][0-9]*$ ]] ||
    fail "${asset_name} metadata is malformed"
  [[ ! -e "$destination" && ! -L "$destination" ]] || fail "${asset_name} destination exists"
  GH_TOKEN="$GITHUB_TOKEN" gh api -H 'Accept: application/octet-stream' \
    "repos/${RELEASE_REPOSITORY}/releases/assets/${asset_id}" >"$destination" ||
    fail "cannot download ${asset_name}"
  [[ -s "$destination" && ! -L "$destination" ]] || fail "downloaded ${asset_name} is unsafe"
  actual_size=$(wc -c <"$destination" | tr -d '[:space:]')
  actual_digest=$(shasum -a 256 "$destination" | awk '{print $1}')
  [[ "$actual_size" == "$asset_size" && "sha256:${actual_digest}" == "$api_digest" ]] ||
    fail "downloaded ${asset_name} differs from GitHub metadata"
}
download_release_asset 'checksums.txt' "$checksums"
download_release_asset 'checksums.txt.bundle' "$bundle"
download_release_asset 'checksums.txt.signatures' "$envelope"
download_release_asset "$BRIDGE_MANIFEST_NAME" "$bridge_manifest"
download_release_asset "$ROTATION_DOCUMENT_NAME" "$rotation_document"
download_release_asset "$ROTATION_SIGNATURE_NAME" "$rotation_signature"
download_release_asset "$LINEAGE_NAME" "$lineage"
download_release_asset "$LINEAGE_SIGNATURE_NAME" "$lineage_signature"
download_release_asset "$ARM64_ARCHIVE_NAME" "$arm64_archive"
[[ "$(wc -c <"$lineage_signature" | tr -d '[:space:]')" == '64' ]] ||
  fail 'released lineage signature is not raw Ed25519 bytes'
[[ "$(wc -c <"$rotation_signature" | tr -d '[:space:]')" == '64' ]] ||
  fail 'released rotation signature is not raw Ed25519 bytes'
jq -e --arg mode "$RELEASE_MODE" \
  '.schema_version == "omp-context-bridge-release.v1" and .release_mode == $mode' \
  "$bridge_manifest" >/dev/null || fail 'released bridge manifest is not canonical-full'
released_rotation_ref_commit=$(jq -er '.rotation_ref_commit |
  select(type == "string" and test("^[0-9a-f]{40}$"))' "$bridge_manifest") ||
  fail 'released bridge rotation ref commit is malformed'
released_rotation_document_sha256=$(jq -er '.rotation_document_sha256 |
  select(type == "string" and test("^[0-9a-f]{64}$"))' "$bridge_manifest") ||
  fail 'released bridge rotation document digest is malformed'
[[ -z "${ADK_KEY_ROTATION_REF_COMMIT:-}" ||
   "$ADK_KEY_ROTATION_REF_COMMIT" == "$released_rotation_ref_commit" ]] ||
  fail 'release-time rotation ref commit differs from immutable bridge manifest'
[[ -z "${ADK_KEY_ROTATION_DOCUMENT_SHA256:-}" ||
   "$ADK_KEY_ROTATION_DOCUMENT_SHA256" == "$released_rotation_document_sha256" ]] ||
  fail 'release-time rotation document digest differs from immutable bridge manifest'
rotation_document_sha256=$(shasum -a 256 "$rotation_document" | awk '{print $1}')
[[ "$rotation_document_sha256" == "$released_rotation_document_sha256" ]] ||
  fail 'released rotation document differs from bridge manifest digest'
channel_key_id=$(<"$script_dir/adk-channel-2026-q3-a0.key-id")
channel_public_key=$(<"$script_dir/adk-channel-2026-q3-a0.public.b64")
verified_rotation="$temp_dir/verified-rotation-document"
env -i PATH="$PATH" HOME="${HOME:-/}" TMPDIR="${TMPDIR:-/tmp}" \
  AUTOPUS_ADK_CHANNEL_KEY_ID="$channel_key_id" \
  AUTOPUS_ADK_CHANNEL_PUBLIC_KEY="$channel_public_key" \
  "$ADK_KEY_ROTATION_VERIFIER" verify-rotation-historical \
  --document "$rotation_document" --signature "$rotation_signature" \
  --source-commit "$COMPANION_SOURCE_COMMIT" --source-tree "$COMPANION_SOURCE_TREE" \
  --next-tag-public-key "$script_dir/release-tag-signing-2026-q3-r2.pub" \
  --next-tag-fingerprint "$script_dir/release-tag-signing-2026-q3-r2.fingerprint" \
  --next-promotion-public-key "$script_dir/omp-context-promotion-2026-q3-k3.pub" \
  >"$verified_rotation" || fail 'released historical rotation sidecar is invalid'
cmp -s "$rotation_document" "$verified_rotation" ||
  fail 'released historical rotation verifier changed canonical bytes'

expected_bridge_manifest="$temp_dir/expected-$BRIDGE_MANIFEST_NAME"
env -i PATH="$PATH" HOME="${HOME:-/}" TMPDIR="${TMPDIR:-/tmp}" \
  COMPANION_RELEASE_TAG="$RELEASE_TAG" COMPANION_SOURCE_COMMIT="$COMPANION_SOURCE_COMMIT" \
  COMPANION_SOURCE_TREE="$COMPANION_SOURCE_TREE" \
  OMP_CONTEXT_CANDIDATE_ARTIFACT_SHA256="$OMP_CONTEXT_CANDIDATE_ARTIFACT_SHA256" \
  ADK_KEY_ROTATION_REF_COMMIT="$released_rotation_ref_commit" \
  ADK_KEY_ROTATION_DOCUMENT_SHA256="$released_rotation_document_sha256" \
  "$script_dir/produce-omp-context-bridge-manifest.sh" "$expected_bridge_manifest"
cmp -s "$expected_bridge_manifest" "$bridge_manifest" ||
  fail 'released bridge manifest differs from canonical-full authorized coordinates'

archive_listing="$temp_dir/arm64-archive-entries"
tar -tzf "$arm64_archive" >"$archive_listing" || fail 'Darwin arm64 archive cannot be listed'
if grep -Eq '(^/|(^|/)\.\.(/|$)|//)' "$archive_listing"; then fail 'archive contains unsafe path'; fi
bundle_dir="$temp_dir/adk-companion-public-key-receipt.bundle"
install -m 0700 -d "$bundle_dir"
extract_archive_entry() {
  local name=$1 output=$2
  [[ "$(grep -Fxc "$name" "$archive_listing")" == '1' ]] ||
    fail "archive entry is absent or duplicate: ${name}"
  tar -xOzf "$arm64_archive" "$name" >"$output" || fail "cannot extract archive entry: ${name}"
  chmod 0600 "$output"
}
archive_auto="$temp_dir/auto"
archive_manifest="$temp_dir/adk-companion-manifest.json"
archive_manifest_signature="$temp_dir/adk-companion-manifest.sig"
archive_lineage="$temp_dir/archive-$LINEAGE_NAME"
archive_lineage_signature="$temp_dir/archive-$LINEAGE_SIGNATURE_NAME"
extract_archive_entry auto "$archive_auto"
extract_archive_entry adk-companion-manifest.json "$archive_manifest"
extract_archive_entry adk-companion-manifest.sig "$archive_manifest_signature"
extract_archive_entry adk-companion-public-key-receipt.bundle/public-key-receipt.json \
  "$bundle_dir/public-key-receipt.json"
extract_archive_entry adk-companion-public-key-receipt.bundle/public-key-receipt.sig \
  "$bundle_dir/public-key-receipt.sig"
extract_archive_entry "$LINEAGE_NAME" "$archive_lineage"
extract_archive_entry "$LINEAGE_SIGNATURE_NAME" "$archive_lineage_signature"
cmp -s "$lineage" "$archive_lineage" || fail 'standalone and archived lineage bytes differ'
cmp -s "$lineage_signature" "$archive_lineage_signature" ||
  fail 'standalone and archived lineage signatures differ'
chmod 0700 "$archive_auto"
env -i PATH="$PATH" HOME="${HOME:-/}" TMPDIR="${TMPDIR:-/tmp}" \
  "$COMPANION_MANIFEST_VERIFIER" --artifact "$archive_auto" \
  --manifest "$archive_manifest" --signature "$archive_manifest_signature" \
  --receipt "$bundle_dir/public-key-receipt.json" \
  --receipt-signature "$bundle_dir/public-key-receipt.sig" \
  --key-id "$BRIDGE_COMPANION_KEY_ID" --version "$RELEASE_VERSION" \
  --platform darwin --architecture arm64 --handoff "$BRIDGE_COMPANION_HANDOFF" \
  --minimum-rollback-floor "$BRIDGE_COMPANION_ROLLBACK_FLOOR" \
  --public-key-sha256 "$BRIDGE_COMPANION_PUBLIC_KEY_SHA256" ||
  fail 'released Darwin arm64 companion authority is invalid'

expected_archives_json=$(printf '%s\n' "${EXPECTED_ARCHIVES[@]}" |
  jq -Rsc 'split("\n") | map(select(length > 0))')
checksum_entries_json=$(jq -Rsc '
  if endswith("\n") then .[0:-1] else error("missing final newline") end |
  split("\n") | map(capture("^(?<digest>[0-9a-f]{64})  (?<name>[A-Za-z0-9._-]+)$"))
' "$checksums") || fail 'checksums.txt contains malformed lines'
printf '%s' "$checksum_entries_json" | jq -e --argjson expected "$expected_archives_json" '
  length == ($expected | length) and ([.[].name] | sort) == ($expected | sort) and
  ([.[].name] | unique | length) == ($expected | length)
' >/dev/null || fail 'checksums.txt does not describe exactly eight A22 archives'
for archive in "${EXPECTED_ARCHIVES[@]}"; do
  api_digest=$(jq -er --arg name "$archive" '.assets[] | select(.name == $name) | .digest' "$release_json")
  checksum_digest=$(printf '%s' "$checksum_entries_json" |
    jq -er --arg name "$archive" '.[] | select(.name == $name) | .digest')
  [[ "$api_digest" == "sha256:${checksum_digest}" ]] ||
    fail "checksums.txt differs from GitHub API digest for ${archive}"
done
env -i PATH="$PATH" HOME="${HOME:-/}" TMPDIR="${TMPDIR:-/tmp}" \
  "$signature_helper" "$checksums" "$bundle" "$envelope" ||
  fail 'A22 checksum ECDSA/cosign evidence is invalid'

distributed_artifact_sha=$(jq -er '.artifact_digest |
  select(type == "string" and test("^sha256:[0-9a-f]{64}$"))' "$archive_manifest") ||
  fail 'released Darwin arm64 manifest digest is malformed'
env -i PATH="$PATH" HOME="${HOME:-/}" TMPDIR="${TMPDIR:-/tmp}" \
  "$OMP_CONTEXT_LINEAGE_VERIFIER" --lineage "$lineage" --signature "$lineage_signature" \
  --receipt-bundle "$bundle_dir" --key-id "$BRIDGE_COMPANION_KEY_ID" \
  --handoff "$BRIDGE_COMPANION_HANDOFF" \
  --minimum-rollback-floor "$BRIDGE_COMPANION_ROLLBACK_FLOOR" \
  --upstream-sha256 "sha256:$OMP_CONTEXT_CANDIDATE_ARTIFACT_SHA256" \
  --executable-sha256 "$distributed_artifact_sha" \
  --source-repository "$RELEASE_REPOSITORY" --source-commit "$COMPANION_SOURCE_COMMIT" \
  --source-tree "$COMPANION_SOURCE_TREE" --target darwin-arm64 --version "$RELEASE_VERSION" ||
  fail 'released canonical-full bridge U-to-D lineage is invalid'
install -m 0600 "$checksums" "$checksums_output" || fail 'cannot materialize verified checksums.txt'
cmp -s "$checksums" "$checksums_output" || fail 'materialized checksums differ'
printf 'current release evidence: exactly sixteen A22 canonical-full bridge assets verified\n'
