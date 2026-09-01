# SPEC-QAMESH-013 — Plan

Ordered so that each step is independently verifiable and nothing lands on an
unproven assumption. The previous attempt failed by plumbing identity before
checking what the provider actually returns; that ordering is inverted here.

## Phase 1 — Tree parser (no wiring)

New `pkg/qa/run/desktop_observe_tree.go`.

1. `parseDesktopTree(text string) (desktopTree, error)` producing:
   - `AppIdentifier`, `PID` from `App=<id> (pid <n>)`
   - `WindowTitle`, `AppName` from `Window: "<title>", App: <name>.`
   - `Nodes []desktopTreeNode` with `ID`, `Depth`, `RolePhrase`, `Name`,
     `StateMarkers []string`, `Attributes map[string]string`
   - `FocusedElementID` from the trailing `The focused UI element is <id> ...`
2. Depth from leading tab count. Reject any other leading whitespace.
3. State markers from parenthesised tokens in the role phrase: `(selected)`,
   `(expanded)`.
4. Attributes from `, <Key>: <value>` segments: `Text`, `Value`,
   `Secondary Actions`.
5. Fail closed on: `\r`, NUL, a depth jump greater than one, a duplicate element
   id, a missing header, a missing focus line.

Verification: table tests over the two real trees captured in `research.md`
(Autopus Desktop 7 nodes/484 B English, Finder 152 nodes/7364 B Korean) plus
malformed variants. No production wiring in this phase, so nothing can regress.

## Phase 2 — Reason codes and bounds

1. Add `declared_landmark_not_found` to `pkg/qa/desktopobserve/reasons.go` with
   its `NextStep`, satisfying REQ-5.
2. Add a bound-exceeded reason satisfying REQ-6, or reuse an existing one if
   `reasons.go` already carries a suitable code — decide by reading it, do not
   assume.
3. Remove the 2048-byte `treeText` cap and the `elementCount == 9` /
   `focusedElementId == 2` pins from `validOrcaSnapshot`
   (`pkg/qa/run/desktop_observe_orca_state.go:115-142`). Keep every other
   envelope check.

Verification: unit tests asserting each new code round-trips through
`Validate()` and yields its fixed `NextStep`.

## Phase 3 — Shared projection builder

1. Extract the recursive decoder at
   `pkg/qa/run/desktop_observe_v2_projection.go:83-177` into a shared builder
   that takes the pack-declared allowlist instead of the hardcoded
   `Autopus|Disclosure|Status` set
   (`desktop_observe_v2_projection_validate.go:54-83`).
2. Implement REQ-4 selection: project a node only when its name is declared;
   otherwise count it.
3. Derive the application and window nodes from the parsed header per REQ-2.

Verification: the local v2 fixture still decodes; a Finder-shaped tree with one
declared landmark projects exactly that landmark and counts the rest.

## Phase 4 — Pack schema

1. Add `DesktopObservationPolicy.ProviderAppID`
   (`pkg/qa/journey/types.go:92-103`) with yaml/json tag `provider_app_id`.
2. Validate it in `validateDesktopObservationTarget`
   (`pkg/qa/journey/desktop_observation_policy.go:70-89`): non-empty, bounded,
   no characters that could alter a provider request. Not alias-shaped.
3. Update the six pack fixtures named in `research.md` section 7.

Verification: the strict-decode gate (`pkg/qa/journey/load.go:35-52`) still
rejects unknown keys; a pack missing `provider_app_id` fails with a named error;
a Finder-shaped pack validates.

## Phase 5 — Provider plumbing

1. Thread `ProviderAppID`, the declared window title, and the declared app name
   from `desktopRequestFromPack` (`pkg/qa/run/desktop_observe_pack.go:112-145`)
   through `DesktopObservationRunRequest`
   (`desktop_observe_contract.go:37-44`) to the client.
2. Replace the Orca constants (`desktop_observe_orca.go:18-21`) and the
   hardcoded `autopus-desktop` / `main-window` comparisons
   (`desktop_observe_orca_client.go:78-135`) with the request values.
3. Remove the runner receipt pins
   (`desktop_observe_runner.go:102-154,186-201`) and the protocol pins
   (`pkg/qa/desktopobserve/protocol.go:218-246`).
4. Keep `app_ref` / `window_ref` as the published aliases; assert in a test that
   `provider_app_id` appears in no manifest, artifact, or receipt.

Verification: a request built from a Finder pack addresses `com.apple.finder`.

## Phase 6 — End-to-end

1. Autopus Desktop: launch, focus, run `auto qa run --lane desktop-native
   --runtime-provider orca`, expect a pass with the two declared landmarks
   projected.
2. A second app that is not ours, to prove REQ-3 and REQ-4 are real.
3. Negative controls: wrong window title yields
   `declared_landmark_not_found`; an undeclared node's name appears nowhere in
   the published evidence.
4. Full regression, both Makefile lanes.

## Rejected alternatives

**Map localized role phrases to `AXRole`.** Needs a table per macOS locale and
mis-types silently on an unmapped phrase. REQ-2 forbids it.

**Publish the observed tree.** Leaks user content, breaks the 8 KiB bound, and
would require weakening the publication scan. REQ-4 forbids it.

**Reuse `app_ref` as the provider identifier.** The public-ref grammar rejects
dots and spaces (`pkg/qa/desktopobserve/canonical.go:57-60`), and refs are
published. This was the previous attempt's mistake; a test caught it when app
matching dropped to zero.

**Truncate oversized trees.** Produces a pass over partial observation. REQ-6
forbids it.

## File budget

Every touched or created Go file stays under 300 lines. `desktop_observe_tree.go`
is expected to need a split between parsing and validation; that is anticipated,
not a surprise.
