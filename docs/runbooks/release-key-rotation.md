# Autopus ADK Release-Key Rotation

## Purpose

This runbook rotates the Autopus ADK SSH release-tag signer and prepositions the next OMP context-promotion key without allowing a release candidate to trust its own new keys.

The v0.50.109 release is a one-time `canonical-full-bridge`. It carries the new public keys but carries no active OMP promotion static policy, report, or attestation. The first release allowed to use the new promotion key is v0.50.110.

## Trust chain

```text
preexisting ADK channel key A0
  -> signs exact v0.50.109 rotation sidecar
  -> sidecar binds source commit/tree + old/new tag signer + promotion K3
  -> immutable sidecar ref authorizes new SSH signer R2 for v0.50.109
  -> protected release environment signs/notarizes bridge assets
  -> installed v0.50.109 prepositions promotion K3 but grants no active policy
  -> v0.50.110 may require K3-signed active evidence
```

Green CI, a mutable source-tree public key, and artifacts produced after tag creation do not independently authorize tag-key rotation. The signed sidecar must exist before the v0.50.109 tag is created.

## Fixed identities

| Role | Identity |
|---|---|
| Repository | `Insajin/autopus-adk` |
| Channel | `stable` |
| Channel key | `adk-channel-2026-q3-a0` |
| Immutable verifier ref | `refs/heads/release-key-rotation-authority-v2` |
| Immutable verifier commit | `f8f48cff2ec0b5fe4c3240d1ea80e4544b960740` |
| Immutable verifier SHA-256 | `b16fefb0fafa518d5d13d050ec2a6d3c0b001b9fa0adb8f5c74f8a9c78ae1fb0` |
| Bridge tag | `v0.50.109` |
| Bridge mode | `canonical-full-bridge` |
| Previous tag fingerprint | `SHA256:bhW+YA+FZ6G4d9Z8BM/eBss6l0I/fcVmV7k986GupK0` |
| Next tag fingerprint | `SHA256:7FISPXCi8p7cFEdh4Fcyyp8RPQbXYZwmo3Mxi5+YjrQ` |
| Next promotion key | `omp-context-promotion-2026-q3-k3` |
| K3 raw-public SHA-256 | `2a9b41dec1330f65937d9b25b20967cb29fd9209c722ce5fe1a9afd6ca45b937` |
| Rotation distribution ref | `refs/heads/release-key-rotation-v0.50.109` |

Private keys remain outside every repository in owner-only files. Never paste them into workflow inputs, logs, issues, commits, artifacts, or release bodies.

## Rotation sidecar

The sidecar uses schema `adk-key-rotation.v1`. It is compact canonical JSON with exactly these fields in this order:

1. `schema_version`
2. `channel`
3. `repository`
4. `bridge_tag`
5. `release_mode`
6. `source_commit`
7. `source_tree`
8. `issued_at`
9. `expires_at`
10. `channel_key_id`
11. `previous_tag_fingerprint`
12. `next_tag_public_key`
13. `next_tag_fingerprint`
14. `next_promotion_key_id`
15. `next_promotion_public_key`
16. `next_promotion_public_key_sha256`

The validity window must not exceed 24 hours. Sign these exact bytes:

```text
autopus.adk-channel.key-rotation.v1\x00 || canonical_json
```

The detached signature is the raw 64-byte Ed25519 signature. Dispatch inputs are canonical base64 of the document and signature.

## Bridge procedure

### 1. Freeze and verify source

1. Land the bridge implementation and public pins on `main`.
2. Require CI and Security Scan success for the exact commit.
3. Confirm a clean worktree and `HEAD == origin/main`.
4. Record exact `HEAD^{commit}` and `HEAD^{tree}`. Any later source change invalidates the sidecar.

### 2. Authorize the rotation

