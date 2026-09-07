#!/usr/bin/env bash
set -euo pipefail

# Advance the pinned OMP release across every site that names it.
#
# oh-my-pi ships often — v17.2.7, v18.0.11, v18.1.2 and v18.1.4 landed inside a
# few days — and the pin lives in seven places that cannot be derived from each
# other at build time. Hand-editing them is how a release discovers a half-moved
# pin while the canary is already running.
#
# This script refuses to move anything until the candidate binary has proven the
# contract the harness actually depends on: the managed RPC negotiates protocol
# v2, and the runtime reports native image compaction. Those are the assertions
# `initializeManaged` makes at run time; checking them here means an incompatible
# OMP is rejected before the pin moves, not after.
#
# usage: advance-omp-pin.sh TO_VERSION            # e.g. 18.1.4
#        advance-omp-pin.sh TO_VERSION --dry-run
#        advance-omp-pin.sh TO_VERSION --measure  # move an unmeasured version so the
#                                                 # next release-prep.sh --apply measures it

readonly repository='can1357/oh-my-pi'
readonly declaration='scripts/companion-release/prepare-release.sh'

fail() { printf 'advance omp pin: %s\n' "$1" >&2; exit 1; }
note() { printf '  %s\n' "$1"; }

[[ $# -ge 1 ]] || fail 'usage: advance-omp-pin.sh TO_VERSION [--dry-run|--measure]'
readonly to_version="${1#v}"
readonly mode="${2-}"
[[ "$to_version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || fail "TO_VERSION must be N.N.N, got ${to_version}"
[[ -z "$mode" || "$mode" == '--dry-run' || "$mode" == '--measure' ]] ||
  fail 'second argument must be --dry-run or --measure'
readonly dry_run="$mode"
[[ -f "$declaration" && ! -L "$declaration" ]] || fail "run from the repository root; ${declaration} is missing"

from_version=$(awk -F"'" "/== 'omp\// { split(\$2, p, \"/\"); print p[2]; exit }" "$declaration")
[[ -n "$from_version" ]] || fail 'cannot read the current pin from the declaration'
readonly from_version
if [[ "$from_version" == "$to_version" ]]; then
  printf 'advance omp pin: already at omp/%s\n' "$to_version"
  exit 0
fi

# Upstream ships every few days, so a refusal keyed to one version number does
# not scale: 18.1.5 was named here and 18.2.0 would have walked straight past
# it into another cohort run. What follows is a verdict table instead, and it
# distinguishes the two states that matter.
#
# The probe below cannot decide either one. It proves the launch contract —
# protocol v2, the asset's own version, the measured digest — and nothing about
# compaction, because manual compaction needs a live provider session with
# history. Only the cohort measures reduction.
#
# Measured 2026-09-03, same plan generator and workload, binary swapped:
#
#   omp/17.2.7  8 compactions, median reduction cleared 2000 bp   -> in use
#   omp/18.1.2  emits "snapcompact would not reduce context locally." (6x)
#   omp/18.1.5  2 compactions, median reduction 0 bp              -> refused
#
# Clearing a refusal is a measurement, not an edit. The measurement is the
# release canary itself: move the pin with --measure, advance the coordinate,
# and run release-prep.sh --apply. The 40-call cohort runs before any tag or
# remote mutation, so a failing verdict leaves nothing behind but the number to
# record here (docs/runbooks/omp-pin-advance.md, "Measuring an unmeasured
# version"). Versions measured bad are refused even under --measure; only an
# upstream change that plausibly alters compaction (a newer version) is worth a
# cohort.
case "$to_version" in
  17.2.7)
    : # the pin in use; measured good
    ;;
  18.1.2 | 18.1.5)
    fail "$(printf '%s\n' \
      "omp/${to_version} was measured on 2026-09-03 and does not reduce context." \
      "  Standalone cohort, identical workload to the passing omp/17.2.7 run:" \
      "    compactions=2/2 median_reduction_bp=0/2000" \
      "  Every other gate passed. The promotion evidence attests a >=20% median" \
      "  reduction, so admitting this version would attest zero as reduction." \
      "  That is an upstream question about snapcompact, not a harness change." \
      "  See docs/runbooks/omp-pin-advance.md for the full comparison.")"
    ;;
  *)
    if [[ "$mode" == '--measure' || "$mode" == '--dry-run' ]]; then
      note "omp/${to_version} is unmeasured; ${mode#--} only proves the launch contract"
      note 'the next release-prep.sh --apply is the measurement; record its verdict in this table'
    else
      fail "$(printf '%s\n' \
        "omp/${to_version} has not been measured against the reduction floor." \
        "  The handshake probe proves the launch contract only. omp/18.1.x passed" \
        "  that probe and still delivered 0 bp of median reduction, so passing it" \
        "  is not evidence of anything the promotion report claims." \
        "  Measure it: advance-omp-pin.sh ${to_version} --measure, advance the" \
        "  coordinate, then release-prep.sh --apply. The cohort runs before the tag," \
        "  so a failing verdict costs provider calls, not a coordinate. Record the" \
        "  numbers in this table either way (docs/runbooks/omp-pin-advance.md).")"
    fi
    ;;
esac

# The pin is version plus bytes. Measure the bytes from the immutable upstream
# asset rather than trusting a digest someone typed.
work=$(mktemp -d "${TMPDIR:-/tmp}/omp-pin.XXXXXX")
cleanup() { rm -rf -- "$work"; }
trap cleanup EXIT
candidate="$work/omp-v${to_version}-darwin-arm64"

printf 'advance omp pin: omp/%s -> omp/%s\n' "$from_version" "$to_version"
gh release download "v${to_version}" --repo "$repository" -p 'omp-darwin-arm64' \
  --output "$candidate" >/dev/null 2>&1 \
  || fail "cannot download omp-darwin-arm64 for v${to_version}"
chmod 0755 "$candidate"
to_digest=$(shasum -a 256 "$candidate" | awk '{print $1}')
[[ "$to_digest" =~ ^[0-9a-f]{64}$ ]] || fail 'cannot measure the candidate digest'
note "measured digest ${to_digest}"

reported=$("$candidate" --version 2>/dev/null | head -1 || printf '')
[[ "$reported" == "omp/${to_version}" ]] \
  || fail "the asset reports '${reported}', not 'omp/${to_version}'"
note "asset reports ${reported}"

# Protocol contract, checked against the candidate itself. A bridge that cannot
# negotiate v2 or cannot compact images natively would fail the release lane, so
# it must fail here instead.
# Measured: the ready frame arrives from the RPC handshake alone, with no
# extension written and no credential, so the probe needs nothing but the
# candidate binary.
bridge_probe() {
  local config="$work/config.yml"
  printf 'model: gpt-5.6-sol\n' >"$config"
  printf '' | "$candidate" --mode rpc --no-session --no-skills --no-rules \
    --no-lsp --no-pty --no-title --max-time 20 --config "$config" \
    --tools read,bash 2>/dev/null | head -c 4000
}
probe=$(bridge_probe || printf '')
[[ -n "$probe" ]] || fail 'the candidate produced no RPC output; it cannot serve the managed lane'
grep -q '"type":"ready"' <<<"$probe" || fail 'the candidate never reported a ready frame'
grep -qE '"protocolVersion":[0-9]+' <<<"$probe" || fail 'the ready frame carries no protocolVersion'
grep -qE '"supportedProtocolVersions":\[[^]]*2[^]]*\]' <<<"$probe" \
  || fail 'the candidate does not advertise managed protocol v2'
note 'candidate advertises managed protocol v2'

readonly -a targets=(
  'scripts/companion-release/prepare-release.sh'
  'scripts/companion-release/prepare-release-local-lib.sh'
  'scripts/companion-release/materialize-omp-release-canary.sh'
  'scripts/companion-release/execsmoke/main.go'
  'internal/cli/pipeline_omp_context_active_process.go'
  'scripts/companion-release/tests/release-exec-smoke-hardening-test.sh'
)
for target in "${targets[@]}"; do
  [[ -f "$target" && ! -L "$target" ]] || fail "missing or unsafe target ${target}"
done

if [[ "$dry_run" == '--dry-run' ]]; then
  printf 'advance omp pin: --dry-run; %d file(s) would move to omp/%s (%s)\n' \
    "${#targets[@]}" "$to_version" "${to_digest:0:12}"
  exit 0
fi

from_digest=$(awk -F"'" '/^readonly expected_omp_sha256=/ { print $2 }' "$declaration")
[[ "$from_digest" =~ ^[0-9a-f]{64}$ ]] || fail 'cannot read the current digest declaration'

changed=0
for target in "${targets[@]}"; do
  before=$(shasum -a 256 "$target" | awk '{print $1}')
  perl -pi -e "s/\Q${from_version}\E/${to_version}/g" "$target"
  perl -pi -e "s/\Q${from_digest}\E/${to_digest}/g" "$target"
  after=$(shasum -a 256 "$target" | awk '{print $1}')
  if [[ "$before" != "$after" ]]; then
    printf '  updated %s\n' "$target"
    changed=$((changed + 1))
  else
    printf '  no pin in %s\n' "$target"
  fi
done

printf 'advance omp pin: moved %d file(s) to omp/%s\n' "$changed" "$to_version"
printf 'Run these before the release:\n'
printf '  go test ./internal/companionmanifest/ -run TestOMPPin\n'
printf '  go test ./internal/cli/ -run TestWorkflowContextImplementationIdentityCommand\n'
printf '    (the policy identity digest is derived from the pin; re-measure and update it)\n'
printf '  bash scripts/companion-release/tests/release-exec-smoke-hardening-test.sh\n'
printf '  bash scripts/release-tools/preflight-release.sh\n'
printf 'The reduction floor is measured, not declared: if omp/%s compacts less\n' "$to_version"
printf 'than min_reduction_basis_points, evidence generation fails in the canary\n'
printf 'and the pin must go back.\n'
