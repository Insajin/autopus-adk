# SPEC-QAMESH-013 — Research

Real desktop accessibility observation for the `desktop-native` lane.

All findings below were produced by running the real CLI and reading the real
source. Nothing here is inferred.

## 1. What the `desktop-native` lane does today

The lane advertises typed desktop accessibility observation. Both shipped
provider adapters are fixture conformance harnesses, not observers.

### Orca provider

`pkg/qa/run/desktop_observe_orca_state.go:61-112` never builds a projection from
the observed tree. It returns a hardcoded three-node projection:

```
AXApplication "Autopus" (enabled)
  AXWindow "Autopus" (focused)
    AXButton "Disclosure" (enabled, expanded=<the one parsed bit>)
```

`parseOrcaFixtureTree` (`:144-168`) requires `treeText` to be exactly 14 lines
matching a specific fixture UI and extracts a single boolean from line 6:

```go
if len(lines) != 14 ||
    lines[1] != `Window: "Autopus", App: Autopus Desktop.` ||
    lines[3] != "0 standard window Autopus" ||
    lines[7] != "\t\t\t4 container Autopus fixture status" ||
    lines[8] != "\t\t\t\t5 text Ready" ||
    lines[13] != "The focused UI element is 2 HTML content Autopus." {
```

`validOrcaSnapshot` (`:115-142`) additionally pins `focusedElementId == 2`,
`elementCount == 9`, and `len(treeText) <= 2048`.

The string `Autopus fixture status` exists only in this repository's own test
helpers (`pkg/qa/run/desktop_observe_orca_test_helpers_test.go:158-188`,
`desktop_observe_orca_client_test.go:49`). No shipped application produces it.

### Local v2 provider

`pkg/qa/run/desktop_observe_v2_projection.go:83-177` DOES contain a real
recursive tree decoder and mapper. It is the model to reuse. However
`desktop_observe_v2_projection_validate.go:54-83` restricts every node name to
`Autopus|Disclosure|Status` and mandates an application, a window, and a
`Disclosure` button.

## 2. What Orca actually returns

Measured with `orca computer get-app-state --app <id> --no-screenshot --json`.

`result.snapshot` keys: `app`, `coordinateSpace`, `elementCount`,
`focusedElementId`, `id`, `treeText`, `truncation`, `window`.

**There is no structured node array and no flag that produces one.**
`orca computer get-app-state --help` offers only `--window-id`,
`--window-index`, `--restore-window`, `--no-screenshot`, `--json`. The tree is
available exclusively as rendered text.

### Autopus Desktop (`co.autopus.desktop`), 7 elements, 406 bytes

```
App=co.autopus.desktop (pid 31492)
Window: "Autopus Desktop", App: Autopus Desktop.

0 standard window Autopus Desktop
	1 scroll area
		2 HTML content Autopus Desktop — AI가 회사를 운영합니다
			3 container, Text: 데스크탑 셸을 깨우는 중 Autopus를 준비하는 중입니다.
	4 close button
	5 full screen button, Secondary Actions: zoom the window
	6 minimize button

The focused UI element is 2 HTML content Autopus Desktop — AI가 회사를 운영합니다.
```

### Finder (`com.apple.finder`), 152 elements, 5598 bytes

```
App=com.apple.finder (pid 532)
Window: "응용 프로그램", App: Finder.

0 표준 윈도우 응용 프로그램
	1 자르기 그룹
		2 스크롤 영역, Secondary Actions: scroll up, scroll down
			3 윤곽체 사이드바
				4 윤곽체 행 최근 항목
					5 셀 최근 항목, Secondary Actions: open
						6 텍스트, Value: 최근 항목
```

### Consequences

1. **Role phrases are localized by the OS.** Finder yields `표준 윈도우`,
   `자르기 그룹`, `스크롤 영역`, `셀`, `텍스트`, `이미지`. Autopus Desktop yields
   English only because its UI is a web view. Mapping role phrases to
   `AXApplication`/`AXWindow`/`AXButton` by string comparison would work in one
   locale and silently mis-type in every other.
2. **The 2048-byte cap and `elementCount == 9` pin fail on real apps.** The two
   measured apps produce 406/7 and 5598/152.
3. **Real trees contain user content.** Finder exposes the user's folder names;
   Autopus Desktop exposes in-flight UI copy. Any design that publishes tree
   text publishes user data.
4. The header lines carry app and window identity in a **locale-independent**
   form: `App=<bundleId> (pid N)` and `Window: "<title>", App: <appName>.`

## 3. Existing guards that constrain the design

