package cli

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWorkflowContextImplementationIdentityCommand(t *testing.T) {
	cmd := newWorkflowContextImplementationIdentityCmd()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"--format", "json"})
	require.NoError(t, cmd.Execute())

	var identity workflowContextImplementationIdentityV1
	require.NoError(t, json.Unmarshal(output.Bytes(), &identity))
	require.Equal(t, workflowContextImplementationIdentitySchemaV1, identity.SchemaVersion)
	require.Equal(t, "sha256:b338d5c6ee2a681e7c67555f1dee4743837c30ed347af5867412966de50e0b9e",
		identity.PipelineImplementationDigest)
	require.Equal(t, "autopus.omp-pipeline-managed-rpc.v2", identity.RPCIdentity)
	require.Equal(t, pipelineOMPActivePolicyIdentity, identity.PolicyIdentity)
	require.NotEmpty(t, identity.BridgeTarget)
	require.NotEmpty(t, identity.BridgeSHA256)
	require.NotEmpty(t, identity.RouteTarget)
	require.NotEmpty(t, identity.RouteSHA256)
}

func TestWorkflowContextImplementationIdentityRejectsUnsupportedFormat(t *testing.T) {
	cmd := newWorkflowContextImplementationIdentityCmd()
	cmd.SetArgs([]string{"--format", "yaml"})
	require.EqualError(t, cmd.Execute(),
		"workflow context-runtime implementation-identity requires --format json")
}
