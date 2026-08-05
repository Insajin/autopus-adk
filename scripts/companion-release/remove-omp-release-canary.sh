#!/usr/bin/env bash
set -euo pipefail

fail() { printf 'OMP release canary cleanup: %s\n' "$1" >&2; exit 1; }

[[ "$(uname -s)" == 'Darwin' && "$(uname -m)" == 'arm64' ]] \
  || fail 'host is not Darwin arm64'
[[ "${GITHUB_RUN_ID:-}" =~ ^[0-9]+$ && "${GITHUB_RUN_ATTEMPT:-}" =~ ^[0-9]+$ ]] \
  || fail 'GitHub run identity is malformed'
readonly root="/private/tmp/autopus-adk-omp-canary-${GITHUB_RUN_ID}-${GITHUB_RUN_ATTEMPT}"
readonly destination="$root/omp-darwin-arm64"
readonly receipt="$root/.cleanup-identity"

if [[ -e "$root" || -L "$root" ]]; then
  [[ -d "$root" && ! -L "$root" ]] || fail 'canary root is unsafe'
  [[ "$(/usr/bin/stat -f '%u:%Lp' "$root")" == '0:755' ]] \
    || fail 'canary root ownership differs'
  parent_identity=$(/usr/bin/stat -f '%d:%i' /private/tmp)
  root_identity=$(/usr/bin/stat -f '%d:%i' "$root")
  if [[ -e "$destination" || -L "$destination" ]]; then
    [[ -f "$destination" && ! -L "$destination" ]] || fail 'canary executable is unsafe'
    [[ "$(/usr/bin/stat -f '%u:%Lp' "$destination")" == '0:555' ]] \
      || fail 'canary executable ownership differs'
  fi
  if [[ -e "$receipt" || -L "$receipt" ]]; then
    [[ -f "$receipt" && ! -L "$receipt" ]] || fail 'cleanup receipt is unsafe'
    [[ "$(/usr/bin/stat -f '%u:%Lp' "$receipt")" == '0:444' ]] \
      || fail 'cleanup receipt ownership differs'
    [[ "$(/usr/bin/sed -n '1p' "$receipt")" == "parent=$parent_identity" ]] \
      || fail 'cleanup parent identity differs'
    [[ "$(/usr/bin/sed -n '2p' "$receipt")" == "root=$root_identity" ]] \
      || fail 'cleanup root identity differs'
    [[ -e "$destination" && ! -L "$destination" ]] \
      || fail 'cleanup receipt exists without canary executable'
    [[ "$(/usr/bin/sed -n '3p' "$receipt")" == \
       "omp=$(/usr/bin/stat -f '%d:%i' "$destination")" ]] \
      || fail 'cleanup executable identity differs'
    [[ "$(/usr/bin/wc -l < "$receipt" | /usr/bin/tr -d ' ')" == '3' ]] \
      || fail 'cleanup receipt shape differs'
  fi
  /usr/bin/sudo -n /bin/rm -rf -- "$root"
  [[ "$(/usr/bin/stat -f '%d:%i' /private/tmp)" == "$parent_identity" ]] \
    || fail '/private/tmp identity changed during cleanup'
fi
[[ ! -e "$root" && ! -L "$root" ]] || fail 'canary cleanup is incomplete'
