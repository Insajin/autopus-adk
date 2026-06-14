package mobile

import qaevidence "github.com/insajin/autopus-adk/pkg/qa/evidence"

const (
	StatusReady    = "ready"
	StatusMissing  = "missing"
	StatusSetupGap = "setup_gap"
	StatusDeferred = "deferred"

	ReasonMissingDeviceInventory   = "missing_device_inventory"
	ReasonMissingSimulatorEmulator = "missing_simulator_emulator"
	ReasonMissingAppArtifact       = "missing_app_artifact"
	ReasonMissingCredentials       = "missing_credentials"
	ReasonCloudLabPolicyIncomplete = "cloud_lab_policy_incomplete"
	ReasonProjectLocalFlowRequired = "project_local_flow_required"

	ReasonDeviceRefUnresolved       = "device_ref_unresolved"
	ReasonAppArtifactDigestMismatch = "app_artifact_digest_mismatch"
)

type Finding = qaevidence.Finding

type Readiness struct {
	Status            string                   `json:"status"`
	DeviceInventory   DeviceInventoryReadiness `json:"device_inventory"`
	SimulatorEmulator SimulatorReadiness       `json:"simulator_emulator"`
	AppArtifact       AppArtifactReadiness     `json:"app_artifact"`
	Credentials       CredentialReadiness      `json:"credentials"`
	CloudLab          CloudLabReadiness        `json:"cloud_lab"`
	SetupGaps         []SetupGap               `json:"setup_gaps"`
	RedactionStatus   RedactionStatus          `json:"redaction_status"`
	SideEffects       []string                 `json:"side_effects"`
}

type DeviceInventoryReadiness struct {
	Status  string         `json:"status"`
	Devices []DeviceTarget `json:"devices"`
}

type SimulatorReadiness struct {
	Status  string         `json:"status"`
	Targets []DeviceTarget `json:"targets"`
}

type DeviceTarget struct {
	DeviceRef string `json:"device_ref,omitempty" yaml:"device_ref"`
	TargetRef string `json:"target_ref,omitempty" yaml:"target_ref"`
	Platform  string `json:"platform,omitempty" yaml:"platform"`
}

type AppArtifactReadiness struct {
	Status string `json:"status"`
	Path   string `json:"path"`
	Digest string `json:"digest"`
}

type CredentialReadiness struct {
	Status string   `json:"status"`
	Refs   []string `json:"refs"`
}

type CloudLabReadiness struct {
	Status          string `json:"status"`
	Provider        string `json:"provider"`
	OptIn           bool   `json:"opt_in"`
	BudgetPolicy    string `json:"budget_policy"`
	RedactionPolicy string `json:"redaction_policy"`
	RetentionPolicy string `json:"retention_policy"`
}

type SetupGap struct {
	ReasonCode string `json:"reason_code"`
	Message    string `json:"message"`
}

type RedactionStatus struct {
	Status   string    `json:"status"`
	Findings []Finding `json:"findings"`
}
