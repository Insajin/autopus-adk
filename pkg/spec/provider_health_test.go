package spec_test

// Phase 1.5 test scaffold for SPEC-SPECREV-001 REQ-VERD-1 / REQ-VERD-2 / REQ-VERD-4.
// References spec.ProviderStatus, spec.ClassifyProviderStatuses,
// spec.ShouldLabelDegraded, spec.RenderProviderHealthSection — none yet exist.
//
// Phase 3 coverage extensions: BuildProviderStatuses + classifyResponse paths
// (timeout, exit-error, missing-response, FailedProvider).

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/insajin/autopus-adk/pkg/orchestra"
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

// TestBuildProviderStatuses_OrchestraResponses covers the orchestra
// ProviderResponse → ProviderStatus mapping for the four observable classes:
// success, timeout, exit-error (with custom Error string), and exit-error
// (with empty Error so the exit code is encoded into the Note).
func TestBuildProviderStatuses_OrchestraResponses(t *testing.T) {
	t.Parallel()

	responses := []orchestra.ProviderResponse{
		{Provider: "claude", ExitCode: 0, Output: "VERDICT: PASS"},
		{Provider: "gemini", TimedOut: true},
		{Provider: "codex", ExitCode: 1, Error: "subprocess crashed"},
		{Provider: "opus2", ExitCode: 137},
	}
	configured := []string{"claude", "gemini", "codex", "opus2"}

	got := spec.BuildProviderStatuses(responses, nil, configured)

	require := assert.New(t)
	require.Len(got, 4)
	require.Equal(spec.ProviderStatus{Provider: "claude", Status: "success", Note: "-"}, got[0])
	require.Equal(spec.ProviderStatus{Provider: "gemini", Status: "timeout", Note: "-"}, got[1])
	require.Equal(spec.ProviderStatus{Provider: "codex", Status: "error", Note: "subprocess crashed"}, got[2])
	require.Equal(spec.ProviderStatus{Provider: "opus2", Status: "error", Note: "exit=137"}, got[3])
}

// TestBuildProviderStatuses_FailedProviderAndMissing covers two paths missing
// from the response slice: providers in the FailedProvider list (preflight
// failure) and providers absent from BOTH responses and failed (silent drop).
// Both must surface as Status="error" so review.md never silently omits a
// configured provider — REQ-VERD-1 invariant.
func TestBuildProviderStatuses_FailedProviderAndMissing(t *testing.T) {
	t.Parallel()

	responses := []orchestra.ProviderResponse{
		{Provider: "claude", ExitCode: 0},
	}
	failed := []orchestra.FailedProvider{
		{Name: "gemini", FailureClass: "binary_or_transport"},
		{Name: "opus2"}, // empty FailureClass — placeholder fallback
	}
	configured := []string{"claude", "gemini", "codex", "opus2"}

	got := spec.BuildProviderStatuses(responses, failed, configured)

	require := assert.New(t)
	require.Len(got, 4)
	require.Equal(spec.ProviderStatus{Provider: "claude", Status: "success", Note: "-"}, got[0])
	require.Equal(spec.ProviderStatus{Provider: "gemini", Status: "error", Note: "binary_or_transport"}, got[1])
	require.Equal(spec.ProviderStatus{Provider: "codex", Status: "error", Note: "no response"}, got[2])
	require.Equal(spec.ProviderStatus{Provider: "opus2", Status: "error", Note: "-"}, got[3])
}

// TestDegradedLabel_FormatsExactly covers REQ-VERD-2 label format: the suffix
// MUST match " (degraded — N/M providers responded)" where N is success count
// and M is configured count. Empty string when below threshold.
func TestDegradedLabel_FormatsExactly(t *testing.T) {
	t.Parallel()

	// 1/3 success → degraded
	statuses := []spec.ProviderStatus{
		{Provider: "claude", Status: "success"},
		{Provider: "gemini", Status: "timeout"},
		{Provider: "codex", Status: "timeout"},
	}
	assert.Equal(t, " (degraded — 1/3 providers responded)", spec.DegradedLabel(statuses, 3))

	// 2/3 success → not degraded → empty label
	statuses2 := []spec.ProviderStatus{
		{Provider: "claude", Status: "success"},
		{Provider: "gemini", Status: "success"},
		{Provider: "codex", Status: "timeout"},
	}
	assert.Equal(t, "", spec.DegradedLabel(statuses2, 3))

	// totalConfigured=0 → empty label (defensive)
	assert.Equal(t, "", spec.DegradedLabel(statuses, 0))
}
