package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/insajin/autopus-adk/pkg/telemetry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestAgentRunEvidence_PassPersistsSanitizedActualReceipt(t *testing.T) {
	dir, runsDir, countFile := setupEvidenceRun(t, "pass")
	secret := "PROMPT-SECRET-MUST-NOT-PERSIST"
	writeEvidenceContext(t, runsDir, secret, "")

	err := runAgentTask("E01")
	require.NoError(t, err)
	assert.Equal(t, 1, invocationCount(t, countFile))

	resultBytes, err := os.ReadFile(filepath.Join(runsDir, "result.yaml"))
	require.NoError(t, err)
	assert.NotContains(t, string(resultBytes), secret)
	assert.NotContains(t, string(resultBytes), `{"verdict"`)
	assert.NotContains(t, string(resultBytes), "session_id")
	assertSanitizedEvidenceArtifacts(t, dir, runsDir, secret, `{"verdict":"PASS","finding_count":0}`)
	var result taskResult
	require.NoError(t, yaml.Unmarshal(resultBytes, &result))
	assert.Equal(t, "success", result.Status)
	assert.Equal(t, "PASS", result.Verdict)
	require.NotNil(t, result.FindingCount)
	assert.Zero(t, *result.FindingCount)
	require.NotNil(t, result.RawTotalTokens)
	assert.Equal(t, int64(30), *result.RawTotalTokens)
	assert.Equal(t, telemetry.UsageStatusActual, result.UsageStatus)
	require.NotNil(t, result.UniqueModelCallCount)
	assert.Equal(t, 1, *result.UniqueModelCallCount)
	require.NotNil(t, result.ToolCalls)
	assert.Zero(t, *result.ToolCalls)
	assert.Len(t, result.OutputSHA256, 64)

	run := readOnlyTelemetryRun(t, dir)
	assert.Equal(t, telemetry.StatusPass, run.Status)
	assert.Equal(t, "SPEC-ADK-ULTRA-EFFICIENCY-001", run.SpecID)
	assert.Equal(t, "codex", run.Provider)
	assert.Equal(t, "gpt-5.6-sol", run.Model)
	assert.Equal(t, "xhigh", run.Effort)
	assert.Equal(t, "review", run.Phase)
	assert.Equal(t, "reviewer", run.Role)
	assert.Zero(t, run.ToolCalls)
	require.Len(t, run.Usage, 1)
	assert.Equal(t, "codex-cli-0.144.1", run.Usage[0].ProviderVersion)
	assert.Equal(t, "gpt-5.6-sol-2026-07", run.Usage[0].ModelVersion)
	assert.Equal(t, "risk-v1", run.Usage[0].RiskPolicy)
	assert.Equal(t, "cold", run.Usage[0].CacheStratum)
	assert.Equal(t, "sha256:config", run.Usage[0].ConfigHash)

	pwdBytes, err := os.ReadFile(filepath.Join(dir, "codex.pwd"))
	require.NoError(t, err)
	wantPWD, err := filepath.EvalSymlinks(runsDir)
	require.NoError(t, err)
	assert.Equal(t, wantPWD, strings.TrimSpace(string(pwdBytes)))
	argsBytes, err := os.ReadFile(filepath.Join(dir, "codex.args"))
	require.NoError(t, err)
	args := string(argsBytes)
	assert.Contains(t, args, "--sandbox read-only")
	assert.Contains(t, args, "--ephemeral")
	assert.Contains(t, args, "--ignore-user-config")
	assert.Contains(t, args, "--ignore-rules")
	assert.Contains(t, args, "--skip-git-repo-check")
	assert.Contains(t, args, "--output-schema schema.json")
	assert.Contains(t, args, "--disable multi_agent")
	assert.Contains(t, args, "--disable shell_tool")
	assert.Contains(t, args, "-m gpt-5.6-sol")
	assert.Contains(t, args, `model_reasoning_effort="xhigh"`)
	assert.Contains(t, args, "limit_tokens=1000")
	assert.NotContains(t, args, "dangerously-bypass")
}

