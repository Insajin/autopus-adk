#!/usr/bin/env bash
set -euo pipefail
umask 077

tests_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
script_dir=$(cd -- "$tests_dir/.." && pwd)
repo=$(cd -- "$script_dir/../.." && pwd)
fail() { printf 'release hardening test: %s\n' "$1" >&2; exit 1; }
contains() { grep -Fq -- "$2" "$1" || fail "$1 missing $2"; }
not_contains() { ! grep -Fq -- "$2" "$1" || fail "$1 unexpectedly contains $2"; }

config="$repo/.goreleaser.yaml"
release="$repo/.github/workflows/release.yaml"
recovery="$repo/.github/workflows/homebrew-formula-bridge-recovery.yaml"
source_gate="$script_dir/validate-source.sh"
environment_gate="$script_dir/validate-environment.sh"
lineage_archive="$script_dir/verify-public-key-lineage-archive.sh"
lineage="$script_dir/verify-public-key-lineage.sh"
lineage_pins="$script_dir/verify-public-key-lineage-pins.sh"
producer="$script_dir/produce.sh"
homebrew_bridge="$script_dir/publish-homebrew-formula-bridge.sh"

# GoReleaser must render, but never publish, the Cask or mutate tagged source.
contains "$config" 'skip_upload: true'
not_contains "$config" 'token: "{{ .Env.HOMEBREW_TAP_TOKEN }}"'
not_contains "$config" 'go mod tidy'
contains "$release" 'go mod tidy -diff'

# A fresh, narrowly scoped tap token must be created after immutable release publish.
release_index=$(grep -n 'goreleaser release --clean' "$release" | cut -d: -f1)
token_index=$(grep -n 'name: Create Homebrew tap token' "$release" | cut -d: -f1)
bridge_index=$(grep -n 'scripts/companion-release/publish-homebrew-formula-bridge.sh' "$release" | cut -d: -f1)
(( release_index < token_index && token_index < bridge_index )) || fail 'tap token ordering is unsafe'
goreleaser_step=$(sed -n '/name: Run GoReleaser/,/name: Create Homebrew tap token/p' "$release")
[[ "$goreleaser_step" != *HOMEBREW_TAP_TOKEN* ]] || fail 'GoReleaser receives tap token'
[[ "$goreleaser_step" != *APPLE_CERTIFICATE_PASSWORD* ]] || fail 'GoReleaser receives certificate password'
contains "$release" "COMPANION_CASK_PATH='dist/homebrew/Casks/auto.rb'"
contains "$producer" '--signing-key "$COMPANION_SIGNING_KEY_FILE"'
contains "$homebrew_bridge" "readonly PRIOR_CASK_BLOB='ceed648bfece4555e8310b6e894fedc847520960'"
contains "$homebrew_bridge" 'COMPANION_HOMEBREW_POLICY'
not_contains "$homebrew_bridge" 'Formula/auto.rb'
not_contains "$homebrew_bridge" 'FORMULA_PATH'
not_contains "$homebrew_bridge" 'PRIOR_FORMULA'
not_contains "$homebrew_bridge" 'verify_frozen_formula'
not_contains "$homebrew_bridge" 'reconcile_tap_file formula Formula'

# Production/recovery source coordinates must bind to externally approved exact values.
for workflow in "$release" "$recovery"; do
  contains "$workflow" 'ADK_COMPANION_APPROVED_SOURCE_COMMIT'
  contains "$workflow" 'ADK_COMPANION_APPROVED_SOURCE_TREE'
  contains "$workflow" 'COMPANION_SOURCE_PIN_REQUIRED='
