package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/adapter"
	ompadapter "github.com/insajin/autopus-adk/pkg/adapter/omp"
	"github.com/insajin/autopus-adk/pkg/config"
)

func TestOMPAgentCatalog_ConfigOnlyProjectsExactInheritedBaselineAndBlocks(t *testing.T) {
	root := writeOMPConfigOnlyWorkspace(t)
	runner := &ompCLIFakeRunner{catalog: ompCLIReadyCatalogJSON()}
	deps := normalizeOMPPlatformDependencies(ompPlatformDependencies{
		newRunner: func() ompadapter.OMPModelCatalogRunner { return runner },
	})

	status := decodeOMPAgentCatalogStatus(t, executeOMPSubcommand(
		t, newStatusCmdWithOMPDependencies(deps), "--dir", root, "--platform", "omp", "--json",
	))
	dir := root
	explain := decodeOMPAgentCatalogExplain(t, executeOMPSubcommand(
		t, newPlatformOMPExplainCmd(&dir, deps), "--json",
	))

	assertOMPAgentCatalogBaseline(t, status.Models.Models, false)
	assertOMPAgentCatalogBaseline(t, explain.Models.Models, false)
	assert.Equal(t, status.Models.Models, explain.Models.Models)
	for _, projection := range []ompModelOperatorProjection{status.Models, explain.Models} {
		assert.Equal(t, "blocked", projection.AgentCatalogStatus)
		assert.Equal(t, "agent_catalog_incomplete", projection.AgentCatalogReason)
		assert.Equal(t, 16, projection.ExpectedAgents)
		assert.Zero(t, projection.InstalledAgents)
		assert.Zero(t, projection.VerifiedAgents)
	}
	assert.Equal(t, "blocked", status.Status)
	assert.Equal(t, "blocked", explain.Status)
	assert.Contains(t, status.Blockers, "agents:agent_catalog_incomplete")
	assert.Contains(t, explain.Blockers, "agents:agent_catalog_incomplete")
	assert.Empty(t, runner.calls, "an inherited catalog must not probe provider routing")
}

func TestOMPAgentCatalog_GeneratedWorkspaceVerifiesAllManifestDefinitionsAndIsReady(t *testing.T) {
	root := writeOMPConfigOnlyWorkspace(t)
	cfg, err := config.LoadPreview(root)
	require.NoError(t, err)
	_, err = ompadapter.NewWithRoot(root).Generate(context.Background(), cfg)
	require.NoError(t, err)

	manifest, err := adapter.LoadManifest(root, "omp")
	require.NoError(t, err)
	require.NotNil(t, manifest)
	mapping := config.OMPAgentRoleMapping()
	for _, name := range sortedOMPAgentCatalogNames(mapping) {
		path := filepath.ToSlash(filepath.Join(".omp", "agents", name+".md"))
		entry, tracked := manifest.Files[path]
		require.True(t, tracked, "generated definition is absent from manifest: %s", path)
		data, readErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		require.NoError(t, readErr)
		assert.Equal(t, entry.Checksum, adapter.Checksum(string(data)), path)
	}

	runner := &ompCLIFakeRunner{catalog: ompCLIReadyCatalogJSON()}
	projection := buildOMPPlatformProjection(context.Background(), root, runner, configTimeForOMPAgentCatalog())
	assertOMPAgentCatalogBaseline(t, projection.Models.Models, true)
	assert.Equal(t, "ready", projection.Models.AgentCatalogStatus)
	assert.Equal(t, 16, projection.Models.ExpectedAgents)
	assert.Equal(t, 16, projection.Models.InstalledAgents)
	assert.Equal(t, 16, projection.Models.VerifiedAgents)
	assert.Equal(t, "ready", projection.Status)
	assert.NotContains(t, projection.Blockers, "agents:agent_catalog_incomplete")
	assert.False(t, projection.Models.ReceiptVerified,
		"installed-definition integrity must not claim routing receipt verification")
	for _, row := range projection.Models.Models {
		assert.True(t, row.DefinitionVerified, row.Agent)
		assert.False(t, row.Verified, "definition verification must remain separate from routing: %s", row.Agent)
	}
	corruptName := sortedOMPAgentCatalogNames(mapping)[0]
	corruptPath := filepath.Join(root, ".omp", "agents", corruptName+".md")
	original, err := os.ReadFile(corruptPath)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(corruptPath, append(original, []byte("\ncorrupted\n")...), 0o600))
	corrupt := buildOMPPlatformProjection(context.Background(), root, runner, configTimeForOMPAgentCatalog())
	assert.Equal(t, "blocked", corrupt.Models.AgentCatalogStatus)
	assert.Equal(t, "agent_catalog_incomplete", corrupt.Models.AgentCatalogReason)
	assert.Equal(t, 16, corrupt.Models.InstalledAgents)
	assert.Equal(t, 15, corrupt.Models.VerifiedAgents)
	assert.Equal(t, "blocked", corrupt.Status)
	assert.Contains(t, corrupt.Blockers, "agents:agent_catalog_incomplete")
	require.Len(t, corrupt.Models.Models, 16)
	assert.Equal(t, corruptName, corrupt.Models.Models[0].Agent)
	assert.False(t, corrupt.Models.Models[0].DefinitionVerified)
	assert.Empty(t, runner.calls, "definition verification must not probe provider routing")
}

