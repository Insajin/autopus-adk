package pipeline_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/pipeline"
)

func TestOMPExecutionView_IsSealedAndBoundToTheExactPhase(t *testing.T) {
	input := pipeline.OMPExecutionViewInput{
		ProjectDir:    "/workspace",
		SpecID:        "SPEC-OMP-004",
		SpecDir:       ".autopus/specs/SPEC-OMP-004",
		SnapshotHash:  "sha256:" + repeatHex("a"),
		GitCommitHash: repeatN("b", 40),
		PhaseID:       pipeline.PhaseImplement,
		Attempt:       2,
		Prompt:        "authoritative phase prompt",
		CompletedHistory: []string{
			"completed planner output",
		},
	}
	view, err := pipeline.NewOMPExecutionView(input)
	require.NoError(t, err)
	binding, err := view.Binding()
	require.NoError(t, err)
	require.Equal(t, pipeline.OMPExecutionViewBinding{
		SpecID: input.SpecID, SnapshotHash: input.SnapshotHash,
		GitCommitHash: input.GitCommitHash, PhaseID: input.PhaseID, Attempt: input.Attempt,
	}, binding)

	input.CompletedHistory[0] = "mutated caller history"
	snapshot, err := view.Open(binding)
	require.NoError(t, err)
	require.Equal(t, "completed planner output", snapshot.CompletedHistory[0])
	snapshot.CompletedHistory[0] = "mutated reader history"

	again, err := view.Open(pipeline.OMPExecutionViewBinding{
		SpecID: input.SpecID, SnapshotHash: input.SnapshotHash,
		GitCommitHash: input.GitCommitHash, PhaseID: input.PhaseID, Attempt: input.Attempt,
	})
	require.NoError(t, err)
	require.Equal(t, "completed planner output", again.CompletedHistory[0])

	_, err = view.Open(pipeline.OMPExecutionViewBinding{
		SpecID: input.SpecID, SnapshotHash: "sha256:" + repeatHex("c"),
		GitCommitHash: input.GitCommitHash, PhaseID: input.PhaseID, Attempt: input.Attempt,
	})
	require.ErrorContains(t, err, "binding")
	_, err = json.Marshal(view)
	require.ErrorContains(t, err, "sealed")
}

func TestOMPExecutionView_RejectsIncompleteAuthority(t *testing.T) {
	_, err := pipeline.NewOMPExecutionView(pipeline.OMPExecutionViewInput{})
	require.Error(t, err)

	var nilView *pipeline.OMPExecutionView
	_, err = nilView.Binding()
	require.ErrorContains(t, err, "unavailable")
	_, err = (&pipeline.OMPExecutionView{}).Binding()
	require.ErrorContains(t, err, "unavailable")
}

func repeatHex(value string) string { return repeatN(value, 64) }

func repeatN(value string, count int) string {
	out := ""
	for range count {
		out += value
	}
	return out
}
