# SPEC-QAMESH-013 — Real desktop accessibility observation

Status: draft
Owner: autopus-adk
Supersedes the fixture-bound behavior introduced under SPEC-QAMESH-012.

## Problem

The `desktop-native` lane advertises typed desktop accessibility observation and
is a `must` lane in the `prelaunch` and `release-candidate` profiles. Neither
shipped provider adapter observes anything.

The Orca adapter validates the observed accessibility tree against an exact
14-line fixture, extracts one boolean, and returns a hardcoded three-node
projection. The local adapter has a real recursive decoder but restricts every
node name to `Autopus`, `Disclosure`, or `Status`. The fixture UI the Orca path
demands exists only in this repository's test helpers.

Consequence: any project that adopts ADK and has a desktop surface inherits a
release gate it cannot pass, and Autopus Desktop itself cannot pass it either.
See `research.md` for measured evidence.

## Non-goals

- Driving or mutating a desktop app. Observation stays read-only; the operation
  set remains `capabilities, permissions, list_apps, list_windows, get_state`.
- Screenshot capture or OCR.
- Publishing the observed accessibility tree. That is explicitly refused; see
  REQ-4.
- Supporting providers other than the existing `local` and `orca` selections.

## Requirements

Written in EARS form.

### REQ-1 — Observe the real tree

WHEN the `desktop-native` lane runs with a resolved provider, the harness SHALL
parse the provider's observed accessibility tree into an internal hierarchy of
nodes carrying element id, depth, role phrase, name, state markers, and
advertised actions.

WHERE the provider is `orca`, the tree source SHALL be
`result.snapshot.treeText`, since that is the only tree representation the Orca
CLI offers.

### REQ-2 — Derive identity without translating role words

The harness SHALL derive the application and window landmarks from the tree
header lines `App=<identifier> (pid <n>)` and `Window: "<title>", App: <name>.`,
and SHALL NOT infer `AXApplication` or `AXWindow` from localized role phrases.

WHEN a node below the window carries a role phrase, the harness SHALL treat that
phrase as opaque and SHALL NOT map it to an `AXRole`.

Rationale: role phrases are emitted in the operating system's locale. Measured:
`standard window` versus `표준 윈도우`, `scroll area` versus `스크롤 영역`.

### REQ-3 — Address the app the pack names

The Journey Pack SHALL declare the platform identifier of the app under
observation in a new `desktop_observation.provider_app_id` field.

WHEN the harness issues a provider request, it SHALL address
`provider_app_id`, and SHALL NOT use a compiled-in identifier.

WHILE `provider_app_id` is absent, the harness SHALL report a setup gap naming
the missing field, and SHALL NOT fall back to a compiled-in identifier.

`provider_app_id` SHALL be request-only and SHALL NOT appear in any manifest,
artifact, receipt, or projection. `app_ref` and `window_ref` remain the
publishable opaque aliases.

### REQ-4 — Observe fully, publish selectively

The published `SemanticProjection` SHALL contain only nodes whose names the pack
declared through `required_landmarks`.

WHEN the observed tree contains a node the pack did not declare, the harness
SHALL count it and SHALL NOT publish its name, value, or text.

Rationale: real trees carry user content. Measured: Finder exposes the user's
folder names; Autopus Desktop exposes in-flight UI copy. This requirement is what
keeps the existing name allowlist
(`pkg/qa/desktopobserve/oracle.go:214-239`), the 8 KiB typed-evidence bound, and
the publication scan (`pkg/qa/evidence/desktop_publication_scan.go`) satisfiable
by construction rather than by luck.

### REQ-5 — Report a missing landmark as a missing landmark

WHEN a declared landmark is absent from the observed tree, the harness SHALL
report reason code `declared_landmark_not_found` with the declared role and
name, and SHALL NOT report `provider_unavailable` or
`semantic_projection_unavailable`.

Rationale: the current taxonomy folded this case into provider unavailability.
Measured consequence: a window-title mismatch was reported as a provider
lifecycle failure.

### REQ-6 — Refuse oversized trees by name

WHEN the observed tree exceeds the node, depth, or byte bounds, the harness SHALL
report a reason code naming the exceeded bound and SHALL NOT truncate the tree,
SHALL NOT sample it, and SHALL NOT report a pass.

The existing bounds (256 nodes, depth 32, 8 KiB typed evidence) SHALL remain in
force. Only the 2048-byte `treeText` cap, which is a fixture artifact, is
removed.

### REQ-7 — Window title comes from the declared landmark

The harness SHALL take the expected window title from the `AXWindow` landmark's
`name`, which already permits spaces
(`pkg/qa/journey/desktop_observation_policy.go:139-152`).

WHEN the observed window title does not equal that name, the harness SHALL report
`declared_landmark_not_found` for the window landmark.

### REQ-8 — Both providers share one projection builder

The `local` and `orca` adapters SHALL produce projections through one shared
builder, so a projection accepted from one provider is accepted from the other.

The existing recursive decoder at
`pkg/qa/run/desktop_observe_v2_projection.go:83-177` SHALL be the basis; its
`Autopus|Disclosure|Status` name allowlist
(`desktop_observe_v2_projection_validate.go:54-83`) SHALL be replaced by the
pack-declared allowlist.

### REQ-9 — Preserve the trust anchor

The `local` provider SHALL continue to require a signature-verified release
artifact (`pkg/qa/run/desktop_observe_process.go:60-74`). Nothing in this SPEC
weakens provider identity verification, envelope strictness, the publication
scan, or the read-only operation set.

## Data model change

```yaml
desktop_observation:
  platform: macos
  provider_app_id: co.autopus.desktop   # NEW: request-only platform identifier
  app_ref: autopus-desktop              # unchanged publishable alias
  window_ref: main-window               # unchanged publishable alias
  operations: [capabilities, permissions, list_apps, list_windows, get_state]
  required_landmarks:
    - role: AXApplication
      name: Autopus Desktop             # real app name, spaces allowed
      required_state: enabled
    - role: AXWindow
      name: Autopus Desktop             # real window title, spaces allowed
      required_state: focused
```

`provider_app_id` validation: non-empty, bounded length, and restricted to
characters that cannot alter a provider request. It is not required to be
alias-shaped, because it is never published.

## Risks

| Risk | Mitigation |
|---|---|
| Publishing user content from a real tree | REQ-4 publishes only declared names; publication scan stays fail-closed |
| Locale-dependent parsing | REQ-2 forbids role-word translation; identity comes from header lines |
| Apps larger than the node bound | REQ-6 refuses by name instead of truncating |
| A pack that declares nothing observes nothing | Landmark shape validation already requires exactly one application and one window landmark |
| Regression in the first-party path | Autopus Desktop is verified end to end in acceptance |

## Out of scope, recorded for later

The `orca` provider currently requires the Orca CLI on `PATH`; resolution is
`LookPath(ctx, "orca")` (`pkg/qa/run/desktop_observe_contract.go:103`). Shipping
a discovery path for the Orca app bundle is separate work.
