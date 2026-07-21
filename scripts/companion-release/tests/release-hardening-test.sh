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
current_release_gate="$script_dir/verify-current-release.sh"

# GoReleaser must render, but never publish, the Cask or mutate tagged source.
contains "$config" 'skip_upload: true'
not_contains "$config" 'token: "{{ .Env.HOMEBREW_TAP_TOKEN }}"'
not_contains "$config" 'go mod tidy'
contains "$release" 'go mod tidy -diff'

# A fresh, narrowly scoped tap token must be created after immutable release publish.
release_index=$(grep -n 'goreleaser release --clean' "$release" | cut -d: -f1)
evidence_index=$(grep -n 'scripts/companion-release/verify-current-release.sh' "$release" | cut -d: -f1)
token_index=$(grep -n 'name: Create Homebrew tap token' "$release" | cut -d: -f1)
bridge_index=$(grep -n 'scripts/companion-release/publish-homebrew-formula-bridge.sh' "$release" | cut -d: -f1)
(( release_index < evidence_index && evidence_index < token_index && token_index < bridge_index )) \
  || fail 'release evidence or tap token ordering is unsafe'
goreleaser_step=$(sed -n '/name: Run GoReleaser/,/name: Verify current immutable release evidence/p' "$release")
[[ "$goreleaser_step" != *HOMEBREW_TAP_TOKEN* ]] || fail 'GoReleaser receives tap token'
[[ "$goreleaser_step" != *APPLE_CERTIFICATE_PASSWORD* ]] || fail 'GoReleaser receives certificate password'
contains "$release" "COMPANION_CASK_PATH='dist/homebrew/Casks/auto.rb'"
contains "$release" 'COMPANION_CHECKSUMS_PATH: ${{ steps.release-evidence.outputs.checksums-path }}'
contains "$release" 'COMPANION_CHECKSUMS_PATH="$COMPANION_CHECKSUMS_PATH"'
not_contains "$release" "COMPANION_CHECKSUMS_PATH='dist/checksums.txt'"
contains "$producer" '--signing-key "$COMPANION_SIGNING_KEY_FILE"'
contains "$homebrew_bridge" "readonly PRIOR_TAP_COMMIT='8acf53e1bea9711ca3063c121b52e5d160f43b67'"
contains "$homebrew_bridge" "readonly PRIOR_CASK_BLOB='3f3a38e9a2ae556acc0f7d0974895d6189f266dd'"
contains "$homebrew_bridge" "readonly FROZEN_FORMULA_BLOB='4ebc6c38925002dec00759823d4dd847a499818a'"
contains "$homebrew_bridge" 'COMPANION_HOMEBREW_POLICY'
contains "$homebrew_bridge" "readonly FORMULA_PATH='Formula/auto.rb'"
contains "$homebrew_bridge" 'verify_frozen_formula'
not_contains "$homebrew_bridge" 'reconcile_tap_file formula Formula'
contains "$homebrew_bridge" "api_json POST 'git/blobs'"
contains "$homebrew_bridge" "api_json POST 'git/trees'"
contains "$homebrew_bridge" "api_json POST 'git/commits'"
contains "$homebrew_bridge" 'api_json PATCH "git/refs/heads/${TAP_BRANCH}"'
contains "$homebrew_bridge" '{base_tree:$base,tree:['
contains "$homebrew_bridge" "'{sha:\$sha,force:false}'"
not_contains "$homebrew_bridge" '--method PUT'

# Production/recovery source coordinates must bind to externally approved exact values.
for workflow in "$release" "$recovery"; do
  contains "$workflow" 'ADK_COMPANION_APPROVED_SOURCE_COMMIT'
  contains "$workflow" 'ADK_COMPANION_APPROVED_SOURCE_TREE'
  contains "$workflow" 'COMPANION_SOURCE_PIN_REQUIRED='
