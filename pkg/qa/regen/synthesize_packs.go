package regen

import "github.com/insajin/autopus-adk/pkg/qa/journey"

// The pack constructors below mirror the scaffold starter templates field for
// field so every synthesized pack passes journey.Validate. They are structural
// mirrors, NOT behavioral extraction: the argv/lanes/source_refs match the
// canonical starters (pkg/qa/scaffold/starters.go, templates.go,
// mobile_starter.go) verified against the live code.

// @AX:NOTE [AUTO] @AX:SPEC: SPEC-QAMESH-011: starter IDs, timeout, and mobile path/device constants are structural mirrors of pkg/qa/scaffold starters — any mismatch causes synthesized packs to diverge from scaffold expectations and fail journey.Validate.
const (
	webStarterID     = "browser-staging-playwright"
	desktopStarterID = "desktop-native"
	mobileStarterID  = "mobile-scripted-maestro"

	defaultTimeout      = "240s"
	mobileFlowPath      = ".autopus/qa/mobile/flows/smoke.yaml"
	mobileDeviceTarget  = "device-ref:local-emulator"
	mobilePlaceholderID = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
)

func standardSourceRefs(spec, acceptance string) journey.SourceRefs {
	return journey.SourceRefs{
		SourceSpec:     spec,
		AcceptanceRefs: []string{acceptance},
		OwnedPaths:     []string{"."},
		DoNotModifyPaths: []string{
			".codex/**",
			".opencode/**",
			".autopus/plugins/**",
		},
	}
}

func deterministicChecks(id string) []journey.Check {
	return []journey.Check{{
		ID:       id,
		Type:     "deterministic",
		Expected: map[string]any{"exit_code": 0},
	}}
}

// webStarterPack mirrors browserStagingStarter: adapter playwright, lane
// browser-staging, argv [npm exec playwright test], source_spec SPEC-QAMESH-005.
func webStarterPack() journey.Pack {
	return journey.Pack{
		ID:         webStarterID,
		Title:      "Browser staging Playwright lane",
		Surface:    "frontend",
		Lanes:      []string{"browser-staging"},
		Adapter:    journey.AdapterRef{ID: "playwright"},
		Command:    journey.Command{Argv: []string{"npm", "exec", "playwright", "test"}, CWD: ".", Timeout: defaultTimeout},
		Checks:     deterministicChecks(webStarterID),
		SourceRefs: standardSourceRefs("SPEC-QAMESH-005", "AC-QAMESH2-005"),
	}
}

// desktopStarterPack mirrors desktopNativeStarter (node-script branch): adapter
// node-script, lane desktop-native, argv [npm run build], source_spec
// SPEC-QAMESH-005.
func desktopStarterPack() journey.Pack {
	return journey.Pack{
		ID:         desktopStarterID,
		Title:      "Desktop native release lane",
		Surface:    "desktop",
		Lanes:      []string{"desktop-native"},
		Adapter:    journey.AdapterRef{ID: "node-script"},
		Command:    journey.Command{Argv: []string{"npm", "run", "build"}, CWD: ".", Timeout: defaultTimeout},
		Checks:     deterministicChecks(desktopStarterID),
		SourceRefs: standardSourceRefs("SPEC-QAMESH-005", "AC-QAMESH2-005"),
	}
}

// mobileStarterPack mirrors mobileScriptedStarter exactly so validateMaestroPolicy
// passes: project-local YAML flow, opaque device target, argv matching flow_path.
func mobileStarterPack() journey.Pack {
	return journey.Pack{
		ID:      mobileStarterID,
		Title:   "Mobile scripted Maestro lane",
		Surface: "mobile",
		Lanes:   []string{"mobile-scripted"},
		Adapter: journey.AdapterRef{ID: "maestro-scripted"},
		Command: journey.Command{
			Argv:    []string{"maestro", "test", mobileFlowPath},
			CWD:     ".",
			Timeout: defaultTimeout,
		},
		Checks: deterministicChecks(mobileStarterID),
		Mobile: journey.MobilePolicy{
			FlowPath:          mobileFlowPath,
			DeviceTarget:      mobileDeviceTarget,
			AppArtifactDigest: mobilePlaceholderID,
		},
		SourceRefs: standardSourceRefs("SPEC-QAMESH-008", "AC-QAMESH8-009"),
	}
}
