package orchestra

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const managedReviewSnapshotDigest = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func TestPrepareManagedReview_ThreeProviders_ReturnsPureBoundedContracts(t *testing.T) {
	t.Parallel()

	contractJSON := managedReviewContractJSON(t)
	contract, err := DecodeReviewPrepareContractStrict(contractJSON, 256*1024)
	require.NoError(t, err)
	preparation, err := PrepareManagedReview(contract)
	require.NoError(t, err)

	encoded, err := json.Marshal(preparation)
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(encoded, &payload))
	assert.Equal(t, "review.preparation.v1", payload["schema_version"])
	assert.Equal(t, managedReviewSnapshotDigest, payload["snapshot_digest"])
	providers, ok := payload["provider_contracts"].([]any)
	require.True(t, ok)
	require.Len(t, providers, 3)
	for _, value := range providers {
		provider, ok := value.(map[string]any)
		require.True(t, ok)
		assert.Contains(t, []string{"claude", "codex", "gemini"}, provider["adapter_id"])
		assert.Regexp(t, `^[0-9a-f]{64}$`, provider["prompt_digest"])
		assert.Equal(t, "review.provider_result.v1", provider["result_schema"])
		assert.NotEmpty(t, provider["prompt"])
		assert.EqualValues(t, 1<<20, provider["max_result_bytes"])
		assert.EqualValues(t, 200, provider["max_findings"])
	}
	assertManagedReviewNoKeys(t, payload,
		"command", "cwd", "env", "terminal_handle", "next_state",
		"raw_response", "failure_preview", "provider_process",
	)
}

func TestDecodeReviewPrepareContractStrict_UnknownField_FailsClosed(t *testing.T) {
	t.Parallel()

	var payload map[string]any
	require.NoError(t, json.Unmarshal(managedReviewContractJSON(t), &payload))
	payload["command"] = "codex review"
	encoded, err := json.Marshal(payload)
	require.NoError(t, err)

	_, err = DecodeReviewPrepareContractStrict(encoded, 256*1024)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "review_prepare_invalid")
}

func TestPrepareManagedReview_AdapterIdentityMustBeCanonicalLowercase(t *testing.T) {
	t.Parallel()

	var payload map[string]any
	require.NoError(t, json.Unmarshal(managedReviewContractJSON(t), &payload))
	providers := payload["providers"].([]any)
	providers[0].(map[string]any)["adapter_id"] = "Claude"
	encoded, err := json.Marshal(payload)
	require.NoError(t, err)

	_, err = DecodeReviewPrepareContractStrict(encoded, ReviewPrepareMaximumBytes)
	require.ErrorIs(t, err, ErrReviewPrepareInvalid)
}

func managedReviewContractJSON(t *testing.T) []byte {
	t.Helper()
	payload := map[string]any{
		"schema_version": "review.prepare.v1",
		"request_id":     "req-review-1", "workspace_id": "ws-a",
		"repo_scope_ref": "repo-scope-a", "work_item_id": "work-a",
		"review_run_id": "review-1", "snapshot_digest": managedReviewSnapshotDigest,
		"role": "reviewer", "contract_digest": "sha256:" + managedReviewSnapshotDigest,
		"providers": []map[string]any{
			{"adapter_id": "claude", "model": "claude-review", "role": "reviewer"},
			{"adapter_id": "codex", "model": "codex-review", "role": "reviewer"},
			{"adapter_id": "gemini", "model": "gemini-review", "role": "reviewer"},
		},
		"bounds": map[string]any{"max_result_bytes": 1 << 20, "max_findings": 200},
	}
	encoded, err := json.Marshal(payload)
	require.NoError(t, err)
	return encoded
}

func assertManagedReviewNoKeys(t *testing.T, value any, forbidden ...string) {
	t.Helper()
	blocked := make(map[string]bool, len(forbidden))
	for _, key := range forbidden {
		blocked[key] = true
	}
	var walk func(any)
	walk = func(current any) {
		switch typed := current.(type) {
		case map[string]any:
			for key, nested := range typed {
				assert.Falsef(t, blocked[key], "forbidden preparation key %q", key)
				walk(nested)
			}
		case []any:
			for _, nested := range typed {
				walk(nested)
			}
		}
	}
	walk(value)
}
