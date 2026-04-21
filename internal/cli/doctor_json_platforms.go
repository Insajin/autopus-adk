package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/adapter/claude"
	"github.com/insajin/autopus-adk/pkg/adapter/codex"
	"github.com/insajin/autopus-adk/pkg/adapter/gemini"
	"github.com/insajin/autopus-adk/pkg/adapter/opencode"
	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/detect"
)

func (r *doctorJSONReport) collectPlatformChecks(ctx context.Context, dir string, cfg *config.HarnessConfig) {
	for _, platformName := range cfg.Platforms {
		validationErrs, validateErr := validateDoctorPlatform(ctx, dir, platformName)
		payload := doctorPlatformPayload{Name: platformName, Valid: validateErr == nil && len(validationErrs) == 0}

		if validateErr != nil {
			payload.Messages = append(payload.Messages, doctorMessagePayload{
				Level:   "error",
				Message: validateErr.Error(),
			})
			r.checks = append(r.checks, jsonCheck{
				ID:       "doctor.platform." + platformName,
				Severity: "error",
				Status:   "fail",
				Detail:   fmt.Sprintf("%s validation failed: %v", platformName, validateErr),
			})
			r.status = jsonStatusWarn
			r.data.Platforms = append(r.data.Platforms, payload)
			continue
		}

		if len(validationErrs) == 0 {
			r.checks = append(r.checks, jsonCheck{
				ID:       "doctor.platform." + platformName,
				Severity: "info",
				Status:   "pass",
				Detail:   platformName + " validated successfully.",
			})
			r.data.Platforms = append(r.data.Platforms, payload)
			continue
		}

		for _, validationErr := range validationErrs {
			level := strings.ToLower(strings.TrimSpace(validationErr.Level))
			if level == "" {
				level = "info"
			}
			payload.Messages = append(payload.Messages, doctorMessagePayload{
				Level:   level,
				Message: validationErr.Message,
			})
			checkStatus := "pass"
			if level == "error" {
				checkStatus = "fail"
				r.status = jsonStatusWarn
			} else if level == "warn" {
				checkStatus = "warn"
				r.status = jsonStatusWarn
			}
			r.checks = append(r.checks, jsonCheck{
				ID:       "doctor.platform." + platformName,
				Severity: level,
				Status:   checkStatus,
				Detail:   fmt.Sprintf("%s: %s", platformName, validationErr.Message),
			})
		}
		r.data.Platforms = append(r.data.Platforms, payload)
	}
}

func (r *doctorJSONReport) collectRuleConflictChecks(dir string, cfg *config.HarnessConfig) {
	conflicts := detect.CheckParentRuleConflicts(dir)
	for _, conflict := range conflicts {
		r.data.RuleConflicts = append(r.data.RuleConflicts, doctorRuleConflictPayload{
			ParentDir: conflict.ParentDir,
			Namespace: conflict.Namespace,
			Ignored:   cfg.IsolateRules,
		})

		check := jsonCheck{
			ID:       "doctor.rule_conflict." + conflict.Namespace,
			Severity: "warning",
			Status:   "warn",
			Detail:   fmt.Sprintf("Parent rules detected: %s/.claude/rules/%s/", conflict.ParentDir, conflict.Namespace),
		}
		if cfg.IsolateRules {
			check.Severity = "info"
			check.Status = "pass"
			check.Detail = fmt.Sprintf("%s/.claude/rules/%s/ ignored due to isolate_rules", conflict.ParentDir, conflict.Namespace)
		} else {
			r.status = jsonStatusWarn
		}
		r.checks = append(r.checks, check)
	}
}

func (r *doctorJSONReport) collectCLIChecks() {
	detected := detect.DetectPlatforms()
	for _, platform := range detected {
		r.data.InstalledCLIs = append(r.data.InstalledCLIs, doctorCLIPayload{
			Name:    platform.Name,
			Binary:  platform.Binary,
			Version: platform.Version,
		})
		r.checks = append(r.checks, jsonCheck{
			ID:       "doctor.cli." + platform.Binary,
			Severity: "info",
			Status:   "pass",
			Detail:   fmt.Sprintf("%s (%s)", platform.Name, platform.Version),
		})
	}
	if len(detected) == 0 {
		r.status = jsonStatusWarn
		r.warnings = append(r.warnings, jsonMessage{
			Code:    "coding_clis_missing",
			Message: "No coding CLIs detected in PATH.",
		})
		r.checks = append(r.checks, jsonCheck{
			ID:       "doctor.cli.detect",
			Severity: "warning",
			Status:   "warn",
			Detail:   "No coding CLIs detected in PATH.",
		})
	}
}

func validateDoctorPlatform(
	ctx context.Context,
	dir string,
	platformName string,
) ([]adapter.ValidationError, error) {
	switch platformName {
	case "claude-code":
		return claude.NewWithRoot(dir).Validate(ctx)
	case "codex":
		return codex.NewWithRoot(dir).Validate(ctx)
	case "gemini-cli":
		return gemini.NewWithRoot(dir).Validate(ctx)
	case "opencode":
		return opencode.NewWithRoot(dir).Validate(ctx)
	default:
		return nil, fmt.Errorf("unknown platform: %s", platformName)
	}
}
