#!/usr/bin/env bash
# One-argument wrapper for prepare-release.sh: release-prep.sh --preflight|--apply
#
# Everything prepare-release.sh needs is either resolvable from the repository
# and the operator key store, or lives in one operator-local env file. Retyping
# nine flags per attempt is how a wrong value gets pasted at 2am.
#
# The credential contract is unchanged on purpose. prepare-release.sh reads the
# variable named by --credential-locator out of its own environment and unsets it
# immediately, and the observe-session evidence writer asserts the credential
# string never appears in emitted evidence. Passing a secret as an argument would
# put it in the process list and the shell history, so this wrapper exports it
# into the same process instead and never prints it.
set -euo pipefail
umask 077

fail() { printf 'release prep: %s\n' "$1" >&2; exit 1; }
usage() {
  printf '%s\n' 'usage: release-prep.sh (--preflight|--apply) [--inherit-parent-sandbox]' >&2
  exit 64
}

mode=''
extra=()
while [[ $# -gt 0 ]]; do
  case "$1" in
    --preflight | --apply) [[ -z "$mode" ]] || usage; mode=$1; shift ;;
    --inherit-parent-sandbox) extra+=("$1"); shift ;;
    *) usage ;;
  esac
done
[[ -n "$mode" ]] || usage

repo_root=$(git rev-parse --show-toplevel) || fail 'not a git repository'
cd "$repo_root"

readonly key_store="${HOME}/.config/autopus/release-keys"
readonly env_file="${ADK_RELEASE_PREP_ENV:-$key_store/prep-env.sh}"

# The env file carries only what cannot be derived: the gateway URL and the
# credential. It is operator-local and must not be readable by anyone else.
if [[ ! -f "$env_file" || -L "$env_file" ]]; then
  cat >&2 <<EOF
release prep: no env file at $env_file

Create it once, with mode 0600:

  install -m 0600 /dev/null "$env_file"
  cat >"$env_file" <<'ENV'
  ADK_GATEWAY_URL='http://127.0.0.1:PORT'
  ADK_CREDENTIAL_LOCATOR='AUTOPUS_OMP_CONTEXT_PROVIDER_APPROVED'
  export AUTOPUS_OMP_CONTEXT_PROVIDER_APPROVED='<credential>'
  ENV

The credential stays in this file rather than a command line because
prepare-release.sh reads it from the environment and the evidence writer
refuses to emit it. Never commit it; \$HOME is outside the repository.
EOF
  exit 1
fi

case "$(uname -s)" in
  Darwin) mode_bits=$(/usr/bin/stat -f '%Lp' "$env_file") ;;
  Linux) mode_bits=$(stat -c '%a' "$env_file") ;;
  *) fail 'unsupported platform' ;;
esac
[[ "$mode_bits" == '600' ]] || fail "env file mode is ${mode_bits}, expected 600"

# shellcheck source=/dev/null
. "$env_file"

[[ -n "${ADK_GATEWAY_URL:-}" ]] || fail 'env file does not set ADK_GATEWAY_URL'
[[ -n "${ADK_CREDENTIAL_LOCATOR:-}" ]] || fail 'env file does not set ADK_CREDENTIAL_LOCATOR'
[[ "$ADK_GATEWAY_URL" =~ ^http://127\.0\.0\.1:[1-9][0-9]{0,4}$ ]] ||
  fail 'ADK_GATEWAY_URL is not an exact loopback HTTP URL'
[[ "$ADK_CREDENTIAL_LOCATOR" =~ ^AUTOPUS_OMP_CONTEXT_PROVIDER_[A-Z0-9_]{1,96}$ ]] ||
  fail 'ADK_CREDENTIAL_LOCATOR does not match the required name shape'
[[ -n "${!ADK_CREDENTIAL_LOCATOR-}" ]] ||
  fail "env file does not export ${ADK_CREDENTIAL_LOCATOR}"

# Resolved values. Each is measured rather than typed: the OMP version and digest
# come from the pins prepare-release.sh itself enforces, so they cannot drift
# apart from it.
omp_pin=$(awk -F"'" '/^readonly expected_omp_sha256=/ { print $2 }' \
  scripts/companion-release/prepare-release.sh)
omp_version=$(awk -F"'" '/verified OMP version differs/ { print $2 }' \
  scripts/companion-release/prepare-release.sh | head -1)
omp_executable="${HOME}/.cache/autopus/release/omp-${omp_version/omp\//v}-darwin-arm64"
[[ -f "$omp_executable" && ! -L "$omp_executable" ]] ||
  fail "verified OMP binary is missing at $omp_executable"
[[ "$(shasum -a 256 "$omp_executable" | awk '{print $1}')" == "$omp_pin" ]] ||
  fail 'staged OMP binary does not match the pinned digest'

oracle_policy_digest=$(gh variable get OMP_CONTEXT_STATIC_POLICY_B64 \
  --repo Insajin/autopus-adk 2>/dev/null |
  tr '_-' '/+' | base64 -d 2>/dev/null |
  sed -n 's/.*"oracle_policy_digest":"\([^"]*\)".*/\1/p')
[[ "$oracle_policy_digest" =~ ^sha256:[0-9a-f]{64}$ ]] ||
  fail 'cannot read the oracle policy digest from the active static policy'

provider=$(gh release view v0.50.111 --repo Insajin/autopus-adk \
  --json assets --jq '.assets[].name' 2>/dev/null | grep -qx 'omp-context-promotion-report.v1.json' &&
  printf 'openai-codex' || printf '')
[[ -n "$provider" ]] || fail 'cannot confirm the provider from the predecessor evidence'

model=$(awk -F'"' '/^model[[:space:]]*=/ { print $2; exit }' "${HOME}/.codex/config.toml")
model_context_window=$(awk -F'=' '/^model_context_window[[:space:]]*=/ { gsub(/[^0-9]/, "", $2); print $2; exit }' \
  "${HOME}/.codex/config.toml")
[[ -n "$model" ]] || fail 'cannot read the configured model from ~/.codex/config.toml'
[[ "$model_context_window" =~ ^[0-9]+$ && "$model_context_window" -ge 8192 ]] ||
  fail 'configured model context window is missing or below the required minimum'

r2_key="$key_store/release-tag-signing-2026-q3-r2"
k3_key="$key_store/omp-context-promotion-2026-q3-k3.b64"
for key in "$r2_key" "$k3_key"; do
  [[ -f "$key" && ! -L "$key" ]] || fail "signing key is missing at $key"
done

printf 'release prep: %s with provider=%s model=%s omp=%s\n' \
  "$mode" "$provider" "$model" "$omp_version" >&2

exec scripts/companion-release/prepare-release.sh \
  --endpoint "$ADK_GATEWAY_URL" \
  --credential-locator "$ADK_CREDENTIAL_LOCATOR" \
  --provider "$provider" \
  --model "$model" \
  --model-context-window "$model_context_window" \
  --omp "$omp_executable" \
  --oracle-policy-digest "$oracle_policy_digest" \
  --tag-signing-key "$r2_key" \
  --promotion-signing-key "$k3_key" \
  ${extra[@]+"${extra[@]}"} "$mode"
