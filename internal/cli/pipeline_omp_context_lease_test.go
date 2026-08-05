package cli

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPipelineOMPActiveLease_BindsAllCoordinatesAndConsumesOnce(t *testing.T) {
	now := time.Date(2026, 8, 4, 9, 0, 0, 0, time.UTC)
	binding := pipelineOMPActiveLeaseBindingFixture()
	lease, err := newPipelineOMPActiveLease(binding, now, 4*time.Minute)
	require.NoError(t, err)
	require.NotEmpty(t, lease.NonceHash())
	require.NotEmpty(t, lease.Digest())

	assert.NoError(t, lease.Consume(binding, now.Add(time.Minute)))
	assert.ErrorContains(t, lease.Consume(binding, now.Add(2*time.Minute)), "already consumed")
	_, err = json.Marshal(lease)
	assert.Error(t, err, "a process-private lease must not be serialized")
}

func TestPipelineOMPActiveLease_RejectsMismatchStaleAndOverlongTTL(t *testing.T) {
	now := time.Date(2026, 8, 4, 9, 0, 0, 0, time.UTC)
	binding := pipelineOMPActiveLeaseBindingFixture()

	lease, err := newPipelineOMPActiveLease(binding, now, 5*time.Minute)
	require.NoError(t, err)
	changed := binding
	changed.DecisionDeltaHash = workflowContextRuntimeHash("changed phase prompt")
	assert.ErrorContains(t, lease.Consume(changed, now.Add(time.Minute)), "binding mismatch")
	assert.ErrorContains(t, lease.Consume(binding, now.Add(5*time.Minute)), "expired")

	_, err = newPipelineOMPActiveLease(binding, now, 5*time.Minute+time.Nanosecond)
	assert.ErrorContains(t, err, "validity")
	invalid := binding
	invalid.ModelScopeDigest = ""
	_, err = newPipelineOMPActiveLease(invalid, now, time.Minute)
	assert.ErrorContains(t, err, "binding")
}

func pipelineOMPActiveLeaseBindingFixture() pipelineOMPActiveLeaseBinding {
	return pipelineOMPActiveLeaseBinding{
		GrantDigest: workflowContextRuntimeHash("signed-v2-grant"), WorkspaceID: "autopus-adk",
		PolicyDigest: workflowContextRuntimeHash("policy"),
		SpecID:       "SPEC-OMP-004", TaskID: "pipeline-task-1234", Phase: "implement",
		SessionID: "pipeline-session-1234", BindingHash: workflowContextRuntimeHash("binding"),
		OptionsHash: workflowContextRuntimeHash("options"), SnapshotHash: "sha256:" + strings.Repeat("a", 64),
		GitCommitHash: strings.Repeat("b", 40), OriginalTaskHash: workflowContextRuntimeHash("/auto go SPEC-OMP-004"),
		DecisionDeltaHash: workflowContextRuntimeHash("implement phase"), RuntimeVersion: "omp/17.2.7",
		ExecutableSHA256: workflowContextRuntimeHash("omp executable"), Provider: "openai",
		ModelScopeDigest: workflowContextRuntimeHash("model scope"),
		AutoVersion:      "0.50.95", AutoExecutableSHA256: workflowContextRuntimeHash("auto executable"),
		AutoSourceCommit: strings.Repeat("d", 40), AutoSourceTree: strings.Repeat("e", 40),
		Model: "gpt-5.4", CohortDigest: workflowContextRuntimeHash("20-pair cohort"),
		OracleDigest:        workflowContextRuntimeHash("quality oracle"),
		EligibleHistoryHash: workflowContextRuntimeHash("eligible history"),
	}
}
