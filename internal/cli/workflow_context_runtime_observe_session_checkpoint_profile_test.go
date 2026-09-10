package cli

// The bounded retry is a property of one measured executable observed by an
// explicitly enabled probe. Everything else — the ordinary v1 producer, a
// version that was not measured, a digest that does not match — keeps the
// single pre-checkpoint contract, and the fixture repeats the same exchange in
// every case so only the profile can explain the difference.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/insajin/autopus-adk/pkg/ompprobe"
	"github.com/insajin/autopus-adk/pkg/promptlayer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkflowContextObserveSession_UnverifiedIdentityKeepsSinglePreCheckpoint(t *testing.T) {
	for _, testCase := range []struct {
		name    string
		version string
		digest  string
	}{
		{
			name:    "the pinned release version was not the measured one",
			version: "omp/17.2.7",
		},
		{
			name:   "the executable digest does not match the measured one",
			digest: "sha256:" + strings.Repeat("b", 64),
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			run := runWorkflowContextObserveCheckpoint(t, workflowContextCheckpointRequest{
				variant: pipelineOMPActiveCheckpointRepeat,
				version: testCase.version,
				digest:  testCase.digest,
			})

			require.Error(t, run.err)
			assert.NotErrorIs(t, run.err, errWorkflowContextObserveSessionProbeCompleted)
			compactions := workflowContextObserveProbeRecordsOfKind(run.records, "compaction")
			require.Len(t, compactions, 1)
			assert.Equal(t, ompprobe.OutcomeFailed, compactions[0].Outcome)
			assert.Equal(t, "pre_ack_out_of_order", compactions[0].AbortReason)
			assert.Equal(t, 2, compactions[0].PreCheckpoints,
				"the repeated checkpoint was received and counted, then refused")
			assert.Zero(t, compactions[0].PostCheckpoints)
			assert.Equal(t, workflowContextCheckpointFirstACKs("pre-1"), run.acks,
				"an unmeasured identity acknowledges one pre-checkpoint only")
			assertWorkflowContextCheckpointRemainsPartial(t, run)
		})
	}
}

// The evidence producer has no probe at all. It must refuse the same exchange
// the probe profile admits, because REQ-CHECKPOINT-001 changes nothing about
// the signed v1 path.
func TestWorkflowContextObserveSession_OrdinaryProducerKeepsSinglePreCheckpoint(t *testing.T) {
	run := runWorkflowContextObserveCheckpoint(t, workflowContextCheckpointRequest{
		variant: pipelineOMPActiveCheckpointRepeat,
		noProbe: true,
	})

	require.Error(t, run.err)
	assert.Empty(t, run.probePath, "the producer path never opens a probe record file")
	assert.Equal(t, workflowContextCheckpointFirstACKs("pre-1"), run.acks,
		"the producer acknowledges one pre-checkpoint and refuses the repeat")
	last := run.responses[len(run.responses)-1]
	assert.Equal(t, "error", last.Type)
	assert.NotEqual(t, "probe_completed", last.ErrorCode)
	assert.Empty(t, last.EvidenceID)
	assert.Empty(t, last.ReportDigest)
	_, err := os.Stat(promptlayer.OMPContextPromotionReportPathV1(run.options.ProjectDir))
	assert.ErrorIs(t, err, os.ErrNotExist)
	_, err = os.Stat(promptlayer.OMPContextEvidenceStorePath(run.options.ProjectDir))
	assert.ErrorIs(t, err, os.ErrNotExist)
}

// The dimensions a cohort cannot reach on its own. An A session never
// compacts and the production overlay is never installed on a probe B
// session, so the grant itself is exercised directly. The measured identity
// is written here as an independent literal: it is external evidence about a
// binary, so a wrong production pin has to break this table rather than move
// with it.
func TestPipelineOMPActiveCheckpointProfileGrantsOnlyTheMeasuredRuntime(t *testing.T) {
	recorder, err := ompprobe.NewRecorder(filepath.Join(t.TempDir(), "probe"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = recorder.Close() })
	watching := &pipelineOMPActiveProbe{recorder: recorder}
	chain := pipelineOMPActiveOverlayChain(true)
	require.Equal(t, []string{"remote", "snapcompact"}, chain,
		"the probe overlay is the only chain the bounded retry was measured on")

	for _, testCase := range []struct {
		name      string
		probe     *pipelineOMPActiveProbe
		optimized bool
		version   string
		digest    string
		chain     []string
		limit     int
	}{
		{
			name:      "the measured probe profile",
			probe:     watching,
			optimized: true,
			version:   workflowContextCheckpointProbeVersion,
			digest:    workflowContextCheckpointProbeDigest,
			chain:     chain,
			limit:     2,
		},
		{
			name:      "an A session on the same runtime",
			probe:     watching,
			optimized: false,
			version:   workflowContextCheckpointProbeVersion,
			digest:    workflowContextCheckpointProbeDigest,
			chain:     chain,
			limit:     1,
		},
		{
			name:      "the production compaction chain",
			probe:     watching,
			optimized: true,
			version:   workflowContextCheckpointProbeVersion,
			digest:    workflowContextCheckpointProbeDigest,
			chain:     pipelineOMPActiveOverlayChain(false),
			limit:     1,
		},
		{
			name:      "a version that was not measured",
			probe:     watching,
			optimized: true,
			version:   "omp/18.1.14",
			digest:    workflowContextCheckpointProbeDigest,
			chain:     chain,
			limit:     1,
		},
		{
			name:      "a digest that does not match",
			probe:     watching,
			optimized: true,
			version:   workflowContextCheckpointProbeVersion,
			digest:    "sha256:" + strings.Repeat("a", 64),
			chain:     chain,
			limit:     1,
		},
		{
			name:      "an unmeasured identity",
			probe:     watching,
			optimized: true,
			chain:     chain,
			limit:     1,
		},
		{
			name:      "a probe that records nothing",
			probe:     &pipelineOMPActiveProbe{},
			optimized: true,
			version:   workflowContextCheckpointProbeVersion,
			digest:    workflowContextCheckpointProbeDigest,
			chain:     chain,
			limit:     1,
		},
		{
			name:      "production, which carries no probe",
			optimized: true,
			version:   workflowContextCheckpointProbeVersion,
			digest:    workflowContextCheckpointProbeDigest,
			chain:     chain,
			limit:     1,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			protocol := newPipelineOMPRPCProtocol(nil)
			assert.Equal(t, 1, protocol.preCheckpointLimit, "an unconfigured protocol is single-pre")
			protocol.probe = testCase.probe

			protocol.configureProbeCheckpointProfile(
				testCase.version, testCase.digest, testCase.optimized, testCase.chain)

			assert.Equal(t, testCase.limit, protocol.preCheckpointLimit)
		})
	}
}
