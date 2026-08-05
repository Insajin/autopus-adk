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
	baseline, err := pipelineOMPActiveProviderAuthorityDigest(
		policy, implementation, modelScope, pipelineOMPActiveDefaultContextWindow, endpoint, credential)
	require.NoError(t, err)
	repeated, err := pipelineOMPActiveProviderAuthorityDigest(
		policy, implementation, modelScope, pipelineOMPActiveDefaultContextWindow, endpoint, credential)
	require.NoError(t, err)
	assert.Equal(t, baseline, repeated)

	cases := []struct {
		policy, implementation, modelScope, endpoint, credential string
		contextWindow                                            int
	}{
		{policy: workflowContextRuntimeHash("other-policy"), implementation: implementation,
			modelScope: modelScope, contextWindow: pipelineOMPActiveDefaultContextWindow, endpoint: endpoint, credential: credential},
		{policy: policy, implementation: workflowContextRuntimeHash("other-implementation"),
			modelScope: modelScope, contextWindow: pipelineOMPActiveDefaultContextWindow, endpoint: endpoint, credential: credential},
		{policy: policy, implementation: implementation, modelScope: workflowContextRuntimeHash("other-model"),
			contextWindow: pipelineOMPActiveDefaultContextWindow, endpoint: endpoint, credential: credential},
		{policy: policy, implementation: implementation, modelScope: modelScope,
			contextWindow: 1_000_000, endpoint: endpoint, credential: credential},
		{policy: policy, implementation: implementation, modelScope: modelScope,
			contextWindow: pipelineOMPActiveDefaultContextWindow, endpoint: "http://127.0.0.1:43124", credential: credential},
		{policy: policy, implementation: implementation, modelScope: modelScope,
			contextWindow: pipelineOMPActiveDefaultContextWindow, endpoint: endpoint, credential: "other-provider-credential-123456"},
	}
	for _, testCase := range cases {
		digest, err := pipelineOMPActiveProviderAuthorityDigest(
			testCase.policy, testCase.implementation, testCase.modelScope,
			testCase.contextWindow, testCase.endpoint, testCase.credential,
		)
		require.NoError(t, err)
		assert.NotEqual(t, baseline, digest)
	}
}

func TestPipelineOMPActiveProviderAuthorityDigest_RejectsMissingOrNonLoopbackAuthority(t *testing.T) {
	hash := workflowContextRuntimeHash("authority")
	for _, input := range []struct {
		endpoint, credential string
		contextWindow        int
	}{
		{"https://api.example.com", "credential-value-123456", pipelineOMPActiveDefaultContextWindow},
		{"http://127.0.0.1:43123", "", pipelineOMPActiveDefaultContextWindow},
		{"http://127.0.0.1:43123", "bad\x00credential", pipelineOMPActiveDefaultContextWindow},
		{"http://127.0.0.1:43123", "credential-value-123456", 0},
	} {
		_, err := pipelineOMPActiveProviderAuthorityDigest(
			hash, hash, hash, input.contextWindow, input.endpoint, input.credential)
		require.Error(t, err)
	}
}
