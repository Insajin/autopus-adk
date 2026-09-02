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

# Preferred path: the operator's own OMP auth-gateway. It is a loopback forward
# proxy backed by the OMP credential vault, so a subscription account is already
# in the broker and nothing has to be retyped per release. The env file below
# stays supported for setups without a gateway.
#
# The credential the ADK carries is the gateway's inbound bearer token, not the
# upstream provider secret: the upstream stays in the broker and never enters
# this process. That is strictly less exposure than pasting a provider key.
resolve_from_omp_gateway() {
  local omp=$1 broker=${OMP_AUTH_BROKER_URL:-} status url token
  command -v curl >/dev/null || return 1
  [[ -x "$omp" ]] || return 1
  status=$(OMP_AUTH_BROKER_URL="$broker" "$omp" auth-gateway status --json 2>/dev/null) || return 1
  grep -q '"ready":true' <<<"$status" || return 1
  url=${ADK_GATEWAY_URL:-}
  [[ -n "$url" ]] || return 1
  token=$(OMP_AUTH_BROKER_URL="$broker" "$omp" auth-gateway token 2>/dev/null | tr -d '\n')
  [[ -n "$token" ]] || return 1
  ADK_GATEWAY_URL=$url
  ADK_CREDENTIAL_LOCATOR='AUTOPUS_OMP_CONTEXT_PROVIDER_GATEWAY'
  export AUTOPUS_OMP_CONTEXT_PROVIDER_GATEWAY="$token"
  return 0
}

gateway_source='env file'
# The gateway probe uses whichever omp is on PATH rather than the release-pinned
# path. The pin belongs to the evidence oracle; this call only asks the gateway
# whether it is ready, and an earlier hardcoded version here went stale the
# moment the pin moved.
if resolve_from_omp_gateway "$(command -v omp || printf /nonexistent)"; then
  gateway_source='omp auth-gateway'
elif [[ ! -f "$env_file" || -L "$env_file" ]]; then
  cat >&2 <<EOF
release prep: no gateway and no env file at $env_file

Either bring up the OMP auth-gateway, which needs no per-release typing because
the subscription already lives in the broker:

  omp auth-broker serve --bind 127.0.0.1:47311
  OMP_AUTH_BROKER_URL=http://127.0.0.1:47311 omp auth-gateway serve --bind 127.0.0.1:47312
  export OMP_AUTH_BROKER_URL=http://127.0.0.1:47311
  export ADK_GATEWAY_URL=http://127.0.0.1:47312

or create the env file once, with mode 0600:

  install -m 0600 /dev/null "$env_file"
  cat >"$env_file" <<'ENV'
  ADK_GATEWAY_URL='http://127.0.0.1:PORT'
  ADK_CREDENTIAL_LOCATOR='AUTOPUS_OMP_CONTEXT_PROVIDER_APPROVED'
  export AUTOPUS_OMP_CONTEXT_PROVIDER_APPROVED='<credential>'
  ENV

Either way the credential reaches prepare-release.sh through the environment,
never a flag: it reads the named variable and unsets it, and the evidence writer
refuses to emit it. \$HOME is outside the repository.
EOF
  exit 1
fi

if [[ "$gateway_source" == 'env file' ]]; then
  case "$(uname -s)" in
    Darwin) mode_bits=$(/usr/bin/stat -f '%Lp' "$env_file") ;;
    Linux) mode_bits=$(stat -c '%a' "$env_file") ;;
    *) fail 'unsupported platform' ;;
  esac
  [[ "$mode_bits" == '600' ]] || fail "env file mode is ${mode_bits}, expected 600"
  # shellcheck source=/dev/null
  . "$env_file"
fi

# Both paths converge here, so the shape checks run once and apply to either.
[[ -n "${ADK_GATEWAY_URL:-}" ]] || fail 'ADK_GATEWAY_URL is unset'
[[ -n "${ADK_CREDENTIAL_LOCATOR:-}" ]] || fail 'ADK_CREDENTIAL_LOCATOR is unset'
[[ "$ADK_GATEWAY_URL" =~ ^http://127\.0\.0\.1:[1-9][0-9]{0,4}$ ]] ||
  fail 'ADK_GATEWAY_URL is not an exact loopback HTTP URL'
[[ "$ADK_CREDENTIAL_LOCATOR" =~ ^AUTOPUS_OMP_CONTEXT_PROVIDER_[A-Z0-9_]{1,96}$ ]] ||
  fail 'ADK_CREDENTIAL_LOCATOR does not match the required name shape'
[[ -n "${!ADK_CREDENTIAL_LOCATOR-}" ]] ||
  fail "${ADK_CREDENTIAL_LOCATOR} is not exported"

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

# One fetch, two values. Both live in the active static policy, which is the
# authority that selected them for the predecessor release. An earlier draft
# inferred the provider from the predecessor's asset list through a grep in a
# command substitution; it worked once and then silently resolved to empty.
active_policy=$(gh variable get OMP_CONTEXT_STATIC_POLICY_B64 \
  --repo Insajin/autopus-adk 2>/dev/null | tr '_-' '/+' | base64 -d 2>/dev/null) ||
  fail 'cannot read the active static policy'
oracle_policy_digest=$(sed -n 's/.*"oracle_policy_digest":"\([^"]*\)".*/\1/p' <<<"$active_policy")
provider=$(sed -n 's/.*"provider":"\([^"]*\)".*/\1/p' <<<"$active_policy")
[[ "$oracle_policy_digest" =~ ^sha256:[0-9a-f]{64}$ ]] ||
  fail 'active static policy has no usable oracle policy digest'
[[ "$provider" =~ ^[a-z0-9][a-z0-9-]{0,63}$ ]] ||
  fail 'active static policy has no usable provider id'

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

printf 'release prep: %s via %s with provider=%s model=%s omp=%s\n' \
  "$mode" "$gateway_source" "$provider" "$model" "$omp_version" >&2

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
