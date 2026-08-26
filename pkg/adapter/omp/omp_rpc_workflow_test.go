package omp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type ompLiveReceipt struct {
	Command              string   `json:"command"`
	Version              string   `json:"version"`
	CommandBodyHash      string   `json:"command_body_hash"`
	AutoBodyHash         string   `json:"auto_body_hash"`
	PlanBodyHash         string   `json:"plan_body_hash"`
	ContextProfile       string   `json:"context_profile"`
	Projection           []string `json:"projection"`
	LoopbackRequests     int      `json:"loopback_requests"`
	ExternalRequests     int      `json:"external_network_requests"`
	AllowedConnections   int      `json:"sandbox_allowed_endpoint_connections"`
	DeniedOtherLoopback  int      `json:"sandbox_denied_other_loopback"`
	DeniedNonLoopback    int      `json:"sandbox_denied_non_loopback"`
	OutboundControlScope string   `json:"outbound_control_scope"`
	Status               string   `json:"status"`
}

func TestOMPRPCWorkflowSmoke(t *testing.T) {
	if os.Getenv("AUTOPUS_OMP_LIVE") != "1" {
		t.Skip("set AUTOPUS_OMP_LIVE=1 to run the isolated actual OMP RPC smoke")
	}
	executable, err := exec.LookPath(cliBinary)
	require.NoError(t, err, "actual omp binary is required for the live gate")
	versionProfile := t.TempDir()
	versionOverlay := writeOMPLiveOverlay(t, versionProfile)
	version := probeOMPLiveVersion(t, executable, versionProfile, versionOverlay)

	tests := []struct {
		name, prompt   string
		commandNeedles []string
	}{
		{
			name:   "router",
			prompt: `/auto plan "fixture parity"`,
			commandNeedles: []string{
				"## Router Contract",
				"Route to exactly one matching detail skill",
				"fixture parity",
			},
		},
		{
			name:   "direct-alias",
			prompt: `/auto-plan "fixture parity"`,
			commandNeedles: []string{
				"Load exact detail skill `auto-plan`",
				"fixture parity",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			receipt := runOMPLiveWorkflowTurn(t, executable, version, tc.prompt, tc.commandNeedles)
			assert.Equal(t, ompLiveOutboundControlScope, receipt.OutboundControlScope)
			assert.Zero(t, receipt.ExternalRequests)
			assert.Equal(t, 1, receipt.AllowedConnections)
			assert.Equal(t, 1, receipt.DeniedOtherLoopback)
			assert.Equal(t, 2, receipt.DeniedNonLoopback)
			encoded, marshalErr := json.Marshal(receipt)
			require.NoError(t, marshalErr)
			for _, forbidden := range []string{"sk-", executable, os.Getenv("HOME")} {
				if forbidden != "" {
					assert.NotContains(t, string(encoded), forbidden)
				}
			}
			t.Logf("sanitized_live_receipt=%s", encoded)
		})
	}
}