func setupEvidenceRun(t *testing.T, mode string) (string, string, string) {
	t.Helper()
	dir := t.TempDir()
	runsDir := filepath.Join(dir, ".autopus", "runs", "E01")
	require.NoError(t, os.MkdirAll(runsDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(runsDir, "schema.json"), []byte(`{"type":"object"}`), 0o600))
	binDir := filepath.Join(dir, "bin")
	require.NoError(t, os.MkdirAll(binDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(binDir, "codex"), []byte(fakeCodexScript), 0o755))
	countFile := filepath.Join(dir, "codex.count")
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FAKE_CODEX_MODE", mode)
	t.Setenv("FAKE_CODEX_COUNT_FILE", countFile)
	t.Setenv("FAKE_CODEX_PWD_FILE", filepath.Join(dir, "codex.pwd"))
	t.Setenv("FAKE_CODEX_ARGS_FILE", filepath.Join(dir, "codex.args"))
	t.Setenv("FAKE_CODEX_PID_FILE", filepath.Join(dir, "codex.pid"))
	chdirForTest(t, dir)
	return dir, runsDir, countFile
}

func writeEvidenceContext(t *testing.T, runsDir, description, overrides string) {
	t.Helper()
	contextYAML := `task_id: E01
description: ` + description + `
provider: codex
model: gpt-5.6-sol
effort: xhigh
spec_id: SPEC-ADK-ULTRA-EFFICIENCY-001
run_id: run-01
call_id: call-01
attempt: 1
phase: review
role: reviewer
provider_version: codex-cli-0.144.1
model_version: gpt-5.6-sol-2026-07
risk_policy: risk-v1
cache_stratum: cold
config_hash: sha256:config
evidence_mode: true
strict_verdict: true
zero_tool_calls_required: true
codex:
  sandbox: read-only
  ephemeral: true
  ignore_user_config: true
  ignore_rules: true
  skip_git_repo_check: true
  output_schema: schema.json
  zero_tool_mode: true
  raw_token_budget: 1000
` + overrides
	require.NoError(t, os.WriteFile(filepath.Join(runsDir, "context.yaml"), []byte(contextYAML), 0o600))
}

func readOnlyTelemetryRun(t *testing.T, dir string) telemetry.AgentRun {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, ".autopus", "telemetry", "*.jsonl"))
	require.NoError(t, err)
	require.Len(t, files, 1)
	data, err := os.ReadFile(files[0])
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	require.Len(t, lines, 1)
	var event telemetry.Event
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &event))
	var run telemetry.AgentRun
	require.NoError(t, json.Unmarshal(event.Data, &run))
	return run
}

func assertSanitizedEvidenceArtifacts(t *testing.T, dir, runsDir string, secrets ...string) {
	t.Helper()
	paths := []string{filepath.Join(runsDir, "result.yaml")}
	telemetryFiles, err := filepath.Glob(filepath.Join(dir, ".autopus", "telemetry", "*.jsonl"))
	require.NoError(t, err)
	paths = append(paths, telemetryFiles...)
	for _, path := range paths {
		data, readErr := os.ReadFile(path)
		require.NoError(t, readErr)
		for _, secret := range secrets {
			assert.NotContains(t, string(data), secret, "raw prompt/response leaked to %s", path)
		}
	}
}

func invocationCount(t *testing.T, path string) int {
	t.Helper()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return 0
	}
	require.NoError(t, err)
	return len(strings.Fields(string(data)))
}

func chdirForTest(t *testing.T, dir string) {
	t.Helper()
	original, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(original) })
}

