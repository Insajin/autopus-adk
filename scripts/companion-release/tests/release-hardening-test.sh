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
producer_receipt="$script_dir/produce-public-key-receipt.sh"
homebrew_bridge="$script_dir/publish-homebrew-formula-bridge.sh"
homebrew_git_helper="$script_dir/publish-homebrew-formula-bridge-git.sh"
current_release_gate="$script_dir/verify-current-release.sh"
current_signature_gate="$script_dir/verify-current-release-signatures.sh"

# GoReleaser must render, but never publish, the Cask or mutate tagged source.
contains "$config" 'skip_upload: true'
not_contains "$config" 'token: "{{ .Env.HOMEBREW_TAP_TOKEN }}"'
not_contains "$config" 'go mod tidy'
contains "$release" 'go mod tidy -diff'

# A fresh, narrowly scoped tap token must be created after immutable release publish.
release_index=$(grep -n 'goreleaser release --clean' "$release" | cut -d: -f1)
signing_cleanup_index=$(grep -n 'name: Remove release signing credentials' "$release" | cut -d: -f1)
evidence_index=$(grep -n 'scripts/companion-release/verify-current-release.sh' "$release" | cut -d: -f1)
token_index=$(grep -n 'name: Create Homebrew tap token' "$release" | cut -d: -f1)
bridge_index=$(grep -n 'scripts/companion-release/publish-homebrew-formula-bridge.sh' "$release" | cut -d: -f1)
(( release_index < signing_cleanup_index && signing_cleanup_index < evidence_index && \
   evidence_index < token_index && token_index < bridge_index )) \
  || fail 'release evidence or tap token ordering is unsafe'
goreleaser_step=$(sed -n '/name: Run GoReleaser/,/name: Verify current immutable release evidence/p' "$release")
[[ "$goreleaser_step" != *HOMEBREW_TAP_TOKEN* ]] || fail 'GoReleaser receives tap token'
[[ "$goreleaser_step" != *APPLE_CERTIFICATE_PASSWORD* ]] || fail 'GoReleaser receives certificate password'
contains "$release" "COMPANION_CASK_PATH='dist/homebrew/Casks/auto.rb'"
contains "$release" 'COMPANION_CHECKSUMS_PATH: ${{ steps.release-evidence.outputs.checksums-path }}'
contains "$release" 'COMPANION_CHECKSUMS_PATH="$COMPANION_CHECKSUMS_PATH"'
not_contains "$release" "COMPANION_CHECKSUMS_PATH='dist/checksums.txt'"
contains "$producer_receipt" '--signing-key "$COMPANION_SIGNING_KEY_FILE"'
contains "$homebrew_bridge" "readonly PRIOR_TAP_COMMIT='bb84d874af4c9187603f36c3ca06460c90b7caea'"
contains "$homebrew_bridge" "readonly PRIOR_CASK_BLOB='f9baefd8723dad6afb3d60999bde44d3913ecb10'"
contains "$homebrew_bridge" "readonly FROZEN_FORMULA_BLOB='4ebc6c38925002dec00759823d4dd847a499818a'"
contains "$homebrew_bridge" 'COMPANION_HOMEBREW_POLICY'
contains "$homebrew_bridge" "readonly FORMULA_PATH='Formula/auto.rb'"
contains "$homebrew_bridge" 'source "$git_helper"'
contains "$homebrew_git_helper" 'verify_frozen_formula'
not_contains "$homebrew_git_helper" 'reconcile_tap_file formula Formula'
contains "$homebrew_git_helper" "api_json POST 'git/blobs'"
contains "$homebrew_git_helper" "api_json POST 'git/trees'"
contains "$homebrew_git_helper" "api_json POST 'git/commits'"
contains "$homebrew_git_helper" 'api_json PATCH "git/refs/heads/${TAP_BRANCH}"'
contains "$homebrew_git_helper" '{base_tree:$base,tree:['
contains "$homebrew_git_helper" "'{sha:\$sha,force:false}'"
not_contains "$homebrew_git_helper" '--method PUT'

# Production/recovery source coordinates must bind to externally approved exact values.
for workflow in "$release" "$recovery"; do
  contains "$workflow" 'ADK_COMPANION_APPROVED_SOURCE_COMMIT'
  contains "$workflow" 'ADK_COMPANION_APPROVED_SOURCE_TREE'
  contains "$workflow" 'COMPANION_SOURCE_PIN_REQUIRED='
