# Release v0.50.117 (A28)

## SHIPPED 2026-09-07 (A28)

`v0.50.117` is published and immutable. The protected `release` run
`34082844951` was green end to end (tag CI, security scan, evidence
verification, build, signing, notarization, immutable asset verification,
Homebrew Cask publication). Two earlier apply attempts failed before any tag
existed and left no remote state: `go.sum` lacked chroma's test-dependency
hashes, and the operator's Codex default model `gpt-6-astra` is absent from
the pinned `omp/17.2.7` catalog. Both gates now live in the tooling this
release carries — `preflight-release.sh` runs `go mod tidy -diff`, and
`release-prep.sh` checks the pinned catalog before `sudo` and honours
`ADK_RELEASE_MODEL`. The successful apply ran on `gpt-5.6-sol`. The public
upgrade canary `34086372717` admitted the exact public signed `0.50.116 ->
0.50.117` upgrade on Darwin/arm64.

| Coordinate | Value |
|---|---|
| Release | `383826825`, `draft=false`, `immutable=true`, 15 assets, published `2026-09-07T05:18:04Z` |
| Tag object | `edcd4eb98f4e52e8ff868d1f8440cfa0d0c5bc3a` (R2 `SHA256:7FISPXCi8p7cFEdh4Fcyyp8RPQbXYZwmo3Mxi5+YjrQ`) |
| Source commit | `620e29a44d004cb199d5f1c22ae92878f9b6930e` |
| Source tree | `fce0d047ae8cf5c519762fe4ebf530e7ccc01ed1` |
| Evidence tag object | `62fa28194cbe146d2f1377cab9659e2ce3849985` |
| Report sha256 | `6c7213d0476124fc707d46dcc0b4601084654c28e13df1a62506b871b347efce` |
| Attestation sha256 | `e0c48a051ce61cf0882c595dd15ac955a2f9c1cf1cf51825f2a9e11e972b7573` (K3 `omp-context-promotion-2026-q3-k3`) |
| Checksums sha256 | `1a50d39789026708cea1f4d0352f14c244b59bbeddb0afaa52463970ce2684ef` |
| Darwin amd64 archive | `0f0fea1f7f16f61f049ebf2ad6d9af789ad8380187d85718ff75d031c538432a` |
| Darwin arm64 archive | `3a2dcdf7d0e89ca93dae32f6560b096f3c20903f8a7a7a8407a53f017c50261c` |
| Linux amd64 archive | `f44896aef208b2dc819355a7cd89ec3d62c58bf26fe8bccf06528d7da1a08117` |
| Linux arm64 archive | `f084d2a4526c295d876b46db3638101a79f44a7344843d487a8db3a83f12e9f7` |
| Darwin amd64 manifest | `9d7e50b0303048707ebde925cfdc2cad95ba411b9069b3832198d6cd7a1b76f4` |
| Darwin arm64 manifest | `78d8d2064ef11f69eab9c29c809d0441f7034db32927415d690cb58bb6c1578c` |
| Homebrew tap | `f6b06e4ad58c` "Publish signed Cask for v0.50.117" (predecessor `6a53f34d00bf`, cask blob `99df4d8bdc20`) |
| Frozen Formula blob | `4ebc6c38925002dec00759823d4dd847a499818a` (unchanged since v0.50.71) |
| Tag ruleset `22415096` | `autopus-v0.50.117-release-authority`, sealed (`bypass_actors == []`), verifier `--sealed` passes |
| Deployment tag policy | `59291706` (`v0.50.117`) on `adk-companion-release` |
| OMP pin | `omp/17.2.7` — cohort 40/40, 20 tasks / 40 observations, 14/14 gates |
| Evidence | provider `openai-codex`, token reduction `2335`bp (floor 2000), compaction admissions `8/20` (floor 2/20) |

## What this release carries

- Comment-aware 300-line file limit. The size gate judges code lines instead
  of physical lines: `pkg/linecount` identifies comment-only lines for 28
  extensions with the Chroma lexer, treats comment markers inside strings,
  template literals, regexes, and heredocs as content, and falls back to the
  physical count when lexing fails so no file is ever reported smaller than it
  is. `auto check --gate`, `cmd/source-lines`, `preflight-release.sh`, and the
  hardening tests all apply that rule.
- Review verdict consistency (#187). `REVISE`/`REJECT` must carry
  `blocking_reasons`, and blocking is decided by finding category —
  `correctness`, `contract`, `data_loss`, `security` block regardless of
  severity, while `style`, `suggestion`, `improvement` stay advisory. An input
  identical to the previous revision ends as `loop_status=awaiting_changes`
  without calling a provider.
- Compact change contract (#187). `auto spec change` records one `change.md`
  against an existing SPEC and AC id; risk grading routes low-risk work to
  `compact_contract` and sends `security_or_data`, `multi_domain`, or new APIs
  to `escalate_to_full_spec`.
- Gate applicability (#186 AC1). `auto spec gates --change-class --json`
  reports per-gate `required`/`not_applicable`/`blocked` with reasons, and the
  safety gates stay required in every change class.

## Coordinate move notes (A28 -> A29)

`advance-release-coordinate.sh v0.50.117 A28 v0.50.118 A29` measured A28 as
published and appended the A29 phase beside it; it reported `present` for
every history and phase-list file and no MANUAL item. Every A28 predecessor
pin was measured from release `383826825` (checksums, four archives, two
darwin manifests, tag object, commit, tree) and from the live tap head
`f6b06e4a` (cask blob `99df4d8b` reproduced byte-for-byte by
`render_homebrew_cask`; the blob at the parent commit `6a53f34d` equals the
outgoing `PRIOR_CASK_BLOB`, so the pin chain is continuous). The
`upgrade-canary.yaml` predecessor block was re-measured to A28 as the
v0.50.116 runbook asked. `verify-public-key-lineage.sh` is now carried by the
script as a phase-keyed accumulator (`phase_list_files`), so its three lists
were prepared by hand before the run rather than discovered afterwards. The
homebrew hardening test's current-release labels had drifted at A28 — the
`fail 'A27 accepted …'` arms were never bumped — and were repaired to A29
together with their sibling assertions.

## Next release

- Run `advance-release-coordinate.sh v0.50.118 A29 v0.50.119 A30` and add the
  A29 predecessor pins by measurement from the v0.50.118 release.
- Include `verify-public-key-lineage.sh` and the `upgrade-canary.yaml`
  predecessor block in the measurement table.
- A29 is the omp/18.1.13 measurement attempt: the pin moved off `omp/17.2.7`
  for the first time since A25, so the `release-prep.sh --apply` that ships
  v0.50.118 doubles as the cohort measurement `docs/runbooks/omp-pin-advance.md`
  needs to clear an 18.x candidate. If the cohort fails closed before a tag
  exists, the coordinate survives and the pin goes back to `omp/17.2.7`, as it
  did for A25.
- `ADK_RELEASE_MODEL` must name a model the pinned catalog lists.