func TestOMPAgentCatalog_RejectsSymlinkedManifestAndDefinition(t *testing.T) {
	t.Run("manifest", func(t *testing.T) {
		root := writeOMPConfigOnlyWorkspace(t)
		cfg, err := config.LoadPreview(root)
		require.NoError(t, err)
		_, err = ompadapter.NewWithRoot(root).Generate(context.Background(), cfg)
		require.NoError(t, err)
		manifestPath := filepath.Join(root, ".autopus", "omp-manifest.json")
		data, err := os.ReadFile(manifestPath)
		require.NoError(t, err)
		outside := filepath.Join(t.TempDir(), "manifest.json")
		require.NoError(t, os.WriteFile(outside, data, 0o600))
		require.NoError(t, os.Remove(manifestPath))
		if err := os.Symlink(outside, manifestPath); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
		projection := buildOMPPlatformProjection(
			context.Background(), root, &ompCLIFakeRunner{}, configTimeForOMPAgentCatalog(),
		)
		assert.Equal(t, "blocked", projection.Models.AgentCatalogStatus)
		assert.Equal(t, 16, projection.Models.InstalledAgents)
		assert.Zero(t, projection.Models.VerifiedAgents)
	})

	t.Run("definition", func(t *testing.T) {
		root := writeOMPConfigOnlyWorkspace(t)
		cfg, err := config.LoadPreview(root)
		require.NoError(t, err)
		_, err = ompadapter.NewWithRoot(root).Generate(context.Background(), cfg)
		require.NoError(t, err)
		name := sortedOMPAgentCatalogNames(config.OMPAgentRoleMapping())[0]
		definitionPath := filepath.Join(root, ".omp", "agents", name+".md")
		data, err := os.ReadFile(definitionPath)
		require.NoError(t, err)
		outside := filepath.Join(t.TempDir(), name+".md")
		require.NoError(t, os.WriteFile(outside, data, 0o600))
		require.NoError(t, os.Remove(definitionPath))
		if err := os.Symlink(outside, definitionPath); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
		projection := buildOMPPlatformProjection(
			context.Background(), root, &ompCLIFakeRunner{}, configTimeForOMPAgentCatalog(),
		)
		assert.Equal(t, "blocked", projection.Models.AgentCatalogStatus)
		assert.Equal(t, 15, projection.Models.InstalledAgents)
		assert.Equal(t, 15, projection.Models.VerifiedAgents)
	})
}

