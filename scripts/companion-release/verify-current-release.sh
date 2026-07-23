#!/usr/bin/env bash
set -euo pipefail
umask 077

fail() {
  printf 'current release evidence: %s\n' "$1" >&2
  exit 1
}

readonly RELEASE_REPOSITORY='Insajin/autopus-adk'
readonly RELEASE_TAG='v0.50.87'
readonly RELEASE_VERSION='0.50.87'

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
)
readonly EXPECTED_ASSETS
[[ $# == 1 ]] || fail 'usage: verify-current-release.sh CHECKSUMS_OUTPUT'
readonly checksums_output=$1
[[ -n "${GITHUB_TOKEN:-}" ]] || fail 'missing GITHUB_TOKEN'
[[ "${COMPANION_SOURCE_COMMIT:-}" =~ ^[0-9a-f]{40}$ ]] \
  || fail 'exact source commit is missing or malformed'
for tool in gh jq shasum; do
  command -v "$tool" >/dev/null 2>&1 || fail "required tool is unavailable: ${tool}"
done
script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd) \
  || fail 'cannot resolve current release verifier directory'
signature_helper="$script_dir/verify-current-release-signatures.sh"
[[ -f "$signature_helper" && ! -L "$signature_helper" && -x "$signature_helper" ]] \
  || fail 'current release signature helper is missing or unsafe'

output_dir=$(dirname -- "$checksums_output")
[[ -d "$output_dir" && ! -L "$output_dir" ]] \
  || fail 'checksums output parent must be a non-symlink directory'
[[ ! -e "$checksums_output" && ! -L "$checksums_output" ]] \
  || fail 'checksums output already exists'

temp_dir=$(mktemp -d "${TMPDIR:-/tmp}/adk-current-release.XXXXXX") \
  || fail 'cannot create release evidence directory'
cleanup() {
  local status=$?
  rm -rf -- "$temp_dir" || status=$?
  if ((status != 0)); then
    rm -f -- "$checksums_output" || true
  fi
  return "$status"
}
trap cleanup EXIT
release_json="$temp_dir/release.json"
downloaded_checksums="$temp_dir/checksums.txt"
downloaded_bundle="$temp_dir/checksums.txt.bundle"
downloaded_envelope="$temp_dir/checksums.txt.signatures"

if ! GH_TOKEN="$GITHUB_TOKEN" gh api \
  -H 'Accept: application/vnd.github+json' \
  "repos/${RELEASE_REPOSITORY}/releases/tags/${RELEASE_TAG}" > "$release_json"; then
  fail 'cannot read the exact A16 GitHub release'
fi
[[ -f "$release_json" && ! -L "$release_json" && -s "$release_json" ]] \
  || fail 'A16 GitHub release metadata is empty or unsafe'

expected_assets_json=$(printf '%s\n' "${EXPECTED_ASSETS[@]}" \
  | jq -Rsc 'split("\n") | map(select(length > 0))') \
  || fail 'cannot construct the expected A16 asset set'
if ! jq -e --arg tag "$RELEASE_TAG" --arg commit "$COMPANION_SOURCE_COMMIT" \
  --argjson expected "$expected_assets_json" '
    type == "object" and
    .tag_name == $tag and
    .target_commitish == $commit and
    .draft == false and
    .prerelease == false and
    .immutable == true and
    (.assets | type) == "array" and
    (.assets | length) == ($expected | length) and
    ([.assets[].name] | sort) == ($expected | sort) and
    ([.assets[].name] | unique | length) == ($expected | length) and
    all(.assets[];
      (.id | type) == "number" and .id > 0 and
      (.name | type) == "string" and
      .state == "uploaded" and
      (.size | type) == "number" and .size > 0 and
      (.digest | type) == "string" and
      (.digest | test("^sha256:[0-9a-f]{64}$")))
  ' "$release_json" >/dev/null; then
  fail 'A16 release is not exact, final, immutable, complete, and digest-bound'
fi

# @AX:ANCHOR [AUTO]: Keep all three immutable release asset downloads on one digest-bound path.
# @AX:REASON [AUTO]: Checksums, the Sigstore bundle, and the K1 envelope must share identical GitHub metadata and byte verification before publication continues.
download_release_asset() {
  local asset_name=$1
  local destination=$2
  local metadata asset_id asset_size api_digest downloaded_size downloaded_digest
  metadata=$(jq -er --arg name "$asset_name" '
    .assets[] | select(.name == $name) | [.id, .size, .digest] | @tsv
  ' "$release_json") || fail "${asset_name} metadata is unavailable"
  IFS=$'\t' read -r asset_id asset_size api_digest <<< "$metadata"
  [[ "$asset_id" =~ ^[1-9][0-9]*$ && "$asset_size" =~ ^[1-9][0-9]*$ ]] \
    || fail "${asset_name} identifier or size is malformed"
  [[ ! -e "$destination" && ! -L "$destination" ]] \
    || fail "${asset_name} destination already exists"
  if ! GH_TOKEN="$GITHUB_TOKEN" gh api \
    -H 'Accept: application/octet-stream' \
    "repos/${RELEASE_REPOSITORY}/releases/assets/${asset_id}" > "$destination"; then
    fail "cannot download ${asset_name} from the exact A16 release"
  fi
  [[ -f "$destination" && ! -L "$destination" && -s "$destination" ]] \
    || fail "downloaded ${asset_name} is empty or unsafe"
  downloaded_size=$(wc -c < "$destination" | tr -d '[:space:]')
  [[ "$downloaded_size" == "$asset_size" ]] \
    || fail "downloaded ${asset_name} size differs from GitHub metadata"
  downloaded_digest=$(shasum -a 256 "$destination" | awk '{print $1}') \
    || fail "cannot digest downloaded ${asset_name}"
  [[ "$downloaded_digest" =~ ^[0-9a-f]{64}$ \
     && "sha256:${downloaded_digest}" == "$api_digest" ]] \
    || fail "downloaded ${asset_name} differs from its GitHub API digest"
}

download_release_asset 'checksums.txt' "$downloaded_checksums"
download_release_asset 'checksums.txt.bundle' "$downloaded_bundle"
download_release_asset 'checksums.txt.signatures' "$downloaded_envelope"

expected_archives_json=$(printf '%s\n' "${EXPECTED_ARCHIVES[@]}" \
  | jq -Rsc 'split("\n") | map(select(length > 0))') \
  || fail 'cannot construct the expected archive set'
checksum_entries_json=$(jq -Rsc '
  if endswith("\n") then .[0:-1] else error("missing final newline") end |
  split("\n") |
  map(capture("^(?<digest>[0-9a-f]{64})  (?<name>[A-Za-z0-9._-]+)$"))
' "$downloaded_checksums") || fail 'checksums.txt contains malformed lines'
if ! printf '%s' "$checksum_entries_json" | jq -e \
  --argjson expected "$expected_archives_json" '
    length == ($expected | length) and
    ([.[].name] | sort) == ($expected | sort) and
    ([.[].name] | unique | length) == ($expected | length)
  ' >/dev/null; then
  fail 'checksums.txt does not describe exactly the eight A16 archives'
fi

for archive in "${EXPECTED_ARCHIVES[@]}"; do
  api_digest=$(jq -er --arg name "$archive" \
    '.assets[] | select(.name == $name) | .digest' "$release_json") \
    || fail "API digest is unavailable for ${archive}"
  checksum_digest=$(printf '%s' "$checksum_entries_json" \
    | jq -er --arg name "$archive" '.[] | select(.name == $name) | .digest') \
    || fail "checksum entry is unavailable for ${archive}"
  [[ "$api_digest" == "sha256:${checksum_digest}" ]] \
    || fail "checksums.txt differs from the API digest for ${archive}"
done

# @AX:ANCHOR [AUTO]: Drop every release/API credential before parsing signed evidence.
# @AX:REASON [AUTO]: OpenSSL and Cosign need only local evidence, PATH, HOME, and TMPDIR; repository tokens must not cross this trust boundary.
env -i PATH="$PATH" HOME="${HOME:-/}" TMPDIR="${TMPDIR:-/tmp}" \
  "$signature_helper" \
  "$downloaded_checksums" "$downloaded_bundle" "$downloaded_envelope" \
  || fail 'A16 release signature evidence is invalid'

install -m 0600 "$downloaded_checksums" "$checksums_output" \
  || fail 'cannot materialize verified checksums.txt'
cmp -s "$downloaded_checksums" "$checksums_output" \
  || fail 'materialized checksums.txt differs from verified bytes'
printf 'current release evidence: exact immutable A16 release verified\n'
