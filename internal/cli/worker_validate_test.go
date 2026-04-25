package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/worker/controlplane"
	"github.com/insajin/autopus-adk/pkg/worker/security"
)

func TestRunWorkerValidate_VerifiesPolicySignature(t *testing.T) {
	t.Setenv(controlplane.PolicySigningSecretEnv, "secret")

	dir := t.TempDir()
	policyPath := filepath.Join(dir, "autopus-policy-task-1.json")
	policy := security.SecurityPolicy{
		AllowFS:         true,
		AllowedCommands: []string{"go test"},
		TimeoutSec:      30,
	}

	data, err := json.Marshal(policy)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(policyPath, data, 0o600))

	signature, err := controlplane.SignSecurityPolicy("task-1", policy, "secret")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(policyPath+".sig", []byte(signature+"\n"), 0o600))

	var stdout bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&stdout)

	err = runWorkerValidate(cmd, policyPath, "go test ./...", "")
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "PASS")
}

func TestRunWorkerValidate_VerifiesFixedPolicySignatureVector(t *testing.T) {
	t.Setenv(controlplane.PolicySigningSecretEnv, "secret")

	dir := t.TempDir()
	policyPath := filepath.Join(dir, "autopus-policy-task-1.json")
	policyJSON := []byte(`{"allow_network":false,"allow_fs":true,"allowed_commands":["go test"],"timeout_sec":30}`)
	const signature = "6f287247fd97a7751f1bc68c3f7b51e5e7d995b1ca3b623dd7eaf779afb8d750"
	require.NoError(t, os.WriteFile(policyPath, policyJSON, 0o600))
	require.NoError(t, os.WriteFile(policyPath+".sig", []byte(signature+"\n"), 0o600))

	var stdout bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&stdout)

	err := runWorkerValidate(cmd, policyPath, "go test ./...", "")
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "PASS")
}
