package cli

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/telemetry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestAgentRunEvidence_ExecutionFailuresAreFailClosedAndSingleShot(t *testing.T) {
	for _, mode := range []string{
		"tool", "missing_usage", "ambiguous_usage", "invalid_output", "over_budget",
		"fail_verdict", "parse_error", "provider_error", "exec_error", "auth_error", "provider_model_error",
		"provider_turn_failed_shape", "oversized_stream",
	} {
		t.Run(mode, func(t *testing.T) {
			dir, runsDir, countFile := setupEvidenceRun(t, mode)
			writeEvidenceContext(t, runsDir, "sensitive-prompt", "")

			started := time.Now()
			err := runAgentTask("E01")
			require.Error(t, err)
			assert.Equal(t, 1, invocationCount(t, countFile), "evidence execution must never retry")
			if mode == "oversized_stream" {
				assert.Less(t, time.Since(started), 5*time.Second, "scanner failure did not reap the live subprocess")
				pidBytes, readErr := os.ReadFile(filepath.Join(dir, "codex.pid"))
				require.NoError(t, readErr)
				pid, parseErr := strconv.Atoi(strings.TrimSpace(string(pidBytes)))
				require.NoError(t, parseErr)
				assert.False(t, processAliveForTest(pid), "scanner failure left the provider subprocess alive")
			}

			data, readErr := os.ReadFile(filepath.Join(runsDir, "result.yaml"))
			require.NoError(t, readErr)
			assert.NotContains(t, string(data), "sensitive-prompt")
			assert.NotContains(t, string(data), "SECRET-RESPONSE")
			assertSanitizedEvidenceArtifacts(t, dir, runsDir, "sensitive-prompt", "SECRET-RESPONSE")
			var result taskResult
			require.NoError(t, yaml.Unmarshal(data, &result))
			assert.Equal(t, "failed", result.Status)
			if mode == "tool" {
				require.NotNil(t, result.ToolCalls)
				assert.Equal(t, 1, *result.ToolCalls)
			}
			if mode == "auth_error" {
				assert.Equal(t, "authentication", result.OperationalErrorClass)
				assert.Regexp(t, `^sha256:[0-9a-f]{64}$`, result.OperationalErrorFingerprint)
				assert.Equal(t, "process_wait", result.OperationalErrorStage)
				assert.Equal(t, []string{"stderr"}, result.OperationalErrorSignals)
				assert.NotContains(t, string(data), "SUPER-SECRET-AUTH")
			}
			if mode == "provider_model_error" {
				assert.Equal(t, "model_access", result.OperationalErrorClass)
				assert.Equal(t, "process_wait", result.OperationalErrorStage)
				assert.Equal(t, []string{"provider_failure_event"}, result.OperationalErrorSignals)
				assert.Equal(t, "error", result.OperationalProviderEventKind)
				assert.Equal(t, []string{"top_level_message"}, result.OperationalProviderEventShape)
				assert.NotContains(t, string(data), "MODEL-EVENT-SECRET")
			}
			if mode == "provider_turn_failed_shape" {
				assert.Equal(t, "unknown", result.OperationalErrorClass)
				assert.Equal(t, "process_wait", result.OperationalErrorStage)
				assert.Equal(t, []string{"provider_failure_event"}, result.OperationalErrorSignals)
				assert.Equal(t, "turn_failed", result.OperationalProviderEventKind)
				assert.Equal(t, []string{"nested_error_object", "nested_error_message", "nested_error_type", "nested_error_code"}, result.OperationalProviderEventShape)
				assert.NotContains(t, string(data), "PROVIDER-SHAPE-SECRET")
			}
			if mode == "oversized_stream" {
				assert.Equal(t, "stream_scan", result.OperationalErrorStage)
				assert.Equal(t, []string{"stderr"}, result.OperationalErrorSignals)
				assert.NotContains(t, string(data), "OVERSIZED-STDERR-SECRET")
			}

			run := readOnlyTelemetryRun(t, dir)
			assert.Equal(t, telemetry.StatusFail, run.Status)
			if mode == "tool" {
				assert.Equal(t, 1, run.ToolCalls)
			}
		})
	}
}

func TestAgentRunEvidence_TelemetryAppendFailureStillWritesFailedResult(t *testing.T) {
	_, runsDir, countFile := setupEvidenceRun(t, "pass")
	writeEvidenceContext(t, runsDir, "sensitive-prompt", "")
	require.NoError(t, os.WriteFile(filepath.Join(".autopus", "telemetry"), []byte("conflict"), 0o600))

	err := runAgentTask("E01")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "telemetry")
	assert.Equal(t, 1, invocationCount(t, countFile))
	data, readErr := os.ReadFile(filepath.Join(runsDir, "result.yaml"))
	require.NoError(t, readErr)
	var result taskResult
	require.NoError(t, yaml.Unmarshal(data, &result))
	assert.Equal(t, "failed", result.Status)
}

func TestValidateEvidenceUsage_IdentityMismatchFails(t *testing.T) {
	input, output := int64(5), int64(2)
	usage := telemetry.NormalizeUsage(telemetry.UsageInput{
		RunID: "wrong-run", CallID: "call-01", TaskID: "E01", Attempt: 1,
		Provider: "codex", Model: "gpt-5.6-sol", Effort: "xhigh",
		ProviderVersion: "codex-cli-0.144.1", ModelVersion: "gpt-5.6-sol-2026-07",
		RiskPolicy: "risk-v1", CacheStratum: "cold", ConfigHash: "sha256:config",
		Phase: "review", Role: "reviewer", Source: telemetry.UsageSourceProvider,
		InputTokensTotal: &input, OutputTokensTotal: &output,
	})
	ctx := validEvidenceContext()

	_, err := validateEvidenceUsage(ctx, []telemetry.UsageEnvelope{usage})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "identity")
}
