package run

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/insajin/autopus-adk/pkg/qa/journey"
)

func applyOracle(projectDir string, pack journey.Pack, result *commandResult, check *IndexCheck) {
	expected := firstExpected(pack)
	exitCode := expectedExitCode(expected)
	check.Expected = formatExpected(expected)
	check.Actual = fmt.Sprintf("exit_code=%d", result.ExitCode)
	if result.Status == "blocked" {
		check.Status = "blocked"
		check.FailureSummary = result.FailureSummary
		return
	}
	if result.ExitCode != exitCode {
		result.Status = "failed"
		result.FailureSummary = fmt.Sprintf("expected exit_code=%d, got %d", exitCode, result.ExitCode)
		check.Status = "failed"
		check.FailureSummary = result.FailureSummary
		return
	}
	if needle, ok := stringExpected(expected, "stdout_contains"); ok && !strings.Contains(result.StdoutText, needle) {
		result.Status = "failed"
		result.FailureSummary = "stdout did not contain expected text"
		check.Status = "failed"
		check.Actual += "; stdout_contains=false"
		check.FailureSummary = result.FailureSummary
		return
	}
	if file, ok := stringExpected(expected, "file_exists"); ok && !safeFileExists(projectDir, file) {
		result.Status = "failed"
		result.FailureSummary = "expected file was not created"
		check.Status = "failed"
		check.Actual += "; file_exists=false"
		check.FailureSummary = result.FailureSummary
		return
	}
	result.Status = "passed"
	result.FailureSummary = ""
	check.Status = "passed"
}

func firstExpected(pack journey.Pack) map[string]any {
	if len(pack.Checks) == 0 || len(pack.Checks[0].Expected) == 0 {
		return map[string]any{"exit_code": 0}
	}
	return pack.Checks[0].Expected
}

func expectedExitCode(expected map[string]any) int {
	value, ok := expected["exit_code"]
	if !ok {
		return 0
	}
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case string:
		parsed, err := strconv.Atoi(typed)
		if err == nil {
			return parsed
		}
	}
	return 0
}

func stringExpected(expected map[string]any, key string) (string, bool) {
	value, ok := expected[key]
	if !ok {
		return "", false
	}
	text, ok := value.(string)
	return text, ok && text != ""
}

func formatExpected(expected map[string]any) string {
	if len(expected) == 0 {
		return "exit_code=0"
	}
	keys := make([]string, 0, len(expected))
	for key := range expected {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%v", key, expected[key]))
	}
	return strings.Join(parts, ", ")
}

func safeFileExists(projectDir, relPath string) bool {
	if filepath.IsAbs(relPath) {
		return false
	}
	clean := filepath.Clean(relPath)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return false
	}
	_, err := os.Stat(filepath.Join(projectDir, clean))
	return err == nil
}
