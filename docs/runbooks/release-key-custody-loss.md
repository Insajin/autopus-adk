# Autopus ADK release key custody loss

> **WITHDRAWN 2026-09-02.** The premise was wrong. Nothing was destroyed. Every
> key named below was found intact in `~/.config/autopus/release-keys/` on the
> release machine, verified by fingerprint. This document is kept because the
> analysis of what survives without operator keys is accurate and useful, and
> because withdrawing it silently would hide that a release procedure was changed
> on a false premise. See "What was actually found" at the end.

## What this document is

`release-key-rotation.md` rotates keys while the channel key is in hand. This one covers the case that runbook cannot: the channel key itself is gone, so no surviving key can authorize a replacement.

Every claim below was measured on 2026-09-02 against the live repository and the published v0.50.111 release. Re-measure before acting; do not treat this as current state.

## The decisive question, answered first

**Lost or compromised?** These are different incidents and the response diverges immediately.

| | Lost — destroyed, unreachable, provably gone | Compromised — whereabouts unknown, possibly copied |
|---|---|---|
| Past releases | remain trustworthy; signatures made while the key was sound | suspect from the compromise date onward |
| Channel key risk | none; nobody can use it | **an attacker can mint a rotation sidecar authorizing their own tag signer, and the committed authority document plus the GitHub channel variables would validate it** |
| Required response | re-anchor forward | re-anchor forward **and** revoke: invalidate the channel authority, announce, and treat any unexpected sidecar or tag as hostile |

Do not proceed past this section until the answer is established. Choosing "lost" because it is the more comfortable answer converts an active-threat incident into a silent one.

## What survives without any operator key

Two anchors are intact, and both were verified rather than assumed.

### Sigstore keyless signing

Release checksums are signed by GitHub Actions OIDC through Fulcio, bound to the exact workflow file and tag ref, and logged in the transparency log. No long-lived private key participates.

```
SAN:    https://github.com/Insajin/autopus-adk/.github/workflows/release.yaml@refs/tags/v0.50.111
Issuer: sigstore.dev, via token.actions.githubusercontent.com
```

Verified live:

```sh
cosign verify-blob checksums.txt --bundle checksums.txt.bundle \
  --certificate-identity 'https://github.com/Insajin/autopus-adk/.github/workflows/release.yaml@refs/tags/v0.50.111' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com'
# Verified OK
```

Forging this requires making the repository's release workflow run at a tag, which the tag ruleset and the protected environment with required reviewers already gate. It is the strongest thing still standing.

### K2, the prepositioned offline-next release signer

`pkg/selfupdate/pinnedkey.go` compiles **two** ECDSA P-256 anchors into every binary shipped since v0.50.73:

| Key | Fingerprint | Expires | Used so far |
|---|---|---|---|
| K1 | `e1fdfe066484c7eae8ff16fa4b1ee6237b8d06299c2b66ced485f029af77837f` | 2028-07-17 | yes, signs `checksums.txt.signatures` |
| K2 | `93d9f681d829f2d0bdba7e1853e6acf9ae2ffd2c760355853218e920c35cc5ff` | 2030-07-17 | no |

The source comment states the intent plainly: *"K1 is the active release signer. K2 is prepositioned as the offline-next rotation anchor."*

This is the recovery affordance the design already bought. **Every deployed installation already trusts K2.** If K2's private half was kept in separate custody and survives, self-update can switch to signing with K2 and every existing user continues updating with no manual step and no re-install.

`VerifyReleaseSignature` is fail-closed against this compiled set, so if both K1 and K2 are gone, no new release can satisfy an already-installed binary. That case is a re-install migration, not a rotation.

## Custody, measured

Most of these keys were never in the operator's hands. They are GitHub Actions secrets, and they still exist. Checking this before planning changed the incident from "the release chain is dead" to "one audit link is dead."

| Key | Role | Custody | State |
|---|---|---|---|
| K1 ECDSA `e1fdfe06…` | signs `checksums.txt.signatures`, consumed by self-update | environment secret `ADK_RELEASE_ECDSA_PRIVATE_KEY` in `adk-companion-release`, created 2026-07-17 | **alive** |
| `adk-release-2026-q3-b0` | public-key receipt, feeding the lineage gate | repository secret `ADK_COMPANION_ED25519_PRIVATE_KEY`, created 2026-07-14; materialized at `COMPANION_SIGNING_KEY_FILE` inside the workflow | **alive** |
| `omp-context-promotion-2026-q3-k3` | promotion attestation | repository secret `OMP_CONTEXT_EVIDENCE_SIGNING_KEY`, created 2026-08-05 | **alive** |
| Sigstore keyless | `checksums.txt.bundle` | none; workflow OIDC | **alive** |
| R2 SSH ed25519 `SHA256:7FISPXCi8p7cFEdh4Fcyyp8RPQbXYZwmo3Mxi5+YjrQ` | git tag signature | operator | **destroyed** |
| `adk-channel-2026-q3-a0` | rotation authority for the tag signer | operator | **destroyed** |
| K2 ECDSA `93d9f681…` | prepositioned next release signer | unknown; no secret corresponds to it and it has never signed | **unknown** |