| Guard | Location | Effect |
|---|---|---|
| Exact name allowlist for every node | `pkg/qa/desktopobserve/oracle.go:214-239` | A node whose name is not declared fails the oracle |
| Allowlist construction | `pkg/qa/run/desktop_observe_pack.go:112-143` | Built from landmarks plus `Disclosure`, `Status` |
| 256 nodes, depth 32 | `pkg/qa/desktopobserve/canonical.go:19-22` | Finder's 152 fits; deeper apps will not |
| 8 KiB typed evidence bound | `pkg/qa/desktopobserve/observation_evidence.go:10-15`, `pkg/qa/evidence/desktop_observation.go:167-197` | ~152 nodes with 64-hex node refs already exceeds this |
| Publication scan | `pkg/qa/evidence/desktop_publication_scan.go:13-104` | Rejects raw AX text, screenshots, paths, handles in published evidence, fail-closed |
| Public-ref grammar | `pkg/qa/desktopobserve/canonical.go:57-60`, `receipt.go:20-21` | Rejects dots and spaces, so `co.autopus.desktop` can never be a published ref |

**Reading:** the name allowlist, the byte bound, and the publication scan are not
first-party pins. They are a privacy boundary. The design must preserve it.

## 4. First-party pins to remove

| Pin | Location |
|---|---|
| Orca bundle id, app name, window title constants | `pkg/qa/run/desktop_observe_orca.go:18-21` |
| Hardcoded `autopus-desktop` / `main-window` in client calls | `pkg/qa/run/desktop_observe_orca_client.go:78-135` |
| Runner receipt scope pins | `pkg/qa/run/desktop_observe_runner.go:102-154,186-201` |
| Protocol `app_ref` / `window_ref` pins | `pkg/qa/desktopobserve/protocol.go:218-246` |
| Local v2 name allowlist and mandatory `Disclosure` | `pkg/qa/run/desktop_observe_v2_projection_validate.go:54-83` |

Already removed earlier in this line of work: the journey-layer `app_ref ==
"autopus-desktop"` identity pin, and the publication contract's requirement that
a project declare Autopus's own `SPEC-QAMESH-012` / `AC-QAMESH12-NNN`.

## 5. Reason-code gap

`pkg/qa/desktopobserve/reasons.go:5-63` defines ten reason codes. There is no
code meaning "the declared landmark was not present in the observed tree"; that
outcome is folded into `semantic_projection_unavailable`
(`pkg/qa/desktopobserve/oracle.go:20-30,241-261`).

Observed consequence: with the app running and permissions granted, the lane
reported `provider_unavailable`, which sent this investigation to the provider
lifecycle instead of to the window-title mismatch that actually caused it.

## 6. Why a new pack field is required

`desktop_observation.app_ref` is an opaque alias restricted to letters, digits,
dot, underscore, hyphen (`pkg/qa/journey/desktop_observation_policy.go:120-135`),
and it flows into published evidence, where the public-ref grammar rejects dots
and spaces. Orca needs a platform identifier (`co.autopus.desktop`) and the real
window title (`Autopus Desktop`, containing a space).

Therefore `app_ref` and `window_ref` must remain publishable aliases, and the
platform identifier must be a separate, request-only field that never reaches
evidence. Landmark `name` already accepts spaces
(`desktop_observation_policy.go:139-152`) and is the correct home for the real
window title.

## 7. Fixtures that must change

Pack fixtures (a new field plus relaxed name rules):

- `pkg/qa/journey/desktop_observation_policy_test.go:194-219,222-254`
- `internal/cli/qa_runtime_provider_test.go:164-189`
- `internal/cli/qa_runtime_provider_unit_test.go:86-108`
- `pkg/qa/release/desktop_observation_release_test.go:93-116`
- `pkg/qa/releasereadiness/desktop_observation_dispatch_test.go:190-207`
- `pkg/qa/run/desktop_observe_test_helpers_test.go:10-84,102-163`

Tree fixtures:

- `pkg/qa/run/desktop_observe_orca_test_helpers_test.go:158-188` (14-line tree,
  `focusedElementId=2`, `elementCount=9`)
- `pkg/qa/run/desktop_observe_v2_test_helpers_test.go:119-175` (6 nodes, three
  `Disclosure` variants)

Not affected, verified by search: `pkg/qa/scaffold/**` emits `node-script` or
`cargo-test` packs for `desktop-native` and never a `desktop_observation` block
(`starters.go:139-150`, `templates.go:8-38`); `pkg/qa/regen/synthesize_packs.go:59-73`
likewise. No desktop-observation pack fixture exists under `testdata/**`,
`templates/**`, `content/**`, `docs/**`, or `.autopus/**`.
