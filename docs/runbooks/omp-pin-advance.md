# Advancing the pinned OMP release

`oh-my-pi` ships often. Between 2026-08-29 and 2026-09-03 it moved v17.2.7 →
v18.0.11 → v18.1.2 → v18.1.4 → v18.1.5. The release pin does not follow
automatically, and this is the procedure that moves it.

## What the pin is, and what it is not

The pin has two halves, both declared in
`scripts/companion-release/prepare-release.sh`:

- `expected_omp_sha256` — the bytes
- the `omp/N.N.N` version, repeated at six more sites that cannot derive it

It is **not** what enforces the protocol contract. `initializeManaged` in
`internal/cli/pipeline_omp_context_active_rpc.go` negotiates managed protocol
v2, turns auto-retry and auto-compaction off, refuses a runtime whose idle state
still reports compaction enabled, and refuses one without native image
compaction — all against the binary that actually runs.

The pin's job is narrower and still worth keeping: before the release starts, it
declares exactly which executable the signed evidence will name. That is a
supply-chain pre-commitment a reviewer can read in the repository.

## Move it with one command

```bash
scripts/release-tools/advance-omp-pin.sh 18.1.5 --dry-run   # inspect first
scripts/release-tools/advance-omp-pin.sh 18.1.5
```

The script refuses to touch a file until the candidate has proven itself:

1. downloads `omp-darwin-arm64` from the immutable upstream release
2. measures the digest rather than trusting a typed one
3. requires the asset to self-report `omp/<version>`
4. runs the RPC handshake and requires `supportedProtocolVersions` to contain
   `2` — the same version `initializeManaged` negotiates

Then it rewrites all seven sites and prints the verification list.

`go test ./internal/companionmanifest/ -run TestOMPPin` fails the moment the
sites disagree, so a half-moved pin is a test failure rather than a discovery
made while the release canary is already running.

## The one thing the script cannot check

The compaction reduction floor. `min_reduction_basis_points` is measured, not
declared: the 40-call cohort in `--apply` computes the actual reduction and
fails evidence generation if the new OMP compacts less than the floor.

`--preflight` does not measure it. The `canary_records:42,provider_calls:40`
fields in the preflight receipt are a declared plan, and the script exits before
the cohort block runs. Running the cohort by hand means reproducing the release
lane's gateway, credential locator, policy digest and producer identity, and the
evidence it produced would be unsigned and unusable for a release anyway.

So the floor is validated where it is measured:

1. advance the pin
2. run the release script tests and `preflight-release.sh`
3. start the release with `release-prep.sh --apply`
4. if the cohort fails the floor, revert the pin with
   `advance-omp-pin.sh <previous>` and release on the previous pin

A failed cohort costs the coordinate, so treat step 4 as expected rather than
exceptional when the OMP major version moves.

## Why not drop the pin and measure at release time

Because then nothing in the repository states which executable the evidence will
name until after it is signed. The evidence records the measured version and
digest either way, but a reviewer would have to trust the run instead of reading
the declaration. The churn is a smaller cost than losing that.

## Why not alarm on drift in CI

Upstream ships every few days. A CI job that fails on a stale pin would make CI
red on someone else's schedule. `preflight-release.sh` warns instead, and now
names the exact command:

```
warn  upstream latest is v18.1.5; the pin is v17.2.7. Advancing is its own task
      because the reduction floor is measured in the canary, not declared: run
      scripts/release-tools/advance-omp-pin.sh 18.1.5
```
