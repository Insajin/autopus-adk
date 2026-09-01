# Autopus ADK v0.50.112 Release

## Purpose and stop conditions

This document is a procedure, not proof that v0.50.112 evidence or a release already exists. Record every live result during execution. Stop on any mismatch. Never repair a mismatch by moving, deleting, overwriting, or reusing an existing tag, evidence tag, release, or predecessor coordinate.

A24 is the first phase whose direct predecessor is a *successful* release since A22. Its lineage points at A23/v0.50.111, which published fifteen assets on 2026-08-31 including the promotion report, the promotion attestation, and the signed release lineage. That is verified below, not assumed.

## Fixed coordinates

| Coordinate | Required value |
|---|---|
| Repository | `Insajin/autopus-adk` |
| Release phase | `A24` |
| Release tag / ref | `v0.50.112` / `refs/tags/v0.50.112` |
| Version | `0.50.112` |
| Workflow trigger | only `refs/tags/v0.50.112` |
| Evidence tag / ref | `omp-context-evidence-v0.50.112` / `refs/tags/omp-context-evidence-v0.50.112` |
| Prep-lock ref | `refs/heads/omp-context-evidence-v0.50.112-source` |
| Release tag ruleset | `autopus-v0.50.112-release-authority` |
| Protected environment | `adk-companion-release` |
| Release operator user ID | `204883817` |
| Tag signer | R2, `SHA256:7FISPXCi8p7cFEdh4Fcyyp8RPQbXYZwmo3Mxi5+YjrQ` |
| Active promotion signer | `omp-context-promotion-2026-q3-k3` |
| Active policy ID | `omp-context-active-v1` |
| Go toolchain | `1.26.6` |
| OMP oracle | `omp/17.2.7`, with the source-pinned executable digest |

The A24 source commit and tree are not filled in here. Freeze them from the exact clean `origin/main` used for the release. All later coordinates must bind those observed values.

The candidate is deliberately not pinned here. An earlier draft named a commit, and the next commit to this very file made that value stale — a pinned candidate in a living document is a value that invalidates itself and then misleads whoever reads it next. Derive it instead, on a clean tree:

```sh
git fetch origin && git status --short   # must be empty
git rev-parse origin/main origin/main^{tree}
git log v0.50.111..origin/main --oneline | wc -l
```

Freeze that commit and tree into the workflow inputs and every later coordinate. The predecessor pins in the next section are the opposite case: they describe immutable published history, so they are written down.

## Immutable v0.50.111 predecessor

A24's direct release predecessor is the immutable v0.50.111 release. The lineage gate must match all of these public pins. Every value below was read from the live tag and the published release assets, not copied from the A23 runbook.

| Predecessor coordinate | Required value |
|---|---|
| Tag | `v0.50.111` |
| Tag object | `b751c5beba4374534b1a73615ff0d6d57bdb4131` |
| Source commit | `954f60a77acb59fd4106537020693fdcadb3d640` |
| Source tree | `fcd3f2aed498955235ae7807ba031d32a053db09` |
| GitHub release ID | `379595447` |
| Published at | `2026-08-31T08:36:03Z`, `draft=false`, `prerelease=false` |
| Darwin arm64 archive API SHA-256 | `a0a06284a86dfaf2175b9c8114dc6f5c72bdf4553637605455b44f85cf59973b` |
| Lineage upstream digest U | `fed7ee0fa4bfd47d3f60d983e2ce1a4b10e0d6aee1b9464be02b31cad0e27817` |
| Lineage executable digest D | `5a9ef6b41fea1a3b794288f069f344275850905b743a8095b0eeec41f69decfb` |
| Lineage signing key | `adk-release-2026-q3-b0`, ed25519 |
| Evidence tag | `omp-context-evidence-v0.50.111` |

Run the committed lineage verifier and require it to identify A24 → A23 with these exact values. The v0.50.111 release ID, tag, and asset bytes are immutable history and are predecessor proof only: the v0.50.112 lane must not copy, rename, re-upload, or otherwise reuse any v0.50.111 asset.

The companion manifest and signature digests are not restated here. Read them from the A23 release at execution time and bind the observed values; restating them from a prior runbook is how a stale digest survives a rotation.

### Inputs the lineage gate needs, and what it does not prove

`VerifyOMPContextReleaseLineage` requires a `TrustedPublicKeyReceipt`, so the gate needs three inputs and not two:

| Input | Source |
|---|---|
| `release-lineage-v1.json` + `release-lineage-v1.sig` | published v0.50.111 assets |
| A0 receipt bundle: receipt bytes + signature | produced by release prep from the operator's Ed25519 key, then opened through the fd-pinned transaction in `pkg/companionmanifest/public_key_receipt_bundle_unix.go` |
| A0 anchor pins | committed at `pkg/companionmanifest/public_key_receipt_trust.go:15-18` |

