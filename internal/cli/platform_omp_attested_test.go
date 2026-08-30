package cli

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/adapter/omp"
	"github.com/insajin/autopus-adk/pkg/config"
)

func TestPlatformOMPProfileApply_UsesExistingOperatorAttestedProfile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	proposal, err := buildOMPProfileProposal("balanced", mustOMPCatalog(t, ompCLIReadyCatalogJSON()))
	require.NoError(t, err)
	profile := proposal.profile
	profile.CatalogTrust = config.RoleModelCatalogTrustOperatorAttested
	cfg := config.DefaultFullConfig("omp-attested-apply")
	cfg.Platforms = []string{"omp"}
	cfg.RoleModelPolicy = config.RoleModelPolicyConf{
		Version: config.RoleModelPolicyVersionV1, Profile: "balanced",
		Profiles: map[string]config.RoleModelProfileConf{"balanced": profile},
	}
	require.NoError(t, config.Save(root, cfg))

	runner := &ompCLIFakeRunner{catalog: ompCLIAttestedAvailableCatalogJSON()}
	activationCalls := 0
	payload, err := applyOMPProfile(
		context.Background(), root, "balanced", runner,
		func(_ context.Context, _ string, applied *config.HarnessConfig) error {
			activationCalls++
			name, selected, ok := applied.RoleModelPolicy.SelectedRoleModelProfile()
			require.True(t, ok)
			require.Equal(t, "balanced", name)
			require.Equal(t, config.RoleModelCatalogTrustOperatorAttested, selected.CatalogTrust)
			return nil
		},
	)
	require.NoError(t, err)
	require.Equal(t, "applied", payload.Status)
	require.False(t, payload.Generated)
	require.Equal(t, 1, activationCalls)

	loaded, err := config.LoadPreview(root)
	require.NoError(t, err)
	_, selected, ok := loaded.RoleModelPolicy.SelectedRoleModelProfile()
	require.True(t, ok)
	require.Equal(t, config.RoleModelCatalogTrustOperatorAttested, selected.CatalogTrust)
	assertNoSensitiveOMPCLICalls(t, runner.calls)
}

func TestPlatformOMPProfileInit_AvailabilityOnlyCatalogRemainsStrictAndWritesNothing(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	runner := &ompCLIFakeRunner{catalog: ompCLIAttestedAvailableCatalogJSON()}
	deps := normalizeOMPPlatformDependencies(ompPlatformDependencies{
		newRunner: func() omp.OMPModelCatalogRunner { return runner },
	})
	cmd := newPlatformOMPProfileInitCmd(&root, deps)
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs([]string{"--name", "balanced", "--plan"})

	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "catalog_metadata_insufficient")
	entries, readErr := os.ReadDir(root)
	require.NoError(t, readErr)
	require.Empty(t, entries)
	assertNoSensitiveOMPCLICalls(t, runner.calls)
}

func ompCLIAttestedAvailableCatalogJSON() []byte {
	return []byte(`{"models":[
		{"provider":"anthropic","id":"alpha-reasoner","available":true},
		{"provider":"openai","id":"beta-coder","available":true},
		{"provider":"google","id":"gamma-vision","available":true}
	]}`)
}

func assertNoSensitiveOMPCLICalls(t *testing.T, calls []string) {
	t.Helper()
	for _, call := range calls {
		lower := strings.ToLower(call)
		for _, forbidden := range []string{" prompt", " agent", "credential", "api-key", "api_key", "token", "auth"} {
			require.NotContains(t, lower, forbidden, call)
		}
	}
}
