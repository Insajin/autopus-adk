#!/usr/bin/env bash
set -euo pipefail

fail() {
  printf 'companion release: %s\n' "$1" >&2
  exit 1
}

require_environment() {
  local name="$1"
  [[ -n "${!name-}" ]] || fail "required environment variable ${name} is missing"
}

secure_regular_file() {
  local path="$1"
  local label="$2"
  [[ -f "$path" && ! -L "$path" && -r "$path" ]] || fail "${label} file is invalid"
  local permissions owner
  permissions=$(stat -c '%a' "$path" 2>/dev/null || stat -f '%Lp' "$path" 2>/dev/null) \
    || fail "cannot inspect ${label} file permissions"
  owner=$(stat -c '%u' "$path" 2>/dev/null || stat -f '%u' "$path" 2>/dev/null) \
    || fail "cannot inspect ${label} file owner"
  [[ "$permissions" == '600' && "$owner" == "$(id -u)" ]] \
    || fail "${label} file must be owner-only mode 0600"
}

timestamp_epoch() {
  local value="$1"
  if [[ "$(uname -s)" == 'Darwin' ]]; then
    date -j -u -f '%Y-%m-%dT%H:%M:%SZ' "$value" '+%s' 2>/dev/null
  else
    date -u -d "$value" '+%s' 2>/dev/null
  fi
}

for name in \
  COMPANION_BUILD_PROVENANCE \
  COMPANION_HANDOFF \
  COMPANION_ROLLBACK_FLOOR \
  COMPANION_ISSUED_AT \
  COMPANION_EXPIRES_AT \
  COMPANION_KEY_ID \
  COMPANION_SIGNING_KEY_FILE \
  COMPANION_SIGNER \
  COMPANION_EXEC_SMOKE_GATE \
  APPLE_SIGNING_IDENTITY \
  APPLE_API_KEY \
  APPLE_API_ISSUER \
  APPLE_API_KEY_PATH
do
  require_environment "$name"
done

slug_pattern='^[A-Za-z0-9][A-Za-z0-9._:@/+_-]{0,255}$'
[[ "$COMPANION_KEY_ID" =~ $slug_pattern ]] || fail 'COMPANION_KEY_ID is invalid'
[[ "$COMPANION_HANDOFF" =~ $slug_pattern ]] || fail 'COMPANION_HANDOFF is invalid'
[[ "$COMPANION_ROLLBACK_FLOOR" =~ ^[0-9]+$ ]] || fail 'COMPANION_ROLLBACK_FLOOR is invalid'

