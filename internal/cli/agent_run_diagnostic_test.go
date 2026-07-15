package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/insajin/autopus-adk/pkg/telemetry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestParseDiagnosticVerdict_AcceptsBoundedSanitizedReceipts(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want diagnosticVerdictReceipt
	}{
		{name: "pass", raw: `{"verdict":"PASS","finding_count":0,"finding_codes":[],"finding_scope_hashes":[]}`,
			want: diagnosticVerdictReceipt{Verdict: "PASS", FindingCount: 0, FindingCodes: []string{}, FindingScopeHashes: []string{}}},
		{name: "fail", raw: `{"verdict":"FAIL","finding_count":2,"finding_codes":["correctness","test_gap"],"finding_scope_hashes":["sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"]}`,
			want: diagnosticVerdictReceipt{Verdict: "FAIL", FindingCount: 2, FindingCodes: []string{"correctness", "test_gap"},
				FindingScopeHashes: []string{"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseDiagnosticVerdict(tt.raw)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseDiagnosticVerdict_RejectsRawOrUnboundedReceipts(t *testing.T) {
	tests := map[string]string{
		"legacy shape":    `{"verdict":"PASS","finding_count":0}`,
		"unknown code":    `{"verdict":"FAIL","finding_count":1,"finding_codes":["style"],"finding_scope_hashes":["sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"]}`,
		"too many codes":  `{"verdict":"FAIL","finding_count":4,"finding_codes":["correctness","security","regression","test_gap"],"finding_scope_hashes":["sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"]}`,
		"bad scope hash":  `{"verdict":"FAIL","finding_count":1,"finding_codes":["security"],"finding_scope_hashes":["internal/cli/file.go"]}`,
		"raw finding":     `{"verdict":"FAIL","finding_count":1,"finding_codes":["security"],"finding_scope_hashes":["sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"],"finding":"secret text"}`,
		"pass with code":  `{"verdict":"PASS","finding_count":0,"finding_codes":["correctness"],"finding_scope_hashes":[]}`,
		"count mismatch":  `{"verdict":"FAIL","finding_count":2,"finding_codes":["correctness"],"finding_scope_hashes":["sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"]}`,
		"duplicate code":  `{"verdict":"FAIL","finding_count":2,"finding_codes":["security","security"],"finding_scope_hashes":["sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"]}`,
		"duplicate scope": `{"verdict":"FAIL","finding_count":1,"finding_codes":["security"],"finding_scope_hashes":["sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"]}`,
	}
	for name, raw := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := parseDiagnosticVerdict(raw)
			require.Error(t, err)
		})
	}
}

