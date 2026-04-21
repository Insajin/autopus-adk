package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	setuppkg "github.com/insajin/autopus-adk/pkg/setup"
	"github.com/insajin/autopus-adk/pkg/telemetry"
)

func decodeJSONMap(t *testing.T, data []byte) map[string]any {
	t.Helper()

	var payload map[string]any
	require.NoError(t, json.Unmarshal(data, &payload))
	return payload
}

func assertCommonJSONEnvelope(t *testing.T, payload map[string]any, wantCommand string) {
	t.Helper()

	require.Equal(t, cliJSONSchemaVersion, payload["schema_version"])
	require.Equal(t, wantCommand, payload["command"])
	require.Contains(t, payload, "status")
	require.Contains(t, payload, "generated_at")
	require.Contains(t, payload, "data")

	status, ok := payload["status"].(string)
	require.True(t, ok)
	assert.NotEmpty(t, status)

	generatedAt, ok := payload["generated_at"].(string)
	require.True(t, ok)
	_, err := time.Parse(time.RFC3339Nano, generatedAt)
	require.NoError(t, err)
}

func executeRoot(t *testing.T, workdir string, args ...string) ([]byte, error) {
	t.Helper()

	_, currentFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	moduleRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", ".."))

	binPath := filepath.Join(t.TempDir(), "auto-json-contract")
	buildCmd := exec.Command("go", "build", "-o", binPath, "./cmd/auto") //nolint:gosec
	buildCmd.Dir = moduleRoot
	var buildStderr bytes.Buffer
	buildCmd.Stderr = &buildStderr
	require.NoError(t, buildCmd.Run(), buildStderr.String())

	execCmd := exec.Command(binPath, args...) //nolint:gosec
	execCmd.Dir = moduleRoot
	if workdir != "" {
		execCmd.Dir = workdir
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	execCmd.Stdout = &stdout
	execCmd.Stderr = &stderr

	err := execCmd.Run()
	if err != nil {
		return stdout.Bytes(), errors.Join(err, errors.New(stderr.String()))
	}
	return stdout.Bytes(), nil
}

func makeJSONContractWorkspace(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/jsoncontract\n\ngo 1.24.0\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644))

	specPath := filepath.Join(dir, ".autopus", "specs", "SPEC-JSON-001", "spec.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(specPath), 0o755))
	require.NoError(t, os.WriteFile(specPath, []byte(`# SPEC-JSON-001: Contract Fixture

**Status**: approved
`), 0o644))

	_, err := setuppkg.Generate(dir, &setuppkg.GenerateOptions{})
	require.NoError(t, err)

	writeScenariosFile(t, dir, sampleScenariosContent)
	writePipelineRun(t, dir, telemetry.PipelineRun{
		SpecID:      "SPEC-JSON-001",
		FinalStatus: "PASS",
		QualityMode: "balanced",
	})
	writePipelineRun(t, dir, telemetry.PipelineRun{
		SpecID:      "SPEC-JSON-002",
		FinalStatus: "FAIL",
		QualityMode: "ultra",
	})

	return dir
}

func TestJSONContract_CommonEnvelopeSupportsPhaseOneCommands(t *testing.T) {
	t.Parallel()

	commands := []string{
		"auto doctor",
		"auto status",
		"auto setup status",
		"auto setup validate",
		"auto telemetry summary",
		"auto telemetry cost",
		"auto telemetry compare",
	}

	for _, command := range commands {
		command := command
		t.Run(command, func(t *testing.T) {
			t.Parallel()

			var out bytes.Buffer
			require.NoError(t, writeJSONEnvelope(&out, jsonEnvelopeOptions{
				Command: command,
				Status:  jsonStatusOK,
				Data:    map[string]any{"command_id": command},
			}))

			payload := decodeJSONMap(t, out.Bytes())
			assertCommonJSONEnvelope(t, payload, command)
			data := payload["data"].(map[string]any)
			assert.Equal(t, command, data["command_id"])
		})
	}
}

