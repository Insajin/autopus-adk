package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/config"
)

type doctorJSONReport struct {
	status   jsonEnvelopeStatus
	data     doctorJSONData
	warnings []jsonMessage
	checks   []jsonCheck
}

type doctorJSONData struct {
	OverallOK     bool                          `json:"overall_ok"`
	Config        *doctorConfigPayload          `json:"config,omitempty"`
	Platforms     []doctorPlatformPayload       `json:"platforms,omitempty"`
	Dependencies  []doctorDependencyPayload     `json:"dependencies,omitempty"`
	Runtime       []doctorRuntimeProcessPayload `json:"runtime_processes,omitempty"`
	RuleConflicts []doctorRuleConflictPayload   `json:"rule_conflicts,omitempty"`
	InstalledCLIs []doctorCLIPayload            `json:"installed_clis,omitempty"`
}

type doctorConfigPayload struct {
	Loaded       bool     `json:"loaded"`
	Mode         string   `json:"mode,omitempty"`
	Platforms    []string `json:"platforms,omitempty"`
	IsolateRules bool     `json:"isolate_rules,omitempty"`
}

type doctorPlatformPayload struct {
	Name     string                 `json:"name"`
	Valid    bool                   `json:"valid"`
	Messages []doctorMessagePayload `json:"messages,omitempty"`
}

type doctorDependencyPayload struct {
	Name       string `json:"name"`
	Binary     string `json:"binary"`
	Installed  bool   `json:"installed"`
	Required   bool   `json:"required"`
	InstallCmd string `json:"install_cmd,omitempty"`
}

type doctorRuntimeProcessPayload struct {
	PID        int    `json:"pid"`
	Executable string `json:"executable"`
	Command    string `json:"command,omitempty"`
	Reason     string `json:"reason,omitempty"`
}

type doctorRuleConflictPayload struct {
	ParentDir string `json:"parent_dir"`
	Namespace string `json:"namespace"`
	Ignored   bool   `json:"ignored"`
}

type doctorCLIPayload struct {
	Name    string `json:"name"`
	Binary  string `json:"binary"`
	Version string `json:"version"`
}

type doctorMessagePayload struct {
	Level   string `json:"level"`
	Message string `json:"message"`
}

func runDoctorJSON(cmd *cobra.Command, opts doctorOptions) error {
	report := collectDoctorJSONReport(cmd, opts)
	report.data.OverallOK = report.status == jsonStatusOK
	return writeJSONResult(cmd, report.status, report.data, report.warnings, report.checks)
}

func collectDoctorJSONReport(cmd *cobra.Command, opts doctorOptions) doctorJSONReport {
	report := doctorJSONReport{status: jsonStatusOK}

	cfg, err := config.Load(opts.dir)
	if err != nil {
		report.status = jsonStatusWarn
		report.data.Config = &doctorConfigPayload{Loaded: false}
		report.warnings = append(report.warnings, jsonMessage{
			Code:    "config_load_failed",
			Message: fmt.Sprintf("autopus.yaml load failed: %v", err),
		})
		report.checks = append(report.checks, jsonCheck{
			ID:       "doctor.config.autopus_yaml",
			Severity: "error",
			Status:   "fail",
			Detail:   fmt.Sprintf("autopus.yaml load failed: %v", err),
		})
		return report
	}

	report.data.Config = &doctorConfigPayload{
		Loaded:       true,
		Mode:         string(cfg.Mode),
		Platforms:    append([]string{}, cfg.Platforms...),
		IsolateRules: cfg.IsolateRules,
	}
	report.checks = append(report.checks, jsonCheck{
		ID:       "doctor.config.autopus_yaml",
		Severity: "info",
		Status:   "pass",
		Detail:   fmt.Sprintf("autopus.yaml loaded (mode: %s)", cfg.Mode),
	})

	report.collectPlatformChecks(context.Background(), opts.dir, cfg)
	report.collectDependencyChecks(cmd, opts)
	report.collectRuntimeProcessChecks(opts)
	report.collectRuleConflictChecks(opts.dir, cfg)
	report.collectCLIChecks()
	report.collectQualityGateChecks(cfg)
	report.collectHookChecks(opts.dir)

	return report
}
