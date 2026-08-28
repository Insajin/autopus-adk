#!/usr/bin/env bash
set -euo pipefail
umask 077

fail() { printf 'OMP context bridge manifest: %s\n' "$1" >&2; exit 1; }
[[ $# -eq 1 ]] || fail 'usage: produce-omp-context-bridge-manifest.sh OUTPUT'
readonly output=$1
readonly repository='Insajin/autopus-adk'
readonly release_tag='v0.50.109'
readonly release_mode='canonical-full-bridge'
readonly rotation_ref='refs/heads/release-key-rotation-v0.50.109'
readonly promotion_key_id='omp-context-promotion-2026-q3-k3'
readonly promotion_public_sha256='2a9b41dec1330f65937d9b25b20967cb29fd9209c722ce5fe1a9afd6ca45b937'
readonly expected_k3_public='YkTuNcfWGTLgTglPmZq/Dj4OXwcoUwnkM2ExIGIz+jM='

for name in OMP_CONTEXT_STATIC_POLICY_B64 OMP_CONTEXT_PROMOTION_REPORT_PATH \
  OMP_CONTEXT_PROMOTION_ATTESTATION_PATH OMP_CONTEXT_EVIDENCE_REPORT_SHA256 \
  OMP_CONTEXT_EVIDENCE_ATTESTATION_SHA256
do
  [[ "${!name+x}" != x ]] || fail "bridge manifest forbids ${name}"
done
[[ "${COMPANION_RELEASE_TAG:-}" == "$release_tag" ]] || fail 'release tag is not exact A22'
[[ "${COMPANION_SOURCE_COMMIT:-}" =~ ^[0-9a-f]{40}$ ]] || fail 'source commit is malformed'
[[ "${COMPANION_SOURCE_TREE:-}" =~ ^[0-9a-f]{40}$ ]] || fail 'source tree is malformed'
[[ "${OMP_CONTEXT_CANDIDATE_ARTIFACT_SHA256:-}" =~ ^[0-9a-f]{64}$ ]] || fail 'candidate digest is malformed'
[[ "${ADK_KEY_ROTATION_DOCUMENT_SHA256:-}" =~ ^[0-9a-f]{64}$ ]] || fail 'rotation document digest is malformed'
[[ "${ADK_KEY_ROTATION_REF_COMMIT:-}" =~ ^[0-9a-f]{40}$ ]] || fail 'rotation ref commit is malformed'
[[ ! -e "$output" && ! -L "$output" ]] || fail 'output already exists'
for tool in awk jq mktemp openssl shasum tr wc; do
  command -v "$tool" >/dev/null || fail "$tool is unavailable"
done

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd) || fail 'cannot resolve script directory'
k3_pin="$script_dir/omp-context-promotion-2026-q3-k3.pub"
[[ -f "$k3_pin" && ! -L "$k3_pin" ]] || fail 'K3 public pin is missing or unsafe'
IFS= read -r k3_public <"$k3_pin" || fail 'cannot read K3 public pin'
[[ "$k3_public" == "$expected_k3_public" && "$(wc -l <"$k3_pin" | tr -d ' ')" == '1' ]] || fail 'K3 public pin differs'
raw_key=$(mktemp "${TMPDIR:-/tmp}/omp-context-k3.XXXXXX") || fail 'cannot stage K3 public key'
trap 'rm -f -- "$raw_key"' EXIT
printf '%s' "$k3_public" | openssl base64 -d -A >"$raw_key" || fail 'K3 public pin is not base64'
[[ "$(wc -c <"$raw_key" | tr -d ' ')" == '32' &&
   "$(shasum -a 256 "$raw_key" | awk '{print $1}')" == "$promotion_public_sha256" ]] || fail 'K3 public pin digest differs'

jq -cnS \
  --arg schema_version 'omp-context-bridge-release.v1' \
  --arg repository "$repository" --arg release_mode "$release_mode" \
  --arg release_tag "$release_tag" --arg source_commit "$COMPANION_SOURCE_COMMIT" \
  --arg source_tree "$COMPANION_SOURCE_TREE" \
  --arg candidate_sha256 "$OMP_CONTEXT_CANDIDATE_ARTIFACT_SHA256" \
  --arg rotation_ref "$rotation_ref" \
  --arg rotation_ref_commit "$ADK_KEY_ROTATION_REF_COMMIT" \
  --arg rotation_document_sha256 "$ADK_KEY_ROTATION_DOCUMENT_SHA256" \
  --arg promotion_key_id "$promotion_key_id" \
  --arg promotion_public_sha256 "$promotion_public_sha256" \
  '{schema_version:$schema_version,repository:$repository,release_mode:$release_mode,
    release_tag:$release_tag,source_commit:$source_commit,source_tree:$source_tree,
    candidate_artifact_sha256:$candidate_sha256,rotation_ref:$rotation_ref,
    rotation_ref_commit:$rotation_ref_commit,
    rotation_document_sha256:$rotation_document_sha256,
    promotion_key_id:$promotion_key_id,
    promotion_public_sha256:$promotion_public_sha256}' >"$output" || fail 'cannot write bridge manifest'
chmod 0600 "$output"
[[ "$(wc -l <"$output" | tr -d ' ')" == '1' ]] || fail 'bridge manifest is not canonical'
