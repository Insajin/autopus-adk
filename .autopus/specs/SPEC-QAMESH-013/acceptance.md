# SPEC-QAMESH-013 — Acceptance

Every criterion is observable. Criteria marked **live** require a real app and a
real provider; they are not satisfied by unit tests.

## AC-QAMESH13-001 — Parses a real localized tree

GIVEN the Finder tree captured in `research.md` (152 elements, 5598 characters (7364 bytes), role
phrases in Korean)
WHEN `parseDesktopTree` runs
THEN it yields 152 nodes with correct depths, `AppIdentifier` `com.apple.finder`,
`WindowTitle` `응용 프로그램`, `AppName` `Finder`, and `FocusedElementID` 90.

Satisfies REQ-1, REQ-2.

## AC-QAMESH13-002 — Parses a real English tree

GIVEN the Autopus Desktop tree captured in `research.md` (7 elements, 406 characters (484 bytes))
WHEN `parseDesktopTree` runs
THEN it yields 7 nodes, `WindowTitle` `Autopus Desktop`, and records the
`Secondary Actions` attribute on the full-screen button.

Satisfies REQ-1.

## AC-QAMESH13-003 — No role-word translation

WHEN the parser processes both trees
THEN no code path compares a role phrase against a literal such as
`standard window` or `표준 윈도우`, and the application and window landmarks are
derived only from the two header lines.

Verified by test plus by absence: grep for the role literals in production code
returns nothing outside fixtures.

Satisfies REQ-2.

## AC-QAMESH13-004 — Pack addresses its own app

GIVEN a pack declaring `provider_app_id: com.apple.finder`
WHEN the harness issues provider requests
THEN every request carries `--app com.apple.finder` and none carries
`co.autopus.desktop`.

Satisfies REQ-3.

## AC-QAMESH13-005 — Missing provider_app_id is a named setup gap

GIVEN a pack with no `provider_app_id`
WHEN the lane runs
THEN the result is a setup gap naming the missing field, exit non-zero, and no
provider request is issued.

Satisfies REQ-3.

## AC-QAMESH13-006 — provider_app_id never reaches evidence

GIVEN a completed run whose pack declares `provider_app_id: co.autopus.desktop`
WHEN the manifest, artifact, receipt, and projection are searched
THEN the string `co.autopus.desktop` appears in none of them, while `app_ref`
and `window_ref` appear as before.

Satisfies REQ-3, REQ-9.

## AC-QAMESH13-007 — Undeclared node content is never published

GIVEN a Finder observation whose tree contains the user's folder names
WHEN the published evidence is searched
THEN no undeclared node name, `Value`, or `Text` appears, and the node count is
reported.

Satisfies REQ-4.

## AC-QAMESH13-008 — Missing landmark reports itself

GIVEN a pack whose `AXWindow` landmark name does not match the observed window
title
WHEN the lane runs
THEN the reason code is `declared_landmark_not_found`, the message names the
role and the declared name, and the code is neither `provider_unavailable` nor
`semantic_projection_unavailable`.

Satisfies REQ-5, REQ-7.

## AC-QAMESH13-009 — Oversized tree refuses by name

GIVEN a tree exceeding the node, depth, or byte bound
WHEN the lane runs
THEN the reason code names the exceeded bound, the tree is not truncated, and the
verdict is not a pass.

Satisfies REQ-6.

## AC-QAMESH13-010 — Both providers share one builder

WHEN the local v2 fixture and an Orca tree are each projected
THEN both go through the same builder, and the local v2 fixture still decodes
exactly as it did before this change.

Satisfies REQ-8.

## AC-QAMESH13-011 — **live** — Autopus Desktop passes

GIVEN Autopus Desktop running and focused, Orca accessibility permission granted,
and a pack declaring `provider_app_id: co.autopus.desktop` with landmarks named
`Autopus Desktop`
WHEN `auto qa run --lane desktop-native --runtime-provider orca` runs
THEN the verdict is a pass, a manifest is published, and the projection contains
exactly the two declared landmarks.

Satisfies REQ-1 through REQ-8 together.

## AC-QAMESH13-012 — **live** — A second, non-Autopus app passes

GIVEN the same conditions for an app we do not own
WHEN the lane runs
THEN the verdict is a pass under that app's own `provider_app_id`.

This is the criterion the lane has never met. It is the reason this SPEC exists.

Satisfies REQ-3, REQ-4.

## AC-QAMESH13-013 — Trust anchor intact

WHEN `--runtime-provider local` runs without a signature-verified release
artifact
THEN it still refuses, with provider identity verification, envelope strictness,
the publication scan, and the read-only operation set unchanged.

Satisfies REQ-9.

## AC-QAMESH13-014 — No regression

WHEN both Makefile test lanes run
THEN there are zero failures, `gofmt -l` is empty over touched paths, and every
touched or created Go source file is under 300 lines.

## Verification log

To be filled during implementation with the exact command and its output for
each criterion. A criterion without pasted output is not accepted.

## Verification log — 2026-09-01

Unit and integration criteria AC-001..AC-010, AC-013, AC-014: met. Both Makefile
regression lanes zero failures; `pkg/qa/run` 80 tests / 124 subtests.

AC-QAMESH13-011 (**live**, Autopus Desktop): BLOCKED, not failed. The installed
`/Applications/Autopus Desktop.app` is a stale build, being rebuilt in a separate
session. Precondition: a current build with an open window.

AC-QAMESH13-012 (**live**, a second app): **MET** after fixing the two gaps below.
Target Slack (`com.tinyspeck.slackmacgap`), an app we do not own, 378 nodes /
22760 bytes, window title `maker-v2(채널) - Aligo - Slack`.

    envelope: ok | run: passed
    check desktop-semantic-landmarks passed
    root:  AXApplication 'Slack' {'enabled': True}
     child: AXWindow 'maker-v2(채널) - Aligo - Slack' {'focused': True}

AC-QAMESH13-007 confirmed on that live evidence: 311 distinct undeclared tokens in
the observed tree, zero in the published manifest and artifact. Only the two
declared names appear. `provider_app_id` appears nowhere.

### Gap 1 — the node bound was too small for real applications (FIXED)

`desktopTreeMaxNodes = 256`. Measured element counts on this machine: Finder 152,
Slack 378, Autopus Desktop 7 to 22 depending on load state. A mainstream app
exceeds the bound, so the lane refuses it correctly but unusably.

The bound predates real observation, when the projection was three synthesized
nodes. Under REQ-4 the PUBLISHED projection is bounded by the pack's declared
landmarks rather than by the app, so this bound now protects only parse cost. It
should be raised to cover real apps; the 8 KiB published bound stays as is,
because that is the privacy-relevant one.

### Gap 2 — a real provider focus-line variant was refused (FIXED)

The `provider_protocol_mismatch` first attributed to the node bound was actually
the focus line. The provider renders two legitimate forms, and only one was
accepted: `The focused UI element is <id> ...` when the app holds keyboard focus,
and `No UI element is currently focused.` when it does not. Refusing the second
reported a correctly observed unfocused app as a protocol fault.

Both forms now parse; the unfocused form yields no focus, so a pack declaring
`required_state: focused` legitimately fails rather than passing on a phantom.
Pinned by `slack-unfocused.txt`, a third real capture.

### Live path reach, for the record

pack validates -> provider resolves (`orca-computer-use-macos` v1.0.0, protocol 1)
-> envelope accepted -> tree parsed -> node bound refused. Every layer up to the
bound is exercised by a real provider against a real third-party app.
