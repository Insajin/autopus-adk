#!/usr/bin/env bash

[[ "${BASH_SOURCE[0]}" != "$0" ]] || {
  printf 'companion release: public key receipt helper must be sourced\n' >&2
  exit 1
}

# @AX:ANCHOR [AUTO]: Keep every shipped tag/version pair in one exact receipt-phase resolver.
# @AX:REASON [AUTO]: The producer must reject mixed or unknown release coordinates before signing artifacts.
resolve_public_key_receipt_release_phase() {
  if [[ "$GITHUB_REF_NAME" == 'v0.50.69' && "$COMPANION_VERSION" == '0.50.69' ]]; then
    release_phase='A0'
  elif [[ "$GITHUB_REF_NAME" == 'v0.50.70' && "$COMPANION_VERSION" == '0.50.70' ]]; then
    release_phase='A1'
  elif [[ "$GITHUB_REF_NAME" == 'v0.50.71' && "$COMPANION_VERSION" == '0.50.71' ]]; then
    release_phase='A2'
  elif [[ "$GITHUB_REF_NAME" == 'v0.50.72' && "$COMPANION_VERSION" == '0.50.72' ]]; then
    release_phase='A3'
  elif [[ "$GITHUB_REF_NAME" == 'v0.50.73' && "$COMPANION_VERSION" == '0.50.73' ]]; then
    release_phase='A4'
  elif [[ "$GITHUB_REF_NAME" == 'v0.50.74' && "$COMPANION_VERSION" == '0.50.74' ]]; then
    release_phase='A5'
  elif [[ "$GITHUB_REF_NAME" == 'v0.50.77' && "$COMPANION_VERSION" == '0.50.77' ]]; then
    release_phase='A6'
  elif [[ "$GITHUB_REF_NAME" == 'v0.50.78' && "$COMPANION_VERSION" == '0.50.78' ]]; then
    release_phase='A7'
  elif [[ "$GITHUB_REF_NAME" == 'v0.50.79' && "$COMPANION_VERSION" == '0.50.79' ]]; then
    release_phase='A8'
  elif [[ "$GITHUB_REF_NAME" == 'v0.50.80' && "$COMPANION_VERSION" == '0.50.80' ]]; then
    release_phase='A9'
  elif [[ "$GITHUB_REF_NAME" == 'v0.50.81' && "$COMPANION_VERSION" == '0.50.81' ]]; then
    release_phase='A10'
  elif [[ "$GITHUB_REF_NAME" == 'v0.50.82' && "$COMPANION_VERSION" == '0.50.82' ]]; then
    release_phase='A11'
  elif [[ "$GITHUB_REF_NAME" == 'v0.50.83' && "$COMPANION_VERSION" == '0.50.83' ]]; then
    release_phase='A12'
  elif [[ "$GITHUB_REF_NAME" == 'v0.50.84' && "$COMPANION_VERSION" == '0.50.84' ]]; then
    release_phase='A13'
  elif [[ "$GITHUB_REF_NAME" == 'v0.50.85' && "$COMPANION_VERSION" == '0.50.85' ]]; then
    release_phase='A14'
  elif [[ "$GITHUB_REF_NAME" == 'v0.50.86' && "$COMPANION_VERSION" == '0.50.86' ]]; then
    release_phase='A15'
  else
    fail 'public_key_receipt_release_identity_mismatch'
  fi
}

produce_public_key_receipt_bundle() {
  local artifact_path=$1
  local manifest_path=$2
  local signature_path=$3
  local public_key_bundle_path=$4
  local signing_key_digest_before=$5
  local release_phase=$6
  local bundle_entry_count entry signing_key_digest_after
  local -a public_key_receipt_args

  public_key_receipt_args=(companion-manifest public-key-receipt
    --key-file "$COMPANION_SIGNING_KEY_FILE"
    --bundle-output "$public_key_bundle_path"
    --key-id "$COMPANION_KEY_ID"
    --issued-at "$COMPANION_PUBLIC_KEY_RECEIPT_ISSUED_AT"
    --expires-at "$COMPANION_PUBLIC_KEY_RECEIPT_EXPIRES_AT"
    --handoff "$COMPANION_HANDOFF"
    --minimum-rollback-floor "$COMPANION_ROLLBACK_FLOOR")
  env -i PATH="$PATH" HOME="${HOME-}" TMPDIR="${TMPDIR:-/tmp}" \
    "$COMPANION_SIGNER" "${public_key_receipt_args[@]}" >/dev/null \
    || fail 'public key receipt production failed'
  [[ -d "$public_key_bundle_path" && ! -L "$public_key_bundle_path" ]] \
    || fail 'public key receipt production failed'
  bundle_entry_count=$(find "$public_key_bundle_path" -mindepth 1 -maxdepth 1 -print | wc -l)
  [[ "${bundle_entry_count//[[:space:]]/}" == '2' ]] \
    || fail 'public key receipt production failed'
  for entry in public-key-receipt.json public-key-receipt.sig; do
    [[ -f "$public_key_bundle_path/$entry" && ! -L "$public_key_bundle_path/$entry" ]] \
      || fail 'public key receipt production failed'
  done
  [[ "$(wc -c <"$public_key_bundle_path/public-key-receipt.sig" | tr -d '[:space:]')" == '64' ]] \
    || fail 'public key receipt production failed'
  env -i PATH="$PATH" HOME="${HOME-}" TMPDIR="${TMPDIR:-/tmp}" \
    "$COMPANION_RECEIPT_VERIFIER" \
    --receipt "$public_key_bundle_path/public-key-receipt.json" \
    --signature "$public_key_bundle_path/public-key-receipt.sig" \
    --signing-key "$COMPANION_SIGNING_KEY_FILE" \
    --key-id "$COMPANION_KEY_ID" \
    --issued-at "$COMPANION_PUBLIC_KEY_RECEIPT_ISSUED_AT" \
    --expires-at "$COMPANION_PUBLIC_KEY_RECEIPT_EXPIRES_AT" \
    --handoff "$COMPANION_HANDOFF" \
    --minimum-rollback-floor "$COMPANION_ROLLBACK_FLOOR" \
    || fail 'public key receipt independent verification failed'
  if [[ -n "${COMPANION_MANIFEST_VERIFIER-}" ]]; then
    env -i PATH="$PATH" HOME="${HOME-}" TMPDIR="${TMPDIR:-/tmp}" \
      "$COMPANION_MANIFEST_VERIFIER" \
      --artifact "$artifact_path" \
      --manifest "$manifest_path" \
      --signature "$signature_path" \
      --receipt "$public_key_bundle_path/public-key-receipt.json" \
      --receipt-signature "$public_key_bundle_path/public-key-receipt.sig" \
      --signing-key "$COMPANION_SIGNING_KEY_FILE" \
      --key-id "$COMPANION_KEY_ID" \
      --version "$COMPANION_VERSION" \
      --platform "$COMPANION_PLATFORM" \
      --architecture "$COMPANION_ARCHITECTURE" \
      --handoff "$COMPANION_HANDOFF" \
      --minimum-rollback-floor "$COMPANION_ROLLBACK_FLOOR" \
      || fail 'manifest and artifact independent verification failed'
  fi
  signing_key_digest_after=$(sha256_file "$COMPANION_SIGNING_KEY_FILE") \
    || fail 'manifest_public_key_digest_mismatch'
  [[ "$signing_key_digest_before" == "$signing_key_digest_after" ]] \
    || fail 'manifest_public_key_digest_mismatch'
  printf 'companion release: public key receipt phase %s produced\n' "$release_phase"
}
