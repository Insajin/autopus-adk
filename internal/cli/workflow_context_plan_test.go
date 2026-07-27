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
)

func TestWorkflowContext_V1JSONKeySetRemainsExact(t *testing.T) {
	t.Parallel()

	root := writeWorkflowContextProject(t)
	raw := executeWorkflowJSON(t, NewWorkflowCmd(nil, nil),
		"context",
		"--project-dir", root,
		"--command", "go",
		"--spec-dir", deliveryCLISpecDir,
		"--format", "json",
	)

	assert.ElementsMatch(t, []string{
		"schema_version",
		"command",
		"spec_dir",
		"required_documents",
		"required_token_estimate",
		"snapshot_hash",
		"prompt_manifest_hash",
		"integrity_status",
		"prompt_manifest",
	}, rawKeys(raw))
	assert.Equal(t, "autopus.context_delivery.v1", rawString(t, raw, "schema_version"))
	for _, forbidden := range []string{
		"status",
		"shadow_only",
		"active_mode",
		"candidate_mode",
		"selected_references",
		"token_delta",
		"reduction_percent",
	} {
		assert.NotContains(t, raw, forbidden)
	}
}

func TestWorkflowContextPlan_MissingProjectionIsUnavailableAndDoesNotGateV1(t *testing.T) {
	t.Parallel()

	root := writeWorkflowContextProject(t)
	raw := executeWorkflowJSON(t, NewWorkflowCmd(nil, nil),
		"context-plan",
		"--project-dir", root,
		"--command", "go",
		"--spec-dir", deliveryCLISpecDir,
		"--query", "raw-query-secret",
		"--format", "json",
	)

	assert.ElementsMatch(t, []string{
		"schema_version",
		"status",
		"shadow_only",
		"active_mode",
		"candidate_mode",
		"pinned_references",
		"selected_references",
		"omitted_count",
		"full_token_estimate",
		"candidate_token_estimate",
		"token_delta",
		"reduction_percent",
		"selection_hits",
		"reason",
	}, rawKeys(raw))
	assert.Equal(t, "autopus.context_plan.v2", rawString(t, raw, "schema_version"))
	assert.Equal(t, "unavailable", rawString(t, raw, "status"))
	assert.Equal(t, "full", rawString(t, raw, "active_mode"))
	assert.Equal(t, "jit", rawString(t, raw, "candidate_mode"))
	assert.Equal(t, "null", string(raw["candidate_token_estimate"]))
	assert.Equal(t, "null", string(raw["token_delta"]))
	assert.Equal(t, "null", string(raw["reduction_percent"]))
	assert.NotContains(t, string(mustJSON(t, raw)), "raw-query-secret")
	assert.NotContains(t, string(mustJSON(t, raw)), filepath.ToSlash(root))

	v1 := executeWorkflowJSON(t, NewWorkflowCmd(nil, nil),
		"context",
		"--project-dir", root,
		"--command", "go",
		"--spec-dir", deliveryCLISpecDir,
		"--format", "json",
	)
	assert.Equal(t, "autopus.context_delivery.v1", rawString(t, v1, "schema_version"))
	assert.Equal(t, "verified", rawString(t, v1, "integrity_status"))
}

func TestWorkflowBinding_RejectsContextPlanV2AsV1Manifest(t *testing.T) {
	t.Parallel()

	root := writeWorkflowContextProject(t)
	path := filepath.Join(root, "context-plan-v2.json")
	body := []byte(`{
		"schema_version":"autopus.context_plan.v2",
		"status":"planned",
		"shadow_only":true,
		"active_mode":"full",
		"candidate_mode":"jit"
	}`)
	require.NoError(t, os.WriteFile(path, body, 0o600))

	got := verifyWorkflowContextManifest(workflowBindingContextOptions{
		manifest: path,
		root:     root,
		command:  "go",
		specDir:  deliveryCLISpecDir,
	})

	assert.Equal(t, contextIntegrityFailed, got)
}

func executeWorkflowJSON(t *testing.T, cmd *cobra.Command, args ...string) map[string]json.RawMessage {
	t.Helper()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs(args)
	require.NoError(t, cmd.Execute(), output.String())
	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(output.Bytes(), &raw), output.String())
	return raw
}

func rawKeys(raw map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(raw))
	for key := range raw {
		keys = append(keys, key)
	}
	return keys
}

func rawString(t *testing.T, raw map[string]json.RawMessage, key string) string {
	t.Helper()
	var value string
	require.NoError(t, json.Unmarshal(raw[key], &value))
	return value
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	require.NoError(t, err)
	return data
}