Evidence for the K1 row: the published `checksums.txt.signatures` for v0.50.111 carries exactly one record, fingerprint `e1fdfe06…`, which is K1. The workflow produced it from the environment secret, so K1 is not operator-held and is not lost. K2 remains unused, which is why its custody is unknown rather than confirmed — it does not currently matter, because K1 works.

The public halves are all still pinned and are not secrets: the channel key in `AUTOPUS_ADK_CHANNEL_PUBLIC_KEY` and the committed authority document, the release key in `ADK_COMPANION_PUBLIC_KEY_BASE64` whose digest equals the compiled `configuredA0PublicKeySHA256`, and K1/K2 in `pinnedkey.go`.

## What the loss actually costs

Consumer-facing integrity is untouched. Users install release archives and verify them through `checksums.txt`, the K1 envelope, and the sigstore bundle. Nothing a consumer runs verifies a git tag signature: `pkg/selfupdate` checks the compiled ECDSA anchors, and the Homebrew formula pins an archive digest.

What is lost is one internal audit link — the guarantee that a release tag was signed by the expected key — plus the ability to rotate that key legitimately, since only the channel key could authorize a replacement and it is gone.

That link is also a hard precondition of the current procedure. `verify_tag_signing_authority` in `scripts/companion-release/prepare-release-local-lib.sh` derives the public key from the operator's SSH key, requires it to equal the R2 pin, requires the fingerprint to equal a value **hard-coded at line 42**, then signs a probe tag and verifies it. Editing the pinned `.pub` and `.fingerprint` files does not get past it, and should not: the published, immutable v0.50.109 rotation sidecar names R2 as the authorized signer, so a different key contradicts published history.

## The shortcut that must not be taken

The channel public key lives in a repository variable and a committed document, both writable with repository admin. Rewriting them would install a new trust root and let releases flow again within the hour.

Do not do this. The property the entire apparatus buys is that repository access alone cannot authorize a release; the channel key exists off-repository precisely so that a compromised or over-broad repository credential cannot mint one. Quietly replacing the root to unblock a feature release spends that property and leaves no trace that it was spent.

If the root must be replaced, it is a deliberate, announced re-anchoring with its own evidence, not a step inside a release runbook.

## Recovery shape

Ordered by what it protects, not by convenience.

1. **Establish lost versus compromised.** Recorded as destroyed on 2026-09-02, so no revocation announcement is required and published releases stay trustworthy. Had it been compromised, revocation would come before anything else.
2. **Leave the working layers alone.** Artifact integrity, the lineage receipt, and the promotion attestation all sign from secrets that still exist, and sigstore needs no key. Nothing here requires recovery, and touching it would only add risk.
3. **Decide the fate of the tag-signature link.** This is the whole remaining decision. Two honest options, below.
4. **Re-establish off-repository custody going forward, whichever option is chosen.** Every surviving signing key now lives in GitHub. That is why the release chain still works, and also why the property the channel key provided — that repository access alone cannot authorize a release — is currently not held by anything. Restoring it needs a new key in custody outside GitHub, held by someone who can refuse.

### Option A: retire the tag-signature precondition, on the record — CHOSEN, applied 2026-09-02

Amend `verify_tag_signing_authority` so the procedure no longer requires a key that cannot exist, and state in the runbook and the release notes that release tags from v0.50.112 onward are not signed by R2 and that verification rests on the K1 envelope and the sigstore bundle.

Cost: the audit trail loses a link, permanently and visibly. Consumers are unaffected. This is reversible in the sense that a future signer can be introduced, but the gap in history stays.

Applied as follows. Every change is a removal of a check that could no longer pass, plus an assertion of what remains true.

