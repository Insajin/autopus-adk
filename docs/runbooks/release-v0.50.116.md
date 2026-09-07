# Release v0.50.116 (A27)

## SHIPPED 2026-09-06 (A27)

`v0.50.116` is published and immutable. The protected `release` run
`34019224937` was green end to end (tag CI, security scan, evidence
verification, build, signing, notarization, immutable asset verification,
Homebrew Cask publication). The apply was operator-run (`sudo -v &&
scripts/release-tools/release-prep.sh --apply`) after an unattended
`--preflight`; the protected job was approved after the seven-item reviewer
checklist from the v0.50.111 runbook passed. The public upgrade canary
`34020839570` admitted the exact public signed `0.50.115 -> 0.50.116` upgrade
on Darwin/arm64.

| Coordinate | Value |
|---|---|
| Release | `383500138`, `draft=false`, `immutable=true`, 15 assets, published `2026-09-06T08:00:33Z` |
| Tag object | `39101f97302267d052ed18b8aafa9ec23278c091` (R2 `SHA256:7FISPXCi8p7cFEdh4Fcyyp8RPQbXYZwmo3Mxi5+YjrQ`) |
| Source commit | `fbe502c05f84d5eeb81b089b2344c47329ab4543` |
| Source tree | `95ba04d8499b00af86f6794e38a4da1b6d497a6d` |
| Evidence tag object | `c3666d3f41179e2a8f9ca45fd08a130d64769f70` |
| Report sha256 | `d0f3cb4777f37a8e83da3f8640adb7338ca5e54ae372eaf15e530af41fa99d0e` |
| Attestation sha256 | `eed4aab7705dc460d84bee177066d3ab3d773dc09ab0800d201046bf4fddd4f6` (K3 `omp-context-promotion-2026-q3-k3`) |
| Checksums sha256 | `8c57e0a9aa7cf86a7b2b4fe4f41e5982d0f04eb501125ec92fc45ae9715adac1` |
| Darwin arm64 archive | `f3c7b2d148b370a7d200a3474ceb342f933c3adead08eca0044f63fbc8ed30bf` |
| Homebrew tap | `6a53f34d00bf` "Publish signed Cask for v0.50.116" (predecessor `61ce41f93ecf`, cask blob `1961f98855dd`) |
| Tag ruleset `22344447` | `autopus-v0.50.116-release-authority`, sealed (`bypass_actors == []`), verifier `--sealed` passes |
| Deployment tag policy | `59198991` (`v0.50.116`) on `adk-companion-release` |
| OMP pin | `omp/17.2.7` — cohort 42/42, 20 tasks / 40 observations, 14/14 gates |
| Evidence | provider `openai-codex`, token reduction `2335`bp (floor 2000), compaction admissions `8/20` (floor 2/20) |

## What this release carries

- Standard balanced role placement unified across OMP, Claude Code, and
  Codex native agents (seven-role Fable 5.1/max and Astra/max core).
- `auto quality` interactive OMP model menu.
- OMP readiness fast-exit RPC race fix.
- `auto update` target and progress reporting that names OMP before any
  platform completes.
- Rule reclassification (#185), doctor Desktop shim diagnosis (#164),
  brainstorm mutation guard (#108), spec gate applicability and probe (#186).

## Coordinate move notes (A27 -> A28)

`advance-release-coordinate.sh v0.50.116 A27 v0.50.117 A28` measured A27 as
published and appended the A28 phase beside it. Every A27 predecessor pin was
measured from release `383500138` (checksums, four archives, two darwin
manifests, tag object, commit, tree) and from the live tap head `6a53f34d`
(cask blob `1961f988` reproduced byte-for-byte by `render_homebrew_cask`).
The `upgrade-canary.yaml` predecessor block was re-measured to A27 as the
v0.50.115 runbook asked. `verify-public-key-lineage.sh` holds three
accumulating phase lists that are in neither the replace nor the review list
of the advance script; A28 was appended by hand and the script should list
that file under `review_targets`.

## Next release

- Run `advance-release-coordinate.sh v0.50.117 A28 v0.50.118 A29` and add the
  A28 predecessor pins by measurement from the v0.50.117 release.
- Include `verify-public-key-lineage.sh` and the `upgrade-canary.yaml`
  predecessor block in the measurement table.
- The OMP pin stays at `omp/17.2.7` until `docs/runbooks/omp-pin-advance.md`
  clears an 18.x candidate at the cohort.