const fakeCodexScript = `#!/bin/sh
printf '1\n' >> "$FAKE_CODEX_COUNT_FILE"
pwd > "$FAKE_CODEX_PWD_FILE"
printf '%s\n' "$*" > "$FAKE_CODEX_ARGS_FILE"
case "$FAKE_CODEX_MODE" in
  pass)
    printf '%s\n' '{"type":"item.completed","item":{"type":"agent_message","text":"{\"verdict\":\"PASS\",\"finding_count\":0}"}}'
    printf '%s\n' '{"type":"turn.completed","usage":{"input_tokens":20,"output_tokens":10}}'
    ;;
  tool)
	printf '%s\n' '{"type":"item.completed","item":{"type":"command_execution","command":"pwd"}}'
    printf '%s\n' '{"type":"item.completed","item":{"type":"agent_message","text":"{\"verdict\":\"PASS\",\"finding_count\":0}"}}'
    printf '%s\n' '{"type":"turn.completed","usage":{"input_tokens":20,"output_tokens":10}}'
    ;;
  missing_usage)
    printf '%s\n' '{"type":"item.completed","item":{"type":"agent_message","text":"{\"verdict\":\"PASS\",\"finding_count\":0}"}}'
    ;;
  ambiguous_usage)
    printf '%s\n' '{"type":"item.completed","item":{"type":"agent_message","text":"{\"verdict\":\"PASS\",\"finding_count\":0}"}}'
    printf '%s\n' '{"type":"turn.completed","usage":{"input_tokens":20,"output_tokens":10}}'
    printf '%s\n' '{"type":"turn.completed","usage":{"input_tokens":21,"output_tokens":10}}'
    ;;
  invalid_output)
    printf '%s\n' '{"type":"item.completed","item":{"type":"agent_message","text":"{\"verdict\":\"PASS\",\"finding_count\":0,\"raw\":\"SECRET-RESPONSE\"}"}}'
    printf '%s\n' '{"type":"turn.completed","usage":{"input_tokens":20,"output_tokens":10}}'
    ;;
  over_budget)
    printf '%s\n' '{"type":"item.completed","item":{"type":"agent_message","text":"{\"verdict\":\"PASS\",\"finding_count\":0}"}}'
    printf '%s\n' '{"type":"turn.completed","usage":{"input_tokens":900,"output_tokens":200}}'
    ;;
  fail_verdict)
    printf '%s\n' '{"type":"item.completed","item":{"type":"agent_message","text":"{\"verdict\":\"FAIL\",\"finding_count\":1}"}}'
    printf '%s\n' '{"type":"turn.completed","usage":{"input_tokens":20,"output_tokens":10}}'
    ;;
  diagnostic_fail)
    printf '%s\n' '{"type":"item.completed","item":{"type":"agent_message","text":"{\"verdict\":\"FAIL\",\"finding_count\":1,\"finding_codes\":[\"security\"],\"finding_scope_hashes\":[\"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\"]}"}}'
    printf '%s\n' '{"type":"turn.completed","usage":{"input_tokens":20,"output_tokens":10}}'
    ;;
  parse_error)
	printf '%s\n' 'not-json'
	printf '%s\n' '{"type":"item.completed","item":{"type":"agent_message","text":"{\"verdict\":\"PASS\",\"finding_count\":0}"}}'
	printf '%s\n' '{"type":"turn.completed","usage":{"input_tokens":20,"output_tokens":10}}'
	;;
  provider_error)
	printf '%s\n' '{"type":"error","message":"SECRET-RESPONSE"}'
	printf '%s\n' '{"type":"item.completed","item":{"type":"agent_message","text":"{\"verdict\":\"PASS\",\"finding_count\":0}"}}'
	printf '%s\n' '{"type":"turn.completed","usage":{"input_tokens":20,"output_tokens":10}}'
	;;
  exec_error)
    printf '%s\n' '{"type":"item.completed","item":{"type":"agent_message","text":"{\"verdict\":\"PASS\",\"finding_count\":0}"}}'
    printf '%s\n' '{"type":"turn.completed","usage":{"input_tokens":20,"output_tokens":10}}'
    exit 9
    ;;
  auth_error)
    printf '%s\n' '401 Unauthorized login required SUPER-SECRET-AUTH' >&2
    exit 1
    ;;
  provider_model_error)
    printf '%s\n' '{"type":"error","message":"model gpt-5.6-sol is not available MODEL-EVENT-SECRET"}'
    exit 1
    ;;
  provider_turn_failed_shape)
    printf '%s\n' '{"type":"turn.failed","error":{"message":"PROVIDER-SHAPE-SECRET","type":"opaque","code":"private"}}'
    exit 1
    ;;
  oversized_stream)
    printf '%s\n' "$$" > "$FAKE_CODEX_PID_FILE"
    printf '%s\n' 'OVERSIZED-STDERR-SECRET' >&2
    (i=0; while [ "$i" -lt 20000 ]; do
      printf '%s\n' 'OVERSIZED-STDERR-SECRET' >&2
      i=$((i + 1))
    done) &
    awk 'BEGIN { for (i = 0; i < 70000; i++) printf "x"; printf "\n" }'
    while :; do :; done
    ;;
esac
`
