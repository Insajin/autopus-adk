package worker

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/insajin/autopus-adk/pkg/worker/a2a"
	"github.com/insajin/autopus-adk/pkg/worker/adapter"
	"github.com/insajin/autopus-adk/pkg/worker/knowledge"
	"github.com/insajin/autopus-adk/pkg/worker/mcpserver"
	"github.com/insajin/autopus-adk/pkg/worker/parallel"
	"github.com/insajin/autopus-adk/pkg/worker/qa"
	"github.com/insajin/autopus-adk/pkg/worker/security"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBridgeFeatureRegression verifies all Bridge capabilities are covered
// by an equivalent ADK Worker package. Each subtest instantiates the key
// type and exercises its primary method.
func TestBridgeFeatureRegression(t *testing.T) {
	t.Parallel()

	features := []struct {
		name       string
		bridgeFunc string
		workerPkg  string
		verify     func(t *testing.T)
	}{
		{"WebSocket connection", "bridge connect", "a2a", verifyA2APackage},
		{"Task execution", "bridge execute", "adapter", verifyAdapterPackage},
		{"Provider API calls", "provider registry", "adapter", verifyProviderRegistry},
		{"Security allowlist", "command_allowlist", "security", verifySecurityPackage},
		{"Policy validation", "validate", "security", verifyPolicyValidation},
		{"MCP server", "mcp-server", "mcpserver", verifyMCPServer},
		{"Parallel execution", "semaphore", "parallel", verifySemaphore},
		{"Knowledge sync", "knowledge", "knowledge", verifyKnowledge},
		{"QA pipeline", "qa", "qa", verifyQAPipeline},
	}

	for _, f := range features {
		f := f
		t.Run(f.name, func(t *testing.T) {
			t.Parallel()
			f.verify(t)
		})
	}
}

// verifyA2APackage confirms the A2A server can be created and types are usable.
func verifyA2APackage(t *testing.T) {
	handler := func(_ context.Context, taskID string, _ json.RawMessage) (*a2a.TaskResult, error) {
		return &a2a.TaskResult{Status: a2a.StatusCompleted}, nil
	}
	cfg := a2a.ServerConfig{
		BackendURL: "ws://localhost:0",
		WorkerName: "test-worker",
		Skills:     []string{"code"},
		Handler:    handler,
		AuthToken:  "test-token",
	}
	srv := a2a.NewServer(cfg)
	require.NotNil(t, srv, "NewServer must return non-nil")

	// Verify core types are constructible.
	task := a2a.Task{ID: "t-1", Status: a2a.StatusWorking}
	assert.Equal(t, a2a.StatusWorking, task.Status)
	assert.Equal(t, "t-1", task.ID)
}

// verifyAdapterPackage confirms provider adapters implement the interface.
func verifyAdapterPackage(t *testing.T) {
	claude := adapter.NewClaudeAdapter()
	require.NotNil(t, claude)

	// Verify interface compliance.
	var pa adapter.ProviderAdapter = claude
	assert.Equal(t, "claude", pa.Name())

	// BuildCommand should return a valid command.
	cfg := adapter.TaskConfig{
		TaskID:  "task-1",
		Prompt:  "hello",
		WorkDir: t.TempDir(),
	}
	cmd := pa.BuildCommand(context.Background(), cfg)
	require.NotNil(t, cmd, "BuildCommand must return non-nil cmd")
}

// verifyProviderRegistry confirms the registry supports register/get/list.
func verifyProviderRegistry(t *testing.T) {
	reg := adapter.NewRegistry()
	require.NotNil(t, reg)

	reg.Register(adapter.NewClaudeAdapter())
	reg.Register(adapter.NewCodexAdapter())
	reg.Register(adapter.NewGeminiAdapter())

	names := reg.List()
	assert.Len(t, names, 3)
	assert.Contains(t, names, "claude")
	assert.Contains(t, names, "codex")
	assert.Contains(t, names, "gemini")

	got, err := reg.Get("claude")
	require.NoError(t, err)
	assert.Equal(t, "claude", got.Name())

	_, err = reg.Get("nonexistent")
	assert.Error(t, err)
}

