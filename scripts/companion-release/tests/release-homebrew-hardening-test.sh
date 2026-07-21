#!/usr/bin/env bash
set -euo pipefail
umask 077

tests_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
script_dir=$(cd -- "$tests_dir/.." && pwd)
fail() { printf 'release homebrew hardening test: %s\n' "$1" >&2; exit 1; }

temp=$(mktemp -d "${TMPDIR:-/tmp}/release-homebrew-hardening-test.XXXXXX")
trap 'rm -rf -- "$temp"' EXIT
state="$temp/tap-state"
mkdir -m 0700 "$state" "$temp/bin"
install -m 0700 "$tests_dir/testdata/mock-tap-gh.sh" "$temp/bin/gh"
checksums="$temp/checksums.txt"
printf '%064d  autopus-adk_0.50.81_darwin_amd64.tar.gz\n' 1 >"$checksums"
printf '%064d  autopus-adk_0.50.81_darwin_arm64.tar.gz\n' 2 >>"$checksums"
printf '%064d  autopus-adk_0.50.81_linux_amd64.tar.gz\n' 3 >>"$checksums"
printf '%064d  autopus-adk_0.50.81_linux_arm64.tar.gz\n' 4 >>"$checksums"

# A10 updates only the Cask from the exact A9 tap head and keeps Formula frozen.
source "$script_dir/publish-homebrew-formula-bridge-render.sh"
render_homebrew_cask "$temp/prior-cask.rb" 0.50.80 \
  '48f80577ff2ef40a843dab0a847895ca7b3877e7fb810a30d328cbe8a55fc51e' \
  '503c338e1ce122e209b9e74bc883492317144b319b0713943bc299e57447024d' \
  '98af6bfc1458291f7c83b389c595d0de06e3ab84aaf18685bcfa62d897cbe0fb' \
  'd1eb553cd407ed978fc9fcc9b33e6d5d871b202f4d91f186dedc63227fd2a0d3'
[[ "$(git -C "$temp" hash-object "$temp/prior-cask.rb")" == \
   '3f3a38e9a2ae556acc0f7d0974895d6189f266dd' ]] \
  || fail 'rendered A9 Cask bytes differ from the pinned predecessor blob'
render_homebrew_formula_bridge "$temp/frozen-formula.rb" v0.50.71 0.50.71 \
  "$(printf '%064d' 1)" "$(printf '%064d' 2)" \
  "$(printf '%064d' 3)" "$(printf '%064d' 4)"
jq -n --arg content "$(base64 <"$temp/prior-cask.rb" | tr -d '\r\n')" \
  '{sha:"3f3a38e9a2ae556acc0f7d0974895d6189f266dd",content:$content}' >"$state/cask.json"
jq -n --arg content "$(base64 <"$temp/frozen-formula.rb" | tr -d '\r\n')" \
  '{sha:"4ebc6c38925002dec00759823d4dd847a499818a",content:$content}' >"$state/formula.json"
jq -n '{ref:"refs/heads/main",object:{type:"commit",sha:"8acf53e1bea9711ca3063c121b52e5d160f43b67",url:"https://example.invalid/prior-commit"}}' \
  >"$state/branch.json"
cp "$state/formula.json" "$temp/formula-before.json"
bridge_env=(PATH="$temp/bin:$PATH" MOCK_TAP_STATE="$state" GITHUB_REF_NAME=v0.50.81
  COMPANION_VERSION=0.50.81 COMPANION_HOMEBREW_POLICY=cask-only
  COMPANION_CHECKSUMS_PATH="$checksums" HOMEBREW_TAP_TOKEN=fixture)
env "${bridge_env[@]}" bash "$script_dir/publish-homebrew-formula-bridge.sh"
[[ "$(<"$state/ref-update.calls")" == 1 &&
   "$(<"$state/blob-create.calls")" == 1 &&
   "$(<"$state/tree-create.calls")" == 1 &&
   "$(<"$state/commit-create.calls")" == 1 &&
   "$(<"$state/formula-get.calls")" == 1 ]] \
  || fail 'A10 did not update only the Cask'
cmp -s "$temp/formula-before.json" "$state/formula.json" \
  || fail 'frozen v0.50.71 Formula blob or bytes changed'