func TestValidateEvidenceContext_DiagnosticModeRequiresExactSchemaPair(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, diagnosticVerdictSchemaBasename), []byte(`{}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "schema.json"), []byte(`{}`), 0o600))
	require.NoError(t, os.Mkdir(filepath.Join(dir, "nested"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "nested", diagnosticVerdictSchemaBasename), []byte(`{}`), 0o600))

	ctx := validEvidenceContext()
	ctx.DiagnosticMode = true
	ctx.Codex.OutputSchema = diagnosticVerdictSchemaBasename
	require.NoError(t, validateEvidenceContext("E01", dir, &ctx))

	ctx.Codex.OutputSchema = "schema.json"
	require.Error(t, validateEvidenceContext("E01", dir, &ctx))
	ctx.Codex.OutputSchema = filepath.Join("nested", diagnosticVerdictSchemaBasename)
	require.Error(t, validateEvidenceContext("E01", dir, &ctx))
	ctx.DiagnosticMode = false
	ctx.Codex.OutputSchema = diagnosticVerdictSchemaBasename
	require.Error(t, validateEvidenceContext("E01", dir, &ctx))
}

func TestEvaluateEvidenceResult_DiagnosticModePersistsOnlyBoundedFields(t *testing.T) {
	ctx := validEvidenceContext()
	ctx.DiagnosticMode = true
	res := validDiagnosticExecResult(t, ctx, `{"verdict":"PASS","finding_count":0,"finding_codes":[],"finding_scope_hashes":[]}`)

	result, err := evaluateEvidenceResult("E01", ctx, res, nil)

	require.NoError(t, err)
	assert.Equal(t, "success", result.Status)
	assert.Empty(t, result.FindingCodes)
	assert.Empty(t, result.FindingScopeHashes)
}

func TestEvaluateEvidenceResult_DiagnosticFailIsValidObservation(t *testing.T) {
	ctx := validEvidenceContext()
	ctx.DiagnosticMode = true
	raw := `{"verdict":"FAIL","finding_count":1,"finding_codes":["security"],"finding_scope_hashes":["sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"]}`
	res := validDiagnosticExecResult(t, ctx, raw)

	result, err := evaluateEvidenceResult("E01", ctx, res, nil)

	require.NoError(t, err)
	assert.Equal(t, "success", result.Status)
	assert.Equal(t, "FAIL", result.Verdict)
	assert.Equal(t, []string{"security"}, result.FindingCodes)
	assert.Len(t, result.FindingScopeHashes, 1)
}

func TestEvaluateEvidenceResult_LegacyFailRemainsOperationalFailure(t *testing.T) {
	ctx := validEvidenceContext()
	res := validDiagnosticExecResult(t, ctx, `{"verdict":"FAIL","finding_count":1}`)

	result, err := evaluateEvidenceResult("E01", ctx, res, nil)

	require.Error(t, err)
	assert.Equal(t, "failed", result.Status)
}

func TestAgentRunEvidence_DiagnosticFailPersistsAsValidSanitizedObservation(t *testing.T) {
	dir, runsDir, countFile := setupEvidenceRun(t, "diagnostic_fail")
	require.NoError(t, os.WriteFile(filepath.Join(runsDir, diagnosticVerdictSchemaBasename), []byte(`{}`), 0o600))
	writeEvidenceContext(t, runsDir, "diagnostic-prompt-secret", "diagnostic_mode: true\n")
	contextPath := filepath.Join(runsDir, "context.yaml")
	data, err := os.ReadFile(contextPath)
	require.NoError(t, err)
	data = []byte(strings.Replace(string(data), "output_schema: schema.json", "output_schema: "+diagnosticVerdictSchemaBasename, 1))
	require.NoError(t, os.WriteFile(contextPath, data, 0o600))

	err = runAgentTask("E01")

	require.NoError(t, err)
	assert.Equal(t, 1, invocationCount(t, countFile))
	resultBytes, readErr := os.ReadFile(filepath.Join(runsDir, "result.yaml"))
	require.NoError(t, readErr)
	assert.NotContains(t, string(resultBytes), "diagnostic-prompt-secret")
	var result taskResult
	require.NoError(t, yaml.Unmarshal(resultBytes, &result))
	assert.Equal(t, "success", result.Status)
	assert.Equal(t, "FAIL", result.Verdict)
	assert.Equal(t, []string{"security"}, result.FindingCodes)
	assert.Len(t, result.FindingScopeHashes, 1)
	run := readOnlyTelemetryRun(t, dir)
	assert.Equal(t, telemetry.StatusPass, run.Status)
	assert.Equal(t, "FAIL", run.AcceptanceStatus)
}

func validDiagnosticExecResult(t *testing.T, ctx taskContext, output string) execResult {
	t.Helper()
	inputTokens, outputTokens := int64(20), int64(10)
	usage := telemetry.NormalizeUsage(telemetry.UsageInput{
		RunID: ctx.RunID, CallID: ctx.CallID, TaskID: ctx.TaskID, Attempt: ctx.Attempt,
		Provider: ctx.Provider, Model: ctx.Model, Effort: ctx.Effort,
		ProviderVersion: ctx.ProviderVersion, ModelVersion: ctx.ModelVersion,
		RiskPolicy: ctx.RiskPolicy, CacheStratum: ctx.CacheStratum, ConfigHash: ctx.ConfigHash,
		Phase: ctx.Phase, Role: ctx.Role, Source: telemetry.UsageSourceProvider,
		InputTokensTotal: &inputTokens, OutputTokensTotal: &outputTokens,
	})
	return execResult{Status: "success", Output: output, Usage: []telemetry.UsageEnvelope{usage}}
}