func runOMPLiveWorkflowTurn(
	t *testing.T,
	executable, version, prompt string,
	commandNeedles []string,
) ompLiveReceipt {
	t.Helper()
	scratch := generateOMPOnly(t)
	commandName := "auto"
	if strings.HasPrefix(prompt, "/auto-plan") {
		commandName = "auto-plan"
	}
	commandBody := readOMPGeneratedBody(t, scratch, ".agents", "commands", commandName+".md")
	autoBody := readOMPGeneratedBody(t, scratch, ".agents", "skills", "auto", "SKILL.md")
	planBody := readOMPGeneratedBody(t, scratch, ".agents", "skills", "auto-plan", "SKILL.md")
	assertOMPPlanProfile(t, planBody)

	expandedCommandBody := expandedOMPLiveCommandBody(prompt, commandBody)
	provider := newOMPFakeProvider(t, commandNeedles, expandedCommandBody, autoBody, planBody)
	sandboxCtx, cancelSandbox := context.WithTimeout(context.Background(), 5*time.Second)
	sandboxEvidence, sandboxErr := probeOMPRPCNetworkSandbox(sandboxCtx, provider.server.URL)
	cancelSandbox()
	require.NoError(t, sandboxErr, "OS-level network sandbox and deny counterexamples are mandatory")
	profile := t.TempDir()
	writeOMPLiveModelConfig(t, profile, provider.server.URL)
	overlay := writeOMPLiveOverlay(t, profile)

	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()
	trace, err := runActualOMPRPC(ctx, executable, scratch, profile, overlay, prompt, provider.server.URL)
	if err != nil {
		requests, stages, loopbackAuthHeaders, unexpectedLoopbackEndpoints, fixtureFailure := provider.receipt()
		t.Fatalf("actual RPC failed: code=%v loopback_requests=%d stages=%d loopback_auth_headers=%d unexpected_loopback_endpoints=%d fixture=%s",
			err, requests, stages, loopbackAuthHeaders, unexpectedLoopbackEndpoints, fixtureFailure)
	}
	require.Equal(t, []string{
		"skill:auto",
		"skill:auto-plan",
		"tool_execution_start:write",
		"tool_execution_end:write",
		"message_end",
	}, trace.projection)
	require.True(t, trace.availableAuto)
	require.True(t, trace.availableAutoPlan)
	require.Equal(t, ompReceiptName, trace.writeTarget)
	require.Equal(t, 3, trace.pairedToolCalls)

	receiptPath := filepath.Join(scratch, ompReceiptName)
	actualReceipt, readErr := os.ReadFile(receiptPath)
	require.NoError(t, readErr)
	assert.Equal(t, ompReceiptJSON, string(actualReceipt))
	require.True(t, pathWithinOMPTestRoot(scratch, receiptPath))

	requests, stages, loopbackAuthHeaders, unexpectedLoopbackEndpoints, fixtureFailure := provider.receipt()
	require.Empty(t, fixtureFailure)
	assert.Equal(t, 4, requests)
	assert.Equal(t, 4, stages)
	assert.Zero(t, loopbackAuthHeaders)
	assert.Zero(t, unexpectedLoopbackEndpoints)

	return ompLiveReceipt{
		Command: commandName, Version: version,
		CommandBodyHash: hashOMPBody(commandBody),
		AutoBodyHash:    hashOMPBody(autoBody),
		PlanBodyHash:    hashOMPBody(planBody),
		ContextProfile:  "plan", Projection: trace.projection,
		LoopbackRequests:     requests,
		ExternalRequests:     0,
		AllowedConnections:   sandboxEvidence.AllowedEndpointConnections,
		DeniedOtherLoopback:  sandboxEvidence.DeniedOtherLoopback,
		DeniedNonLoopback:    sandboxEvidence.DeniedNonLoopback,
		OutboundControlScope: ompLiveOutboundControlScope,
		Status:               "completed",
	}
}

func readOMPGeneratedBody(t *testing.T, root string, parts ...string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(append([]string{root}, parts...)...))
	require.NoError(t, err)
	_, body := splitOMPFrontmatter(string(data))
	require.NotEmpty(t, body)
	return strings.TrimSpace(body)
}

func assertOMPPlanProfile(t *testing.T, body string) {
	t.Helper()
	for _, expected := range []string{
		"## Context Profile: plan",
		"core,architecture,relevant_spec",
		"signature,learning",
		"test,canary",
	} {
		assert.Contains(t, body, expected)
	}
}

func hashOMPBody(body string) string {
	sum := sha256.Sum256([]byte(body))
	return hex.EncodeToString(sum[:])
}

func expandedOMPLiveCommandBody(prompt, body string) string {
	args := "plan fixture parity"
	if strings.HasPrefix(prompt, "/auto-plan") {
		args = "fixture parity"
	}
	return strings.Replace(body, "$ARGUMENTS", args, 1)
}

func writeOMPLiveModelConfig(t *testing.T, profile, serverURL string) {
	t.Helper()
	content := fmt.Sprintf(`providers:
  s7dummy:
    baseUrl: %s/v1
    auth: none
    api: openai-completions
    models:
      - id: s7-probe
        name: S7 Probe
        reasoning: false
        input: [text]
        contextWindow: 128000
        maxTokens: 4096
`, serverURL)
	require.NoError(t, os.WriteFile(filepath.Join(profile, "models.yml"), []byte(content), 0o600))
}

func writeOMPLiveOverlay(t *testing.T, profile string) string {
	t.Helper()
	path := filepath.Join(profile, "live-config.yml")
	content := `skills:
  enableCodexUser: false
  enableClaudeUser: false
  enableClaudeProject: false
  enablePiUser: false
  enablePiProject: false
  enableAgentsUser: false
  enableAgentsProject: false
  enableSkillCommands: true
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func pathWithinOMPTestRoot(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