provenance_pattern='^[A-Za-z0-9][A-Za-z0-9._:@/+_-]*$'
[[ ${#COMPANION_BUILD_PROVENANCE} -le 512 ]] || fail 'COMPANION_BUILD_PROVENANCE is invalid'
[[ "$COMPANION_BUILD_PROVENANCE" =~ $provenance_pattern ]] || fail 'COMPANION_BUILD_PROVENANCE is invalid'

timestamp_pattern='^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$'
[[ "$COMPANION_ISSUED_AT" =~ $timestamp_pattern ]] || fail 'COMPANION_ISSUED_AT is invalid'
[[ "$COMPANION_EXPIRES_AT" =~ $timestamp_pattern ]] || fail 'COMPANION_EXPIRES_AT is invalid'
[[ "$COMPANION_ISSUED_AT" < "$COMPANION_EXPIRES_AT" ]] || fail 'companion validity window is invalid'

[[ "$APPLE_SIGNING_IDENTITY" == 'Developer ID Application: '*'(GP2PFA2PUV)' ]] \
  || fail 'APPLE_SIGNING_IDENTITY is not the expected Developer ID team'
[[ "$APPLE_SIGNING_IDENTITY" != *$'\n'* && "$APPLE_SIGNING_IDENTITY" != '-' ]] \
  || fail 'APPLE_SIGNING_IDENTITY is invalid'
[[ "$APPLE_API_KEY" =~ ^[A-Za-z0-9]{1,64}$ ]] || fail 'APPLE_API_KEY is invalid'
issuer_pattern='^[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{12}$'
[[ "$APPLE_API_ISSUER" =~ $issuer_pattern ]] || fail 'APPLE_API_ISSUER is invalid'

secure_regular_file "$COMPANION_SIGNING_KEY_FILE" 'companion signing key'
secure_regular_file "$APPLE_API_KEY_PATH" 'Apple API key'
[[ -f "$COMPANION_SIGNER" && ! -L "$COMPANION_SIGNER" && -x "$COMPANION_SIGNER" ]] \
  || fail 'COMPANION_SIGNER is not a regular executable'
exec_smoke_label='companion execution smoke gate'
[[ -f "$COMPANION_EXEC_SMOKE_GATE" && ! -L "$COMPANION_EXEC_SMOKE_GATE" &&
   -x "$COMPANION_EXEC_SMOKE_GATE" ]] \
  || fail "${exec_smoke_label} is not a regular executable"
if [[ -n "${COMPANION_MANIFEST_VERIFIER-}" ]]; then
  [[ -f "$COMPANION_MANIFEST_VERIFIER" && ! -L "$COMPANION_MANIFEST_VERIFIER" &&
     -x "$COMPANION_MANIFEST_VERIFIER" ]] \
    || fail 'COMPANION_MANIFEST_VERIFIER is not a regular executable'
fi

receipt_policy_present=0
for name in \
  COMPANION_PUBLIC_KEY_RECEIPT_ISSUED_AT \
  COMPANION_PUBLIC_KEY_RECEIPT_EXPIRES_AT \
  COMPANION_PUBLIC_KEY_RECEIPT_MINIMUM_LIFETIME_SECONDS
do
  if [[ -n "${!name-}" ]]; then
    receipt_policy_present=1
  fi
done

if [[ "${COMPANION_RELEASE_PRODUCTION-}" == '1' ]]; then
  for name in \
    APPLE_SIGNING_KEYCHAIN \
    COMPANION_MANIFEST_VERIFIER \
    COMPANION_PUBLIC_KEY_RECEIPT_ISSUED_AT \
    COMPANION_PUBLIC_KEY_RECEIPT_EXPIRES_AT \
    COMPANION_PUBLIC_KEY_RECEIPT_MINIMUM_LIFETIME_SECONDS
  do
    require_environment "$name"
  done
  [[ "$(uname -s)" == 'Darwin' ]] || fail 'production Darwin release requires macOS'
  secure_regular_file "$APPLE_SIGNING_KEYCHAIN" 'Apple signing keychain'
  [[ -f "$COMPANION_MANIFEST_VERIFIER" && ! -L "$COMPANION_MANIFEST_VERIFIER" &&
     -x "$COMPANION_MANIFEST_VERIFIER" ]] \
    || fail 'COMPANION_MANIFEST_VERIFIER is not a regular executable'
  receipt_policy_present=1
fi

if [[ "$receipt_policy_present" == '1' ]]; then
  require_environment COMPANION_RECEIPT_VERIFIER
  [[ -f "$COMPANION_RECEIPT_VERIFIER" && ! -L "$COMPANION_RECEIPT_VERIFIER" &&
     -x "$COMPANION_RECEIPT_VERIFIER" ]] \
    || fail 'COMPANION_RECEIPT_VERIFIER is not a regular executable'
  for name in \
    COMPANION_PUBLIC_KEY_RECEIPT_ISSUED_AT \
    COMPANION_PUBLIC_KEY_RECEIPT_EXPIRES_AT \
    COMPANION_PUBLIC_KEY_RECEIPT_MINIMUM_LIFETIME_SECONDS
  do
    require_environment "$name"
  done
  [[ "$COMPANION_PUBLIC_KEY_RECEIPT_MINIMUM_LIFETIME_SECONDS" == '31536000' ]] \
    || fail 'receipt_window_not_long_lived'
  [[ "$COMPANION_PUBLIC_KEY_RECEIPT_ISSUED_AT" =~ $timestamp_pattern &&
     "$COMPANION_PUBLIC_KEY_RECEIPT_EXPIRES_AT" =~ $timestamp_pattern ]] \
    || fail 'receipt_window_not_long_lived'
  receipt_issued_epoch=$(timestamp_epoch "$COMPANION_PUBLIC_KEY_RECEIPT_ISSUED_AT") \
    || fail 'receipt_window_not_long_lived'
  receipt_expires_epoch=$(timestamp_epoch "$COMPANION_PUBLIC_KEY_RECEIPT_EXPIRES_AT") \
    || fail 'receipt_window_not_long_lived'
  manifest_issued_epoch=$(timestamp_epoch "$COMPANION_ISSUED_AT") \
    || fail 'manifest_window_outside_receipt'
  manifest_expires_epoch=$(timestamp_epoch "$COMPANION_EXPIRES_AT") \
    || fail 'manifest_window_outside_receipt'
  receipt_lifetime=$((receipt_expires_epoch - receipt_issued_epoch))
  (( receipt_lifetime >= COMPANION_PUBLIC_KEY_RECEIPT_MINIMUM_LIFETIME_SECONDS )) \
    || fail 'receipt_window_not_long_lived'
  (( manifest_issued_epoch >= receipt_issued_epoch &&
     manifest_expires_epoch <= receipt_expires_epoch )) \
    || fail 'manifest_window_outside_receipt'
fi

time_validation_required="${COMPANION_RELEASE_TIME_VALIDATION_REQUIRED-0}"
if [[ "${COMPANION_RELEASE_PRODUCTION-}" == '1' ]]; then
  time_validation_required=1
fi
case "$time_validation_required" in
  0) ;;
  1)
    [[ "$receipt_policy_present" == '1' ]] \
      || fail 'current-time validation requires the complete receipt policy'
    now_epoch=$(date -u '+%s') || fail 'cannot obtain current UTC release time'
    (( now_epoch >= manifest_issued_epoch && now_epoch < manifest_expires_epoch )) \
      || fail 'companion manifest is outside its current validity window'
    (( now_epoch >= receipt_issued_epoch && now_epoch < receipt_expires_epoch )) \
      || fail 'public key receipt is outside its current validity window'
    ;;
  *) fail 'COMPANION_RELEASE_TIME_VALIDATION_REQUIRED must be 0 or 1' ;;
esac