The anchor is four committed SHA-256 pins over the receipt, its signature, the public key, and the record. They are immutable source, so the gate cannot be satisfied by substituting a different receipt.

State the boundary plainly: the lineage signing key `adk-release-2026-q3-b0` is published in no A22 or A23 asset and is not in the source tree. It appears only as the `key_id` field of the artifact it signs. The v0.50.109 rotation sidecar carries the channel, tag, and promotion keys, not this one. Therefore the A23 lineage signature is verifiable by the release operator holding the A0 bundle, and is not independently verifiable by a third party from published assets alone.

That is a boundary of the current trust chain, not a defect this release must fix. Do not weaken the gate to route around it, and do not describe the lineage signature as publicly verifiable in release notes.

The committed verifier is `scripts/companion-release/ompcontextlineageverify`. Every flag is required — there is no coordinate-only mode, so `--receipt-bundle` cannot be omitted to get a partial check. The invocation below has every publicly derivable pin already filled from the A23 assets, leaving only the operator's own inputs:

```sh
go run ./scripts/companion-release/ompcontextlineageverify \
  --lineage release-lineage-v1.json \
  --signature release-lineage-v1.sig \
  --receipt-bundle "$A0_RECEIPT_BUNDLE" \
  --key-id adk-release-2026-q3-b0 \
  --handoff v1 \
  --minimum-rollback-floor "$COMPANION_ROLLBACK_FLOOR" \
  --upstream-sha256 fed7ee0fa4bfd47d3f60d983e2ce1a4b10e0d6aee1b9464be02b31cad0e27817 \
  --executable-sha256 5a9ef6b41fea1a3b794288f069f344275850905b743a8095b0eeec41f69decfb \
  --source-repository Insajin/autopus-adk \
  --source-commit 954f60a77acb59fd4106537020693fdcadb3d640 \
  --source-tree fcd3f2aed498955235ae7807ba031d32a053db09 \
  --target darwin-arm64 \
  --version 0.50.111
```

Download the two lineage files from the v0.50.111 release rather than reusing a local copy, so the check reads what the public sees.

## Burned failure coordinates

`v0.50.110` remains failed release history. Its release tag and `omp-context-evidence-v0.50.110` evidence tag exist and must never be moved, deleted, recreated, or reused. GitHub release ID `379549016` remains an unpublished failed draft. No v0.50.110 asset was published.

A24 maps only to `v0.50.112`. It requires a new one-shot evidence object and a fresh K3 report and attestation. Never copy or reuse any v0.50.110 or v0.50.111 report, attestation, evidence commit, evidence tag object, draft, or workflow artifact.

## Active K3 policy and fresh evidence

The active static policy is the sole authority that selects the promotion signing key and provider authority. Follow the A23 procedure section of `release-v0.50.111.md` unchanged for policy verification, one-shot evidence generation, and attestation, with these observed A23 values as the shape to expect rather than values to copy:

| Field | A23 observed value |
|---|---|
| `policy.policy_id` | `omp-context-active-v1` |
| `policy.history_mode` | `active` |
| `policy.memory_mode` | `off` |
| `policy.min_pair_count` | `20` |
| `policy.min_reduction_basis_points` | `2000` |
| `provider` | `openai-codex` |
| `runtime.omp_version` | `omp/17.2.7` |
| `runtime.execution_class` | `external-live` |

Generate a fresh report and attestation for A24. A copied `evidence_id`, `challenge_digest`, or `order_seed` fails the one-shot property that makes the evidence meaningful.

## Prerequisites

Follow `release-v0.50.111.md` "Prerequisites" unchanged, plus these A24-specific gates.

### Release-authority ruleset

`autopus-v0.50.112-release-authority` now exists as ruleset `21986875`, created for this release. The rulesets present are:

| Ruleset | ID |
|---|---|
| `autopus-v0.50.109-release-authority` | `21713201` |
| `autopus-v0.50.109-rotation-ref-authority` | `21713791` |
| `autopus-v0.50.110-release-authority` | `21901908` |
| `autopus-v0.50.111-release-authority` | `21909571` |
| `autopus-v0.50.112-release-authority` | `21986875` |

Never widen an existing ruleset's ref pattern to cover a new tag: a per-version ruleset is what makes the authority boundary auditable.

The seal is two phases, which the A23 runbook does not state and which the A23 ruleset history proves. Ruleset `21909571` was created at 16:50:54 with the release operator as an `always` bypass actor, the tag was created at 17:07:11, and the ruleset was updated at 17:07:19 — eight seconds later — to remove the bypass. Reading only the current ruleset shows the sealed end state, and creating a new ruleset in that shape blocks the operator's own tag push.