done
contains "$recovery" "if: github.ref == 'refs/tags/v0.50.76'"
contains "$recovery" 'gh workflow run homebrew-formula-bridge-recovery.yaml --ref v0.50.76'
not_contains "$release" "'v0.50.75'"
not_contains "$release" 'refs/tags/v0.50.75'
not_contains "$recovery" 'refs/tags/v0.50.75'
contains "$source_gate" "readonly A6_A5_ANCESTOR_SHA='b27252cb1148192a8ae1a95195c50e5f221453a4'"
contains "$lineage" 'source "$pins_helper"'
contains "$lineage_pins" "A4_TAG_OBJECT_SHA='b1ebab0af82536f8a4bc1ed93f31f82f6c53d008'"
contains "$lineage_pins" "A4_CHECKSUMS_SHA256='a30e0893f1565919e9e90dd7e1f2b19e5487024b0373f66de56729e1d747e7d1'"
contains "$lineage_pins" "A4_AMD64_ARCHIVE_SHA256='da7f6ef4396591ff0b728f976536d261ecb084038fffab7c7662a6f7329ade2a'"
contains "$lineage_pins" "A4_ARM64_ARCHIVE_SHA256='ff046f6af316236166d514608a1b432c2f3a01efbd8aab03b54d2c2639d2f422'"
contains "$lineage_pins" "A4_AMD64_MANIFEST_SHA256='86940b9c7eb89308aff4260d9a6178d933d3f1a9833e601ac8c1e914c225a7b5'"
contains "$lineage_pins" "A4_ARM64_MANIFEST_SHA256='a68a10a46b0778ccc858855323fd45cf0b9727f76fa45b16efdbc83b320128f0'"
contains "$lineage_pins" "A5_TAG_OBJECT_SHA='c79f133f0108bf3f07cee0162c1abeecf9d379d1'"
contains "$lineage_pins" "A5_CHECKSUMS_SHA256='48c79e1fb47444aa83909794cd041bdfed18bf263bf5c0209578540382824ad4'"
contains "$lineage_pins" "A5_AMD64_ARCHIVE_SHA256='aeb9d048579c77ab17f4a4ec3a1160778d16c627747c5af5f341e664e1417cb0'"
contains "$lineage_pins" "A5_ARM64_ARCHIVE_SHA256='bc90e594c91de61dabc2982f60249b638d448fa3f6643004fe6d45cdd0cc5eab'"
contains "$lineage_pins" "A5_AMD64_MANIFEST_SHA256='5b4381d3f2180b19c0da9d419ebc8452b9ba04c73c8d0921c2a74c09ab38b85c'"
contains "$lineage_pins" "A5_ARM64_MANIFEST_SHA256='62a9f78302ee000c16c1c73669282e955fc3abc82f850ff4a77d0e04069f4aed'"
contains "$lineage" "release_phase='A6' prior_phase='A5'"
contains "$release" 'timeout-minutes: 60'
contains "$recovery" 'timeout-minutes: 20'

temp=$(mktemp -d "${TMPDIR:-/tmp}/release-hardening-test.XXXXXX")
trap 'rm -rf -- "$temp"' EXIT
git clone -q --no-hardlinks --no-tags "$repo" "$temp/source"
git -C "$temp/source" config user.name 'Release Test'
git -C "$temp/source" config user.email release-test@example.invalid
git -C "$temp/source" tag -am 'A6 fixture' v0.50.76
commit=$(git -C "$temp/source" rev-parse HEAD)
tree=$(git -C "$temp/source" rev-parse 'HEAD^{tree}')
if [[ "${tree: -1}" == '0' ]]; then
  wrong_tree="${tree%?}1"
else
  wrong_tree="${tree%?}0"
fi
run_source_gate() {
  local approved_commit="${1-}" approved_tree="${2-}"
  env GITHUB_REF_NAME=v0.50.76 GITHUB_REF_TYPE=tag GITHUB_SHA="$commit" \
    GITHUB_OUTPUT="$temp/source-output" COMPANION_SOURCE_PIN_REQUIRED=1 \
    COMPANION_APPROVED_SOURCE_COMMIT="$approved_commit" \
    COMPANION_APPROVED_SOURCE_TREE="$approved_tree" \
    bash "$source_gate"
}
if (cd "$temp/source" && run_source_gate '' '') >/dev/null 2>&1; then
  fail 'missing approved source pins passed'
fi
if (cd "$temp/source" && run_source_gate "$commit" "$wrong_tree") >/dev/null 2>&1; then
  fail 'wrong approved source tree passed'
fi
(cd "$temp/source" && run_source_gate "$commit" "$tree") >/dev/null