// verifySecurityPackage confirms command validation with allowlist.
func verifySecurityPackage(t *testing.T) {
	policy := security.SecurityPolicy{
		AllowNetwork:    false,
		AllowFS:         true,
		AllowedCommands: []string{"go ", "npm "},
		DeniedPatterns:  []string{`rm\s+-rf`},
		AllowedDirs:     []string{"/tmp"},
	}

	ok, reason := policy.ValidateCommand("go test ./...", "/tmp/work")
	assert.True(t, ok, "go test should be allowed: %s", reason)

	ok, reason = policy.ValidateCommand("rm -rf /", "/tmp/work")
	assert.False(t, ok, "rm -rf should be denied")
	assert.Contains(t, reason, "denied pattern")

	ok, _ = policy.ValidateCommand("python script.py", "/tmp/work")
	assert.False(t, ok, "python should not be in allowlist")
}

// verifyPolicyValidation confirms fail-closed behavior and dir restriction.
func verifyPolicyValidation(t *testing.T) {
	// Fail-closed: empty AllowedCommands denies everything.
	empty := security.SecurityPolicy{}
	ok, reason := empty.ValidateCommand("go build", "")
	assert.False(t, ok)
	assert.Contains(t, reason, "fail-closed")

	// Directory restriction.
	policy := security.SecurityPolicy{
		AllowedCommands: []string{"go "},
		AllowedDirs:     []string{"/safe"},
	}
	ok, _ = policy.ValidateCommand("go build", "/unsafe/path")
	assert.False(t, ok, "should deny non-allowed directory")

	ok, _ = policy.ValidateCommand("go build", "/safe/sub")
	assert.True(t, ok, "should allow subdirectory of allowed dir")
}

// verifyMCPServer confirms MCP server can be instantiated with tools.
func verifyMCPServer(t *testing.T) {
	srv := mcpserver.NewMCPServer("http://localhost:0", "token", "ws-1")
	require.NotNil(t, srv, "NewMCPServer must return non-nil")
}

// verifySemaphore confirms the semaphore limits concurrency.
func verifySemaphore(t *testing.T) {
	sem := parallel.NewTaskSemaphore(3)
	require.NotNil(t, sem)

	assert.Equal(t, 3, sem.Limit())
	assert.Equal(t, 3, sem.Available())

	err := sem.Acquire(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2, sem.Available())

	sem.Release()
	assert.Equal(t, 3, sem.Available())

	// Cancelled context should return error.
	ctx, cancel := context.WithCancel(context.Background())
	// Fill all slots.
	for i := 0; i < 3; i++ {
		require.NoError(t, sem.Acquire(context.Background()))
	}
	cancel()
	err = sem.Acquire(ctx)
	assert.ErrorIs(t, err, context.Canceled)
	// Clean up.
	for i := 0; i < 3; i++ {
		sem.Release()
	}
}

// verifyKnowledge confirms knowledge syncer and searcher instantiation.
func verifyKnowledge(t *testing.T) {
	syncer := knowledge.NewSyncer("http://localhost:0", "token", "ws-1", "src-1")
	require.NotNil(t, syncer, "NewSyncer must return non-nil")

	searcher := knowledge.NewKnowledgeSearcher("http://localhost:0", "token", "ws-1")
	require.NotNil(t, searcher, "NewKnowledgeSearcher must return non-nil")

	watcher := knowledge.NewFileWatcher(t.TempDir(), 0, func(string) {}, nil)
	require.NotNil(t, watcher, "NewFileWatcher must return non-nil")
}

// verifyQAPipeline confirms pipeline stages can be created and run.
func verifyQAPipeline(t *testing.T) {
	stages := []qa.Stage{
		&qa.BuildStage{Command: "go vet ./..."},
	}
	p := qa.NewPipeline(t.TempDir(), stages)
	require.NotNil(t, p, "NewPipeline must return non-nil")

	// Run with a context — stage may fail but pipeline should not panic.
	result, _ := p.Run(context.Background())
	require.NotNil(t, result, "Pipeline.Run must return a result")
	assert.NotEmpty(t, result.Status)
}
