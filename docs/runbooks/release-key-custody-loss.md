# Autopus ADK release key custody loss

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

## What is lost, and what each loss costs

| Key | Role | Consequence |
|---|---|---|
| R2, SSH ed25519 `SHA256:7FISPXCi8p7cFEdh4Fcyyp8RPQbXYZwmo3Mxi5+YjrQ` | git tag signature | new release tags cannot carry the expected signature |
| `adk-channel-2026-q3-a0` | rotation authority for tag and promotion keys | no key can authorize a replacement tag or promotion key |
| `adk-release-2026-q3-b0` | public-key receipt, feeding the lineage gate | the A0 receipt bundle cannot be produced, so `VerifyOMPContextReleaseLineage` cannot run |
| `omp-context-promotion-2026-q3-k3` | promotion attestation | fresh K3 evidence cannot be signed |
| K1 ECDSA | `checksums.txt.signatures` | self-update breaks unless K2 signs instead |

The public halves are all still pinned and are not secrets: the channel key in `AUTOPUS_ADK_CHANNEL_PUBLIC_KEY` and the committed authority document, the release key in `ADK_COMPANION_PUBLIC_KEY_BASE64` whose digest equals the compiled `configuredA0PublicKeySHA256`, and K1/K2 in `pinnedkey.go`. Nothing needs recovering from the repository. Only private halves are gone.

## The shortcut that must not be taken

The channel public key lives in a repository variable and a committed document, both writable with repository admin. Rewriting them would install a new trust root and let releases flow again within the hour.

Do not do this. The property the entire apparatus buys is that repository access alone cannot authorize a release; the channel key exists off-repository precisely so that a compromised or over-broad repository credential cannot mint one. Quietly replacing the root to unblock a feature release spends that property and leaves no trace that it was spent.

If the root must be replaced, it is a deliberate, announced re-anchoring with its own evidence, not a step inside a release runbook.

## Recovery shape

Ordered by what it protects, not by convenience.

1. **Establish lost versus compromised.** If compromised, revoke first: the channel authority ref, the tag ruleset expectations, and a public statement naming the affected window.
2. **Confirm K2 custody.** This single fact decides whether existing installations can be reached at all. Test it by signing a scratch payload and verifying against the compiled K2 anchor before relying on it.
3. **Keep shipping artifact integrity on sigstore.** It needs nothing from the operator and it already verifies.
4. **Decide the fate of the companion evidence layer.** Lineage, promotion attestation, and the signed tag all need new anchors that no surviving key can authorize. Options are a fresh out-of-band root ceremony, or narrowing releases to the sigstore-plus-K2 guarantee and retiring the companion layer rather than leaving it present and unverifiable.
5. **Only then plan the next release.** A release that silently omits a layer it used to carry is worse than a delayed one, because consumers cannot see what stopped being true.

## Effect on v0.50.112

`release-v0.50.112.md` is written and its repository-side work is complete: the ruleset exists, content gates pass, the asset gate and lineage invocation are pinned. It stops at the tag boundary and stays stopped until this document's step 4 is decided.

Do not resolve that stop by signing with a different key. A tag signed by a key nobody expects fails verification for every downstream consumer while looking finished locally.