done
contains "$release" "- 'v0.50.81'"
contains "$release" "if: github.ref == 'refs/tags/v0.50.81'"
contains "$recovery" "if: github.ref == 'refs/tags/v0.50.81'"
contains "$recovery" 'gh workflow run homebrew-formula-bridge-recovery.yaml --ref v0.50.81'
not_contains "$release" "'v0.50.80'"
not_contains "$release" 'refs/tags/v0.50.80'
not_contains "$recovery" 'refs/tags/v0.50.80'
not_contains "$release" "'v0.50.79'"
not_contains "$release" 'refs/tags/v0.50.79'
not_contains "$recovery" 'refs/tags/v0.50.79'
not_contains "$release" "'v0.50.77'"
not_contains "$release" 'refs/tags/v0.50.77'
not_contains "$recovery" 'refs/tags/v0.50.77'
not_contains "$release" "'v0.50.75'"
not_contains "$release" 'refs/tags/v0.50.75'
not_contains "$recovery" 'refs/tags/v0.50.75'
not_contains "$release" "'v0.50.76'"
not_contains "$release" 'refs/tags/v0.50.76'
not_contains "$recovery" 'refs/tags/v0.50.76'
contains "$source_gate" "readonly A6_A5_ANCESTOR_SHA='b27252cb1148192a8ae1a95195c50e5f221453a4'"
contains "$source_gate" "readonly A7_A6_ANCESTOR_SHA='902f1acfa91f1d0a2ac9471d5cd79117031a2599'"
contains "$source_gate" "readonly A8_A7_ANCESTOR_SHA='51de6030a69a8e36fcf7e5790ef157eff6fedf00'"
contains "$source_gate" "readonly A9_A8_ANCESTOR_SHA='dd0c2759ed5435d4634011e349caad62ea3df414'"
contains "$source_gate" "readonly A10_A9_ANCESTOR_SHA='c9c4f49d48022eb0c8d72ee7b520136a4f21f176'"
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
contains "$lineage_pins" "A6_COMMIT_SHA='902f1acfa91f1d0a2ac9471d5cd79117031a2599'"
contains "$lineage_pins" "A6_TAG_OBJECT_SHA='41feed7decafac33d8f7f43e06804e3c9bf37ef3'"
contains "$lineage_pins" "A6_CHECKSUMS_SHA256='fb1a35dcdb44255aad43b7ae74950ed59f05ccf44abde9cadf28ecfa0dfce37a'"
contains "$lineage_pins" "A6_AMD64_ARCHIVE_SHA256='d5e47076c1fc898d2b3f5880b6edfcf9a12e805633dcba2691da22f300d41dc9'"
contains "$lineage_pins" "A6_ARM64_ARCHIVE_SHA256='d6d092177a5406c194eea1de4fbd11b8af92a03814eb143a294541a3a578b9ab'"
contains "$lineage_pins" "A6_AMD64_MANIFEST_SHA256='64c634130b16a74cbb33f666d316a05d9a7a1012246dc58fde6e15350b71d0c5'"
contains "$lineage_pins" "A6_ARM64_MANIFEST_SHA256='b6611c04990b048bc5545e37c942bc8e7e4fab8592d546eaab80d7084991bea6'"
contains "$lineage_pins" "A7_COMMIT_SHA='51de6030a69a8e36fcf7e5790ef157eff6fedf00'"
contains "$lineage_pins" "A7_TREE_SHA='3cd00b17bd8bd6aa8def213de1c5765c3611765d'"
contains "$lineage_pins" "A7_TAG_OBJECT_SHA='417a318fb6a11a720e2c4102e92e39ea9ed676e9'"
contains "$lineage_pins" "A7_CHECKSUMS_SHA256='322d2ef21dff55f02ca36944aba88ee5da92fdae6bcd16a89319f1697efb9733'"
contains "$lineage_pins" "A7_AMD64_ARCHIVE_SHA256='43018046ab37027b7fba3888d288961cb5abc136e478deaa9f878586bcce6629'"
contains "$lineage_pins" "A7_ARM64_ARCHIVE_SHA256='e72653fd3094537caa60398e2017d409796d7ceef88a7662ca93b6299e9d00ec'"
contains "$lineage_pins" "A7_AMD64_MANIFEST_SHA256='3f7c879c93dea0d119805987bef434b65c1a53684e80f78b5d9a0c9c2cd011d5'"
contains "$lineage_pins" "A7_ARM64_MANIFEST_SHA256='87ef2a30d6ee8c9abe9e679d597d0a4fbe9bb5cdee1266572476ad6a66aef975'"
contains "$lineage_pins" "A8_COMMIT_SHA='dd0c2759ed5435d4634011e349caad62ea3df414'"
contains "$lineage_pins" "A8_TREE_SHA='4325913ba332c583dd573ccf9248b38497d76926'"
contains "$lineage_pins" "A8_TAG_OBJECT_SHA='8c6dcef91407e3321704014559cfd929d14768d0'"
contains "$lineage_pins" "A8_CHECKSUMS_SHA256='1d0bdbfe50f85c381fde11c334c97a1b783dcfa4e12e0c4023152f38119a0bcd'"
contains "$lineage_pins" "A8_AMD64_ARCHIVE_SHA256='19e317cdabc9dde976ca772d9ddbbf693b444dd44eefa70c8d0313a32de89a9b'"
contains "$lineage_pins" "A8_ARM64_ARCHIVE_SHA256='41e29ae1c3c48dd6e3e5f4dfe8076472704d00a7d479b5cc8a90f53c0af6ef31'"
contains "$lineage_pins" "A8_AMD64_MANIFEST_SHA256='c5ac37874bac5de87152e781bd82a17c7705894f24be81657ccc907f15ba1f65'"
contains "$lineage_pins" "A8_ARM64_MANIFEST_SHA256='ebcf563c11f0836be2b2bd4423ea315283eeec12cfa200d479e1a56f5909f5f1'"
contains "$lineage_pins" "A9_COMMIT_SHA='c9c4f49d48022eb0c8d72ee7b520136a4f21f176'"
contains "$lineage_pins" "A9_TREE_SHA='3a71fa56bd917f447a6b1705772b6ab99bbcfbc8'"
contains "$lineage_pins" "A9_TAG_OBJECT_SHA='b7d05fa76eed41b1dfb4eddbd9873525e0aac15f'"
contains "$lineage_pins" "A9_CHECKSUMS_SHA256='9ed1f99d22a761abb7953c70aab3c7de5ab0b7ec3524cf3798fcd3815c53bde7'"
contains "$lineage_pins" "A9_AMD64_ARCHIVE_SHA256='48f80577ff2ef40a843dab0a847895ca7b3877e7fb810a30d328cbe8a55fc51e'"
contains "$lineage_pins" "A9_ARM64_ARCHIVE_SHA256='503c338e1ce122e209b9e74bc883492317144b319b0713943bc299e57447024d'"
contains "$lineage_pins" "A9_AMD64_MANIFEST_SHA256='589f02503aa02338ed14d67b1eb6b31e2b96a9e83b47c99e5cd5a31b75ede9b7'"
contains "$lineage_pins" "A9_ARM64_MANIFEST_SHA256='ffdd6ccbecff2b8ea38bc5c5f65ff7f078b229bd4658f90d08bb5e801c184a7f'"
contains "$lineage" "release_phase='A8' prior_phase='A7'"
contains "$lineage" "release_phase='A9' prior_phase='A8'"
contains "$lineage" "release_phase='A10' prior_phase='A9'"
contains "$lineage" '.commit.tree.sha'
contains "$producer" "GITHUB_REF_NAME\" == 'v0.50.81'"
contains "$producer" "release_phase='A10'"
contains "$homebrew_bridge" "readonly RELEASE_TAG='v0.50.81'"
contains "$homebrew_bridge" "readonly RELEASE_VERSION='0.50.81'"
contains "$release" 'timeout-minutes: 60'
contains "$recovery" 'timeout-minutes: 20'

