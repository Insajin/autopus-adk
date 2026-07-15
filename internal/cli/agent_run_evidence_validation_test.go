package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/insajin/autopus-adk/pkg/telemetry"
	"github.com/insajin/autopus-adk/pkg/worker/adapter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestAgentRunEvidence_PreflightRejectsUnsafeContextBeforeSubprocess(t *testing.T) {
	tests := []struct {
		name    string
		oldText string
		newText string
	}{
		{name: "provider", oldText: "provider: codex\n", newText: "provider: claude\n"},
		{name: "model", oldText: "model: gpt-5.6-sol\n", newText: "model: gpt-5.6-terra\n"},
		{name: "effort", oldText: "effort: xhigh\n", newText: "effort: high\n"},
		{name: "attempt", oldText: "attempt: 1\n", newText: "attempt: 2\n"},
		{name: "absolute schema", oldText: "  output_schema: schema.json\n", newText: "  output_schema: /tmp/schema.json\n"},
		{name: "traversal schema", oldText: "  output_schema: schema.json\n", newText: "  output_schema: ../schema.json\n"},
		{name: "unknown field", newText: "unexpected_secret_field: value\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir, runsDir, countFile := setupEvidenceRun(t, "pass")
			writeEvidenceContext(t, runsDir, "safe-prompt", "")
			contextPath := filepath.Join(runsDir, "context.yaml")
			data, readErr := os.ReadFile(contextPath)
			require.NoError(t, readErr)
			if tt.oldText == "" {
				data = append(data, tt.newText...)
			} else {
				data = []byte(strings.Replace(string(data), tt.oldText, tt.newText, 1))
			}
			require.NoError(t, os.WriteFile(contextPath, data, 0o600))

			err := runAgentTask("E01")

			require.Error(t, err)
			assert.Zero(t, invocationCount(t, countFile))
			data, readErr = os.ReadFile(filepath.Join(runsDir, "result.yaml"))
			require.NoError(t, readErr)
			assert.Contains(t, string(data), "status: failed")
			var result taskResult
			require.NoError(t, yaml.Unmarshal(data, &result))
			assert.Equal(t, telemetry.UsageStatusUnavailable, result.UsageStatus)
			require.NotNil(t, result.UniqueModelCallCount)
			assert.Zero(t, *result.UniqueModelCallCount)
			require.NotNil(t, result.ToolCalls)
			assert.Zero(t, *result.ToolCalls)
			assert.Len(t, result.OutputSHA256, 64)
			assert.Equal(t, telemetry.StatusFail, readOnlyTelemetryRun(t, dir).Status)
		})
	}
}

func TestValidateEvidenceContext_RejectsSchemaSymlinkEscape(t *testing.T) {
	dir := t.TempDir()
	runsDir := filepath.Join(dir, ".autopus", "runs", "E01")
	require.NoError(t, os.MkdirAll(runsDir, 0o755))
	outside := filepath.Join(dir, "outside.json")
	require.NoError(t, os.WriteFile(outside, []byte(`{}`), 0o600))
	require.NoError(t, os.Symlink(outside, filepath.Join(runsDir, "schema.json")))
	ctx := validEvidenceContext()

	err := validateEvidenceContext("E01", runsDir, &ctx)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "schema")
}

func TestValidateEvidenceContext_RequiresEverySafetyAndIdentityControl(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*taskContext)
	}{
		{name: "missing spec", mutate: func(ctx *taskContext) { ctx.SpecID = "" }},
		{name: "missing description", mutate: func(ctx *taskContext) { ctx.Description = "" }},
		{name: "task mismatch", mutate: func(ctx *taskContext) { ctx.TaskID = "other" }},
		{name: "workspace write", mutate: func(ctx *taskContext) { ctx.Codex.Sandbox = "workspace-write" }},
		{name: "not ephemeral", mutate: func(ctx *taskContext) { ctx.Codex.Ephemeral = false }},
		{name: "user config", mutate: func(ctx *taskContext) { ctx.Codex.IgnoreUserConfig = false }},
		{name: "rules", mutate: func(ctx *taskContext) { ctx.Codex.IgnoreRules = false }},
		{name: "git check", mutate: func(ctx *taskContext) { ctx.Codex.SkipGitRepoCheck = false }},
		{name: "tools enabled", mutate: func(ctx *taskContext) { ctx.Codex.ZeroToolMode = false }},
		{name: "verdict not strict", mutate: func(ctx *taskContext) { ctx.StrictVerdict = false }},
		{name: "tool receipt optional", mutate: func(ctx *taskContext) { ctx.ZeroToolCallsRequired = false }},
		{name: "zero budget", mutate: func(ctx *taskContext) { ctx.Codex.RawTokenBudget = 0 }},
		{name: "missing schema", mutate: func(ctx *taskContext) { ctx.Codex.OutputSchema = "missing.json" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(dir, "schema.json"), []byte(`{}`), 0o600))
			ctx := validEvidenceContext()
			tt.mutate(&ctx)

			err := validateEvidenceContext("E01", dir, &ctx)

			require.Error(t, err)
		})
	}
}

