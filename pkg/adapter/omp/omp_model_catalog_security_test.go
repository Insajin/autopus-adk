package omp

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/config"
)

func TestNormalizeOMPModelCatalog_SemanticPresenceDistinguishesMissingFromInvalid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "missing family",
			raw:  `{"models":[{"provider":"p","id":"m","capabilities":["coding_tool_use"],"thinking":["high"],"keyless":true}]}`,
			want: "catalog_metadata_insufficient",
		},
		{
			name: "empty family",
			raw:  `{"models":[{"provider":"p","id":"m","family":"","capabilities":["coding_tool_use"],"thinking":["high"],"keyless":true}]}`,
			want: "catalog_invalid",
		},
		{
			name: "unsafe family",
			raw:  `{"models":[{"provider":"p","id":"m","family":"bad/family","capabilities":["coding_tool_use"],"thinking":["high"],"keyless":true}]}`,
			want: "catalog_invalid",
		},
		{
			name: "missing capabilities",
			raw:  `{"models":[{"provider":"p","id":"m","family":"f","thinking":["high"],"keyless":true}]}`,
			want: "catalog_metadata_insufficient",
		},
		{
			name: "empty capabilities",
			raw:  `{"models":[{"provider":"p","id":"m","family":"f","capabilities":[],"thinking":["high"],"keyless":true}]}`,
			want: "catalog_invalid",
		},
		{
			name: "duplicate capabilities",
			raw:  `{"models":[{"provider":"p","id":"m","family":"f","capabilities":["coding_tool_use","coding_tool_use"],"thinking":["high"],"keyless":true}]}`,
			want: "catalog_invalid",
		},
		{
			name: "missing thinking",
			raw:  `{"models":[{"provider":"p","id":"m","family":"f","capabilities":["coding_tool_use"],"keyless":true}]}`,
			want: "catalog_metadata_insufficient",
		},
		{
			name: "empty thinking",
			raw:  `{"models":[{"provider":"p","id":"m","family":"f","capabilities":["coding_tool_use"],"thinking":[],"keyless":true}]}`,
			want: "catalog_invalid",
		},
		{
			name: "duplicate thinking",
			raw:  `{"models":[{"provider":"p","id":"m","family":"f","capabilities":["coding_tool_use"],"thinking":["high","high"],"keyless":true}]}`,
			want: "catalog_invalid",
		},
		{
			name: "missing auth availability",
			raw:  `{"models":[{"provider":"p","id":"m","family":"f","capabilities":["coding_tool_use"],"thinking":["high"]}]}`,
			want: "catalog_metadata_insufficient",
		},
		{
			name: "conflicting auth availability",
			raw:  `{"models":[{"provider":"p","id":"m","family":"f","capabilities":["coding_tool_use"],"thinking":["high"],"auth_enabled":true,"available":false}]}`,
			want: "catalog_invalid",
		},
		{
			name: "duplicate nested JSON field",
			raw:  `{"models":[{"provider":"p","provider":"q","id":"m","family":"f","capabilities":["coding_tool_use"],"thinking":["high"],"keyless":true}]}`,
			want: "catalog_invalid",
		},
		{
			name: "duplicate top-level JSON field",
			raw:  `{"models":[],"models":[]}`,
			want: "catalog_invalid",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			catalog, reason := NormalizeOMPModelCatalog([]byte(test.raw), 4096)
			require.Equal(t, test.want, reason)
			require.Empty(t, catalog.Models)
		})
	}
}

func TestNormalizeOMPModelCatalog_ConsistentAuthAliasesRemainObserved(t *testing.T) {
	t.Parallel()

	catalog, reason := NormalizeOMPModelCatalog([]byte(
		`{"models":[{"provider":"p","id":"m","family":"f","capabilities":["coding_tool_use"],"thinking":["high"],"auth_enabled":true,"available":true}]}`,
	), 4096)

	require.Equal(t, "catalog_ready", reason)
	require.Len(t, catalog.Models, 1)
	require.True(t, catalog.Models[0].AuthEnabled)
}

func TestNormalizeOMPOperatorAttestedCatalog_ValidatesPresentRawSemanticsWithoutTrustingThem(t *testing.T) {
	t.Parallel()

	profile := integrationHarnessConfig(config.RoleModelConfigModeOverlay).RoleModelPolicy.Profiles["p1"]
	invalid := []struct {
		name string
		raw  string
	}{
		{"empty family", `{"models":[{"provider":"anthropic","id":"alpha-reasoner","family":"","available":true}]}`},
		{"unsafe family", `{"models":[{"provider":"anthropic","id":"alpha-reasoner","family":"bad/family","available":true}]}`},
		{"empty capabilities", `{"models":[{"provider":"anthropic","id":"alpha-reasoner","capabilities":[],"available":true}]}`},
		{"unknown capability", `{"models":[{"provider":"anthropic","id":"alpha-reasoner","capabilities":["provider_claim"],"available":true}]}`},
		{"duplicate capabilities", `{"models":[{"provider":"anthropic","id":"alpha-reasoner","capabilities":["deep_reasoning","deep_reasoning"],"available":true}]}`},
		{"empty thinking", `{"models":[{"provider":"anthropic","id":"alpha-reasoner","thinking":[],"available":true}]}`},
		{"unknown thinking", `{"models":[{"provider":"anthropic","id":"alpha-reasoner","thinking":["extreme"],"available":true}]}`},
		{"duplicate thinking", `{"models":[{"provider":"anthropic","id":"alpha-reasoner","thinking":["high","high"],"available":true}]}`},
		{"conflicting auth availability", `{"models":[{"provider":"anthropic","id":"alpha-reasoner","auth_enabled":true,"available":false}]}`},
		{"duplicate JSON field", `{"models":[{"provider":"anthropic","provider":"openai","id":"alpha-reasoner","available":true}]}`},
	}
	for _, test := range invalid {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			catalog, reason := normalizeOMPOperatorAttestedCatalog([]byte(test.raw), 4096, profile)
			require.Equal(t, "catalog_invalid", reason)
			require.Empty(t, catalog.Models)
		})
	}

	raw := []byte(`{"models":[{"provider":"anthropic","id":"alpha-reasoner","family":"raw-family","capabilities":["vision_design"],"thinking":["xhigh","high"]}]}`)
	catalog, reason := normalizeOMPOperatorAttestedCatalog(raw, 4096, profile)
	require.Equal(t, "catalog_ready", reason)
	require.Len(t, catalog.Models, 1)
	require.Equal(t, "anthropic", catalog.Models[0].Family)
	require.Equal(t, []string{"deep_reasoning", "independent_dissent"}, catalog.Models[0].Capabilities)
	require.Equal(t, []string{"high", "xhigh"}, catalog.Models[0].Thinking)
	require.False(t, catalog.Models[0].AuthEnabled)
	require.False(t, catalog.Models[0].Keyless)
}
