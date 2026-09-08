package codexruntime

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The namespace decision must never be derived from a version comparison. A
// boundary invented between two observed releases claimed knowledge about
// every release on both sides of it, including releases nobody ran, so an
// unverified version now resolves to the documented table and says it is an
// assumption.
func TestAgentConcurrencyNamespaceFor_ReportsDocumentedTableWithItsEvidence(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name         string
		version      string
		wantEvidence AgentConcurrencyEvidence
	}{
		{
			name:    "verified release",
			version: "codex-cli 0.153.4\n", wantEvidence: AgentConcurrencyVerifiedRelease,
		},
		{
			name:    "neighbouring patch of a verified release is not verified",
			version: "codex-cli 0.153.5", wantEvidence: AgentConcurrencyAssumedDocumented,
		},
		{
			name:    "older release is an assumption, not a legacy namespace",
			version: "codex-cli 0.149.1", wantEvidence: AgentConcurrencyAssumedDocumented,
		},
		{
			name:    "much older release is still only an assumption",
			version: "codex-cli 0.99.12", wantEvidence: AgentConcurrencyAssumedDocumented,
		},
		{
			name:    "future major carries no verified capacity",
			version: "codex-cli 1.2.3", wantEvidence: AgentConcurrencyAssumedDocumented,
		},
		{
			name:    "empty version",
			version: "", wantEvidence: AgentConcurrencyUnknownVersion,
		},
		{
			name:    "unparseable version",
			version: "codex-cli unknown-build", wantEvidence: AgentConcurrencyUnknownVersion,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			namespace, evidence := AgentConcurrencyNamespaceFor(tc.version)
			assert.Equal(t, AgentsNamespace, namespace,
				"only the documented table may be generated, whatever the release")
			assert.Equal(t, tc.wantEvidence, evidence)
		})
	}
}

// The generated comment must disclose which of the three provenances produced
// the namespace; otherwise a reader takes an assumption for a detected fact.
func TestDescribeAgentConcurrencyNamespace_StatesProvenance(t *testing.T) {
	t.Parallel()

	verified := DescribeAgentConcurrencyNamespace(AgentsNamespace, AgentConcurrencyVerifiedRelease)
	assert.Contains(t, verified, "coordinator thread is not counted")
	assert.Contains(t, verified, "verified to recognise this key in [agents]")
	// Recognising the key is not running with it: the comment must not let a
	// verified release read as verified capacity.
	assert.Contains(t, verified, "not readable")
	assert.NotContains(t, verified, "assuming")

	assumed := DescribeAgentConcurrencyNamespace(AgentsNamespace, AgentConcurrencyAssumedDocumented)
	assert.Contains(t, assumed, "coordinator thread is not counted")
	assert.Contains(t, assumed, "not one Autopus verified")
	assert.Contains(t, assumed, "assuming the documented [agents] namespace")

	unknown := DescribeAgentConcurrencyNamespace(AgentsNamespace, AgentConcurrencyUnknownVersion)
	assert.Contains(t, unknown, "coordinator thread is not counted")
	assert.Contains(t, unknown, "version unknown")
	assert.Contains(t, unknown, "[agents]")
}

// On-disk configuration, a running session's parsed value, and the ceiling
// actually applied are three different things. Both unreadable ones must name
// the read-only limitation rather than borrow the file's number.
func TestAgentConcurrencyUnknownReasons_SeparateLoadedFromEffective(t *testing.T) {
	t.Parallel()

	assert.Contains(t, LoadedAgentConcurrencyUnknownReason, "codex doctor --json")
	assert.Contains(t, LoadedAgentConcurrencyUnknownReason, "not observed session state")
	assert.Contains(t, EffectiveAgentConcurrencyUnknownReason, "new Codex session")
	assert.NotEqual(t, LoadedAgentConcurrencyUnknownReason, EffectiveAgentConcurrencyUnknownReason)
}

// A version the caller cannot read must stay empty. Substituting a plausible
// release here would make the generation path pick a namespace it never
// detected, which is the failure issue #189 reports.
func TestProbeVersion(t *testing.T) {
	t.Parallel()

	t.Run("reads the release from the binary", func(t *testing.T) {
		t.Parallel()
		binary := writeCatalogProbe(t, "printf 'codex-cli 0.153.4\\n'")
		version, ok := ProbeVersion(t.Context(), binary, catalogProbeTestTimeout)
		require.True(t, ok)
		namespace, evidence := AgentConcurrencyNamespaceFor(version)
		assert.Equal(t, AgentConcurrencyVerifiedRelease, evidence)
		assert.Equal(t, AgentsNamespace, namespace)
	})

	t.Run("output without a version triple is unknown", func(t *testing.T) {
		t.Parallel()
		binary := writeCatalogProbe(t, "printf 'codex-cli dev\\n'")
		version, ok := ProbeVersion(t.Context(), binary, catalogProbeTestTimeout)
		assert.False(t, ok)
		assert.Empty(t, version)
	})

	t.Run("missing binary is unknown", func(t *testing.T) {
		t.Parallel()
		version, ok := ProbeVersion(t.Context(), "codex-binary-that-does-not-exist", catalogProbeTestTimeout)
		assert.False(t, ok)
		assert.Empty(t, version)
	})
}
