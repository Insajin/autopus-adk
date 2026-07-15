package worker

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleTask_RawPromptCannotBypassResolvedRequiredSpecDocuments(t *testing.T) {
	fixture := writeRetainedWorkerContext(t)
	provider := &mockAdapter{
		name:   "codex",
		script: `cat >/dev/null; echo '{"type":"result","output":"done"}'`,
	}
	wl := &WorkerLoop{config: LoopConfig{Provider: provider, WorkDir: fixture.root}}
	payload, err := json.Marshal(taskPayloadMessage{
		Prompt: "RAW_BACKEND_PROMPT_MUST_BE_AUGMENTED",
		SpecID: fixture.specID,
	})
	require.NoError(t, err)

	_, err = wl.handleTask(context.Background(), "direct-context-raw", payload)
	require.NoError(t, err)
	require.Len(t, provider.calls, 1)
	assert.Contains(t, provider.calls[0].Prompt, "RAW_BACKEND_PROMPT_MUST_BE_AUGMENTED")
	assert.Contains(t, provider.calls[0].Prompt, "context_ack")
	assert.Contains(t, provider.calls[0].Prompt, "source_hash=sha256:")
	assertCompleteRequiredSpecSnapshot(t, provider.calls[0].Prompt, fixture.documents)
}

func TestHandleTask_DirectBuildCommandReceivesCompleteRequiredSpecTails(t *testing.T) {
	fixture := writeRetainedWorkerContext(t)
	provider := &mockAdapter{
		name:   "codex",
		script: `cat >/dev/null; echo '{"type":"result","output":"done"}'`,
	}
	wl := &WorkerLoop{config: LoopConfig{Provider: provider, WorkDir: fixture.root}}
	payload, err := json.Marshal(taskPayloadMessage{
		Description: "Build using the locally resolved SPEC.",
		SpecID:      fixture.specID,
	})
	require.NoError(t, err)

	_, err = wl.handleTask(context.Background(), "direct-context-complete", payload)
	require.NoError(t, err)
	require.Len(t, provider.calls, 1)
	for name, body := range fixture.documents {
		assert.Greater(t, len(body), 32<<10, "%s fixture must exercise the old truncation boundary", name)
	}
	assertCompleteRequiredSpecSnapshot(t, provider.calls[0].Prompt, fixture.documents)
}

func TestHandleTask_GPTWithoutSpecRetainsCoreDocuments(t *testing.T) {
	root := t.TempDir()
	for rel, body := range map[string]string{
		"AGENTS.md":                     "CORE_AGENTS_CONTEXT",
		".autopus/project/workspace.md": "CORE_WORKSPACE_CONTEXT",
	} {
		path := filepath.Join(root, filepath.FromSlash(rel))
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
		require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	}
	provider := &mockAdapter{name: "codex", script: `cat >/dev/null; echo '{"type":"result","output":"done"}'`}
	wl := &WorkerLoop{config: LoopConfig{Provider: provider, WorkDir: root}}
	payload, err := json.Marshal(taskPayloadMessage{Prompt: "CORE_ONLY_TASK"})
	require.NoError(t, err)

	_, err = wl.handleTask(context.Background(), "core-context", payload)
	require.NoError(t, err)
	require.Len(t, provider.calls, 1)
	assert.Contains(t, provider.calls[0].Prompt, "CORE_AGENTS_CONTEXT")
	assert.Contains(t, provider.calls[0].Prompt, "CORE_WORKSPACE_CONTEXT")
}

func TestHandleTask_DeclaredMissingSpecFailsBeforeBuildCommand(t *testing.T) {
	root := t.TempDir()
	for rel, body := range map[string]string{
		"AGENTS.md":                     "agents",
		".autopus/project/workspace.md": "workspace",
	} {
		path := filepath.Join(root, filepath.FromSlash(rel))
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
		require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	}
	provider := &mockAdapter{name: "codex", script: `echo unexpected`}
	wl := &WorkerLoop{config: LoopConfig{Provider: provider, WorkDir: root}}
	payload, err := json.Marshal(taskPayloadMessage{Prompt: "task", SpecID: "SPEC-MISSING-001"})
	require.NoError(t, err)

	_, err = wl.handleTask(context.Background(), "missing-spec-dir", payload)
	require.Error(t, err)
	assert.Empty(t, provider.calls)
}

