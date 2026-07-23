#!/usr/bin/env bash
set -euo pipefail
umask 077

tests_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
script_dir=$(cd -- "$tests_dir/.." && pwd)
producer="$script_dir/produce.sh"
receipt_helper="$script_dir/produce-public-key-receipt.sh"
fail() { printf 'release producer helper hardening test: %s\n' "$1" >&2; exit 1; }

if bash "$receipt_helper" >/dev/null 2>&1; then
  fail 'source-only receipt helper executed directly'
fi

resolve_phase() {
  GITHUB_REF_NAME=$1 COMPANION_VERSION=$2 PRODUCER_RECEIPT_HELPER="$receipt_helper" \
    bash -c '
      set -euo pipefail
      fail() { printf "%s\n" "$1" >&2; return 1; }
      source "$PRODUCER_RECEIPT_HELPER"
      resolve_public_key_receipt_release_phase
      printf "%s" "$release_phase"
    '
}

while read -r tag version phase; do
  [[ "$(resolve_phase "$tag" "$version")" == "$phase" ]] \
    || fail "exact ${phase} tag/version pair resolved incorrectly"
done <<'CASES'
v0.50.69 0.50.69 A0
v0.50.70 0.50.70 A1
v0.50.71 0.50.71 A2
v0.50.72 0.50.72 A3
v0.50.73 0.50.73 A4
v0.50.74 0.50.74 A5
v0.50.77 0.50.77 A6
v0.50.78 0.50.78 A7
v0.50.79 0.50.79 A8
v0.50.80 0.50.80 A9
v0.50.81 0.50.81 A10
v0.50.82 0.50.82 A11
v0.50.83 0.50.83 A12
v0.50.84 0.50.84 A13
v0.50.85 0.50.85 A14
v0.50.86 0.50.86 A15
CASES

for mismatch in 'v0.50.86 0.50.85' 'v0.50.85 0.50.86' 'v0.50.87 0.50.87'; do
  read -r tag version <<< "$mismatch"
  if resolve_phase "$tag" "$version" >/dev/null 2>&1; then
    fail "mixed or unknown release identity passed: ${tag}/${version}"
  fi
done

temp=$(mktemp -d "${TMPDIR:-/tmp}/adk-producer-helper-gate.XXXXXX")
trap 'rm -rf -- "$temp"' EXIT
cp -- "$producer" "$temp/produce.sh"
assert_gate_failure() {
  local expected=$1 output
  if output=$(COMPANION_PLATFORM=darwin COMPANION_ARTIFACT="$temp/auto" \
    COMPANION_TARGET=darwin_arm64 COMPANION_ARCHITECTURE=arm64 \
    COMPANION_VERSION=0.50.86 bash "$temp/produce.sh" 2>&1); then
    fail "unsafe helper gate passed"
  fi
  [[ "$output" == *"$expected"* ]] || fail "helper gate diagnostic = ${output}"
}

assert_gate_failure 'public key receipt helper is missing or unsafe'
ln -s -- "$receipt_helper" "$temp/produce-public-key-receipt.sh"
assert_gate_failure 'public key receipt helper is missing or unsafe'
rm -- "$temp/produce-public-key-receipt.sh"
printf ':\n' > "$temp/produce-public-key-receipt.sh"
assert_gate_failure 'public key receipt helper contract is incomplete'

printf 'release producer helper hardening test: PASS\n'
