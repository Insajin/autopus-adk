package spec_test

// Phase 1.5 test scaffold for SPEC-SPECREV-001 REQ-VERD-1 / REQ-VERD-2 / REQ-VERD-4.
// References spec.ProviderStatus, spec.ClassifyProviderStatuses,
// spec.ShouldLabelDegraded, spec.RenderProviderHealthSection — none yet exist.

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/insajin/autopus-adk/pkg/spec"
)

// TestShouldLabelDegraded_BoundaryCases covers REQ-VERD-2 inclusive 50% threshold.
func TestShouldLabelDegraded_BoundaryCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		statuses        []spec.ProviderStatus
		totalConfigured int
		want            bool
	}{
		{
			name: "1/3 timeout (33%) is below threshold and not degraded",
			statuses: []spec.ProviderStatus{
				{Provider: "claude", Status: "success"},
				{Provider: "gemini", Status: "success"},
				{Provider: "codex", Status: "timeout"},
			},
			totalConfigured: 3,
			want:            false,
		},
		{
			name: "2/3 timeout (66.6%) is above threshold and degraded",
			statuses: []spec.ProviderStatus{
				{Provider: "claude", Status: "success"},
				{Provider: "gemini", Status: "timeout"},
				{Provider: "codex", Status: "timeout"},
			},
			totalConfigured: 3,
			want:            true,
		},
		{
			name: "2/4 timeout (exactly 50%) is inclusive and degraded",
			statuses: []spec.ProviderStatus{
				{Provider: "claude", Status: "success"},
				{Provider: "gemini", Status: "success"},
				{Provider: "codex", Status: "timeout"},
				{Provider: "opus2", Status: "timeout"},
			},
			totalConfigured: 4,
			want:            true,
		},
		{
			name: "all success is not degraded",
			statuses: []spec.ProviderStatus{
				{Provider: "claude", Status: "success"},
				{Provider: "gemini", Status: "success"},
			},
			totalConfigured: 2,
			want:            false,
		},
		{
			name: "error status counts as failure (1/2 -> 50% degraded)",
			statuses: []spec.ProviderStatus{
				{Provider: "claude", Status: "success"},
				{Provider: "gemini", Status: "error"},
			},
			totalConfigured: 2,
			want:            true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := spec.ShouldLabelDegraded(tt.statuses, tt.totalConfigured)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestRenderProviderHealthSection_TableColumns covers REQ-VERD-1: the rendered
// section MUST include the heading and the documented column order.
func TestRenderProviderHealthSection_TableColumns(t *testing.T) {
	t.Parallel()

	statuses := []spec.ProviderStatus{
		{Provider: "claude", Status: "success", Note: "-"},
		{Provider: "gemini", Status: "timeout", Note: "-"},
		{Provider: "codex", Status: "timeout", Note: "-"},
	}

	out := spec.RenderProviderHealthSection(statuses, 3)

	assert.Contains(t, out, "## Provider Health", "must include the section heading")
	assert.Contains(t, out, "| Provider | Status | Note |", "must include the documented column order")
	// AC-VERD-1: the three required rows.
	assert.Contains(t, out, "| claude | success |")
	assert.Contains(t, out, "| gemini | timeout |")
	assert.Contains(t, out, "| codex | timeout |")
}

// TestClassifyProviderStatuses_TimeoutAndSuccess covers REQ-VERD-1 mapping
// from raw provider responses to a deterministic ProviderStatus slice.
// The exact input shape is left to Phase 2 — but classification of success vs
// timeout MUST be observable in the returned slice.
func TestClassifyProviderStatuses_TimeoutAndSuccess(t *testing.T) {
	t.Parallel()

	statuses := []spec.ProviderStatus{
		{Provider: "claude", Status: "success", Note: "-"},
		{Provider: "gemini", Status: "timeout", Note: "-"},
	}

	// Pass-through behavior assertion: the order is preserved when no
	// reclassification is needed. Phase 2 may add ordering rules, but the
	// per-row Status value must remain a stable observable.
	got := spec.ClassifyProviderStatuses(statuses)
	require := assert.New(t)
	require.Len(got, 2)
	require.Equal("success", got[0].Status)
	require.Equal("timeout", got[1].Status)
	require.Equal("claude", got[0].Provider)
	require.Equal("gemini", got[1].Provider)
}
