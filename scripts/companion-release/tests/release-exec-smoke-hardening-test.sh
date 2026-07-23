#!/usr/bin/env bash
# shellcheck disable=SC2016
set -euo pipefail
umask 077

tests_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
script_dir=$(cd -- "$tests_dir/.." && pwd)
repo=$(cd -- "$script_dir/../.." && pwd)
producer="$script_dir/produce.sh"
environment_gate="$script_dir/validate-environment.sh"
exec_smoke_package="$script_dir/execsmoke"

fail() {
  printf 'release exec smoke hardening test: %s\n' "$1" >&2
  exit 1
}

contains() {
  grep -Fq -- "$2" "$1" || fail "$1 missing $2"
}

[[ -d "$exec_smoke_package" && ! -L "$exec_smoke_package" ]] \
  || fail 'execution smoke helper package is missing or unsafe'
contains "$environment_gate" 'COMPANION_EXEC_SMOKE_GATE'
contains "$environment_gate" "'companion execution smoke gate'"
contains "$producer" '"$COMPANION_EXEC_SMOKE_GATE"'
contains "$producer" '--artifact "$artifact_path"'
contains "$producer" '--expected-version "$COMPANION_VERSION"'
contains "$producer" '--architecture "$COMPANION_ARCHITECTURE"'
contains "$producer" '--timeout 15s'

identity_index=$(grep -n "Signature=adhoc" "$producer" | tail -1 | cut -d: -f1)
smoke_index=$(grep -n '"$COMPANION_EXEC_SMOKE_GATE"' "$producer" | tail -1 | cut -d: -f1)
manifest_index=$(grep -n 'manifest_sign_args=(' "$producer" | head -1 | cut -d: -f1)
(( identity_index < smoke_index && smoke_index < manifest_index )) \
  || fail 'execution smoke gate is not between final identity verification and manifest creation'

go test -count=1 "$repo/scripts/companion-release/execsmoke"

if [[ "$(uname -s)" == 'Darwin' ]]; then
  temp_dir=$(mktemp -d "${TMPDIR:-/tmp}/release-exec-smoke-test.XXXXXX")
  cleanup() {
    local status=$?
    rm -rf -- "$temp_dir" || status=$?
    return "$status"
  }
  trap cleanup EXIT
  gate="$temp_dir/exec-smoke-gate"
  artifact="$temp_dir/auto"
  architecture=$(go env GOARCH)
  case "$architecture" in
    amd64|arm64) ;;
    *) fail "unsupported Darwin test architecture: $architecture" ;;
  esac
  go build -trimpath -o "$gate" "$repo/scripts/companion-release/execsmoke"
  go build -trimpath \
    -ldflags '-X github.com/insajin/autopus-adk/pkg/version.version=0.50.86' \
    -o "$artifact" "$repo/cmd/auto"
  "$gate" --artifact "$artifact" --expected-version 0.50.86 \
    --architecture "$architecture" --timeout 15s
  if "$gate" --artifact "$artifact" --expected-version 0.50.85 \
    --architecture "$architecture" --timeout 15s >/dev/null 2>&1; then
    fail 'wrong expected version passed the execution smoke gate'
  fi
fi

printf 'release exec smoke hardening test: PASS\n'
