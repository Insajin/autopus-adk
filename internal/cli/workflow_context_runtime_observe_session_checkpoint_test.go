package cli

// AC-013 / REQ-CHECKPOINT-001. A stateful fake OMP repeats the authenticated
// pre-checkpoint inside one correlated manual compact transaction, the way the
// two retained T0 runs observed omp/18.1.13 behave. Only an explicitly enabled
// probe on the measured identity may acknowledge the second one, and it must
// prove the transcript did not move while the checkpoint waited.

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/insajin/autopus-adk/pkg/ompprobe"
	"github.com/insajin/autopus-adk/pkg/promptlayer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The Darwin/arm64 identity REQ-CHECKPOINT-001 names. Runtime measures it from
// the executable; a fixture injects it through the observe-session setup seam
// the prepare hook already owns, never through a production bypass.
const (
	workflowContextCheckpointProbeVersion = "omp/18.1.13"
	workflowContextCheckpointProbeDigest  = "sha256:" +
		"a4c5c9cc5b8222184d0d7429b0bb6ac2a92bbe45dd11bf68e4b1360050791909"
)

type workflowContextCheckpointRequest struct {
	variant string
	// version and digest default to the measured probe profile.
	version string
	digest  string
	// noProbe runs the ordinary v1 producer path, which has no probe at all.
	noProbe bool
}

type workflowContextCheckpointRun struct {
	records   []ompprobe.Record
	responses []workflowContextObserveSessionResponse
	options   workflowContextObserveSessionOptions
	probePath string
	// acks lists every extension_ui_response the harness sent, in the order
	// the fake OMP logged them.
	acks []string
	err  error
}

func runWorkflowContextObserveCheckpoint(
	t *testing.T,
	request workflowContextCheckpointRequest,
) workflowContextCheckpointRun {
	t.Helper()
	requireDarwinManagedOMPSandboxForTest(t)
	model := pipelineOMPActiveCheckpointModel(request.variant)
	challenge := workflowContextRuntimeHash("observe-session-checkpoint-" + model)
	setup, options, logPath := workflowContextObserveSessionFixtureWithModel(t, challenge, model)
	setup.ompVersion, setup.ompExecutableSHA256 =
		workflowContextCheckpointProbeVersion, workflowContextCheckpointProbeDigest
	if request.version != "" {
		setup.ompVersion = request.version
	}
	if request.digest != "" {
		setup.ompExecutableSHA256 = request.digest
	}
	if !request.noProbe {
		options.ProbeDir = filepath.Join(t.TempDir(), "probe")
	}
	input, _ := workflowContextObserveEvidenceInput(t, challenge)
	var output bytes.Buffer
	prepared := &setup
	ctx := context.WithValue(context.Background(), workflowContextObserveSessionPrepareKey{},
		workflowContextObserveSessionPrepare(func(
			_ context.Context, _ workflowContextObserveSessionOptions, _ string,
		) (workflowContextObserveSessionSetup, error) {
			return *prepared, nil
		}))
	run := workflowContextCheckpointRun{options: options}
	run.err = RunWorkflowContextObserveSession(ctx, input, &output, options)
	run.responses = decodeWorkflowContextObserveResponses(t, output.Bytes())
	run.acks = workflowContextCheckpointACKs(t, logPath)
	if options.ProbeDir == "" {
		return run
	}
	run.probePath = filepath.Join(options.ProbeDir, "probe.jsonl")
	body, err := os.ReadFile(run.probePath)
	require.NoError(t, err, "an interrupted probe still retains its records")
	run.records = decodeWorkflowContextObserveProbeRecords(t, body)
	return run
}

func workflowContextCheckpointACKs(t *testing.T, logPath string) []string {
	t.Helper()
	acks := make([]string, 0, 64)
	for _, record := range readPipelineOMPRPCRecords(t, logPath) {
		if record.Kind == "command" && record.Type == "extension_ui_response" {
			acks = append(acks, record.ID)
		}
	}
	return acks
}

// workflowContextCheckpointExpectedACKs is the acknowledgement sequence a
// clean cohort produces: two pre-checkpoints and one post-checkpoint per
// compaction, nine compactions per segment, both segments identical because
// each one runs a fresh process.
func workflowContextCheckpointExpectedACKs() []string {
	acks := make([]string, 0, 54)
	for range workflowContextObserveSessionSegmentCount {
		for sequence := 1; sequence < workflowContextObserveSessionSegmentPairs; sequence++ {
			acks = append(acks,
				fmt.Sprintf("active-checkpoint-%d-pre-1", sequence),
				fmt.Sprintf("active-checkpoint-%d-pre-2", sequence),
				fmt.Sprintf("active-checkpoint-%d-post", sequence))
		}
	}
	return acks
}

func assertWorkflowContextCheckpointProducedNoEvidence(t *testing.T, run workflowContextCheckpointRun) {
	t.Helper()
	_, err := os.Stat(promptlayer.OMPContextPromotionReportPathV1(run.options.ProjectDir))
	assert.ErrorIs(t, err, os.ErrNotExist, "a checkpoint probe never writes a promotion report")
	_, err = os.Stat(promptlayer.OMPContextEvidenceStorePath(run.options.ProjectDir))
	assert.ErrorIs(t, err, os.ErrNotExist, "a checkpoint probe never writes an evidence store")
	last := run.responses[len(run.responses)-1]
	assert.Empty(t, last.EvidenceID)
	assert.Empty(t, last.ReportDigest)
}

// S13: the legitimate two-pre transaction. The probe acknowledges p1 and p2
// once each, re-admits the canonical prompt once at q, and files the received
// checkpoint counts without claiming anything about provider attempts.
func TestWorkflowContextObserveSession_ProbeCompletesRepeatedPreCheckpoint(t *testing.T) {
	run := runWorkflowContextObserveCheckpoint(t, workflowContextCheckpointRequest{
		variant: pipelineOMPActiveCheckpointRepeat,
	})

	require.ErrorIs(t, run.err, errWorkflowContextObserveSessionProbeCompleted)
	last := run.responses[len(run.responses)-1]
	assert.Equal(t, "probe_completed", last.ErrorCode)
	assert.Equal(t, "probe", last.ErrorStage)

	compactions := workflowContextObserveProbeRecordsOfKind(run.records, "compaction")
	require.Len(t, compactions, 18)
	for _, record := range compactions {
		assert.Equal(t, "completed", record.Outcome)
		assert.Equal(t, ompprobe.MethodSnapcompact, record.Method)
		assert.Equal(t, 2, record.PreCheckpoints, "both authenticated pre-checkpoints were received")
		assert.Equal(t, 1, record.PostCheckpoints)
		assert.Equal(t, ompprobe.AttemptCoverageUnknown, record.AttemptCoverage,
			"a repeated checkpoint is not a provider request count")
		assert.Empty(t, record.AbortReason)
		assert.Equal(t, []string{"confirm"}, record.UIRequestMethods)
		assert.Contains(t, record.Roles, "compactionSummary")
		require.NotNil(t, record.TokensBefore)
		assert.Equal(t, int64(61904), *record.TokensBefore)
		assert.Zero(t, record.RejectedIdentifiers)
	}
	require.Len(t, workflowContextObserveProbeRecordsOfKind(run.records, "call"), 40)
	assert.Equal(t, workflowContextCheckpointExpectedACKs(), run.acks,
		"each transaction acknowledged p1, then p2, then q, exactly once")
	assertWorkflowContextCheckpointProducedNoEvidence(t, run)
}
