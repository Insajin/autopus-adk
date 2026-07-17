package spec_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/orchestra"
	"github.com/insajin/autopus-adk/pkg/spec"
)

func TestBuildProviderStatuses_TimeoutNoteUsesStructuredSafeMetadata(t *testing.T) {
	t.Parallel()

	responses := []orchestra.ProviderResponse{{
		Provider: "codex",
		Output:   "raw output /Users/alice/private token=secret-value",
		Error:    "raw stderr API_KEY=secret-value",
		TimedOut: true,
	}}
	failed := []orchestra.FailedProvider{{
		Name:               "codex",
		FailureClass:       "timeout",
		TimeoutSource:      "spec_review_timeout",
		ConfiguredDuration: 90 * time.Second,
		ElapsedDuration:    89500 * time.Millisecond,
		CollectionMode:     "subprocess_stdout",
		StderrPreview:      "/Users/alice/private API_KEY=secret-value",
		OutputPreview:      "private review body",
	}}

	got := spec.BuildProviderStatuses(responses, failed, []string{"codex"})

	require.Len(t, got, 1)
	assert.Equal(t, "timeout; source=spec_review_timeout; budget=1m30s; elapsed=1m29.5s; collection=subprocess_stdout; partial_output=true", got[0].Note)
	assert.NotContains(t, got[0].Note, "/Users/")
	assert.NotContains(t, got[0].Note, "secret-value")
	assert.NotContains(t, got[0].Note, "private review body")
	assert.NotContains(t, got[0].Note, "raw stderr")
	assert.NotContains(t, got[0].Note, "raw output")
}

func TestBuildProviderStatuses_TimeoutNoteAllowListsUntrustedMetadata(t *testing.T) {
	t.Parallel()

	failed := []orchestra.FailedProvider{{
		Name:               "codex",
		FailureClass:       "timeout",
		TimeoutSource:      "/Users/alice/private?token=secret",
		ConfiguredDuration: -time.Second,
		ElapsedDuration:    -time.Second,
		CollectionMode:     "../../private/output",
	}}

	got := spec.BuildProviderStatuses(nil, failed, []string{"codex"})

	require.Len(t, got, 1)
	assert.Equal(t, "timeout; source=unknown; budget=unknown; elapsed=unknown; collection=unknown; partial_output=false", got[0].Note)
}

func TestBuildProviderStatuses_ResponseOnlyTimeoutStillUsesStructuredNote(t *testing.T) {
	t.Parallel()

	responses := []orchestra.ProviderResponse{{
		Provider:        "codex",
		Output:          "partial body",
		Error:           "/Users/alice/private API_KEY=secret",
		Duration:        7 * time.Second,
		TimedOut:        true,
		ExecutedBackend: "pane",
	}}

	got := spec.BuildProviderStatuses(responses, nil, []string{"codex"})

	require.Len(t, got, 1)
	assert.Equal(t, "timeout; source=unknown; budget=unknown; elapsed=7s; collection=pane; partial_output=true", got[0].Note)
}
