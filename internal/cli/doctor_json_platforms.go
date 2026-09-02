package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/insajin/autopus-adk/pkg/adapter"
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
		if platformName == "omp" {
			r.collectOMPReadinessChecks(ctx, dir, validationErrs, payload)
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
			r.appendPlatformValidationFinding(platformName, &payload, validationErr)
		}
		r.data.Platforms = append(r.data.Platforms, payload)
	}
}

// appendPlatformValidationFinding projects one platform validation finding into
// the JSON message payload and into the operator-facing check list. The
// finding's File travels to both surfaces so repeated findings (obsolete
// managed surface reports one per stale file) name the path they refer to
// instead of emitting an identical message N times.
func (r *doctorJSONReport) appendPlatformValidationFinding(
	platformName string,
	payload *doctorPlatformPayload,
	validationErr adapter.ValidationError,
) {
	level := strings.ToLower(strings.TrimSpace(validationErr.Level))
	if level == "" {
		level = "info"
	}
	file := doctorValidationFile(validationErr.File)
	payload.Messages = append(payload.Messages, doctorMessagePayload{
		Level:   level,
		Message: validationErr.Message,
		File:    file,
	})
	checkStatus := "pass"
	switch level {
	case "error":
		checkStatus = "fail"
		r.status = jsonStatusWarn
	case "warn":
		checkStatus = "warn"
		r.status = jsonStatusWarn
	}
	r.checks = append(r.checks, jsonCheck{
		ID:       "doctor.platform." + platformName,
		Severity: level,
		Status:   checkStatus,
		Detail:   doctorPlatformValidationDetail(platformName, validationErr.Message, file),
	})
}

// doctorValidationFile normalizes a finding path to the repo-relative slash
// form the rest of doctor output uses. An empty path stays empty: findings such
// as the unknown-platform report carry no file, and callers must render no
// separator for them.
func doctorValidationFile(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return ""
	}
	return filepath.ToSlash(filepath.Clean(trimmed))
}

// doctorPlatformValidationDetail appends the path only when the finding carries
// one, so pathless findings keep their historical "<platform>: <message>"
// wording with no dangling separator.
func doctorPlatformValidationDetail(platformName, message, file string) string {
	if file == "" {
		return fmt.Sprintf("%s: %s", platformName, message)
	}
	return fmt.Sprintf("%s: %s (%s)", platformName, message, file)
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
	descriptor, ok := lookupPlatformDescriptor(platformName)
	if !ok {
		return nil, fmt.Errorf("unknown platform: %s", platformName)
	}
	return descriptor.Validate(ctx, dir)
}
