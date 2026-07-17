package spec_test

// SPEC-ADK-REVIEW-INTEGRITY-001 REQ-RINT-QUORUM-05 quorum policy tests.
// Oracle-first: concrete quorum integers and the exact degraded detail string.

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/insajin/autopus-adk/pkg/spec"
)

// TestDefaultMinProviders covers the majority-quorum formula configured/2+1.
// Single-provider local reviews must pass (n=1 -> 1); a 3-provider config must
// require 2 so a 1-of-3 promotion is blocked (REQ-RINT-QUORUM-05).
func TestDefaultMinProviders(t *testing.T) {
	t.Parallel()

	tests := []struct {
		configured int
		want       int
	}{
		{configured: 1, want: 1},
		{configured: 2, want: 2},
		{configured: 3, want: 2},
		{configured: 4, want: 3},
		{configured: 5, want: 3},
		{configured: 0, want: 1},  // fail-closed: no providers still needs one usable review
		{configured: -1, want: 1}, // defensive: negative treated as unset
	}
	for _, tt := range tests {
		got := spec.DefaultMinProviders(tt.configured)
		assert.Equalf(t, tt.want, got, "DefaultMinProviders(%d)", tt.configured)
	}
}

// TestEffectiveMinProviders covers config resolution: 0 derives the majority
// default; a positive value overrides verbatim.
func TestEffectiveMinProviders(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 2, spec.EffectiveMinProviders(3, 0), "unset (0) derives majority for 3 providers")
	assert.Equal(t, 1, spec.EffectiveMinProviders(3, 1), "positive override is used verbatim")
	assert.Equal(t, 3, spec.EffectiveMinProviders(3, 3), "override above majority is honored")
	assert.Equal(t, 1, spec.EffectiveMinProviders(1, 0), "single provider derives 1")
}

// TestMeetsProviderQuorum covers REQ-RINT-QUORUM-05: usable ("success") provider
// count vs the effective quorum, pinning the exact detail reason string.
func TestMeetsProviderQuorum(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		statuses   []spec.ProviderStatus
		configured int
		minProv    int
		wantMeets  bool
		wantReason string
	}{
		{
			name: "2 of 3 success meets derived quorum 2",
			statuses: []spec.ProviderStatus{
				{Provider: "claude", Status: "success"},
				{Provider: "codex", Status: "timeout"},
				{Provider: "gemini", Status: "success"},
			},
			configured: 3,
			minProv:    0,
			wantMeets:  true,
			wantReason: "",
		},
		{
			name: "1 of 3 success fails derived quorum 2 with detail",
			statuses: []spec.ProviderStatus{
				{Provider: "claude", Status: "success"},
				{Provider: "codex", Status: "timeout"},
				{Provider: "gemini", Status: "timeout"},
			},
			configured: 3,
			minProv:    0,
			wantMeets:  false,
			wantReason: "providers 1/3 < quorum 2",
		},
		{
			name: "single provider success meets quorum 1",
			statuses: []spec.ProviderStatus{
				{Provider: "claude", Status: "success"},
			},
			configured: 1,
			minProv:    0,
			wantMeets:  true,
			wantReason: "",
		},
		{
			name: "error status is not usable and fails quorum",
			statuses: []spec.ProviderStatus{
				{Provider: "claude", Status: "success"},
				{Provider: "codex", Status: "error"},
			},
			configured: 2,
			minProv:    0,
			wantMeets:  false,
			wantReason: "providers 1/2 < quorum 2",
		},
		{
			name: "positive override raises the bar",
			statuses: []spec.ProviderStatus{
				{Provider: "claude", Status: "success"},
				{Provider: "codex", Status: "success"},
			},
			configured: 3,
			minProv:    3,
			wantMeets:  false,
			wantReason: "providers 2/3 < quorum 3",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			meets, reason := spec.MeetsProviderQuorum(tt.statuses, tt.configured, tt.minProv)
			assert.Equal(t, tt.wantMeets, meets)
			assert.Equal(t, tt.wantReason, reason)
		})
	}
}

// TestDegradedReasonProviderQuorum pins the machine-readable degraded reason
// code the promotion gate reads (AC-RINT-QUORUM-4).
func TestDegradedReasonProviderQuorum(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "provider_quorum", spec.DegradedReasonProviderQuorum)
}
