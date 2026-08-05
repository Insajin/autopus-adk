package cli

import (
	"errors"
	"strconv"
	"strings"
)

const pipelineOMPActiveCredentialMaxBytes = 64 << 10

func pipelineOMPActiveProviderAuthorityDigest(
	policyDigest string,
	implementationDigest string,
	modelScopeDigest string,
	contextWindow int,
	endpointRaw string,
	credential string,
) (string, error) {
	endpoint, err := validatePipelineOMPActiveEndpoint(endpointRaw)
	if err != nil || !validPipelineOMPActiveHash(policyDigest) ||
		!validPipelineOMPActiveHash(implementationDigest) || !validPipelineOMPActiveHash(modelScopeDigest) ||
		contextWindow < 8192 || contextWindow > 1<<30 || credential == "" ||
		len(credential) > pipelineOMPActiveCredentialMaxBytes || strings.ContainsRune(credential, 0) {
		return "", errors.New("pipeline: managed active provider authority is invalid")
	}
	credentialDigest := pipelineOMPActiveHash([]byte(credential))
	return pipelineOMPActiveHash([]byte(strings.Join([]string{
		policyDigest, implementationDigest, modelScopeDigest, strconv.Itoa(contextWindow), endpoint, credentialDigest,
	}, "\x00"))), nil
}
