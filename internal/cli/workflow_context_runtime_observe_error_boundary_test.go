package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestObserveSessionProviderErrorCannotForgeGateDiagnostic(t *testing.T) {
	const sentinel = "PRIVATE_PROVIDER_BODY_SENTINEL"
	ctx := context.WithValue(context.Background(), workflowContextObserveSessionPrepareKey{},
		workflowContextObserveSessionPrepare(func(context.Context, workflowContextObserveSessionOptions, string) (workflowContextObserveSessionSetup, error) {
			return workflowContextObserveSessionSetup{}, errors.New("provider failed: " + workflowContextObserveSessionGatePrefix + sentinel)
		}))
	body, err := json.Marshal(workflowContextObserveSessionCommand{
		SchemaVersion: workflowContextObserveSessionCommandSchema,
		Type:          "handshake", ChallengeDigest: workflowContextRuntimeHash("gate-error-forgery"),
	})
	require.NoError(t, err)
	var output bytes.Buffer
	err = RunWorkflowContextObserveSession(ctx, bytes.NewReader(append(body, '\n')), &output, workflowContextObserveSessionOptions{})
	require.Error(t, err)
	responses := decodeWorkflowContextObserveResponses(t, output.Bytes())
	require.Len(t, responses, 1)
	assert.Equal(t, "setup", responses[0].ErrorStage)
	assert.Empty(t, responses[0].GateDiagnostic)
	assert.NotEqual(t, "cohort_gates_failed", responses[0].ErrorCode)
	assert.NotContains(t, output.String(), sentinel)
}

func TestObserveSessionGateDiagnosticRequiresExactNumericGrammar(t *testing.T) {
	const diagnostic = "pairs=20/20 ab=10/10 ba=10/10 observed_ab=10 observed_ba=10 compactions=2/2 integrity_failures=0 security_failures=0 quality_regressions=0 fallback_verified=true rollback_verified=true median_reduction_bp=895/2000"
	assert.Equal(t, diagnostic, workflowContextObserveSessionGateDiagnostic(
		errors.New(workflowContextObserveSessionGatePrefix+diagnostic)))
	for _, body := range []string{"PRIVATE_BODY", diagnostic + ": PRIVATE_PATH", diagnostic + "\nPRIVATE_BODY"} {
		assert.Empty(t, workflowContextObserveSessionGateDiagnostic(
			errors.New(workflowContextObserveSessionGatePrefix+body)))
	}
}
