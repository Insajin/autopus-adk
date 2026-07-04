# Eval Regression Gate — Required-Check Readiness Runbook

Ops runbook for promoting the eval-regression gate to a **required status check**
on branch `main`.

Ref: SPEC-EVAL-REGRESSION-PROV-001 (REQ-EVP-READY-001), extends
SPEC-EVAL-REGRESSION-CI-001. Mirrors the branch-protection registration precedent
in `autopus-desktop/tests/visual/README.md`.

> IMPORTANT: This SPEC performs **no** branch-protection change. Promotion is an
> OPS-ONLY privileged action and MUST NOT be automated by the pipeline. The
> `gh api ... /branches/main/protection` command below is documentation for an
> operator to run manually, out of band, after the readiness gate is met.

## Gate check context name

The workflow `.github/workflows/eval-regression-gate.yml` defines a single job:

```yaml
jobs:
  eval-regression:      # <-- this job name renders as the check context
```

The check **context name** is the job name: `eval-regression`.

GitHub renders the context from the job id/name; confirm the exact rendered
context on a recent commit before registering it (do not guess):

```bash
# Confirm the rendered check context name on a real commit SHA.
gh api repos/:owner/:repo/commits/<sha>/check-runs \
  --jq '.check_runs[].name'
```

Use the value printed for the eval-regression job as the authoritative context
string in the registration step below (it is expected to be `eval-regression`).

## Readiness precondition (LIVE-A gate)

Do NOT promote until ALL of the following hold:

1. **LIVE-A is live.** SPEC-COE-RUNTIME-LIVE-001 emits a real
   `eval_regression_report.v1` report **and** a valid
   `eval_regression_attestation.v1` sidecar, signed with a key whose public half
   is in the committed allowlist, published from a **trusted producer** workflow
   run (not the PR head).
2. **The gate passes on a known-good control PR.** A PR carrying a validly-signed,
   allowlisted, fresh, non-blocked report makes the `eval-regression` check green.
3. **A blocked control PR fails closed.** A PR carrying a validly-signed blocked
   report makes the check red (exit 1, reason `regression_blocked`).

Until (1) holds, the gate is EXPECTED to fail closed on `artifact_missing` /
`artifact_unsigned`; that is why it is intentionally NOT yet required.

## Registration (ops-only, run manually)

Add the `eval-regression` context to branch `main`'s required status checks. This
mirrors the desktop precedent
(`gh api repos/:owner/:repo/branches/main/protection --field required_status_checks[contexts][]='linux-visual-regression'`):

```bash
# OPS-ONLY. Requires an admin token. Not run by CI or this SPEC.
gh api repos/:owner/:repo/branches/main/protection \
  --method PUT \
  --field required_status_checks[strict]=true \
  --field required_status_checks[contexts][]='eval-regression'
```

Or via the GitHub UI: **Settings → Branches → Branch protection rules → `main` →
Require status checks to pass before merging → add `eval-regression`**.

## Non-goals and safety

- No secrets or tokens are inlined in this runbook or the workflow. The admin
  token used for the `gh api` PUT is supplied by the operator's own environment.
- This document changes no repository settings by itself; it only records the
  procedure and its LIVE-A precondition.
- Reversal: remove `eval-regression` from `required_status_checks[contexts][]`
  via the same `gh api` endpoint or the UI if the gate needs to be de-listed.