| Phase | `bypass_actors` | When |
|---|---|---|
| Open | `[{actor_id: 204883817, actor_type: "User", bypass_mode: "always"}]` | before tag push |
| Sealed | `[]` | immediately after the tag exists |

Ruleset `21986875` was created in the open shape and field-compared against the A23 creation-time state: target, enforcement, ref pattern, bypass actor, and the creation/update/deletion rule set all match with only the version string differing. Sealing it is a step of this procedure, not a follow-up: an unsealed release tag is mutable by the operator, which is the property the ruleset exists to remove.

### Operator-held material, and why the procedure stops without it

Everything the operator must bring reduces to key material. Two key sets, and the rest is generated from them — so this is one blocker wearing four hats, not four hunts.

| Input | Origin | State observed while writing this runbook |
|---|---|---|
| R2 tag signing private key | operator-held SSH key; `SHA256:7FISPXCi8p7cFEdh4Fcyyp8RPQbXYZwmo3Mxi5+YjrQ`, extracted from the v0.50.111 tag signature and matching `scripts/companion-release/release-tag-signing-2026-q3-r2.fingerprint` | absent; the only local key is `SHA256:GIa3VFVWeCqBJamdDCJdxDzXE+XxQBC1xsx/iZ9XlF8` and `ssh-add -l` reports no identities |
| `COMPANION_SIGNING_KEY_FILE` | operator-held Ed25519 private key, mode 0600 | absent |
| Nine `COMPANION_*` variables | operator-supplied; validated by `bash scripts/companion-release/validate-environment.sh` | absent; fails at `COMPANION_BUILD_PROVENANCE` |
| A0 receipt bundle | **generated, not located.** `auto companion-manifest public-key-receipt --key-file … --bundle-output …`, driven by `produce_public_key_receipt_bundle` in `scripts/companion-release/produce-public-key-receipt.sh` | cannot be produced here; no key file |

The last row was worth checking rather than assuming. An earlier draft of this runbook told the operator to locate the A0 bundle, which would have sent them looking for a file that release prep creates. Nothing is missing from disk; the key that signs it is missing.

Signing the tag with a different key is not a smaller version of this release. The v0.50.109 rotation established R2 through a signed sidecar, and every consumer that checks a v0.50.112 tag will check it against R2. A tag signed by another key is a tag that fails verification for everyone downstream while looking finished locally.

So the procedure runs to the tag boundary and stops there. Everything before it — ruleset, content gates, lineage inputs, evidence plan — is repository work and is done or specified. The tag, the evidence signing, and the workflow trigger need the operator.

### Content gates

Freeze `origin/main` only after all of these pass on the exact release commit.

| Gate | Command | Required result |
|---|---|---|
| Format | `gofmt -l pkg internal cmd templates` | empty |
| Generated surfaces | `go run ./cmd/generate-templates` then `git status --short` | clean |
| Source file size | every `*.go` under `pkg`, `internal`, `cmd` | under 300 lines |
| Test lane 1 | `go test ./... -count=1 -timeout 25m -skip "$SKIP"` | zero failures |
| Test lane 2 | `go test ./... -count=1 -timeout 10m -p 1 -run "$SKIP"` | zero failures |

`$SKIP` is the process-heavy isolation list in `Makefile`. Both lanes are required; a single `./...` run is not a substitute, because the skip list exists to keep process-heavy tests off the shared parallel scheduler. The split is exhaustive and disjoint, measured on this tree: 8982 top-level tests = 204 isolated + 8778 parallel, zero overlap.

Known environment collision, not a release blocker: `pkg/connect`'s `TestWaitForCallback_Timeout` binds `127.0.0.1:1455` and fails when another process holds that port. Confirm with `lsof -nP -iTCP:1455` before attributing it to the release, and re-run the package once the port is free. Do not skip it silently.

### The harness applied to itself

`./bin/auto qa release --profile release-candidate` was run against this repository as a preflight. Result on the frozen candidate:

| Lane | Policy | Verdict |
|---|---|---|
| `fast` | must | pass |
| `browser-staging`, `desktop-native`, `gui-explore`, `mobile-readiness` | deferred | warn, deferred |
| `canary-explicit` | must | block, `setup_gap:canary-template` |
| `evidence-dashboard` | optional | warn |

Two findings came out of that run and are fixed on the candidate: the `adk-go-fast` pack ran a bare `go test ./...` under a 600s budget and could only ever time out, and `pkg/orchestra`'s `TestExecute_ExportsSessionEnvWhenHookMode` failed under parallel load because a one-second bound raced pane setup. The pack is now split to mirror the Makefile lanes and the test no longer depends on machine speed.

