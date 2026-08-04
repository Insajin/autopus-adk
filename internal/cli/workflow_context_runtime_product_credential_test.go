package cli

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkflowContextManagedRPCProduct_CredentialEnvAuthorityRequiresBearerWithoutPersistingToken(t *testing.T) {
	options := managedProductTestOptions(t)
	const credentialName = "AUTOPUS_OMP_CONTEXT_PROVIDER_TOKEN"
	const credentialValue = "task-owned-secret-token"
	for index, entry := range options.Environment {
		if strings.HasPrefix(entry, credentialName+"=") {
			options.Environment[index] = credentialName + "=" + credentialValue
		}
	}
	modelsPath := filepath.Join(options.RuntimeRoot, "models.yml")
	models := fmt.Sprintf(`providers:
  fake:
    baseUrl: %s/v1
    apiKey: %s
    authHeader: true
    api: openai-completions
    models:
      - id: product
`, options.AllowedEndpoint, credentialName)
	require.NoError(t, os.WriteFile(modelsPath, []byte(models), 0o600))

	modelErr := verifyWorkflowContextManagedRPCModelConfig(options)
	assert.NoError(t, modelErr, "model authority must require an environment-backed bearer credential")
	normalized, optionErr := validateWorkflowContextManagedRPCOptions(options)
	assert.NoError(t, optionErr, "the exact bound credential environment key must cross the managed boundary")
	assert.Contains(t, normalized.Environment, credentialName+"="+credentialValue)

	data, err := os.ReadFile(modelsPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "apiKey: "+credentialName)
	assert.Contains(t, string(data), "authHeader: true")
	assert.NotContains(t, string(data), credentialValue)
	assert.NoError(t, filepath.Walk(options.RuntimeRoot, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info.IsDir() {
			return walkErr
		}
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if bytes.Contains(body, []byte(credentialValue)) {
			return errors.New("credential value persisted in managed runtime")
		}
		return nil
	}))

	childEnvironment := workflowContextManagedRPCEnvironment(normalized.Environment, managedProductBinding())
	assert.Contains(t, childEnvironment, credentialName+"="+credentialValue,
		"the secret may exist only in the process-private child environment")
}
