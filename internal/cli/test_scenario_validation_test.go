package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type scenarioValidationCLISummary struct {
	Passed  int `json:"passed"`
	Failed  int `json:"failed"`
	Skipped int `json:"skipped"`
	Invalid int `json:"invalid"`
	Total   int `json:"total"`
}

type scenarioValidationCLIDiagnostic struct {
	Code        string `json:"code"`
	ScenarioRef string `json:"scenario_ref"`
	Field       string `json:"field"`
	Line        int    `json:"line"`
}

type scenarioValidationCLIEnvelope struct {
	Status string `json:"status"`
	Data   struct {
		Summary     scenarioValidationCLISummary      `json:"summary"`
		Results     []scenarioJSONResult              `json:"results"`
		Diagnostics []scenarioValidationCLIDiagnostic `json:"diagnostics"`
	} `json:"data"`
}

func TestTestRunCmd_ScenarioValidationModesQuarantineBeforeBuildAndShell(t *testing.T) {
	t.Parallel()

	type modeResult struct {
		payload scenarioValidationCLIEnvelope
		err     error
	}
	results := make(map[string]modeResult)

	for _, mode := range []string{"default", "warn", "enforce"} {
		mode := mode
		t.Run(mode, func(t *testing.T) {
			projectDir := t.TempDir()
			buildSentinel := filepath.Join(projectDir, "build-called")
			shellSentinel := filepath.Join(projectDir, "shell-called")
			writeScenariosFile(t, projectDir, invalidScenarioWithSentinels(buildSentinel, shellSentinel))

			cmd := newAutoTestCmd()
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			args := []string{"run", "--project-dir", projectDir, "--format", "json"}
			if mode != "default" {
				args = append(args, "--scenario-validation", mode)
			}
			cmd.SetArgs(args)

			err := cmd.Execute()
			if mode == "enforce" {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			assert.NoFileExists(t, buildSentinel, "validation must run before build construction")
			assert.NoFileExists(t, shellSentinel, "invalid scenario command must stay quarantined")

			var payload scenarioValidationCLIEnvelope
			require.NoError(t, json.Unmarshal(out.Bytes(), &payload), out.String())
			assert.Equal(t, scenarioValidationCLISummary{
				Passed:  0,
				Failed:  0,
				Skipped: 0,
				Invalid: 1,
				Total:   0,
			}, payload.Data.Summary)
			assert.Empty(t, payload.Data.Results, "zero runnable scenarios must not fabricate PASS")
			require.Len(t, payload.Data.Diagnostics, 1)
			assert.Equal(t, "scenario_missing_verify", payload.Data.Diagnostics[0].Code)
			assert.Equal(t, "S15A", payload.Data.Diagnostics[0].ScenarioRef)
			assert.Equal(t, "Verify", payload.Data.Diagnostics[0].Field)
			assert.Positive(t, payload.Data.Diagnostics[0].Line)
			results[mode] = modeResult{payload: payload, err: err}
		})
	}

	require.Contains(t, results, "default")
	require.Contains(t, results, "warn")
	require.Contains(t, results, "enforce")
	assert.Equal(t, "warn", results["default"].payload.Status)
	assert.Equal(t, "warn", results["warn"].payload.Status)
	assert.Equal(t, results["warn"].payload.Data.Diagnostics, results["enforce"].payload.Data.Diagnostics)
	assert.Equal(t, results["warn"].payload.Data.Summary, results["enforce"].payload.Data.Summary)
	assert.NoError(t, results["warn"].err)
	assert.Error(t, results["enforce"].err)
}

func TestTestRunCmd_WarnTextReportsInvalidWithoutPass(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	buildSentinel := filepath.Join(projectDir, "build-called")
	shellSentinel := filepath.Join(projectDir, "shell-called")
	writeScenariosFile(t, projectDir, invalidScenarioWithSentinels(buildSentinel, shellSentinel))

	cmd := newAutoTestCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{
		"run",
		"--project-dir", projectDir,
		"--scenario-validation", "warn",
	})

	err := cmd.Execute()
	require.NoError(t, err)
	assert.NoFileExists(t, buildSentinel)
	assert.NoFileExists(t, shellSentinel)
	assert.Contains(t, out.String(), "scenario_missing_verify S15A Verify")
	assert.Contains(t, out.String(), "0 runnable")
	assert.NotContains(t, out.String(), "PASS")
}

func invalidScenarioWithSentinels(buildSentinel, shellSentinel string) string {
	return fmt.Sprintf(`# E2E Scenarios — validation-cli

## Project Type: CLI
## Binary: auto
## Build: touch %q

### S15A: invalid-active — Missing verify
- **Command**: `+"`touch %q`"+`
- **Status**: active
`, buildSentinel, shellSentinel)
}
