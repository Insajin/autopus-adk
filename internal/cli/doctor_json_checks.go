package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/detect"
)

func (r *doctorJSONReport) collectDependencyChecks(cmd *cobra.Command, opts doctorOptions) {
	statuses := detect.CheckDependencies(detect.FullModeDeps)

	if opts.fix {
		missingDeps := filterMissing(statuses)
		if opts.requiredOnly {
			missingDeps = filterRequired(missingDeps)
		}
		if len(missingDeps) > 0 {
			if err := runDoctorFix(cmd.ErrOrStderr(), missingDeps, opts.yes); err != nil {
				r.status = jsonStatusWarn
				r.warnings = append(r.warnings, jsonMessage{
					Code:    "doctor_fix_failed",
					Message: fmt.Sprintf("Auto-install failed: %v", err),
				})
				r.checks = append(r.checks, jsonCheck{
					ID:       "doctor.dependencies.fix",
					Severity: "warning",
					Status:   "warn",
					Detail:   fmt.Sprintf("Auto-install failed: %v", err),
				})
			}
			statuses = detect.CheckDependencies(detect.FullModeDeps)
		}
	}

	for _, dependency := range statuses {
		r.data.Dependencies = append(r.data.Dependencies, doctorDependencyPayload{
			Name:       dependency.Name,
			Binary:     dependency.Binary,
			Installed:  dependency.Installed,
			Required:   dependency.Required,
			InstallCmd: dependency.InstallCmd,
		})

		check := jsonCheck{
			ID:       "doctor.dependency." + dependency.Binary,
			Severity: "info",
			Status:   "pass",
			Detail:   dependency.Name + " installed",
		}
		if !dependency.Installed && dependency.Required {
			check.Severity = "error"
			check.Status = "fail"
			check.Detail = fmt.Sprintf("%s not installed (install: %s)", dependency.Name, dependency.InstallCmd)
			r.status = jsonStatusWarn
		} else if !dependency.Installed {
			check.Severity = "warning"
			check.Status = "warn"
			check.Detail = fmt.Sprintf("%s not installed (optional, install: %s)", dependency.Name, dependency.InstallCmd)
			r.status = jsonStatusWarn
		}
		r.checks = append(r.checks, check)
	}
}

func (r *doctorJSONReport) collectQualityGateChecks(cfg *config.HarnessConfig) {
	if cfg.Quality.Default != "" {
		if _, ok := cfg.Quality.Presets[cfg.Quality.Default]; ok {
			r.checks = append(r.checks, jsonCheck{
				ID:       "doctor.quality.default",
				Severity: "info",
				Status:   "pass",
				Detail:   fmt.Sprintf("quality preset: %s", cfg.Quality.Default),
			})
		} else {
			r.status = jsonStatusWarn
			r.checks = append(r.checks, jsonCheck{
				ID:       "doctor.quality.default",
				Severity: "error",
				Status:   "fail",
				Detail:   fmt.Sprintf("quality preset %q not found in presets", cfg.Quality.Default),
			})
		}
	} else {
		r.status = jsonStatusWarn
		r.checks = append(r.checks, jsonCheck{
			ID:       "doctor.quality.default",
			Severity: "warning",
			Status:   "warn",
			Detail:   "quality preset: not configured",
		})
	}

	if cfg.Spec.ReviewGate.Enabled {
		r.checks = append(r.checks, jsonCheck{
			ID:       "doctor.review_gate.enabled",
			Severity: "info",
			Status:   "pass",
			Detail:   "review gate: enabled",
		})
		installedCount := 0
		for _, provider := range cfg.Spec.ReviewGate.Providers {
			check := jsonCheck{
				ID:       "doctor.review_gate.provider." + provider,
				Severity: "info",
				Status:   "pass",
				Detail:   "provider installed: " + provider,
			}
			if !detect.IsInstalled(provider) {
				check.Severity = "error"
				check.Status = "fail"
				check.Detail = "provider not installed: " + provider
				r.status = jsonStatusWarn
			} else {
				installedCount++
			}
			r.checks = append(r.checks, check)
		}
		if installedCount < 2 {
			r.status = jsonStatusWarn
			r.checks = append(r.checks, jsonCheck{
				ID:       "doctor.review_gate.provider_count",
				Severity: "warning",
				Status:   "warn",
				Detail:   "review gate: fewer than 2 providers available",
			})
		}
	} else {
		r.status = jsonStatusWarn
		r.checks = append(r.checks, jsonCheck{
			ID:       "doctor.review_gate.enabled",
			Severity: "warning",
			Status:   "warn",
			Detail:   "review gate: disabled",
		})
	}

	r.checks = append(r.checks, jsonCheck{
		ID:       "doctor.methodology.mode",
		Severity: "info",
		Status:   "pass",
		Detail:   fmt.Sprintf("methodology: %s (enforce: %v)", cfg.Methodology.Mode, cfg.Methodology.Enforce),
	})
}