done
contains "$release" "- 'v0.50.87'"
contains "$release" "if: github.ref == 'refs/tags/v0.50.87'"
contains "$recovery" "if: github.ref == 'refs/tags/v0.50.87'"
contains "$recovery" 'gh workflow run homebrew-formula-bridge-recovery.yaml --ref v0.50.87'
not_contains "$release" "'v0.50.86'"
not_contains "$release" 'refs/tags/v0.50.86'
not_contains "$recovery" 'refs/tags/v0.50.86'
not_contains "$release" "'v0.50.85'"
not_contains "$release" 'refs/tags/v0.50.85'
not_contains "$recovery" 'refs/tags/v0.50.85'
not_contains "$release" "'v0.50.83'"
not_contains "$release" 'refs/tags/v0.50.83'
not_contains "$recovery" 'refs/tags/v0.50.83'
not_contains "$release" "'v0.50.82'"
not_contains "$release" 'refs/tags/v0.50.82'
not_contains "$recovery" 'refs/tags/v0.50.82'
not_contains "$release" "'v0.50.81'"
not_contains "$release" 'refs/tags/v0.50.81'
not_contains "$recovery" 'refs/tags/v0.50.81'
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
contains "$producer_receipt" "GITHUB_REF_NAME\" == 'v0.50.87'"
contains "$producer_receipt" "release_phase='A16'"
contains "$homebrew_bridge" "readonly RELEASE_TAG='v0.50.87'"
contains "$homebrew_bridge" "readonly RELEASE_VERSION='0.50.87'"
contains "$release" 'timeout-minutes: 60'
contains "$recovery" 'timeout-minutes: 20'

# Production and recovery must share one exact, fail-closed current-release gate.
for workflow in "$release" "$recovery"; do
  contains "$workflow" 'scripts/companion-release/verify-current-release.sh'
  workflow_evidence_index=$(grep -n 'scripts/companion-release/verify-current-release.sh' "$workflow" | cut -d: -f1)
  workflow_token_index=$(grep -n 'name: Create Homebrew tap token' "$workflow" | cut -d: -f1)
  (( workflow_evidence_index < workflow_token_index )) || fail 'tap token precedes release evidence'
done
contains "$current_release_gate" "readonly RELEASE_TAG='v0.50.87'"
contains "$current_release_gate" "readonly RELEASE_VERSION='0.50.87'"
contains "$current_release_gate" '.target_commitish == $commit'
contains "$current_release_gate" '.immutable == true'
contains "$current_release_gate" '(.assets | length) == ($expected | length)'
contains "$current_release_gate" '[.assets[].name] | unique | length'
contains "$current_release_gate" '.state == "uploaded"'
contains "$current_release_gate" '.size > 0'
contains "$current_release_gate" '^sha256:[0-9a-f]{64}$'
contains "$current_release_gate" 'for archive in "${EXPECTED_ARCHIVES[@]}"'
contains "$current_release_gate" 'verify-current-release-signatures.sh'
contains "$current_release_gate" 'env -i PATH="$PATH" HOME="${HOME:-/}" TMPDIR="${TMPDIR:-/tmp}"'
contains "$current_release_gate" "download_release_asset 'checksums.txt.bundle'"
contains "$current_release_gate" "download_release_asset 'checksums.txt.signatures'"
contains "$current_signature_gate" 'verify_release_checksums_v1'
contains "$current_signature_gate" 'cosign verify-blob'
contains "$current_signature_gate" 'unset GITHUB_TOKEN GH_TOKEN'
for workflow in "$release" "$recovery"; do
  contains "$workflow" 'sigstore/cosign-installer@6f9f17788090df1f26f669e9d70d6ae9567deba6'
  contains "$workflow" "cosign-release: 'v3.1.2'"
  not_contains "$workflow" 'sigstore/cosign-installer@59acb6260d9c0ba8f4a2f9d9b48431a222b68e20'
done

bash "$tests_dir/release-runtime-hardening-test.sh"
bash "$tests_dir/release-exec-smoke-hardening-test.sh"
bash "$tests_dir/release-homebrew-hardening-test.sh"
bash "$tests_dir/release-producer-helper-hardening-test.sh"
bash "$tests_dir/release-current-signature-hardening-test.sh"
bash "$tests_dir/release-lineage-pins-hardening-test.sh"

printf 'release hardening test: PASS\n'