1. Materialize `release-key-rotation-authority-v2` through `materialize-key-rotation-authority.sh`; repository and protected-environment commit pins must both equal the immutable authority commit.
2. Create the canonical sidecar for the exact commit/tree with a validity window of at most 24 hours.
3. Sign the domain-separated document with the preexisting A0 channel private key.
4. Confirm the active `autopus-v0.50.109-rotation-ref-authority` branch ruleset allows only the operator user to create, update, or delete the fixed ref.
5. Run `publish-key-rotation-sidecar.sh DOCUMENT SIGNATURE` as the authorized operator. The publisher verifies the A0 signature and exact source/pins, creates an orphan commit, and pushes the fixed ref without force.
6. Run the manual `Receive authorized ADK key rotation` workflow from the exact frozen `main` commit. This workflow is a read-only auditor: it fetches the existing ref, compares the supplied bytes, and re-verifies the committed blobs.
7. Verify that `refs/heads/release-key-rotation-v0.50.109` contains only:
   - `adk-key-rotation-v1.json`
   - `adk-key-rotation-v1.sig`
8. Reverify the committed blobs without GitHub credentials.

Neither the local publisher nor the audit workflow has the channel signing key. They can distribute or verify a valid authorization but cannot create one. The fixed rotation ref is protected, one-shot, and must never be force-updated or reused.

The verifier used by the publisher, prep, release, audit, and recovery paths comes only from the immutable authority ref. Candidate-local rotation verifier code is not an admission source.

### 3. Publish v0.50.109 bridge

1. Confirm the exact v0.50.109 tag ruleset, required environment reviewer, and disabled administrator bypass.
2. Run bridge preflight with the R2 SSH private key and the immutable signed sidecar.
3. Confirm zero remote mutations, exact source coordinates, bridge mode, exact Go 1.26.6 candidate toolchain, and absence of K2/provider/static-policy inputs.
4. Run bridge apply with the same inputs.
5. The signed annotated tag must be the final external mutation.
6. Wait for CI, Security Scan, notarization, checksum signatures, companion lineage, immutable-release verification, and Homebrew publication.
7. Confirm that the release contains the bridge manifest and signed rotation sidecar pair, and contains no OMP promotion report or attestation.

The bridge binary must be built without the active OMP static-policy ldflag. Missing policy/evidence must fail closed to canonical-full behavior; K3 presence in the verifier keyring alone cannot create an active grant.

### 4. Verify the bridge

1. Install published v0.50.109 through the signed installer.
2. Upgrade a v0.50.108 project and run the public upgrade canary.
3. Run `auto doctor --json` and verify the bridge has no active OMP promotion grant.
4. Preserve the release, sidecar, bridge manifest, workflow runs, checksums, signatures, lineage, and upgrade receipt as immutable evidence.

Homebrew recovery may run after the sidecar validity window. It must use the historical verifier, which relaxes only current-time admission while preserving the signed 24-hour window and every cryptographic/coordinate check. Recovery reads the A0 channel and companion authority from immutable v0.50.109 source pins, never mutable live variables. The protected rotation-ref ruleset must still be active.

A failed immutable release is never repaired by moving or reusing v0.50.109. Fix the defect and allocate a new coordinate.

## Activating K3 in v0.50.110

Only after v0.50.109 is immutable and installed:

1. Arm v0.50.110 with R2 as the normal tag signer.
2. Keep K1 and K2 public keys for historical proof verification.
3. Add `promotion_signing_key_id` to the active static policy and require the attestation key ID to match it exactly.
4. Generate a fresh 20-pair/40-call production observation through the OAuth subscription gateway.
5. Sign the fresh promotion attestation with K3.
6. Restore the normal promotion assets and active static-policy release gates.
7. Publish v0.50.110 through the normal protected release procedure.

Do not use a K3 signature in v0.50.109. Do not accept any committed key merely because it exists in the keyring; active authority must be pinned by the reviewed static policy.

## Incident handling

- Missing old channel authority: stop. Branch protection or same-release signatures are not a substitute.
- Sidecar expired before publication: create and sign a new sidecar only if the immutable rotation ref was never published. A published fixed ref is not replaceable.
- New private key exposed: do not publish. Generate another successor key and restart the approval sequence with new pins and a new source commit.
- Unexpected promotion asset in the bridge: fail the release and allocate a new coordinate.
- Static policy present in the bridge binary: fail the release; canonical-full behavior is no longer proven.
- Rotation ref, source commit/tree, public pins, or tag mismatch: fail closed without remote mutation.
