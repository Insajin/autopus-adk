package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPipelineOMPActiveProviderAuthorityDigest_IsolatesEndpointCredentialModelAndPolicy(t *testing.T) {
	policy := workflowContextRuntimeHash("active-policy")
	implementation := pipelineOMPActiveImplementationDigest()
	modelScope := workflowContextRuntimeHash("model-scope")
	endpoint := "http://127.0.0.1:43123"
	credential := "provider-credential-value-123456"
	baseline, err := pipelineOMPActiveProviderAuthorityDigest(policy, implementation, modelScope, endpoint, credential)
	require.NoError(t, err)
	repeated, err := pipelineOMPActiveProviderAuthorityDigest(policy, implementation, modelScope, endpoint, credential)
	require.NoError(t, err)
	assert.Equal(t, baseline, repeated)

	cases := []struct {
		policy, implementation, modelScope, endpoint, credential string
	}{
		{workflowContextRuntimeHash("other-policy"), implementation, modelScope, endpoint, credential},
		{policy, workflowContextRuntimeHash("other-implementation"), modelScope, endpoint, credential},
		{policy, implementation, workflowContextRuntimeHash("other-model"), endpoint, credential},
		{policy, implementation, modelScope, "http://127.0.0.1:43124", credential},
		{policy, implementation, modelScope, endpoint, "other-provider-credential-123456"},
	}
	for _, testCase := range cases {
		digest, err := pipelineOMPActiveProviderAuthorityDigest(
			testCase.policy, testCase.implementation, testCase.modelScope,
			testCase.endpoint, testCase.credential,
		)
		require.NoError(t, err)
		assert.NotEqual(t, baseline, digest)
	}
}

func TestPipelineOMPActiveProviderAuthorityDigest_RejectsMissingOrNonLoopbackAuthority(t *testing.T) {
	hash := workflowContextRuntimeHash("authority")
	for _, input := range []struct{ endpoint, credential string }{
		{"https://api.example.com", "credential-value-123456"},
		{"http://127.0.0.1:43123", ""},
		{"http://127.0.0.1:43123", "bad\x00credential"},
	} {
		_, err := pipelineOMPActiveProviderAuthorityDigest(hash, hash, hash, input.endpoint, input.credential)
		require.Error(t, err)
	}
}
