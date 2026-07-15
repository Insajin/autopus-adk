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

	"github.com/insajin/autopus-adk/pkg/promptlayer"
)

func TestWorkflowContext_EmitsVerifiedManifestAndIsRegistered(t *testing.T) {
	root := writeWorkflowContextProject(t)

	direct := executeWorkflowContext(t, newWorkflowContextCmd(),
		"--project-dir", root, "--command", "go", "--spec-dir", deliveryCLISpecDir, "--format", "json")
	assertWorkflowContextReceipt(t, root, direct)

	registered := executeWorkflowContext(t, NewWorkflowCmd(nil, nil),
		"context", "--project-dir", root, "--command", "go", "--spec-dir", deliveryCLISpecDir, "--format", "json")
	assertWorkflowContextReceipt(t, root, registered)
}

func TestWorkflowContext_BadRequiredInputsFailNonzero(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(t *testing.T, root string)
		specDir string
	}{
		{name: "missing agents", mutate: func(t *testing.T, root string) {
			require.NoError(t, os.Remove(filepath.Join(root, "AGENTS.md")))
		}},
		{name: "empty acceptance", mutate: func(t *testing.T, root string) {
			require.NoError(t, os.WriteFile(filepath.Join(root, deliveryCLISpecDir, "acceptance.md"), nil, 0o600))
		}},
		{name: "escaping spec dir", specDir: "../outside"},
		{name: "escaping core symlink", mutate: func(t *testing.T, root string) {
			outside := filepath.Join(t.TempDir(), "outside.md")
			require.NoError(t, os.WriteFile(outside, []byte("outside"), 0o600))
			require.NoError(t, os.Remove(filepath.Join(root, "AGENTS.md")))
			require.NoError(t, os.Symlink(outside, filepath.Join(root, "AGENTS.md")))
		}},
		{name: "wrong spec identity", mutate: func(t *testing.T, root string) {
			wrong := "# SPEC-DIFFERENT-001: Wrong\n\n---\nid: SPEC-DIFFERENT-001\n---\n"
			require.NoError(t, os.WriteFile(filepath.Join(root, deliveryCLISpecDir, "spec.md"), []byte(wrong), 0o600))
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := writeWorkflowContextProject(t)
			if tt.mutate != nil {
				tt.mutate(t, root)
			}
			specDir := tt.specDir
			if specDir == "" {
				specDir = deliveryCLISpecDir
			}
			var output bytes.Buffer
			cmd := newWorkflowContextCmd()
			cmd.SetOut(&output)
			cmd.SetErr(&output)
			cmd.SetArgs([]string{"--project-dir", root, "--command", "go", "--spec-dir", specDir, "--format", "json"})
			err := cmd.Execute()
			require.Error(t, err, "invalid required context must produce a nonzero command result")
			assert.NotContains(t, output.String(), `"integrity_status":"verified"`)
		})
	}
}

func executeWorkflowContext(t *testing.T, cmd *cobra.Command, args ...string) promptlayer.ContextDeliveryResult {
	t.Helper()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs(args)
	require.NoError(t, cmd.Execute(), output.String())
	var result promptlayer.ContextDeliveryResult
	require.NoError(t, json.Unmarshal(output.Bytes(), &result), output.String())
	return result
}

func assertWorkflowContextReceipt(t *testing.T, root string, result promptlayer.ContextDeliveryResult) {
	t.Helper()
	assert.Equal(t, "verified", result.IntegrityStatus)
	assert.Equal(t, "go", result.Command)
	assert.Len(t, result.RequiredDocuments, 5)
	assert.Empty(t, result.Prompt, "CLI JSON must not carry prompt bodies")
	assert.Empty(t, result.Layers, "CLI JSON must not carry layers")
	require.NoError(t, promptlayer.VerifyContextDelivery(root, result))
}

const deliveryCLISpecDir = ".autopus/specs/SPEC-WORKFLOW-CONTEXT-001"

func writeWorkflowContextProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for rel, body := range map[string]string{
		"AGENTS.md":                           "agents",
		".autopus/project/workspace.md":       "workspace",
		deliveryCLISpecDir + "/spec.md":       "# SPEC-WORKFLOW-CONTEXT-001: Workflow Context\n\n---\nid: SPEC-WORKFLOW-CONTEXT-001\n---\n\nspec",
		deliveryCLISpecDir + "/plan.md":       "plan",
		deliveryCLISpecDir + "/acceptance.md": "acceptance",
	} {
		path := filepath.Join(root, filepath.FromSlash(rel))
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
		require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	}
	return root
}
