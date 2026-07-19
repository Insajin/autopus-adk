package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReviewPrepareCLI_ContractStdin_DoesNotSpawnProviderOrTerminalProcess(t *testing.T) {
	marker := installReviewPrepareSpawnSentinels(t)
	contract := reviewPrepareCLIContract(t)
	root := &cobra.Command{Use: "auto", SilenceErrors: true, SilenceUsage: true}
	root.AddCommand(newReviewCmd())
	var stdout bytes.Buffer
	root.SetIn(bytes.NewReader(contract))
	root.SetOut(&stdout)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"review", "prepare", "--contract-stdin", "--json"})

	require.NoError(t, root.Execute())
	_, err := os.Stat(marker)
	assert.True(t, os.IsNotExist(err), "review prepare must not spawn a provider, pane, or terminal process")

	var envelope map[string]any
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &envelope))
	assert.Equal(t, "success", envelope["status"])
	data, ok := envelope["data"].(map[string]any)
	require.True(t, ok)
	providers, ok := data["provider_contracts"].([]any)
	require.True(t, ok)
	require.Len(t, providers, 3)
	assertCLIReviewPrepareNoKeys(t, data,
		"command", "cwd", "env", "terminal_handle", "next_state", "raw_response", "failure_preview",
	)
}

func installReviewPrepareSpawnSentinels(t *testing.T) string {
	t.Helper()
	directory := t.TempDir()
	marker := filepath.Join(directory, "spawned")
	stub := []byte("#!/bin/sh\nprintf spawned > '" + marker + "'\nexit 97\n")
	for _, name := range []string{"claude", "codex", "gemini", "cmux", "tmux", "open", "pane_runner"} {
		require.NoError(t, os.WriteFile(filepath.Join(directory, name), stub, 0o755))
	}
	t.Setenv("PATH", directory+string(os.PathListSeparator)+os.Getenv("PATH"))
	return marker
}

func reviewPrepareCLIContract(t *testing.T) []byte {
	t.Helper()
	digest := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	payload := map[string]any{
		"schema_version": "review.prepare.v1",
		"request_id":     "req-review-1", "workspace_id": "ws-a",
		"repo_scope_ref": "repo-scope-a", "work_item_id": "work-a",
		"review_run_id": "review-1", "snapshot_digest": digest,
		"role": "reviewer", "contract_digest": "sha256:" + digest,
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

func assertCLIReviewPrepareNoKeys(t *testing.T, value any, forbidden ...string) {
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
				assert.Falsef(t, blocked[key], "forbidden CLI preparation key %q", key)
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
