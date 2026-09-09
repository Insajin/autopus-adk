package cli

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/ompprobe"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProbeAbortDistinguishesCheckpointOrder(t *testing.T) {
	assert.Equal(t, "pre_ack_out_of_order", pipelineOMPActiveProbeAbortReason(
		errors.New("managed active OMP pre-compaction checkpoint is out of order")))
	assert.Equal(t, "post_ack_out_of_order", pipelineOMPActiveProbeAbortReason(
		errors.New("managed active OMP post-compaction rehydration is out of order")))
}

func TestProbeCompactionRecordRetainsElapsedTime(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "probe")
	recorder, err := ompprobe.NewRecorder(dir)
	require.NoError(t, err)
	probe := &pipelineOMPActiveProbe{recorder: recorder, sequence: 6, variant: "B", sessionSequence: 3, sessionSegment: 1}
	probe.beginCompaction()
	probe.attempt.startedAt = time.Now().Add(-5 * time.Second)
	probe.finishCompaction("pre_ack_out_of_order")
	require.NoError(t, probe.failure)
	require.NoError(t, recorder.Close())
	body, err := os.ReadFile(filepath.Join(dir, "probe.jsonl"))
	require.NoError(t, err)
	records := decodeWorkflowContextObserveProbeRecords(t, body)
	require.Len(t, records, 1)
	assert.GreaterOrEqual(t, records[0].ElapsedMS, int64(5000))
	assert.Equal(t, "pre_ack_out_of_order", records[0].AbortReason)
	assert.Equal(t, "failed", records[0].Outcome)
}

func TestProbeFailedCallRetainsElapsedTime(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "probe")
	recorder, err := ompprobe.NewRecorder(dir)
	require.NoError(t, err)
	probe := &pipelineOMPActiveProbe{recorder: recorder}
	probe.beginCall(6, "B", 3, 1)
	probe.callStartedAt = time.Now().Add(-5 * time.Second)
	require.NoError(t, probe.abort(errors.New("failed runtime")))
	body, err := os.ReadFile(filepath.Join(dir, "probe.jsonl"))
	require.NoError(t, err)
	records := decodeWorkflowContextObserveProbeRecords(t, body)
	require.Len(t, records, 1)
	assert.GreaterOrEqual(t, records[0].ElapsedMS, int64(5000))
	assert.Equal(t, "runtime_failed", records[0].AbortReason)
}
