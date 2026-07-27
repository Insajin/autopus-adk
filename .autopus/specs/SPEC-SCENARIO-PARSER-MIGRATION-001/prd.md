# PRD: Executable scenario parser migration

## Problem

`ParseScenarios` ignores malformed fields and only recognizes numeric headers.
`auto test run` can execute an empty command and treat an empty or unknown
verification set as PASS. Existing sparse scenario indexes depend on permissive
parsing, so an immediate strict-default flip would be disruptive.

## Goal

- Preserve numeric and alphanumeric scenario references without field
  bleed-through.
- Add structured validation with stable reason codes.
- Default to `warn`, quarantine invalid entries before build/shell execution,
  and preserve the command's compatibility exit behavior.
- Add opt-in `enforce` that fails before any build or shell execution.
- Implement and validate the verification primitives already exposed by the
  scenario wire.

## Non-Goals

- editing root or product `scenarios.md` data;
- making `enforce` the default before inventory reaches zero invalid entries;
- inferring missing commands or verification;
- accepting arbitrary verification functions;
- archiving or deleting scenario bodies.

## Success Metrics

| Metric | Target |
|---|---|
| invalid scenario shell/build executions | 0 |
| alphanumeric header bleed | 0 |
| enforce invalid acceptance | 0 |
| warn diagnostic reason-code stability | 100% |
| valid parse/render/parse preservation | 100% |

## Constraints

- The lenient `ParseScenarios` API remains available for setup/sync round trips.
- Runnable admission is a separate validation boundary.
- Default promotion to enforce requires an explicit later decision.