func TestValidateEvidenceContext_AllowsOnlyApprovedEfforts(t *testing.T) {
	for _, effort := range []string{"xhigh", "max"} {
		t.Run(effort, func(t *testing.T) {
			dir := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(dir, "schema.json"), []byte(`{}`), 0o600))
			ctx := validEvidenceContext()
			ctx.Effort = effort

			require.NoError(t, validateEvidenceContext("E01", dir, &ctx))
		})
	}
}

func TestEvidenceTaskConfig_MapsAllTrustedFields(t *testing.T) {
	ctx := validEvidenceContext()
	cfg := buildAgentTaskConfig("E01", ".autopus/runs/E01", ctx)

	assert.Equal(t, "E01", cfg.TaskID)
	assert.Equal(t, "run-01", cfg.RunID)
	assert.Equal(t, "call-01", cfg.CallID)
	assert.Equal(t, "gpt-5.6-sol", cfg.Model)
	assert.Equal(t, "xhigh", cfg.Effort)
	assert.Equal(t, "review", cfg.Phase)
	assert.Equal(t, "reviewer", cfg.Role)
	assert.Equal(t, adapter.CodexSandboxReadOnly, cfg.Codex.Sandbox)
	assert.Equal(t, "schema.json", cfg.Codex.OutputSchema)
	assert.Equal(t, int64(1000), cfg.Codex.RawTokenBudget)
}

func TestDecodeTaskContext_LegacyUnknownFieldsRemainPermissive(t *testing.T) {
	ctx, err := decodeTaskContext([]byte("task_id: L01\ndescription: legacy\nunknown: allowed\n"))
	require.NoError(t, err)
	assert.Equal(t, "L01", ctx.TaskID)
	assert.False(t, ctx.EvidenceMode)
}

func validEvidenceContext() taskContext {
	return taskContext{
		TaskID: "E01", Description: "prompt", Provider: "codex", Model: "gpt-5.6-sol",
		Effort: "xhigh", SpecID: "SPEC-ADK-ULTRA-EFFICIENCY-001", RunID: "run-01",
		CallID: "call-01", Attempt: 1, Phase: "review", Role: "reviewer",
		ProviderVersion: "codex-cli-0.144.1", ModelVersion: "gpt-5.6-sol-2026-07",
		RiskPolicy: "risk-v1", CacheStratum: "cold", ConfigHash: "sha256:config",
		EvidenceMode: true, StrictVerdict: true, ZeroToolCallsRequired: true,
		Codex: taskCodexContext{
			Sandbox: "read-only", Ephemeral: true, IgnoreUserConfig: true, IgnoreRules: true,
			SkipGitRepoCheck: true, OutputSchema: "schema.json", ZeroToolMode: true, RawTokenBudget: 1000,
		},
	}
}

func TestParseStrictVerdict_RejectsTrailingAndAbsurdObjects(t *testing.T) {
	for _, raw := range []string{
		`{"verdict":"PASS","finding_count":0}{"verdict":"PASS","finding_count":0}`,
		`{"verdict":"PASS","finding_count":1001}`,
		`{"verdict":"UNKNOWN","finding_count":0}`,
		`{"verdict":"PASS","finding_count":-1}`,
	} {
		t.Run(strings.ReplaceAll(raw, " ", "_"), func(t *testing.T) {
			_, err := parseStrictVerdict(raw)
			require.Error(t, err)
		})
	}
}
