#!/usr/bin/env bash
set -euo pipefail
umask 077

tests_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
script_dir=$(cd -- "$tests_dir/.." && pwd)
runtime_lib="$script_dir/prepare-release-runtime-lib.sh"
mock_gh="$tests_dir/testdata/mock-release-prep-gh.sh"
temp_dir=$(mktemp -d "${TMPDIR:-/tmp}/release-prep-environment.XXXXXX")
trap 'rm -rf -- "$temp_dir"' EXIT
fail() { printf 'release prep environment test: %s\n' "$1" >&2; exit 1; }

state="$temp_dir/state"
mkdir -p "$state" "$temp_dir/bin"
cp "$mock_gh" "$temp_dir/bin/gh"
chmod 0700 "$temp_dir/bin/gh"
printf '%s\n' '0' >"$state/write-count"
printf '%s\n' '[{"name":"ADK_COMPANION_KEY_ID","value":"adk-release-2026-q3-b0"}]' >"$state/repository-variables.json"
export MOCK_RELEASE_PREP_STATE="$state"
export PATH="$temp_dir/bin:$PATH"
repository='Insajin/autopus-adk'
environment_name='adk-companion-release'
# shellcheck source=../prepare-release-runtime-lib.sh
source "$runtime_lib"

printf '%s\n' '[]' >"$state/environment-variables.json"
environment_variables=$(gh variable list --repo "$repository" --env "$environment_name" --json name,value)
actual=$(matched_variable ADK_COMPANION_KEY_ID)
[[ "$actual" == 'adk-release-2026-q3-b0' ]] || fail 'repository inheritance was rejected'

printf '%s\n' '[{"name":"ADK_COMPANION_KEY_ID","value":"adk-release-2026-q3-b0"}]' >"$state/environment-variables.json"
environment_variables=$(gh variable list --repo "$repository" --env "$environment_name" --json name,value)
[[ "$(matched_variable ADK_COMPANION_KEY_ID)" == "$actual" ]] || fail 'matching override was rejected'

printf '%s\n' '[{"name":"ADK_COMPANION_KEY_ID","value":"conflicting-key"}]' >"$state/environment-variables.json"
environment_variables=$(gh variable list --repo "$repository" --env "$environment_name" --json name,value)
if (matched_variable ADK_COMPANION_KEY_ID >/dev/null 2>&1); then
  fail 'conflicting environment override was accepted'
fi

empty_cleanup_dir="$temp_dir/empty-cleanup"
mkdir -p "$empty_cleanup_dir"
(
  evidence_source_commit=''
  retain_prep_lock=0
  isolation_roots=()
  temp_dir="$empty_cleanup_dir"
  cleanup
) || fail 'empty isolation cleanup failed'
[[ ! -e "$empty_cleanup_dir" ]] || fail 'empty isolation cleanup left temporary state'
[[ "$(<"$state/write-count")" == '0' ]] || fail 'read-only checks mutated release state'
printf 'release prep environment test: PASS\n'
