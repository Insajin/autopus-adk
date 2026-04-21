package orchestra

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollectRoundHookResults_TimeoutWritesStructuredEvidence(t *testing.T) {
	t.Parallel()

	sessionID := "test-reliability-timeout"
	runID := "run-reliability-timeout"
	sess, err := NewHookSession(sessionID)
	require.NoError(t, err)
	defer sess.Cleanup()

	cfg := OrchestraConfig{
		Providers:      []ProviderConfig{{Name: "claude"}},
		TimeoutSeconds: 1,
		HookMode:       true,
		SessionID:      sessionID,
		RunID:          runID,
		FallbackMode:   FallbackModeSubprocess,
	}

	responses := collectRoundHookResults(context.Background(), cfg, sess, 2)
	require.Len(t, responses, 1)
	assert.True(t, responses[0].TimedOut)
	require.NotEmpty(t, responses[0].Receipt)

	var receipt CollectionReceipt
	receiptData, err := os.ReadFile(responses[0].Receipt)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(receiptData, &receipt))
	assert.Equal(t, runID, receipt.Correlation.RunID)
	assert.Equal(t, "round-2", receipt.Correlation.RoundID)
	assert.Equal(t, "claude", receipt.Correlation.ProviderID)
	assert.Equal(t, "hook", receipt.CollectionMode)
	assert.Equal(t, "hook", receipt.Provenance)
	assert.Equal(t, "timeout", receipt.Status)
	assert.True(t, receipt.Partial)

	bundlePath := filepath.Join(filepath.Dir(responses[0].Receipt), "failure-bundle.json")
	bundleData, err := os.ReadFile(bundlePath)
	require.NoError(t, err)

	var bundle FailureBundle
	require.NoError(t, json.Unmarshal(bundleData, &bundle))
	assert.Equal(t, runID, bundle.RunID)
	assert.True(t, bundle.Degraded)
	require.Len(t, bundle.Events, 1)
	assert.Equal(t, "hook_timeout", bundle.Events[0].Kind)
	assert.Equal(t, "round-2", bundle.Events[0].Correlation.RoundID)
	assert.Equal(t, "claude", bundle.Events[0].Correlation.ProviderID)
	assert.Contains(t, bundle.NextStep, "subprocess")
}

func TestCollectRoundHookResults_MissingResultWritesPartialReceipt(t *testing.T) {
	t.Parallel()

	sessionID := "test-reliability-missing-result"
	runID := "run-reliability-missing-result"
	sess, err := NewHookSession(sessionID)
	require.NoError(t, err)
	defer sess.Cleanup()

	cfg := OrchestraConfig{
		Providers:      []ProviderConfig{{Name: "claude"}},
		TimeoutSeconds: 2,
		HookMode:       true,
		SessionID:      sessionID,
		RunID:          runID,
	}

	doneName := RoundSignalName("claude", 1, "done")
	require.NoError(t, os.WriteFile(filepath.Join(sess.Dir(), doneName), []byte("1"), 0o644))

	responses := collectRoundHookResults(context.Background(), cfg, sess, 1)
	require.Len(t, responses, 1)
	assert.Empty(t, responses[0].Output)
	require.NotEmpty(t, responses[0].Receipt)

	var receipt CollectionReceipt
	receiptData, err := os.ReadFile(responses[0].Receipt)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(receiptData, &receipt))
	assert.Equal(t, runID, receipt.Correlation.RunID)
	assert.Equal(t, "round-1", receipt.Correlation.RoundID)
	assert.Equal(t, "hook", receipt.Provenance)
	assert.Equal(t, "read_failed", receipt.Status)
	assert.True(t, receipt.Partial)
	assert.Contains(t, receipt.Error, "read result file")
}