| Location | Change |
|---|---|
| `prepare-release-local-lib.sh` | `verify_tag_signing_authority` deleted; it existed to prove the operator held R2 before prep continued |
| `prepare-release.sh` | `--tag-signing-key` flag, its mode/ownership validation, and its staging removed; `release_tag` advanced to `v0.50.112` |
| `prepare-release-runtime-lib.sh` | tag key argument dropped from the publisher call |
| `publish-release-coordinates.sh` | parameter count 13 to 12; R2 derivation and fingerprint comparison removed; `git tag -s` becomes `git tag -a`; `git verify-tag` replaced by an annotated-tag-object plus target-commit assertion |
| `release-coordinate-transaction-lib.sh` | signature verification removed from `verify_remote_release`; the object-type and target-commit checks above it already carried the load |
| `validate-source.sh` | `v0.50.112` registered as phase `A24` with ancestor `954f60a77acb59fd4106537020693fdcadb3d640`; the A23 fallback branch split so A23 and A24 each check their own ancestor; A24 deliberately absent from the tag-signature list, with a comment saying why |
| `release-prep-hardening-test.sh` | R2 fixture machinery and the signer-mismatch case removed; replaced by assertions that the committed tag is an annotated object and carries no signature |

The R2 `.pub` and `.fingerprint` files stay in the tree. `verify-key-rotation-sidecar.sh` and `rotation-authority-test.sh` still need them to verify the published v0.50.109 sidecar, which is immutable history and must remain checkable.

One unrelated defect surfaced while editing and was fixed rather than shipped past: the tag message literal read `"${release_tag} - A23 companion release"`, so every release after v0.50.111 would have carried the wrong phase. The phase is now omitted instead of guessed.

### Option B: re-anchor the tag signer through a new ceremony

Generate a new channel key in off-repository custody, publish a new authority document and ref, and cut a `canonical-full-bridge` release carrying the new public keys before any release uses them.

Cost: two releases instead of one, and an honest caveat — with the old channel key destroyed, the new root can only be introduced by repository admin. The ceremony restores the property *going forward*, from the moment the new key is in real custody; it cannot retroactively make this transition anything other than admin-authorized. Claiming otherwise in release notes would be the same silent spend this document refuses.

Do not resolve the stop by signing with an unexpected key and saying nothing. That contradicts the published v0.50.109 sidecar and leaves the contradiction for someone else to discover.

## Effect on v0.50.112

`release-v0.50.112.md` is written and its repository-side work is complete: the ruleset exists, content gates pass, the asset gate and lineage invocation are pinned. Its blocker is now exactly one thing — the R2 precondition in step 3 above — because every other input it listed as operator-held is in fact a GitHub secret that the workflow materializes.

That correction matters for planning. An earlier draft of this document, and of the v0.50.112 runbook, treated the evidence layer as dead. It is not. Re-read both before acting.

## What was actually found

A local search of the release machine turned up every key, all verified rather than assumed:

| Key | File | Verification |
|---|---|---|
| R2 tag signer | `release-tag-signing-2026-q3-r2` | `ssh-keygen -y` yields `SHA256:7FISPXCi8p7cFEdh4Fcyyp8RPQbXYZwmo3Mxi5+YjrQ`, an exact match; readable without a passphrase |
| K3 promotion | `omp-context-promotion-2026-q3-k3.b64` | public half `YkTuNcfWGTLgTglPmZq/Dj4OXwcoUwnkM2ExIGIz+jM=`, exact match |
| K1 ECDSA | `autopus-adk-k1-2026-07-17.pk8.pem` | `public.pem` SPKI digest equals the compiled `e1fdfe06…`; private half is passphrase-encrypted |
| K2 ECDSA | `autopus-adk-k2-2026-07-17.pk8.pem` | SPKI digest equals the compiled `93d9f681…`; the prepositioned anchor was real all along |
| Channel A0 | `rotation-private-backup-2026-08-28.enc` | integrity matches the `backup_sha256` recorded in the ceremony note; passphrase is in the macOS keychain under service `autopus-adk-rotation-2026-q3-r2-k3` |

The ceremony note `ceremony-2026-08-28-tag-r2-promotion-k3.txt` documents the whole set, including where the backup passphrase lives. It was sitting next to the keys the entire time.

### What this cost, and the lesson

Acting on the reported loss, the R2 tag-signature precondition was removed and release tags were switched to unsigned annotated tags. That change has been reverted: `verify_tag_signing_authority`, the `--tag-signing-key` flag, the R2 derivation in the publisher, `git tag -s`, and the signature verification in `verify_remote_release` are all back, and A24 is in the tag-signature list with its predecessors. The test now asserts the committed tag **does** carry a signature, the inverse of what stood while the key was believed lost.

The lesson is not "the user was wrong." Custody is exactly the kind of state a person cannot be expected to hold in memory, and the honest answer to "is the key gone" was always a filesystem search. Searching first would have taken one command and saved a release-procedure change made on a false premise. Do that before believing any custody claim, including a confident one.
