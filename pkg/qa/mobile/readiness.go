package mobile

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const readinessRelPath = ".autopus/qa/mobile/readiness.yaml"

type readinessConfig struct {
	DeviceInventory struct {
		Devices []DeviceTarget `yaml:"devices"`
	} `yaml:"device_inventory"`
	SimulatorEmulator struct {
		Targets []DeviceTarget `yaml:"targets"`
	} `yaml:"simulator_emulator"`
	AppArtifact struct {
		Path   string `yaml:"path"`
		Digest string `yaml:"digest"`
	} `yaml:"app_artifact"`
	Credentials struct {
		Refs []string `yaml:"refs"`
	} `yaml:"credentials"`
	CloudLab CloudLabReadiness `yaml:"cloud_lab"`
}

func Assess(projectDir string) Readiness {
	cfg := loadConfig(projectDir)
	readiness := Readiness{
		Status: StatusSetupGap,
		DeviceInventory: DeviceInventoryReadiness{
			Status:  StatusMissing,
			Devices: []DeviceTarget{},
		},
		SimulatorEmulator: SimulatorReadiness{
			Status:  StatusMissing,
			Targets: []DeviceTarget{},
		},
		AppArtifact:     AppArtifactReadiness{Status: StatusMissing},
		Credentials:     CredentialReadiness{Status: StatusMissing, Refs: []string{}},
		CloudLab:        CloudLabReadiness{Status: StatusMissing, BudgetPolicy: "missing", RedactionPolicy: "missing", RetentionPolicy: "missing"},
		SetupGaps:       []SetupGap{},
		RedactionStatus: RedactionStatus{Status: "passed", Findings: []Finding{}},
		SideEffects:     []string{},
	}
	applyDevices(&readiness, cfg)
	applyArtifact(&readiness, cfg.AppArtifact.Path, cfg.AppArtifact.Digest)
	applyCredentials(&readiness, cfg.Credentials.Refs)
	applyCloudLab(&readiness, cfg.CloudLab)
	addMissingSetupGaps(&readiness)
	if readiness.RedactionStatus.Status == "blocked" {
		readiness.Status = StatusSetupGap
		return readiness
	}
	if readiness.CloudLab.Status == StatusDeferred {
		readiness.Status = StatusDeferred
		return readiness
	}
	if len(readiness.SetupGaps) == 0 {
		readiness.Status = StatusReady
	}
	return readiness
}

func loadConfig(projectDir string) readinessConfig {
	path := filepath.Join(projectDir, filepath.FromSlash(readinessRelPath))
	body, err := os.ReadFile(path)
	if err != nil {
		return readinessConfig{}
	}
	var cfg readinessConfig
	_ = yaml.Unmarshal(body, &cfg)
	return cfg
}

func applyDevices(readiness *Readiness, cfg readinessConfig) {
	if len(cfg.DeviceInventory.Devices) > 0 {
		readiness.DeviceInventory.Status = StatusReady
		readiness.DeviceInventory.Devices = sanitizeTargets(cfg.DeviceInventory.Devices, readiness)
	}
	if len(cfg.SimulatorEmulator.Targets) > 0 {
		readiness.SimulatorEmulator.Status = StatusReady
		readiness.SimulatorEmulator.Targets = sanitizeTargets(cfg.SimulatorEmulator.Targets, readiness)
	}
}

func applyArtifact(readiness *Readiness, path, digest string) {
	path = strings.TrimSpace(path)
	digest = strings.TrimSpace(digest)
	if path == "" && digest == "" {
		return
	}
	readiness.AppArtifact.Status = StatusReady
	readiness.AppArtifact.Path = sanitizePlanningPath(path, readiness)
	readiness.AppArtifact.Digest = digest
	if digest == "" {
		readiness.AppArtifact.Status = StatusMissing
	}
}

func applyCredentials(readiness *Readiness, refs []string) {
	if len(refs) == 0 {
		return
	}
	readiness.Credentials.Status = StatusReady
	readiness.Credentials.Refs = make([]string, 0, len(refs))
	for _, ref := range refs {
		readiness.Credentials.Refs = append(readiness.Credentials.Refs, sanitizeCredentialRef(ref, readiness))
	}
}

func applyCloudLab(readiness *Readiness, cloud CloudLabReadiness) {
	if strings.TrimSpace(cloud.Provider) == "" {
		return
	}
	readiness.CloudLab = cloud
	readiness.CloudLab.Status = StatusReady
	if !cloud.OptIn || !policyPresent(cloud.BudgetPolicy) ||
		!policyPresent(cloud.RedactionPolicy) || !policyPresent(cloud.RetentionPolicy) {
		readiness.CloudLab.Status = StatusDeferred
		readiness.SetupGaps = append(readiness.SetupGaps, SetupGap{
			ReasonCode: ReasonCloudLabPolicyIncomplete,
			Message:    "cloud lab opt-in, budget, redaction, and retention policies are required",
		})
	}
}

func addMissingSetupGaps(readiness *Readiness) {
	add := func(status, reason, message string) {
		if status == StatusMissing {
			readiness.SetupGaps = append(readiness.SetupGaps, SetupGap{ReasonCode: reason, Message: message})
		}
	}
	add(readiness.DeviceInventory.Status, ReasonMissingDeviceInventory, "device inventory is required before mobile execution")
	add(readiness.SimulatorEmulator.Status, ReasonMissingSimulatorEmulator, "simulator or emulator target is required before mobile execution")
	add(readiness.AppArtifact.Status, ReasonMissingAppArtifact, "app artifact digest and safe project-relative path are required")
	add(readiness.Credentials.Status, ReasonMissingCredentials, "opaque credential refs are required before mobile execution")
}

func policyPresent(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "present", "configured", "ready":
		return true
	default:
		return false
	}
}
