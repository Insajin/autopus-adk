# Release v0.50.115 (A26)

## SHIPPED 2026-09-05 (A26)

`v0.50.115` is published and immutable. The release workflow was green end to
end, the Homebrew Cask published from the exact A25 tap head, and the tag
ruleset was sealed after the push. The apply was operator-run (`sudo -v &&
scripts/release-tools/release-prep.sh --apply`) after an unattended
`--preflight`; the protected `release` job was approved after the seven-item
reviewer checklist from the v0.50.111 runbook passed.

| Coordinate | Value |
|---|---|
| Release | `383249963`, `draft=false`, `immutable=true`, 15 assets, published `2026-09-05T14:21:09Z` |
| Tag object | `058f0fd92cc9c73ea48f376a5ffdd2883ca11b99` (R2 `SHA256:7FISPXCi8p7cFEdh4Fcyyp8RPQbXYZwmo3Mxi5+YjrQ`) |
| Source commit | `77ae668bf7e9eb8d0dae177d1c9b7e41a5d51ef6` |
| Source tree | `e353a4bc1c3e3cca217d40751095fdc8b9a3a3da` |
| Evidence tag object | `6b1e28f41c0566688c73c4d58add5dbeedb74a0d` (commit `357b1176`, tree `71f3cfea`) |
| Report sha256 | `590f4ef94894cff1851ce4b617e708939a8faeb40942791599cf2a5739f2edec` |
| Attestation sha256 | `9f41e402ced2fa9d4a06767e7c90c0987f29dfb01b315f0f981b4a9b3f6b8ca6` (K3 `omp-context-promotion-2026-q3-k3`) |
| Static policy sha256 | `d286edbd9b01197d0e21f34032264d5286c6aedbbd84647425952ea5cece8cc1` |
| Homebrew tap | `61ce41f93ecf` "Publish signed Cask for v0.50.115" (predecessor `7ea4a82e70fa`, cask blob `01f5123f`) |
| Tag ruleset `22334869` | `autopus-v0.50.115-release-authority`, sealed (`bypass_actors == []`), verifier `--sealed` passes |
| Deployment tag policy | `59182501` (`v0.50.115`) on `adk-companion-release` |
| OMP pin | `omp/17.2.7` — cohort 42/42, 20 tasks / 40 observations, 14/14 gates |
| Evidence | provider `openai-codex`, token reduction `2335`bp (floor 2000), compaction admissions `8/20` (floor 2/20), sessions 2+2, contamination 0 |

## What this release carries

- SPEC-OMP-006: `orchestra.providers.<name>.backend: omp` runs SPEC review
  providers in private read-only OMP RPC sessions with the judge stage,
  `get_state.dumpTools` allowlist verification, provider-error classification
  and same-pin retry, and executed-model drift rejection.
- SPEC-OMP-005 role projection (`autopus_<agent>` modelRoles) and the doctor
  fix that stopped flagging `model: '@autopus_<agent>'` as malformed.
- SPEC-OMP-002/003/004 lifecycle closure on exact HEAD evidence.
- Five skills adapted from mattpocock/skills (grilling, domain-modeling,
  codebase-design, wait-what, resolving-merge-conflicts) plus the debugging,
  tdd, review, and writing-skills revisions, refined after a first trial run.
- The process-heavy flake root cause (macOS first-execution evaluation of new
  fixture executables) fixed in `pkg/processprobe`.

## Coordinate move notes (A25 -> A26)

`advance-release-coordinate.sh v0.50.114 A25 v0.50.115 A26` measured A25 as
published and appended the A26 phase beside it. Every A25 predecessor pin was
measured from release `382345734` (checksums, four archives, two darwin
manifests, tag object, commit, tree). Two drifts surfaced and were fixed in the
same commit: the `upgrade-canary.yaml` predecessor tree pin had stayed at the
A22 value across two moves, and fourteen `release-homebrew-hardening-test.sh`
failure labels still said A24.

The armed ruleset and the deployment tag policy were created before preflight
with the same shape as `22169931`; prep sealed the ruleset after the tag
existed.

## Next release

- Run `advance-release-coordinate.sh v0.50.115 A26 v0.50.116 A27` and add the
  A26 predecessor pins by measurement from release `383249963`.
- Include the `upgrade-canary.yaml` predecessor tree pin in the measurement
  table; it is not covered by the replace list.
- The OMP pin stays at `omp/17.2.7` until `docs/runbooks/omp-pin-advance.md`
  clears an 18.x candidate at the cohort.
