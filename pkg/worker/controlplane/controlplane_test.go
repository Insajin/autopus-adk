package controlplane

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/insajin/autopus-adk/pkg/worker/security"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSignAndVerifySecurityPolicySignature(t *testing.T) {
	t.Parallel()

	policy := security.SecurityPolicy{AllowNetwork: true, AllowFS: true, TimeoutSec: 60}
	signature, err := SignSecurityPolicy("task-1", policy, "secret")
	require.NoError(t, err)
	require.NotEmpty(t, signature)

	require.NoError(t, VerifySecurityPolicySignature("task-1", policy, signature, "secret"))
	assert.Error(t, VerifySecurityPolicySignature("task-1", policy, signature, "wrong-secret"))
}

func TestVerifyCachedPolicyFile(t *testing.T) {
	t.Setenv(PolicySigningSecretEnv, "secret")

	taskID := "verify-policy-task"
	policy := security.SecurityPolicy{
		AllowNetwork:    false,
		AllowFS:         true,
		AllowedCommands: []string{"go test"},
		TimeoutSec:      30,
	}
	signature, err := SignSecurityPolicy(taskID, policy, "secret")
	require.NoError(t, err)

	policyPath := filepath.Join(t.TempDir(), "autopus-policy-"+taskID+".json")
	data, err := json.MarshalIndent(policy, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(policyPath, data, 0o600))
	require.NoError(t, WritePolicySignature(policyPath, signature))

	require.NoError(t, VerifyCachedPolicyFile(policyPath, policy))
}

func TestSignAndVerifyControlPlaneSignature(t *testing.T) {
	t.Parallel()

	iterationBudget := struct {
		Limit           int     `json:"limit"`
		WarnThreshold   float64 `json:"warn_threshold"`
		DangerThreshold float64 `json:"danger_threshold"`
	}{Limit: 12, WarnThreshold: 0.7, DangerThreshold: 0.9}

	signature, err := SignControlPlane(
		"task-1",
		"gpt-5.4",
		[]string{"planner", "reviewer"},
		map[string]string{"planner": "Plan carefully."},
		map[string]string{"planner": "SERVER TEMPLATE\n\n{{input}}"},
		iterationBudget,
		[]string{"server_model_v1", "pipeline_phases_v1", "pipeline_instructions_v1"},
		"secret",
	)
	require.NoError(t, err)
	require.NotEmpty(t, signature)

	require.NoError(t, VerifyControlPlaneSignature(
		"task-1",
		"gpt-5.4",
		[]string{"planner", "reviewer"},
		map[string]string{"planner": "Plan carefully."},
		map[string]string{"planner": "SERVER TEMPLATE\n\n{{input}}"},
		iterationBudget,
		[]string{"server_model_v1", "pipeline_phases_v1", "pipeline_instructions_v1"},
		signature,
		"secret",
	))
	assert.Error(t, VerifyControlPlaneSignature(
		"task-1",
		"gpt-5.4",
		[]string{"planner"},
		map[string]string{"planner": "Plan carefully."},
		map[string]string{"planner": "SERVER TEMPLATE\n\n{{input}}"},
		iterationBudget,
		[]string{"server_model_v1", "pipeline_phases_v1"},
		signature,
		"secret",
	))
}