# Current-time checks must reject expired production material.
touch "$temp/key" "$temp/api-key"
chmod 0600 "$temp/key" "$temp/api-key"
printf '#!/usr/bin/env bash\nexit 0\n' >"$temp/tool"
chmod 0700 "$temp/tool"
format_time() {
  if date -u -r "$1" '+%Y-%m-%dT%H:%M:%SZ' >/dev/null 2>&1; then
    date -u -r "$1" '+%Y-%m-%dT%H:%M:%SZ'
  else
    date -u -d "@$1" '+%Y-%m-%dT%H:%M:%SZ'
  fi
}
now=$(date -u '+%s')
validation_env=(
  COMPANION_BUILD_PROVENANCE=github-actions:fixture@0123456789abcdef
  COMPANION_HANDOFF=v1 COMPANION_ROLLBACK_FLOOR=5069
  COMPANION_KEY_ID=adk-release-2026-q3-b0 COMPANION_SIGNING_KEY_FILE="$temp/key"
  COMPANION_SIGNER="$temp/tool" COMPANION_RECEIPT_VERIFIER="$temp/tool"
  COMPANION_MANIFEST_VERIFIER="$temp/tool" COMPANION_RELEASE_TIME_VALIDATION_REQUIRED=1
  COMPANION_PUBLIC_KEY_RECEIPT_MINIMUM_LIFETIME_SECONDS=31536000
  COMPANION_PUBLIC_KEY_RECEIPT_ISSUED_AT="$(format_time "$((now - 86400))")"
  COMPANION_PUBLIC_KEY_RECEIPT_EXPIRES_AT="$(format_time "$((now + 31536001))")"
  APPLE_SIGNING_IDENTITY='Developer ID Application: Fixture (GP2PFA2PUV)'
  APPLE_API_KEY=FIXTURE APPLE_API_ISSUER=123e4567-e89b-42d3-a456-426614174000
  APPLE_API_KEY_PATH="$temp/api-key"
)
env "${validation_env[@]}" COMPANION_ISSUED_AT="$(format_time "$((now - 60))")" \
  COMPANION_EXPIRES_AT="$(format_time "$((now + 3600))")" bash "$environment_gate"
if env "${validation_env[@]}" COMPANION_ISSUED_AT="$(format_time "$((now - 7200))")" \
  COMPANION_EXPIRES_AT="$(format_time "$((now - 3600))")" bash "$environment_gate" >/dev/null 2>&1; then
  fail 'expired manifest window passed'
fi

# The independent verifier must bind receipt key -> manifest signature -> artifact.
verifier="$temp/manifest-verifier"
go build -trimpath -o "$verifier" "$repo/scripts/companion-release/manifestverify"
go run "$tests_dir/testdata/generate-manifest-fixture.go" "$temp/manifest"
trusted_key_sha=$(jq -er '.public_key_sha256' "$temp/manifest/public-key-receipt.json")
install -m 0600 "$temp/manifest/signing-key" "$temp/trusted-signing-key"
common_verify_args=(
  --artifact "$temp/manifest/auto" --manifest "$temp/manifest/adk-companion-manifest.json"
  --signature "$temp/manifest/adk-companion-manifest.sig"
  --receipt "$temp/manifest/public-key-receipt.json"
  --receipt-signature "$temp/manifest/public-key-receipt.sig"
  --key-id adk-release-2026-q3-b0 --version 0.50.71 --platform darwin
  --architecture arm64 --handoff v1 --minimum-rollback-floor 5069
)
signing_verify_args=("${common_verify_args[@]}" --signing-key "$temp/trusted-signing-key")
pin_verify_args=("${common_verify_args[@]}" --public-key-sha256 "$trusted_key_sha")
"$verifier" "${signing_verify_args[@]}"
"$verifier" "${pin_verify_args[@]}"
if "$verifier" "${common_verify_args[@]}" >/dev/null 2>&1; then
  fail 'unanchored self-signed receipt passed'
fi
if "$verifier" "${signing_verify_args[@]}" --public-key-sha256 "$trusted_key_sha" \
  >/dev/null 2>&1; then
  fail 'ambiguous duplicate trust anchors passed'
fi
if "$verifier" "${common_verify_args[@]}" \
  --signing-key "$temp/manifest/inconsistent-signing-key" >/dev/null 2>&1; then
  fail 'inconsistent private/public signing key passed'
fi
printf 'tamper\n' >>"$temp/manifest/auto"
if "$verifier" "${signing_verify_args[@]}" >/dev/null 2>&1; then fail 'tampered artifact passed'; fi
printf 'signed companion artifact\n' >"$temp/manifest/auto"
manifest_signature="$temp/manifest/adk-companion-manifest.sig"
signature_size_before=$(wc -c <"$manifest_signature")
(( signature_size_before == 64 )) || fail 'fixture manifest signature length drifted'
signature_byte_before=$(od -An -tu1 -N1 "$manifest_signature" | tr -d '[:space:]')
case "$signature_byte_before" in
  1) signature_byte_replacement=2 ;;
  ''|*[!0-9]*) fail 'fixture manifest signature byte could not be read' ;;
  *) signature_byte_replacement=1 ;;
esac
if (( signature_byte_replacement == 1 )); then printf '\001'; else printf '\002'; fi |
  dd of="$manifest_signature" bs=1 seek=0 conv=notrunc 2>/dev/null