`canary-explicit` remains an open project decision, not a defect. The adapter refuses to guess — its gap message is `explicit safe canary command is required` — and there is no pack for that lane in this repository. The operator verifier `scripts/companion-release/verify-current-release.sh` is not a candidate: it requires ten pinned inputs including `COMPANION_PUBLIC_KEY_SHA256` and three prebuilt verifier binaries, so wiring it here would make every RC gate run depend on operator-held material. Choose the command deliberately and declare it in a pack; inventing one to turn the lane green would produce exactly the evidence-free pass this profile exists to refuse.

## Release content

A24 carries the QAMESH work landed after v0.50.111, plus this release's own runbook and the two preflight fixes it produced. Enumerate with `git log v0.50.111..origin/main --oneline`; the count is not written down here for the same reason the candidate commit is not.

Derive the scope from that range, not from `CHANGELOG.md` `[Unreleased]`. The changelog's released sections stop at `[v0.50.10]`, so `[Unreleased]` accumulates roughly two hundred entries spanning versions that already shipped. It is the right place to read *wording* for an entry and the wrong place to read *scope*. Cross-check that every commit in the range has a matching changelog entry; a commit with no entry is a stop, because the release description is the only place a user learns what changed.

The load-bearing items are:

- `auto qa scenario`: project-declared user scenarios compiled into runner specs, with a closed read-only step vocabulary.
- `desktop-native` observes the app a Journey Pack names, replacing a fixture conformance path that no application could satisfy. Verified live against a third-party app.
- Release evidence chain repair: `redactReleaseString` expanded `$PROJECT_ROOT` as a nonexistent capture group, so a release run under an absolute `/Users` path recorded empty `run_index_path` and `manifest_paths` while reporting `warn` and exit 0.
- Credential-URL redaction extended to every scheme; seven leaked while the gate reported `passed`.
- Unknown `--lane` now refuses instead of running auto-detected journeys and reporting a pass.

The third item matters for this release specifically: releases cut before v0.50.112 may carry release indexes with an empty evidence chain. That is prior history and must not be repaired retroactively; note it in the release description rather than editing any published artifact.

## Remaining procedure

Sections "Static remote coordinates" through "Reconciliation and rollback boundaries" in `release-v0.50.111.md` apply unchanged, with `v0.50.111` replaced by `v0.50.112`, `A23` by `A24`, and the predecessor table above substituted for the v0.50.109 predecessor table. Do not paraphrase those sections here; a second copy is a second thing to drift.

## Exact asset gate

A23 published fifteen assets. A24 must publish the same fifteen names with the version substituted, no more and no fewer. Read from the A23 release rather than trusting this list, then compare:

```sh
gh release view v0.50.111 --json assets --jq '[.assets[].name]|sort' |
  sed 's/0\.50\.111/0.50.112/g' > /tmp/a24-expected.json
gh release view v0.50.112 --json assets --jq '[.assets[].name]|sort' > /tmp/a24-actual.json
diff /tmp/a24-expected.json /tmp/a24-actual.json && echo ASSET_SHAPE_OK
```

The expected names, for review before the release exists:

| Group | Names |
|---|---|
| Archives | `autopus-adk_0.50.112_{darwin,linux}_{amd64,arm64}.tar.gz`, `autopus-adk_0.50.112_windows_{amd64,arm64}.{tar.gz,zip}` |
| Checksums and signatures | `checksums.txt`, `checksums.txt.bundle`, `checksums.txt.signatures` |
| Evidence | `omp-context-promotion-report.v1.json`, `omp-context-promotion-attestation.v2.json` |
| Lineage | `release-lineage-v1.json`, `release-lineage-v1.sig` |

An extra asset is as much a stop as a missing one: it means the workflow uploaded something this procedure did not describe.

## Stop conditions specific to A24

Stop and escalate rather than improvising if any of these hold.

1. `autopus-v0.50.112-release-authority` (ruleset `21986875`) is missing at tag-push time, or still carries the operator bypass after the tag exists. An unsealed release tag is mutable by the operator, which is the property the ruleset removes.
2. The A0 receipt bundle cannot be produced, or the lineage verifier does not identify A24 → A23 with every predecessor pin above. Skipping the gate is not the smaller option: shipping without it is the failure mode the gate exists to catch.
3. The freshly generated evidence reuses any A22, A23, or v0.50.110 identifier.
4. Either test lane fails for a reason other than the confirmed port-1455 collision.
5. `git status --short` is non-empty after `go run ./cmd/generate-templates`.
6. The published asset set does not match the A23 asset shape one-for-one by name.
