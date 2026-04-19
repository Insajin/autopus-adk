package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/insajin/autopus-adk/pkg/config"
)

const (
	taskCreatedModeWarn          = "warn"
	taskCreatedModeEnforce       = "enforce"
	taskCreatedModeEnvKey        = "TASKCREATED_MODE"
	taskCreatedDefaultModeEnvKey = "AUTOPUS_TASKCREATED_DEFAULT_MODE"
)

type taskCreatedModeResult struct {
	Mode     string
	Source   string
	Warnings []string
}

func normalizeTaskCreatedMode(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

func isValidTaskCreatedMode(mode string) bool {
	switch normalizeTaskCreatedMode(mode) {
	case taskCreatedModeWarn, taskCreatedModeEnforce:
		return true
	default:
		return false
	}
}

func validateTaskCreatedModeFlag(mode string) error {
	if mode == "" {
		return nil
	}
	if isValidTaskCreatedMode(mode) {
		return nil
	}
	return fmt.Errorf("invalid --task-created-mode %q: must be warn or enforce", mode)
}

func taskCreatedModeWarnings(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" || isValidTaskCreatedMode(raw) {
		return nil
	}
	return []string{
		fmt.Sprintf("[warn] %s=%s is invalid; falling back to autopus.yaml/default warn.", taskCreatedModeEnvKey, raw),
	}
}

func resolveTaskCreatedMode(flags globalFlags, cfg *config.HarnessConfig) taskCreatedModeResult {
	envRaw := os.Getenv(taskCreatedModeEnvKey)
	result := taskCreatedModeResult{
		Warnings: taskCreatedModeWarnings(envRaw),
	}

	if mode := normalizeTaskCreatedMode(flags.TaskMode); mode != "" {
		result.Mode = mode
		result.Source = "flag"
		return result
	}

	if mode := normalizeTaskCreatedMode(envRaw); mode != "" && isValidTaskCreatedMode(mode) {
		result.Mode = mode
		result.Source = "env"
		return result
	}

	if cfg != nil {
		if mode := normalizeTaskCreatedMode(cfg.Features.CC21.TaskCreatedMode); mode != "" && isValidTaskCreatedMode(mode) {
			result.Mode = mode
			result.Source = "autopus.yaml"
			return result
		}
	}

	result.Mode = taskCreatedModeWarn
	result.Source = "default"
	return result
}

func applyFlagCC21Overrides(cfg *config.HarnessConfig, flags globalFlags) *config.HarnessConfig {
	if cfg == nil || flags.TaskMode == "" {
		return cfg
	}

	clone := *cfg
	clone.Features.CC21.TaskCreatedMode = flags.TaskMode
	return &clone
}
