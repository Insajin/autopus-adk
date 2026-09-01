package scaffold

// desktopObservationReadme documents the desktop-native observation lane in the
// capture README, for projects that show desktop signals.
//
// It is documentation rather than a generated pack, for the same reason `auto qa
// init` does not generate a gui-explore pack: the harness cannot know the app's
// real window title or its platform identifier, and a guessed pack would be born
// failing. `desktop-native` is a `must` lane in the prelaunch profile, so an
// unrunnable generated pack would turn the release gate red on first use.
//
// Without this section the capability is undiscoverable: `qa init` emits a
// cargo-test or node-script pack for `desktop-native`, and nothing anywhere tells
// a project that read-only accessibility observation exists or what schema it
// takes. Delivered-but-undiscoverable is the same failure as
// advertised-but-not-delivered, one step further out.
func desktopObservationReadme(signals projectSignals) []string {
	if !signals.HasDesktopGUI && !signals.HasTauriRust {
		return nil
	}
	return append([]string{
		"",
		"## Desktop accessibility observation (optional)",
		"",
		"`auto qa init` generates a plain test pack for the `desktop-native` lane.",
		"To observe the running app's accessibility tree instead, replace it with an",
		"observation pack. The harness reads the tree, judges the landmarks you",
		"declare, and publishes only those - no other node's name, value, or text",
		"reaches evidence, because a real tree carries user content.",
		"",
		"Requirements: macOS, the `orca` CLI on PATH, accessibility permission",
		"granted to it, and the app running with a focused window.",
		"",
		"Fill in `provider_app_id` with the platform identifier and the landmark",
		"names with the real labels. Read them from the live app:",
		"",
		"```bash",
		"orca computer list-apps --json",
		"orca computer list-windows --app <bundle-id> --json",
		"```",
		"",
		"```yaml",
	},
		append(desktopObservationReadmePack(), desktopObservationReadmeNotes()...)...)
}

func desktopObservationReadmePack() []string {
	return []string{
		"# Replace .autopus/qa/journeys/desktop-native.yaml with this to observe",
		"# instead of running a test binary. Every value below is project-specific.",
		"id: desktop-accessibility-observe",
		"title: Desktop read-only observation",
		"surface: desktop",
		"lanes: [\"desktop-native\"]",
		"adapter:",
		"  id: desktop-accessibility-observe",
		"pass_fail_authority: deterministic",
		"checks:",
		"  - id: desktop-observation",
		"    type: desktop_observation",
		"    expected:",
		"      accessibility_granted: true",
		"desktop_observation:",
		"  platform: macos",
		"  # Platform identifier the provider resolves. Request-only: it never",
		"  # appears in a manifest, artifact, receipt, or projection.",
		"  provider_app_id: com.example.myapp",
		"  # Opaque aliases that DO appear in published evidence, so they stay",
		"  # alias-shaped: letters, digits, dot, underscore, hyphen.",
		"  app_ref: myapp",
		"  window_ref: main-window",
		"  operations: [capabilities, permissions, list_apps, list_windows, get_state]",
		"  required_landmarks:",
		"    # The first two are mandatory and positional: app and window identity",
		"    # are derived from them. Names are the real accessibility labels and may",
		"    # contain spaces.",
		"    - role: AXApplication",
		"      name: My App",
		"      required_state: enabled",
		"    - role: AXWindow",
		"      name: My App - Main",
		"      required_state: focused",
		"    # Optional. Declare what you actually care about inside the window; only",
		"    # declared landmarks are published. States must be observable:",
		"    # enabled, focused, selected, expanded.",
		"    - role: AXButton",
		"      name: Save",
		"      required_state: enabled",
		"source_refs:",
		"  source_spec: SPEC-MYAPP-001",
		"  acceptance_refs: [AC-MYAPP-001]",
		"```",
	}
}

func desktopObservationReadmeNotes() []string {
	return []string{
		"",
		"- The observation is read-only. The operation set is fixed and contains no",
		"  click, type, or scroll; the lane cannot mutate your app.",
		"- Role phrases in the provider's tree are emitted in the OS locale, so the",
		"  harness never infers a role from them. App and window identity come from",
		"  the tree header; deeper landmarks are matched by the name you declare.",
		"- A declared landmark the tree does not contain is reported as",
		"  `declared_landmark_not_found`, naming the role and name - not as a",
		"  provider failure.",
		"- Run it with `auto qa run --lane desktop-native --runtime-provider orca`.",
		"  The `local` provider additionally requires a signature-verified release",
		"  artifact and is intended for a project shipping its own signed helper.",
	}
}
