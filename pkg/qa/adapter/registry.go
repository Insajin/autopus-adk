package adapter

type Metadata struct {
	ID                   string   `json:"id"`
	Surfaces             []string `json:"surfaces"`
	RequiredBinaries     []string `json:"required_binaries"`
	SupportedPlatforms   []string `json:"supported_platforms,omitempty"`
	DefaultLanes         []string `json:"default_lanes"`
	ArtifactCapabilities []string `json:"artifact_capabilities"`
	ReadinessFields      []string `json:"readiness_fields,omitempty"`
	SetupGapReason       string   `json:"setup_gap_reason,omitempty"`
	SetupGapReasonCodes  []string `json:"setup_gap_reason_codes,omitempty"`
}

func Registry() []Metadata {
	return []Metadata{
		metadata("go-test", []string{"cli"}, []string{"go"}),
		metadata("node-script", []string{"package"}, []string{"node", "npm"}),
		metadata("vitest", []string{"frontend", "package"}, []string{"node", "npm"}),
		metadata("jest", []string{"frontend", "package"}, []string{"node", "npm"}),
		metadata("playwright", []string{"frontend"}, []string{"node", "npm"}),
		metadata("gui-explore", []string{"frontend", "desktop"}, []string{"node", "npm"}),
		metadata("design-visual", []string{"frontend", "design"}, []string{"node", "npm"}),
		metadata("maestro-scripted", []string{"mobile"}, []string{"maestro"}),
		metadata("appium-mobile-explore", []string{"mobile"}, []string{"appium"}),
		metadata("pytest", []string{"cli"}, []string{"pytest"}),
		metadata("cargo-test", []string{"cli"}, []string{"cargo"}),
		metadata("auto-test-run", []string{"multi"}, []string{"auto"}),
		metadata("auto-verify", []string{"frontend"}, []string{"auto"}),
		metadata("canary-template", []string{"multi"}, nil),
		metadata("custom-command", []string{"custom"}, nil),
	}
}

func ByID(id string) (Metadata, bool) {
	for _, item := range Registry() {
		if item.ID == id {
			return item, true
		}
	}
	return Metadata{}, false
}

func metadata(id string, surfaces, binaries []string) Metadata {
	item := Metadata{
		ID:                   id,
		Surfaces:             surfaces,
		RequiredBinaries:     binaries,
		DefaultLanes:         []string{"fast"},
		ArtifactCapabilities: []string{"stdout", "stderr"},
	}
	if id == "gui-explore" {
		item.DefaultLanes = []string{"gui-explore"}
		item.ArtifactCapabilities = append(item.ArtifactCapabilities,
			"journey_graph",
			"aria_snapshot",
			"a11y_violations",
			"console_summary",
			"network_summary",
			"screenshot_quarantine_ref",
			"video_trace_ref",
			"dom_snapshot_digest",
		)
	}
	if id == "design-visual" {
		item.DefaultLanes = []string{"design-visual"}
		item.ArtifactCapabilities = []string{
			"design_pack",
			"visual_gate_report",
			"screenshot_diff_summary",
			"code_connect_audit",
			"figma_node_metadata",
			"stdout",
			"stderr",
		}
		item.SetupGapReasonCodes = []string{
			"design_context_missing",
			"token_refs_missing",
			"component_refs_missing",
			"screenshot_baseline_missing",
			"code_connect_mapping_missing",
			"figma_token_missing",
		}
		item.SetupGapReason = "design visual readiness requires a design pack, rendered screenshot evidence, and optional Figma/Code Connect metadata for stronger component reuse"
	}
	// @AX:NOTE: [AUTO] magic constants — mobile adapter default lanes must stay in sync with laneMobileScripted and mobileAdapter() in pkg/qa/run
	if id == "maestro-scripted" || id == "appium-mobile-explore" {
		item.DefaultLanes = []string{"mobile-readiness"}
		if id == "maestro-scripted" {
			item.DefaultLanes = []string{"mobile-readiness", "mobile-scripted"}
		}
		item.SupportedPlatforms = []string{"ios", "android"}
		item.ArtifactCapabilities = []string{
			"sanitized_log",
			"app_artifact_digest",
			"device_metadata",
			"deterministic_checks",
			"screenshot_quarantine_ref",
			"video_quarantine_ref",
		}
		item.ReadinessFields = []string{
			"device_inventory",
			"simulator_emulator",
			"app_artifact",
			"credentials",
			"cloud_lab",
		}
		item.SetupGapReasonCodes = []string{
			"missing_device_inventory",
			"missing_simulator_emulator",
			"missing_app_artifact",
			"missing_credentials",
			"cloud_lab_policy_incomplete",
			"project_local_flow_required",
			"device_ref_unresolved",
			"app_artifact_digest_mismatch",
		}
		item.SetupGapReason = "mobile readiness requires device inventory, simulator/emulator target, app artifact digest, opaque credentials, and cloud lab policy when used"
	}
	return item
}