signature_size_after=$(wc -c <"$manifest_signature")
(( signature_size_after == signature_size_before )) || fail 'manifest signature tamper changed length'
signature_byte_after=$(od -An -tu1 -N1 "$manifest_signature" | tr -d '[:space:]')
[[ "$signature_byte_after" == "$signature_byte_replacement" ]] ||
  fail 'manifest signature tamper did not write the replacement byte'
[[ "$signature_byte_after" != "$signature_byte_before" ]] ||
  fail 'manifest signature tamper was a no-op'
if "$verifier" "${signing_verify_args[@]}" >/dev/null 2>&1; then fail 'tampered manifest signature passed'; fi
go run "$tests_dir/testdata/generate-manifest-fixture.go" "$temp/manifest" \
  'attacker replacement artifact'
if "$verifier" "${signing_verify_args[@]}" >/dev/null 2>&1; then
  fail 'replacement receipt, manifest, and artifact pair passed the signing-key anchor'
fi
if "$verifier" "${pin_verify_args[@]}" >/dev/null 2>&1; then
  fail 'replacement receipt, manifest, and artifact pair passed the A0 key pin'
fi
contains "$lineage_archive" 'MANIFEST_SIGNATURE_NAME'
contains "$lineage_archive" 'COMPANION_MANIFEST_VERIFIER'

# A6 updates only the Cask and keeps the exact v0.50.71 Formula frozen.
state="$temp/tap-state"
mkdir -m 0700 "$state" "$temp/bin"
install -m 0700 "$tests_dir/testdata/mock-tap-gh.sh" "$temp/bin/gh"
checksums="$temp/checksums.txt"
printf '%064d  autopus-adk_0.50.76_darwin_amd64.tar.gz\n' 1 >"$checksums"
printf '%064d  autopus-adk_0.50.76_darwin_arm64.tar.gz\n' 2 >>"$checksums"
printf '%064d  autopus-adk_0.50.76_linux_amd64.tar.gz\n' 3 >>"$checksums"
printf '%064d  autopus-adk_0.50.76_linux_arm64.tar.gz\n' 4 >>"$checksums"
source "$script_dir/publish-homebrew-formula-bridge-render.sh"
render_homebrew_cask "$temp/prior-cask.rb" 0.50.74 \
  'aeb9d048579c77ab17f4a4ec3a1160778d16c627747c5af5f341e664e1417cb0' \
  'bc90e594c91de61dabc2982f60249b638d448fa3f6643004fe6d45cdd0cc5eab' \
  'ac95ad2bc5d24cccac8a73555d0845738f04306b8e5f7cef0843ce0f7d4b2a6d' \
  'eba5fd17e8b1b66349b26028f4487448bc0a49c3975a66defe1ece76744f7ad7'
render_homebrew_formula_bridge "$temp/frozen-formula.rb" v0.50.71 0.50.71 \
  "$(printf '%064d' 1)" "$(printf '%064d' 2)" \
  "$(printf '%064d' 3)" "$(printf '%064d' 4)"
jq -n --arg content "$(base64 <"$temp/prior-cask.rb" | tr -d '\r\n')" \
  '{sha:"ceed648bfece4555e8310b6e894fedc847520960",content:$content}' >"$state/cask.json"
jq -n --arg content "$(base64 <"$temp/frozen-formula.rb" | tr -d '\r\n')" \
  '{sha:"4ebc6c38925002dec00759823d4dd847a499818a",content:$content}' >"$state/formula.json"
cp "$state/formula.json" "$temp/formula-before.json"
bridge_env=(PATH="$temp/bin:$PATH" MOCK_TAP_STATE="$state" GITHUB_REF_NAME=v0.50.76
  COMPANION_VERSION=0.50.76 COMPANION_HOMEBREW_POLICY=cask-only
  COMPANION_CHECKSUMS_PATH="$checksums" HOMEBREW_TAP_TOKEN=fixture)
env "${bridge_env[@]}" bash "$script_dir/publish-homebrew-formula-bridge.sh"
[[ "$(cat "$state/cask.updates")" == 1 && ! -e "$state/formula.updates" ]] \
  || fail 'A6 did not update only the Cask'
cmp -s "$temp/formula-before.json" "$state/formula.json" \
  || fail 'frozen v0.50.71 Formula blob or bytes changed'
env "${bridge_env[@]}" bash "$script_dir/publish-homebrew-formula-bridge.sh"
[[ "$(cat "$state/cask.updates")" == 1 && ! -e "$state/formula.updates" ]] \
  || fail 'A6 Cask-only reconciler is not idempotent'

printf 'release hardening test: PASS\n'