env "${bridge_env[@]}" bash "$script_dir/publish-homebrew-formula-bridge.sh"
[[ "$(<"$state/ref-update.calls")" == 1 &&
   "$(<"$state/blob-create.calls")" == 1 &&
   "$(<"$state/tree-create.calls")" == 1 &&
   "$(<"$state/commit-create.calls")" == 1 &&
   "$(<"$state/formula-get.calls")" == 2 ]] \
  || fail 'A10 Cask-only reconciler is not idempotent'

# An update-needed retry must reject pre-existing tap-head or Formula drift.
jq -n --arg content "$(base64 <"$temp/prior-cask.rb" | tr -d '\r\n')" \
  '{sha:"3f3a38e9a2ae556acc0f7d0974895d6189f266dd",content:$content}' >"$state/cask.json"
jq -n '{ref:"refs/heads/main",object:{type:"commit",sha:"4444444444444444444444444444444444444444",url:"https://example.invalid/racer"}}' \
  >"$state/branch.json"
if env "${bridge_env[@]}" bash "$script_dir/publish-homebrew-formula-bridge.sh" \
  >/dev/null 2>&1; then
  fail 'A10 accepted a drifted Homebrew tap predecessor commit'
fi
[[ "$(<"$state/ref-update.calls")" == 1 ]] \
  || fail 'A10 updated Cask after predecessor commit drift'
jq -n '{ref:"refs/heads/main",object:{type:"commit",sha:"8acf53e1bea9711ca3063c121b52e5d160f43b67",url:"https://example.invalid/prior-commit"}}' \
  >"$state/branch.json"
jq -n --arg content "$(base64 <"$temp/frozen-formula.rb" | tr -d '\r\n')" \
  '{sha:"5555555555555555555555555555555555555555",content:$content}' >"$state/formula.json"
if env "${bridge_env[@]}" bash "$script_dir/publish-homebrew-formula-bridge.sh" \
  >/dev/null 2>&1; then
  fail 'A10 accepted frozen Formula blob drift'
fi
[[ "$(<"$state/ref-update.calls")" == 1 ]] \
  || fail 'A10 mutated tap state after frozen Formula drift'

# A branch move after the head check must make the non-force ref CAS fail.
jq -n --arg content "$(base64 <"$temp/frozen-formula.rb" | tr -d '\r\n')" \
  '{sha:"4ebc6c38925002dec00759823d4dd847a499818a",content:$content}' >"$state/formula.json"
jq -n '{ref:"refs/heads/main",object:{type:"commit",sha:"8acf53e1bea9711ca3063c121b52e5d160f43b67",url:"https://example.invalid/prior-commit"}}' \
  >"$state/branch.json"
touch "$state/race-before-ref"
if env "${bridge_env[@]}" bash "$script_dir/publish-homebrew-formula-bridge.sh" \
  >/dev/null 2>&1; then
  fail 'A10 accepted a concurrent Homebrew branch move'
fi
rm -f -- "$state/race-before-ref"
[[ "$(<"$state/ref-update.calls")" == 1 &&
   "$(jq -er '.object.sha' "$state/branch.json")" == \
     '4444444444444444444444444444444444444444' ]] \
  || fail 'A10 overwrote a concurrent Homebrew branch move'

# A concurrent Formula-changing commit must also win the race and reject A10.
jq -n --arg content "$(base64 <"$temp/frozen-formula.rb" | tr -d '\r\n')" \
  '{sha:"4ebc6c38925002dec00759823d4dd847a499818a",content:$content}' >"$state/formula.json"
jq -n '{ref:"refs/heads/main",object:{type:"commit",sha:"8acf53e1bea9711ca3063c121b52e5d160f43b67",url:"https://example.invalid/prior-commit"}}' \
  >"$state/branch.json"
touch "$state/formula-race-before-ref"
if env "${bridge_env[@]}" bash "$script_dir/publish-homebrew-formula-bridge.sh" \
  >/dev/null 2>&1; then
  fail 'A10 accepted a concurrent Formula drift commit'
fi
rm -f -- "$state/formula-race-before-ref"
[[ "$(<"$state/ref-update.calls")" == 1 &&
   "$(jq -er '.sha' "$state/formula.json")" == \
     '6666666666666666666666666666666666666666' ]] \
  || fail 'A10 overwrote a concurrent Formula drift commit'

printf 'release homebrew hardening test: PASS\n'