func TestHandleTask_WrongSpecIdentityFailsBeforeBuildCommand(t *testing.T) {
	fixture := writeRetainedWorkerContext(t)
	wrong := retainedWorkerSpecDocument("SPEC-DIFFERENT-001", "wrong identity")
	require.NoError(t, os.WriteFile(filepath.Join(fixture.specDir, "spec.md"), []byte(wrong), 0o600))
	provider := &mockAdapter{name: "codex", script: `echo unexpected`}
	wl := &WorkerLoop{config: LoopConfig{Provider: provider, WorkDir: fixture.root}}
	payload, err := json.Marshal(taskPayloadMessage{Prompt: "task", SpecID: fixture.specID})
	require.NoError(t, err)

	_, err = wl.handleTask(context.Background(), "wrong-spec-identity", payload)
	require.Error(t, err)
	assert.ErrorContains(t, err, "wrong-SPEC")
	assert.Empty(t, provider.calls)
}

func TestHandleTask_IsolatedWorktreeSnapshotsExecutionTreeNotDirtyBase(t *testing.T) {
	fixture := writeRetainedWorkerContext(t)
	require.NoError(t, os.WriteFile(filepath.Join(fixture.root, "AGENTS.md"), []byte("COMMITTED_WORKTREE_CONTEXT"), 0o600))
	runGit(t, fixture.root, "init")
	runGit(t, fixture.root, "config", "user.email", "context@test.invalid")
	runGit(t, fixture.root, "config", "user.name", "Context Test")
	runGit(t, fixture.root, "add", ".")
	runGit(t, fixture.root, "commit", "-m", "context fixture")
	require.NoError(t, os.WriteFile(filepath.Join(fixture.root, "AGENTS.md"), []byte("DIRTY_BASE_CONTEXT_MUST_NOT_LEAK"), 0o600))

	provider := &mockAdapter{name: "codex", script: `cat >/dev/null; echo '{"type":"result","output":"done"}'`}
	wl := NewWorkerLoop(LoopConfig{
		Provider: provider, WorkDir: fixture.root, MaxConcurrency: 2, WorktreeIsolation: true,
	})
	wl.configureExecutionConcurrency()
	payload, err := json.Marshal(taskPayloadMessage{Prompt: "isolated task", SpecID: fixture.specID})
	require.NoError(t, err)

	_, err = wl.handleTask(context.Background(), "context-isolation", payload)
	require.NoError(t, err)
	require.Len(t, provider.calls, 1)
	assert.Contains(t, provider.calls[0].Prompt, "COMMITTED_WORKTREE_CONTEXT")
	assert.NotContains(t, provider.calls[0].Prompt, "DIRTY_BASE_CONTEXT_MUST_NOT_LEAK")
}

func TestHandleTask_MissingOrEmptyRequiredSpecFailsBeforeBuildCommand(t *testing.T) {
	tests := []struct {
		name     string
		document string
		empty    bool
		pipeline bool
	}{
		{name: "direct missing spec", document: "spec.md"},
		{name: "direct empty acceptance", document: "acceptance.md", empty: true},
		{name: "pipeline missing plan", document: "plan.md", pipeline: true},
		{name: "pipeline empty spec", document: "spec.md", empty: true, pipeline: true},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			fixture := writeRetainedWorkerContext(t)
			path := filepath.Join(fixture.specDir, tc.document)
			if tc.empty {
				require.NoError(t, os.WriteFile(path, []byte(" \n\t"), 0o600))
			} else {
				require.NoError(t, os.Remove(path))
			}
			provider := &mockAdapter{
				name:   "codex",
				script: `cat >/dev/null; echo '{"type":"result","output":"unexpected"}'`,
			}
			wl := &WorkerLoop{config: LoopConfig{Provider: provider, WorkDir: fixture.root}}
			message := taskPayloadMessage{Prompt: "RAW_PROMPT_CANNOT_BYPASS_FAILURE", SpecID: fixture.specID}
			if tc.pipeline {
				message.PipelinePhases = []string{"planner"}
			}
			payload, err := json.Marshal(message)
			require.NoError(t, err)

			_, err = wl.handleTask(context.Background(), "invalid-required-context", payload)
			assert.Error(t, err)
			assert.Empty(t, provider.calls, "BuildCommand must not run with incomplete required context")
		})
	}
}

