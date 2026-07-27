package workerreceipt

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/promptlayer"
)

func TestReceipt_CanonicalBodyHasExactlyFiveFieldsAndEvidenceIsSibling(t *testing.T) {
	t.Parallel()

	receipt := Receipt{
		OwnedPaths:       []string{"pkg/workerreceipt"},
		ChangedFiles:     []string{"pkg/workerreceipt/receipt.go"},
		Verification:     []string{"go test ./pkg/workerreceipt"},
		Blockers:         []string{},
		NextRequiredStep: "pipeline consumer",
	}
	envelope := Envelope{
		SchemaVersion: SchemaVersion,
		Receipt:       receipt,
		Evidence: []EvidenceReference{{
			Ref:  ".autopus/runtime/evidence/worker-receipt.json",
			Hash: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		}},
	}

	bodyJSON, err := json.Marshal(receipt)
	require.NoError(t, err)
	var body map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(bodyJSON, &body))
	assert.ElementsMatch(t, []string{
		"owned_paths",
		"changed_files",
		"verification",
		"blockers",
		"next_required_step",
	}, receiptKeys(body))

	envelopeJSON, err := json.Marshal(envelope)
	require.NoError(t, err)
	var outer map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(envelopeJSON, &outer))
	assert.ElementsMatch(t, []string{"schema_version", "receipt", "evidence"}, receiptKeys(outer))
	assert.Equal(t, "autopus.worker_receipt.v1", SchemaVersion)
	assert.LessOrEqual(t, promptlayer.EstimateTokens(string(envelopeJSON)), 2_000)
	assert.NotContains(t, string(bodyJSON), `"evidence"`)
	assert.NotContains(t, string(envelopeJSON), "/Users/")
	assert.NotContains(t, string(envelopeJSON), "sk-proj-")
}

func receiptKeys(values map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}