func (r *doctorJSONReport) collectHookChecks(dir string) {
	settingsPath := filepath.Join(dir, ".claude", "settings.json")
	settingsData, err := os.ReadFile(settingsPath)
	if err != nil {
		r.status = jsonStatusWarn
		r.checks = append(r.checks, jsonCheck{
			ID:       "doctor.hooks.settings",
			Severity: "warning",
			Status:   "warn",
			Detail:   ".claude/settings.json not found (run 'auto init' to generate)",
		})
		return
	}

	var settings map[string]any
	if err := json.Unmarshal(settingsData, &settings); err != nil {
		r.status = jsonStatusWarn
		r.checks = append(r.checks, jsonCheck{
			ID:       "doctor.hooks.settings",
			Severity: "error",
			Status:   "fail",
			Detail:   "settings.json parse failed",
		})
		return
	}

	if hooksValue, ok := settings["hooks"]; ok {
		if hooksMap, ok := hooksValue.(map[string]any); ok && len(hooksMap) > 0 {
			r.checks = append(r.checks, jsonCheck{
				ID:       "doctor.hooks.configured",
				Severity: "info",
				Status:   "pass",
				Detail:   fmt.Sprintf("hooks: %d event(s) configured", len(hooksMap)),
			})
		} else {
			r.status = jsonStatusWarn
			r.checks = append(r.checks, jsonCheck{
				ID:       "doctor.hooks.configured",
				Severity: "warning",
				Status:   "warn",
				Detail:   "hooks: empty or invalid format",
			})
		}
	} else {
		r.status = jsonStatusWarn
		r.checks = append(r.checks, jsonCheck{
			ID:       "doctor.hooks.configured",
			Severity: "warning",
			Status:   "warn",
			Detail:   "hooks: not configured (run 'auto update' to install)",
		})
	}

	if permissionsValue, ok := settings["permissions"]; ok {
		if permissionsMap, ok := permissionsValue.(map[string]any); ok {
			if allowList, ok := permissionsMap["allow"].([]any); ok && len(allowList) > 0 {
				r.checks = append(r.checks, jsonCheck{
					ID:       "doctor.permissions.allow",
					Severity: "info",
					Status:   "pass",
					Detail:   fmt.Sprintf("permissions: %d allow rule(s)", len(allowList)),
				})
				return
			}
		}
		r.status = jsonStatusWarn
		r.checks = append(r.checks, jsonCheck{
			ID:       "doctor.permissions.allow",
			Severity: "warning",
			Status:   "warn",
			Detail:   "permissions.allow: empty",
		})
		return
	}

	r.status = jsonStatusWarn
	r.checks = append(r.checks, jsonCheck{
		ID:       "doctor.permissions.allow",
		Severity: "warning",
		Status:   "warn",
		Detail:   "permissions: not configured (run 'auto update' to install)",
	})
}
