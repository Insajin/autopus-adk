#!/usr/bin/env bash
# Prep-input checks for preflight-release.sh, split out to keep both files under
# the 300-line source limit. Sourced, not executed: it relies on the caller's
# pass/warn/bad/section helpers and on $work being an existing temp directory.

check_release_prep_inputs() {
  section "release prep inputs"
  # prepare-release.sh cannot start without these, and it discovers them one at a
  # time deep into its own argument parsing. Checking here costs a second.
  #
  # The OMP check deliberately does not look at `command -v omp`. Prep consumes the
  # published release asset, not whatever a package manager installed: a Homebrew
  # build of the same version hashes differently, so comparing against it reports a
  # mismatch that means nothing. What matters is that the pinned digest still
  # describes the upstream asset for the pinned version.
  omp_pin=$(awk -F"'" '/^readonly expected_omp_sha256=/ { print $2 }' \
    scripts/companion-release/prepare-release.sh)
  omp_version_pin=$(awk -F"'" "/verified OMP version differs/ { print \$2 }" \
    scripts/companion-release/prepare-release.sh | head -1)
  omp_tag="v${omp_version_pin#omp/}"
  if [[ -z "$omp_pin" || -z "$omp_version_pin" ]]; then
    bad 'cannot read the OMP version and digest pins from prepare-release.sh'
  elif sums=$(gh release download "$omp_tag" --repo can1357/oh-my-pi \
    -p 'SHA256SUMS.txt' -D "$work" --clobber 2>/dev/null && cat "$work/SHA256SUMS.txt"); then
    upstream=$(awk '$2 == "omp-darwin-arm64" { print $1 }' <<<"$sums")
    if [[ "$upstream" == "$omp_pin" ]]; then
      pass "OMP pin ${omp_version_pin} matches the upstream darwin-arm64 asset"
    else
      bad "OMP pin ${omp_pin:0:12}… differs from upstream ${upstream:0:12}… for ${omp_tag}"
    fi
    latest=$(gh api repos/can1357/oh-my-pi/releases/latest --jq .tag_name 2>/dev/null || printf '')
    if [[ -n "$latest" && "$latest" != "$omp_tag" ]]; then
      warn "upstream latest is ${latest}; the pin is ${omp_tag}. Advancing the pin changes the evidence oracle, so decide it rather than drift into it"
    fi
  else
    bad "cannot read upstream SHA256SUMS.txt for ${omp_tag}"
  fi

  # Signing keys are checked by public half only; nothing secret is read out.
  #
  # The default store is checked before reporting anything missing. A release was
  # nearly re-engineered around a "destroyed" key that was sitting here, so this
  # looks before it believes.
  readonly key_store="${HOME}/.config/autopus/release-keys"
  readonly k3_public='YkTuNcfWGTLgTglPmZq/Dj4OXwcoUwnkM2ExIGIz+jM='
  readonly r2_fingerprint='SHA256:7FISPXCi8p7cFEdh4Fcyyp8RPQbXYZwmo3Mxi5+YjrQ'

  r2_key="${ADK_TAG_SIGNING_KEY:-$key_store/release-tag-signing-2026-q3-r2}"
  if [[ ! -f "$r2_key" || -L "$r2_key" ]]; then
    bad "R2 tag signing key not found at $r2_key"
  elif [[ "$(ssh-keygen -y -f "$r2_key" 2>/dev/null | ssh-keygen -lf - -E sha256 2>/dev/null | awk '{print $2}')" == "$r2_fingerprint" ]]; then
    pass 'R2 tag signing key present and matches the pinned fingerprint'
  else
    bad "key at $r2_key is not R2"
  fi

  key_path="${ADK_PROMOTION_SIGNING_KEY:-$key_store/omp-context-promotion-2026-q3-k3.b64}"
  if [[ -z "$key_path" ]]; then
    warn 'ADK_PROMOTION_SIGNING_KEY is unset and no key is in the default store'
  elif [[ ! -f "$key_path" || -L "$key_path" ]]; then
    bad "ADK_PROMOTION_SIGNING_KEY does not name a regular file"
  else
    # Pure shell on purpose. An embedded heredoc broke the moment this block was
    # moved into a function, because the terminator has to sit at column zero.
    # The key file is base64 of a 64-byte Ed25519 pair; the public half is the
    # trailing 32 bytes, and only that is ever printed.
    derived=$(base64 -d <"$key_path" 2>/dev/null | tail -c 32 | base64)
    if [[ "$derived" == "$k3_public" ]]; then
      pass 'ADK_PROMOTION_SIGNING_KEY is the K3 promotion key'
    else
      bad 'ADK_PROMOTION_SIGNING_KEY is not the K3 promotion key'
    fi
  fi

  # The last two inputs are operator infrastructure, not repository state, so they
  # are reported rather than demanded: prep refuses a public endpoint and the
  # credential must never appear in a command line.
  omp_staged="${HOME}/.cache/autopus/release/omp-v${omp_version_pin#omp/}-darwin-arm64"
  if [[ -f "$omp_staged" && "$(shasum -a 256 "$omp_staged" | awk '{print $1}')" == "$omp_pin" ]]; then
    pass "verified OMP binary staged at $omp_staged"
  else
    warn "no verified OMP binary staged; download the pinned asset before prep"
  fi

  credential_names=$(env | awk -F= '/^AUTOPUS_OMP_CONTEXT_PROVIDER_/ { print $1 }')
  if [[ -n "$credential_names" ]]; then
    pass "provider credential locator present: $(tr '\n' ' ' <<<"$credential_names")"
  else
    warn 'no AUTOPUS_OMP_CONTEXT_PROVIDER_* variable is exported; load the credential out of band'
  fi

  if [[ -n "${ADK_GATEWAY_URL:-}" ]]; then
    if [[ "$ADK_GATEWAY_URL" =~ ^http://127\.0\.0\.1:[1-9][0-9]{0,4}$ ]]; then
      if curl -sS -o /dev/null --max-time 5 "$ADK_GATEWAY_URL" 2>/dev/null; then
        pass "gateway answers at $ADK_GATEWAY_URL"
      else
        bad "gateway at $ADK_GATEWAY_URL does not answer"
      fi
    else
      bad 'ADK_GATEWAY_URL is not an exact loopback HTTP URL; prep refuses anything else'
    fi
  else
    warn 'ADK_GATEWAY_URL is unset; start the approved loopback gateway and export it to check it here'
  fi
}
