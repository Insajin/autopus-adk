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
printf '%064d  autopus-adk_0.50.83_darwin_amd64.tar.gz\n' 1 >"$checksums"
printf '%064d  autopus-adk_0.50.83_darwin_arm64.tar.gz\n' 2 >>"$checksums"
printf '%064d  autopus-adk_0.50.83_linux_amd64.tar.gz\n' 3 >>"$checksums"
printf '%064d  autopus-adk_0.50.83_linux_arm64.tar.gz\n' 4 >>"$checksums"

# A12 updates only the Cask from the exact A11 tap head and keeps Formula frozen.
source "$script_dir/publish-homebrew-formula-bridge-render.sh"
render_homebrew_cask "$temp/prior-cask.rb" 0.50.82 \
  'f5825b4aff8ce84e6b18dfb0ae0249a432a1b247477c3a9e2cd14689a405d40d' \
  'c913c51b396e01034e889f43ef4da68fcae851e7f7cba7f2b8ac60a2c4e00c66' \
  'b8fa6f2f124904a3fdc94f0600e4ce4eab374f18e17a915b25d56bb063d613df' \
  'fd244d58b85b6f24b4b2a9ed050fa085ba3547487f73e9dd2559b3bbfdb92540'
[[ "$(git -C "$temp" hash-object "$temp/prior-cask.rb")" == \
   '7f760f85434457fa4cdc7da3718099e43980ce06' ]] \
  || fail 'rendered A11 Cask bytes differ from the pinned predecessor blob'
render_homebrew_formula_bridge "$temp/frozen-formula.rb" v0.50.71 0.50.71 \
  "$(printf '%064d' 1)" "$(printf '%064d' 2)" \
  "$(printf '%064d' 3)" "$(printf '%064d' 4)"
jq -n --arg content "$(base64 <"$temp/prior-cask.rb" | tr -d '\r\n')" \
  '{sha:"7f760f85434457fa4cdc7da3718099e43980ce06",content:$content}' >"$state/cask.json"
jq -n --arg content "$(base64 <"$temp/frozen-formula.rb" | tr -d '\r\n')" \
  '{sha:"4ebc6c38925002dec00759823d4dd847a499818a",content:$content}' >"$state/formula.json"
jq -n '{ref:"refs/heads/main",object:{type:"commit",sha:"624e317d0433fe0efc91efb5dc5e8708ed50a22d",url:"https://example.invalid/prior-commit"}}' \
  >"$state/branch.json"
cp "$state/formula.json" "$temp/formula-before.json"
bridge_env=(PATH="$temp/bin:$PATH" MOCK_TAP_STATE="$state" GITHUB_REF_NAME=v0.50.83
  COMPANION_VERSION=0.50.83 COMPANION_HOMEBREW_POLICY=cask-only
  COMPANION_CHECKSUMS_PATH="$checksums" HOMEBREW_TAP_TOKEN=fixture)
env "${bridge_env[@]}" bash "$script_dir/publish-homebrew-formula-bridge.sh"
[[ "$(<"$state/ref-update.calls")" == 1 &&
   "$(<"$state/blob-create.calls")" == 1 &&
   "$(<"$state/tree-create.calls")" == 1 &&
   "$(<"$state/commit-create.calls")" == 1 &&
   "$(<"$state/formula-get.calls")" == 1 ]] \
  || fail 'A12 did not update only the Cask'
cmp -s "$temp/formula-before.json" "$state/formula.json" \
  || fail 'frozen v0.50.71 Formula blob or bytes changed'
env "${bridge_env[@]}" bash "$script_dir/publish-homebrew-formula-bridge.sh"
[[ "$(<"$state/ref-update.calls")" == 1 &&
   "$(<"$state/blob-create.calls")" == 1 &&
   "$(<"$state/tree-create.calls")" == 1 &&
   "$(<"$state/commit-create.calls")" == 1 &&
   "$(<"$state/formula-get.calls")" == 2 ]] \
  || fail 'A12 Cask-only reconciler is not idempotent'

# An already-current Cask must bind to one stable head with the frozen Formula.
touch "$state/idempotent-formula-race"
if env "${bridge_env[@]}" bash "$script_dir/publish-homebrew-formula-bridge.sh" \
  >/dev/null 2>&1; then
  fail 'A12 accepted idempotent Cask bytes across concurrent Formula drift'
fi
rm -f -- "$state/idempotent-formula-race"
[[ "$(<"$state/ref-update.calls")" == 1 &&
   "$(jq -er '.object.sha' "$state/branch.json")" == \
     '8888888888888888888888888888888888888888' &&
   "$(jq -er '.sha' "$state/formula.json")" == \
     '6666666666666666666666666666666666666666' ]] \
  || fail 'A12 mutated tap state after idempotent Formula drift'