func TestHandleTask_OversizedRequiredContextBlocksInsteadOfTruncating(t *testing.T) {
	fixture := writeRetainedWorkerContext(t)
	oversized := strings.Repeat("required-context-must-not-be-truncated\n", maxRetainedRequiredContextTokens*4/20)
	oversized = retainedWorkerSpecDocument(fixture.specID, oversized)
	require.NoError(t, os.WriteFile(filepath.Join(fixture.specDir, "spec.md"), []byte(oversized), 0o600))
	provider := &mockAdapter{name: "codex", script: `echo unexpected`}
	wl := &WorkerLoop{config: LoopConfig{Provider: provider, WorkDir: fixture.root}}
	payload, err := json.Marshal(taskPayloadMessage{Prompt: "task", SpecID: fixture.specID})
	require.NoError(t, err)

	_, err = wl.handleTask(context.Background(), "oversized-required-context", payload)
	require.Error(t, err)
	assert.ErrorContains(t, err, "split the task instead of truncating")
	assert.Empty(t, provider.calls)
}

type retainedWorkerContextFixture struct {
	root      string
	specID    string
	specDir   string
	documents map[string]string
}

func writeRetainedWorkerContext(t *testing.T) retainedWorkerContextFixture {
	t.Helper()
	root := t.TempDir()
	specID := "SPEC-RETAINED-WORKER-CONTEXT-001"
	specDir := filepath.Join(root, ".autopus", "specs", specID)
	documents := map[string]string{
		"spec.md": retainedWorkerSpecDocument(
			specID, retainedWorkerDocument("spec", "SPEC_REQUIRED_TAIL_MARKER"),
		),
		"plan.md":       retainedWorkerDocument("plan", "PLAN_REQUIRED_TAIL_MARKER"),
		"acceptance.md": retainedWorkerDocument("acceptance", "ACCEPTANCE_REQUIRED_TAIL_MARKER"),
	}
	allFiles := map[string]string{
		"AGENTS.md":                                   "retained worker policy",
		".autopus/project/workspace.md":               "retained worker workspace",
		".autopus/specs/" + specID + "/spec.md":       documents["spec.md"],
		".autopus/specs/" + specID + "/plan.md":       documents["plan.md"],
		".autopus/specs/" + specID + "/acceptance.md": documents["acceptance.md"],
	}
	for rel, body := range allFiles {
		path := filepath.Join(root, filepath.FromSlash(rel))
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
		require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	}
	return retainedWorkerContextFixture{root: root, specID: specID, specDir: specDir, documents: documents}
}

func retainedWorkerDocument(label, tail string) string {
	return label + " required head\n" + strings.Repeat(label+" required body\n", 5000) + tail
}

func retainedWorkerSpecDocument(id, body string) string {
	return "# " + id + ": Retained Worker Context\n\n---\nid: " + id + "\n---\n\n" + body
}

func assertCompleteRequiredSpecSnapshot(t *testing.T, prompt string, documents map[string]string) {
	t.Helper()
	for name, body := range documents {
		assert.True(t, strings.Contains(prompt, body), "BuildCommand prompt is missing complete %s", name)
	}
}