func TestJSONContract_RedactsSecretsAndMasksHomePaths(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	var out bytes.Buffer
	require.NoError(t, writeJSONEnvelope(&out, jsonEnvelopeOptions{
		Command: "auto doctor",
		Status:  jsonStatusWarn,
		Data: map[string]any{
			"access_token": "token-123",
			"config_path":  filepath.Join(tmpHome, "workspace", "config.json"),
			"nested": map[string]any{
				"refresh_token": "refresh-456",
				"log_path":      filepath.Join(tmpHome, "workspace", "doctor.log"),
			},
		},
		Warnings: []jsonMessage{{Message: filepath.Join(tmpHome, "workspace", "warning.md")}},
		Checks:   []jsonCheck{{ID: "doctor.path", Status: "warn", Detail: filepath.Join(tmpHome, "workspace", "detail.md")}},
		Error:    &jsonErrorPayload{Message: filepath.Join(tmpHome, "workspace", "error.md")},
	}))

	payload := decodeJSONMap(t, out.Bytes())
	assertCommonJSONEnvelope(t, payload, "auto doctor")
	assert.NotContains(t, out.String(), tmpHome)
	assert.NotContains(t, out.String(), "token-123")
	assert.NotContains(t, out.String(), "refresh-456")

	data := payload["data"].(map[string]any)
	assert.Equal(t, "[REDACTED]", data["access_token"])
	assert.Equal(t, "~/workspace/config.json", data["config_path"])

	nested := data["nested"].(map[string]any)
	assert.Equal(t, "[REDACTED]", nested["refresh_token"])
	assert.Equal(t, "~/workspace/doctor.log", nested["log_path"])

	warnings := payload["warnings"].([]any)
	assert.Contains(t, warnings[0].(map[string]any)["message"], "~/workspace/warning.md")

	checks := payload["checks"].([]any)
	assert.Contains(t, checks[0].(map[string]any)["detail"], "~/workspace/detail.md")

	jsonErr := payload["error"].(map[string]any)
	assert.Contains(t, jsonErr["message"], "~/workspace/error.md")
}

func TestJSONContract_FatalErrorPathUsesJSONEnvelope(t *testing.T) {
	t.Parallel()

	root := &cobra.Command{Use: "auto"}
	parent := &cobra.Command{Use: "telemetry"}
	cmd := &cobra.Command{Use: "compare"}
	root.AddCommand(parent)
	parent.AddCommand(cmd)

	var out bytes.Buffer
	cmd.SetOut(&out)

	fatal := errors.New("need at least 2 runs to compare")
	err := writeJSONResultAndExit(
		cmd,
		jsonStatusError,
		fatal,
		"telemetry_compare_failed",
		map[string]any{"comparison": "unavailable"},
		nil,
		nil,
	)
	require.Error(t, err)
	require.True(t, isJSONFatalError(err))
	assert.ErrorIs(t, err, fatal)

	payload := decodeJSONMap(t, out.Bytes())
	assertCommonJSONEnvelope(t, payload, "auto telemetry compare")
	assert.Equal(t, string(jsonStatusError), payload["status"])
	assert.Equal(t, "unavailable", payload["data"].(map[string]any)["comparison"])

	jsonErr := payload["error"].(map[string]any)
	assert.Equal(t, "telemetry_compare_failed", jsonErr["code"])
	assert.Equal(t, fatal.Error(), jsonErr["message"])
}

func TestJSONContract_ExistingJSONCommandsRequireEnvelopeAlignment(t *testing.T) {
	dir := makeJSONContractWorkspace(t)
	tests := []struct {
		name        string
		workdir     string
		args        []string
		wantCommand string
	}{
		{name: "permission detect", args: []string{"permission", "detect", "--json"}, wantCommand: "auto permission detect"},
		{name: "test run", args: []string{"test", "run", "--project-dir", dir, "--json"}, wantCommand: "auto test run"},
		{name: "worker status", args: []string{"worker", "status", "--json"}, wantCommand: "auto worker status"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			out, err := executeRoot(t, tt.workdir, tt.args...)
			require.NoError(t, err)
			assertCommonJSONEnvelope(t, decodeJSONMap(t, out), tt.wantCommand)
		})
	}
}

func TestJSONContract_PhaseOneCommandsRequireEnvelopeSupport(t *testing.T) {
	dir := makeJSONContractWorkspace(t)
	tests := []struct {
		name        string
		workdir     string
		args        []string
		wantCommand string
	}{
		{name: "status", args: []string{"status", "--dir", dir, "--json"}, wantCommand: "auto status"},
		{name: "setup status", args: []string{"setup", "status", dir, "--json"}, wantCommand: "auto setup status"},
		{name: "setup validate", args: []string{"setup", "validate", dir, "--json"}, wantCommand: "auto setup validate"},
		{name: "doctor", args: []string{"doctor", "--dir", dir, "--json"}, wantCommand: "auto doctor"},
		{name: "telemetry summary", workdir: dir, args: []string{"telemetry", "summary", "--json"}, wantCommand: "auto telemetry summary"},
		{name: "telemetry cost", workdir: dir, args: []string{"telemetry", "cost", "--json"}, wantCommand: "auto telemetry cost"},
		{name: "telemetry compare", workdir: dir, args: []string{"telemetry", "compare", "--json"}, wantCommand: "auto telemetry compare"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			out, err := executeRoot(t, tt.workdir, tt.args...)
			require.NoError(t, err)
			assertCommonJSONEnvelope(t, decodeJSONMap(t, out), tt.wantCommand)
		})
	}
}
