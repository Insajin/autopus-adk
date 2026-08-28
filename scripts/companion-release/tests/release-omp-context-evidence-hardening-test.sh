#!/usr/bin/env bash
set -euo pipefail

root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../../.." && pwd)
workflow="$root/.github/workflows/release.yaml"
config="$root/.goreleaser.yaml"
current="$root/scripts/companion-release/verify-current-release.sh"
producer="$root/scripts/companion-release/produce-omp-context-bridge-manifest.sh"
binary_gate="$root/scripts/companion-release/verify-omp-context-release-binary.sh"
candidate_builder="$root/scripts/companion-release/build-omp-context-candidate.sh"
temp=$(mktemp -d "${TMPDIR:-/tmp}/omp-context-bridge-test.XXXXXX")
trap 'rm -rf -- "$temp"' EXIT
fail() { printf 'canonical bridge hardening: %s\n' "$1" >&2; exit 1; }

[[ ! -e "$root/scripts/companion-release/verify-omp-context-evidence-tag.sh" ]] ||
  fail 'A22 promotion evidence tag reader still exists'
for required in \
  'omp-canonical-bridge-candidate:' \
  'needs: [ci, security, omp-canonical-bridge-candidate]' \
  'canonical-full-bridge' \
  'omp-context-bridge-release.v1.json' \
  'verify-key-rotation-sidecar.sh'
do
  grep -Fq -- "$required" "$workflow" || fail "workflow missing $required"
done
for forbidden in \
  'omp-context-evidence-v0.50.109' \
  'OMP_CONTEXT_STATIC_POLICY_B64' \
  'OMP_CONTEXT_EVIDENCE_' \
  'omp-context-promotion-report.v1.json' \
  'omp-context-promotion-attestation.v2.json' \
  '--mode active' '--mode historical'
do
  if grep -Fq -- "$forbidden" "$workflow" "$config"; then
    fail "release authority contains forbidden promotion wiring $forbidden"
  fi
done
grep -Fq 'OMP_CONTEXT_BRIDGE_MANIFEST_PATH' "$config" || fail 'GoReleaser bridge manifest is absent'
if grep -Fq 'pipelineOMPActiveStaticPolicyB64' "$config"; then
  fail 'GoReleaser still compiles an active static policy'
fi
grep -Fq "expected_go_toolchain='go1.26.6'" "$candidate_builder" ||
  fail 'bridge candidate builder does not pin Go 1.26.6'
grep -Fq 'GOTOOLCHAIN="$expected_go_toolchain"' "$candidate_builder" ||
  fail 'bridge candidate build does not execute with the exact toolchain'
grep -Fq 'GOENV=off' "$candidate_builder" ||
  fail 'bridge candidate build does not disable ambient Go env'
grep -Fq 'exactly sixteen A22 canonical-full bridge assets' "$current" ||
  fail 'immutable verifier does not require base assets plus bridge manifest and signed rotation pair'

manifest="$temp/omp-context-bridge-release.v1.json"
env -i PATH="$PATH" HOME="${HOME:-/}" TMPDIR="$temp" \
  COMPANION_RELEASE_TAG=v0.50.109 \
  COMPANION_SOURCE_COMMIT="$(printf 'c%.0s' {1..40})" \
  COMPANION_SOURCE_TREE="$(printf 'd%.0s' {1..40})" \
  OMP_CONTEXT_CANDIDATE_ARTIFACT_SHA256="$(printf 'a%.0s' {1..64})" \
  ADK_KEY_ROTATION_REF_COMMIT="$(printf 'f%.0s' {1..40})" \
  ADK_KEY_ROTATION_DOCUMENT_SHA256="$(printf 'b%.0s' {1..64})" \
  bash "$producer" "$manifest"
jq -e '
  .schema_version == "omp-context-bridge-release.v1" and
  .release_mode == "canonical-full-bridge" and
  .release_tag == "v0.50.109" and
  .promotion_key_id == "omp-context-promotion-2026-q3-k3" and
  .promotion_public_sha256 == "2a9b41dec1330f65937d9b25b20967cb29fd9209c722ce5fe1a9afd6ca45b937" and
  (has("static_policy") | not) and (has("active") | not)
' "$manifest" >/dev/null || fail 'bridge manifest claims are invalid'

if env -i PATH="$PATH" HOME="${HOME:-/}" TMPDIR="$temp" \
  COMPANION_RELEASE_TAG=v0.50.109 OMP_CONTEXT_STATIC_POLICY_B64=forbidden \
  bash "$binary_gate" >/dev/null 2>&1; then
  fail 'release binary gate accepted a static policy claim'
fi
printf 'canonical bridge hardening: ok\n'
