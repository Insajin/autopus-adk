package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/adapter/omp"
)

func TestPlatformOMPHelpSurfacesCommandsAndExactFlags(t *testing.T) {
	assertCommandHelpContains(t, []string{"platform", "omp", "--help"}, "models", "profile", "explain", "--dir")
	assertCommandHelpContains(t, []string{"platform", "omp", "models", "--help"}, "--json", "--format")
	assertCommandHelpContains(t, []string{"platform", "omp", "profile", "init", "--help"}, "--name", "--plan", "--json", "--format")
	assertCommandHelpContains(t, []string{"platform", "omp", "profile", "apply", "--help"}, "apply <name>", "--json", "--format")
	assertCommandHelpContains(t, []string{"platform", "omp", "explain", "--help"}, "--json", "--format")
	assertCommandHelpContains(t, []string{"status", "--help"}, "--platform", "--dir", "--json", "--format")
}

func TestPlatformOMPModelsReadyTextAndJSON(t *testing.T) {
	runner := &ompCLIFakeRunner{catalog: ompCLIReadyCatalogJSON()}
	dir := t.TempDir()
	deps := ompPlatformDependencies{newRunner: func() omp.OMPModelCatalogRunner { return runner }}

	text := executeOMPSubcommand(t, newPlatformOMPModelsCmd(&dir, normalizeOMPPlatformDependencies(deps)))
	assert.Contains(t, text, "OMP installed model catalog")
	assert.Contains(t, text, "anthropic/alpha-reasoner")
	assert.NotContains(t, strings.ToLower(text), "secret")

	encoded := executeOMPSubcommand(t, newPlatformOMPModelsCmd(&dir, normalizeOMPPlatformDependencies(deps)), "--json")
	var envelope ompCLIJSONEnvelope
	require.NoError(t, json.Unmarshal([]byte(encoded), &envelope))
	assert.Equal(t, jsonStatusOK, envelope.Status)
	var payload ompCatalogPayload
	require.NoError(t, json.Unmarshal(envelope.Data, &payload))
	assert.Equal(t, "catalog_ready", payload.Reason)
	assert.Len(t, payload.Models, 5)
}

func TestOMPModelsMissingExecutableFailsClosedInJSON(t *testing.T) {
	root := t.TempDir()
	runner := &ompCLIFakeRunner{fail: true}
	deps := normalizeOMPPlatformDependencies(ompPlatformDependencies{
		newRunner: func() omp.OMPModelCatalogRunner { return runner },
	})
	cmd := newPlatformOMPModelsCmd(&root, deps)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--json"})
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	err := cmd.Execute()
	require.Error(t, err)
	var envelope struct {
		Status jsonEnvelopeStatus `json:"status"`
		Data   ompCatalogPayload  `json:"data"`
	}
	require.NoError(t, json.Unmarshal(out.Bytes(), &envelope))
	assert.Equal(t, jsonStatusError, envelope.Status)
	assert.Equal(t, "identity_unverified", envelope.Data.Reason)
}