jq -n '{ref:"refs/heads/main",object:{type:"commit",sha:"3333333333333333333333333333333333333333",url:"https://example.invalid/target-commit"}}' \
  >"$state/branch.json"
jq -n --arg content "$(base64 <"$temp/frozen-formula.rb" | tr -d '\r\n')" \
  '{sha:"4ebc6c38925002dec00759823d4dd847a499818a",content:$content}' >"$state/formula.json"
rm -f -- "$state/branch-get.calls"
touch "$state/idempotent-ref-race"
if env "${bridge_env[@]}" bash "$script_dir/publish-homebrew-formula-bridge.sh" \
  >/dev/null 2>&1; then
  fail 'A12 accepted a branch move after idempotent tree verification'
fi
rm -f -- "$state/idempotent-ref-race"
[[ "$(<"$state/ref-update.calls")" == 1 &&
   "$(jq -er '.object.sha' "$state/branch.json")" == \
     '4444444444444444444444444444444444444444' ]] \
  || fail 'A12 updated Cask during idempotent head verification'

# An update-needed retry must reject pre-existing tap-head or Formula drift.
jq -n --arg content "$(base64 <"$temp/prior-cask.rb" | tr -d '\r\n')" \
  '{sha:"7f760f85434457fa4cdc7da3718099e43980ce06",content:$content}' >"$state/cask.json"
jq -n '{ref:"refs/heads/main",object:{type:"commit",sha:"4444444444444444444444444444444444444444",url:"https://example.invalid/racer"}}' \
  >"$state/branch.json"
if env "${bridge_env[@]}" bash "$script_dir/publish-homebrew-formula-bridge.sh" \
  >/dev/null 2>&1; then
  fail 'A12 accepted a drifted Homebrew tap predecessor commit'
fi
[[ "$(<"$state/ref-update.calls")" == 1 ]] \
  || fail 'A12 updated Cask after predecessor commit drift'
jq -n '{ref:"refs/heads/main",object:{type:"commit",sha:"624e317d0433fe0efc91efb5dc5e8708ed50a22d",url:"https://example.invalid/prior-commit"}}' \
  >"$state/branch.json"
jq -n --arg content "$(base64 <"$temp/frozen-formula.rb" | tr -d '\r\n')" \
  '{sha:"5555555555555555555555555555555555555555",content:$content}' >"$state/formula.json"
if env "${bridge_env[@]}" bash "$script_dir/publish-homebrew-formula-bridge.sh" \
  >/dev/null 2>&1; then
  fail 'A12 accepted frozen Formula blob drift'
fi
[[ "$(<"$state/ref-update.calls")" == 1 ]] \
  || fail 'A12 mutated tap state after frozen Formula drift'

# A branch move after the head check must make the non-force ref CAS fail.
jq -n --arg content "$(base64 <"$temp/frozen-formula.rb" | tr -d '\r\n')" \
  '{sha:"4ebc6c38925002dec00759823d4dd847a499818a",content:$content}' >"$state/formula.json"
jq -n '{ref:"refs/heads/main",object:{type:"commit",sha:"624e317d0433fe0efc91efb5dc5e8708ed50a22d",url:"https://example.invalid/prior-commit"}}' \
  >"$state/branch.json"
touch "$state/race-before-ref"
if env "${bridge_env[@]}" bash "$script_dir/publish-homebrew-formula-bridge.sh" \
  >/dev/null 2>&1; then
  fail 'A12 accepted a concurrent Homebrew branch move'
fi
rm -f -- "$state/race-before-ref"
[[ "$(<"$state/ref-update.calls")" == 1 &&
   "$(jq -er '.object.sha' "$state/branch.json")" == \
     '4444444444444444444444444444444444444444' ]] \
  || fail 'A12 overwrote a concurrent Homebrew branch move'

# A concurrent Formula-changing commit must also win the race and reject A12.
jq -n --arg content "$(base64 <"$temp/frozen-formula.rb" | tr -d '\r\n')" \
  '{sha:"4ebc6c38925002dec00759823d4dd847a499818a",content:$content}' >"$state/formula.json"
jq -n '{ref:"refs/heads/main",object:{type:"commit",sha:"624e317d0433fe0efc91efb5dc5e8708ed50a22d",url:"https://example.invalid/prior-commit"}}' \
  >"$state/branch.json"
touch "$state/formula-race-before-ref"
if env "${bridge_env[@]}" bash "$script_dir/publish-homebrew-formula-bridge.sh" \
  >/dev/null 2>&1; then
  fail 'A12 accepted a concurrent Formula drift commit'
fi
rm -f -- "$state/formula-race-before-ref"
[[ "$(<"$state/ref-update.calls")" == 1 &&
   "$(jq -er '.sha' "$state/formula.json")" == \
     '6666666666666666666666666666666666666666' ]] \
  || fail 'A12 overwrote a concurrent Formula drift commit'

printf 'release homebrew hardening test: PASS\n'