func TestOMPAgentCatalog_SelectedRoutingOverlaysAliasAndExactSelector(t *testing.T) {
	root, runner, _ := writeSelectedOMPProfile(t)
	projection := buildOMPPlatformProjection(context.Background(), root, runner, configTimeForOMPAgentCatalog())
	mapping := config.OMPAgentRoleMapping()
	require.Len(t, projection.Models.Models, len(mapping))

	for index, row := range projection.Models.Models {
		role := mapping[row.Agent]
		require.NotEmpty(t, role, row.Agent)
		assert.Equal(t, sortedOMPAgentCatalogNames(mapping)[index], row.Agent)
		assert.Equal(t, "@"+role, row.ModelAlias, row.Agent)
		require.NotEmpty(t, row.Provider, row.Agent)
		require.NotEmpty(t, row.Model, row.Agent)
		require.NotEmpty(t, row.Thinking, row.Agent)
		assert.Equal(t, fmt.Sprintf("%s/%s:%s", row.Provider, row.Model, row.Thinking),
			row.EffectiveSelector, row.Agent)
		assert.False(t, row.DefinitionVerified, "config-only workspace has no installed definition: %s", row.Agent)
	}
	assert.Equal(t, "blocked", projection.Models.AgentCatalogStatus)
	assert.Contains(t, projection.Blockers, "agents:agent_catalog_incomplete")
}

func assertOMPAgentCatalogBaseline(t *testing.T, rows []ompEffectiveModelProjection, installed bool) {
	t.Helper()
	mapping := config.OMPAgentRoleMapping()
	names := sortedOMPAgentCatalogNames(mapping)
	require.Len(t, mapping, 16)
	require.Len(t, rows, 16)
	for index, row := range rows {
		name := names[index]
		capability, err := config.OMPAgentCapability(name)
		require.NoError(t, err)
		assert.Equal(t, name, row.Agent)
		assert.Equal(t, config.OMPAgentRoleName(name), row.Role, name)
		assert.Equal(t, capability, row.Capability, name)
		assert.Equal(t, "inherit", row.ModelAlias, name)
		assert.Empty(t, row.EffectiveSelector, name)
		assert.Equal(t, "inherited", row.Status, name)
		assert.Equal(t, "profile_not_selected", row.Reason, name)
		assert.Equal(t, filepath.ToSlash(filepath.Join(".omp", "agents", name+".md")), row.DefinitionPath)
		assert.Equal(t, installed, row.DefinitionVerified, name)
		if installed {
			assert.Equal(t, "installed", row.InstallStatus, name)
		} else {
			assert.Equal(t, "missing", row.InstallStatus, name)
		}
	}
}

func sortedOMPAgentCatalogNames(mapping map[string]string) []string {
	names := make([]string, 0, len(mapping))
	for name := range mapping {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func writeOMPConfigOnlyWorkspace(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	cfg := config.DefaultFullConfig("omp-agent-catalog")
	cfg.Platforms = []string{"omp"}
	require.NoError(t, config.Save(root, cfg))
	return root
}

func decodeOMPAgentCatalogStatus(t *testing.T, encoded string) ompPlatformProjection {
	t.Helper()
	var envelope ompCLIJSONEnvelope
	require.NoError(t, json.Unmarshal([]byte(encoded), &envelope))
	var projection ompPlatformProjection
	require.NoError(t, json.Unmarshal(envelope.Data, &projection))
	return projection
}

type ompAgentCatalogExplainPayload struct {
	Status   string                     `json:"status"`
	Models   ompModelOperatorProjection `json:"models"`
	Blockers []string                   `json:"blockers"`
}

func decodeOMPAgentCatalogExplain(t *testing.T, encoded string) ompAgentCatalogExplainPayload {
	t.Helper()
	var envelope ompCLIJSONEnvelope
	require.NoError(t, json.Unmarshal([]byte(encoded), &envelope))
	var projection ompAgentCatalogExplainPayload
	require.NoError(t, json.Unmarshal(envelope.Data, &projection))
	return projection
}

func configTimeForOMPAgentCatalog() time.Time {
	return time.Date(2026, 8, 30, 0, 0, 0, 0, time.UTC)
}