# Production and recovery must share one exact, fail-closed current-release gate.
for workflow in "$release" "$recovery"; do
  contains "$workflow" 'scripts/companion-release/verify-current-release.sh'
  workflow_evidence_index=$(grep -n 'scripts/companion-release/verify-current-release.sh' "$workflow" | cut -d: -f1)
  workflow_token_index=$(grep -n 'name: Create Homebrew tap token' "$workflow" | cut -d: -f1)
  (( workflow_evidence_index < workflow_token_index )) || fail 'tap token precedes release evidence'
done
contains "$current_release_gate" "readonly RELEASE_TAG='v0.50.81'"
contains "$current_release_gate" "readonly RELEASE_VERSION='0.50.81'"
contains "$current_release_gate" '.target_commitish == $commit'
contains "$current_release_gate" '.immutable == true'
contains "$current_release_gate" '(.assets | length) == ($expected | length)'
contains "$current_release_gate" '[.assets[].name] | unique | length'
contains "$current_release_gate" '.state == "uploaded"'
contains "$current_release_gate" '.size > 0'
contains "$current_release_gate" '^sha256:[0-9a-f]{64}$'
contains "$current_release_gate" 'for archive in "${EXPECTED_ARCHIVES[@]}"'

temp=$(mktemp -d "${TMPDIR:-/tmp}/release-hardening-test.XXXXXX")
trap 'rm -rf -- "$temp"' EXIT
git clone -q --no-hardlinks --no-tags "$repo" "$temp/source"
git -C "$temp/source" config user.name 'Release Test'
git -C "$temp/source" config user.email release-test@example.invalid
git -C "$temp/source" tag -am 'A10 fixture' v0.50.81
commit=$(git -C "$temp/source" rev-parse HEAD)
tree=$(git -C "$temp/source" rev-parse 'HEAD^{tree}')
if [[ "${tree: -1}" == '0' ]]; then
  wrong_tree="${tree%?}1"
else
  wrong_tree="${tree%?}0"
fi
run_source_gate() {
  local approved_commit="${1-}" approved_tree="${2-}"
  env GITHUB_REF_NAME=v0.50.81 GITHUB_REF_TYPE=tag GITHUB_SHA="$commit" \
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

bash "$tests_dir/release-homebrew-hardening-test.sh"

printf 'release hardening test: PASS\n'
