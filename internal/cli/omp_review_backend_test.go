package cli

import (
	"context"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/orchestra"
)

func TestOMPReviewBackend_ReadOnlyRPCSessionAndModelSelection(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture executable uses a POSIX launcher")
	}
	projectDir, logPath := configureOMPReviewRPCFixture(t, "success")
	backend := newOMPReviewBackend(projectDir)

	response, err := backend.Execute(context.Background(), orchestra.ProviderRequest{
		Provider: "codex", Role: "reviewer", Prompt: "review this SPEC", Timeout: 5 * time.Second,
		Config: orchestra.ProviderConfig{
			Name: "codex", Backend: config.ProviderBackendOMP, Binary: "must-not-run-provider",
			Model: "openai-codex/gpt-6-astra:max", Tools: []string{"read", "grep"},
		},
	})

	require.NoError(t, err)
	require.NotNil(t, response)
	assert.Equal(t, "review output", response.Output)
	assert.Equal(t, "omp", response.ExecutedBackend)
	assert.Equal(t, "openai", response.ModelFamily)
	assert.Equal(t, "reviewer", response.Role)
	assert.Positive(t, response.Duration)

	start, overlay, commands := splitOMPReviewRPCRecords(t, readOMPReviewRPCRecords(t, logPath))
	args := ompReviewProcessArgs(t, start.Args)
	sessionDir := flagValue(t, args, "--session-dir")
	overlayPath := flagValue(t, args, "--config")
	assert.True(t, filepath.IsAbs(overlayPath))
	assert.Equal(t, overlayPath, overlay.Path)
	assert.Equal(t, ompReviewHardeningOverlayYAML, overlay.Content)
	assert.Equal(t, uint32(0o600), overlay.Mode)
	assert.Equal(t, []string{
		"--mode", "rpc", "--no-session", "--no-extensions", "--session-dir", sessionDir,
		"--no-skills", "--no-lsp", "--config", overlayPath,
		"--tools", "grep,read", "--approval-mode", "yolo", "--max-time", "5",
	}, args)
	wantCWD, err := filepath.EvalSymlinks(projectDir)
	require.NoError(t, err)
	gotCWD, err := filepath.EvalSymlinks(start.CWD)
	require.NoError(t, err)
	assert.Equal(t, wantCWD, gotCWD)
	assert.Equal(t, filepath.Dir(filepath.Dir(sessionDir)), filepath.Dir(overlayPath))
	assert.NoFileExists(t, overlayPath)
	assert.NoDirExists(t, filepath.Dir(overlayPath))
	assert.Equal(t, []string{
		"negotiate_protocol", "set_auto_retry", "set_auto_compaction", "get_state", "set_model",
		"set_thinking_level", "get_state", "prompt", "get_state", "get_last_assistant_text",
	}, ompReviewCommandTypes(commands), "the tool-set verification get_state precedes set_model")
	assert.Equal(t, "openai-codex", commands[4].Provider)
	assert.Equal(t, "gpt-6-astra", commands[4].ModelID)
	assert.Equal(t, "max", commands[5].Level)
	assert.NotContains(t, flagValue(t, args, "--tools"), "write")
	assert.NotContains(t, flagValue(t, args, "--tools"), "edit")
	assert.NotContains(t, flagValue(t, args, "--tools"), "bash")
}

func TestOMPReviewBackend_OmitsThinkingAndDefaultsToolsAndTimeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture executable uses a POSIX launcher")
	}
	projectDir, logPath := configureOMPReviewRPCFixture(t, "success")
	response, err := newOMPReviewBackend(projectDir).Execute(context.Background(), orchestra.ProviderRequest{
		Provider: "claude", Prompt: "review", Config: orchestra.ProviderConfig{
			Backend: config.ProviderBackendOMP, Model: "anthropic/claude-fable-5-1",
		},
	})

	require.NoError(t, err)
	require.NotNil(t, response)
	assert.Equal(t, "anthropic", response.ModelFamily)
	start, _, commands := splitOMPReviewRPCRecords(t, readOMPReviewRPCRecords(t, logPath))
	args := ompReviewProcessArgs(t, start.Args)
	assert.Equal(t, "glob,grep,read", flagValue(t, args, "--tools"))
	assert.Equal(t, "1800", flagValue(t, args, "--max-time"))
	assert.NotContains(t, ompReviewCommandTypes(commands), "set_thinking_level")
}

func TestOMPReviewBackend_ClassifiesFailures(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture executable uses a POSIX launcher")
	}
	t.Run("timeout", func(t *testing.T) {
		projectDir, logPath := configureOMPReviewRPCFixture(t, "hang-prompt")
		response, err := executeOMPReviewFixture(t, projectDir, 2*time.Second)
		require.NoError(t, err)
		require.NotNil(t, response)
		assert.True(t, response.TimedOut)
		assertOMPReviewFixtureRuntimeRemoved(t, logPath)
	})
	t.Run("RPC error", func(t *testing.T) {
		projectDir, logPath := configureOMPReviewRPCFixture(t, "set-model-error")
		response, err := executeOMPReviewFixture(t, projectDir, time.Second)
		require.Error(t, err)
		require.NotNil(t, response)
		assert.Equal(t, 1, response.ExitCode)
		assert.Contains(t, response.Error, "model unavailable")
		assertOMPReviewFixtureRuntimeRemoved(t, logPath)
	})
	t.Run("empty output", func(t *testing.T) {
		projectDir, logPath := configureOMPReviewRPCFixture(t, "empty")
		response, err := executeOMPReviewFixture(t, projectDir, time.Second)
		require.NoError(t, err)
		require.NotNil(t, response)
		assert.True(t, response.EmptyOutput)
		assertOMPReviewFixtureRuntimeRemoved(t, logPath)
	})
}

func TestOMPReviewBackend_RequiresModelBeforeSpawn(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture executable uses a POSIX launcher")
	}
	projectDir, logPath := configureOMPReviewRPCFixture(t, "success")
	response, err := newOMPReviewBackend(projectDir).Execute(context.Background(), orchestra.ProviderRequest{
		Provider: "claude", Config: orchestra.ProviderConfig{Backend: config.ProviderBackendOMP},
	})
	require.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "model")
	assert.NoFileExists(t, logPath)
}

func TestSelectRoutedBackend_OnlyAddsOMPRouteWhenConfigured(t *testing.T) {
	t.Parallel()
	baseConfig := orchestra.OrchestraConfig{}
	ompBackend, ok := newOMPReviewBackend(".").(orchestra.FreshExecutionBackend)
	require.True(t, ok)
	assert.True(t, ompBackend.FreshExecutionPerRequest())

	assert.Equal(t, orchestra.SelectBackend(baseConfig).Name(), selectRoutedBackend(baseConfig).Name())

	providerConfig := baseConfig
	providerConfig.Providers = []orchestra.ProviderConfig{{Name: "reviewer", Backend: config.ProviderBackendOMP}}
	routed := selectRoutedBackend(providerConfig)
	assert.Equal(t, "subprocess+omp", routed.Name())

	judge := orchestra.ProviderConfig{Name: "judge", Backend: config.ProviderBackendOMP}
	judgeConfig := baseConfig
	judgeConfig.JudgeConfig = &judge
	assert.Equal(t, "subprocess+omp", selectRoutedBackend(judgeConfig).Name())
}

func TestOMPBackendFactoriesUseRoutedSelection(t *testing.T) {
	cfg := orchestra.OrchestraConfig{
		Providers: []orchestra.ProviderConfig{{Name: "reviewer", Backend: config.ProviderBackendOMP}},
	}

	assert.Equal(t, "subprocess+omp", specReviewBackendFactory(cfg).Name())
	assert.Equal(t, "subprocess+omp", orchestraRunBackendFactory(cfg).Name())
}

func executeOMPReviewFixture(t *testing.T, projectDir string, timeout time.Duration) (*orchestra.ProviderResponse, error) {
	t.Helper()
	return newOMPReviewBackend(projectDir).Execute(context.Background(), orchestra.ProviderRequest{
		Provider: "reviewer", Role: "reviewer", Prompt: "review",
		Timeout: timeout, Config: orchestra.ProviderConfig{
			Backend: config.ProviderBackendOMP, Model: "openai/model:high", Tools: []string{"read"},
		},
	})
}

func splitOMPReviewRPCRecords(
	t *testing.T,
	records []ompReviewRPCRecord,
) (ompReviewRPCRecord, ompReviewRPCRecord, []ompReviewRPCRecord) {
	t.Helper()
	require.GreaterOrEqual(t, len(records), 2)
	require.Equal(t, "start", records[0].Kind)
	require.Equal(t, "overlay", records[1].Kind)
	return records[0], records[1], records[2:]
}

func ompReviewProcessArgs(t *testing.T, args []string) []string {
	t.Helper()
	for index, arg := range args {
		if arg == "--mode" {
			return args[index:]
		}
	}
	t.Fatalf("OMP argv has no --mode flag: %v", args)
	return nil
}

func flagValue(t *testing.T, args []string, flag string) string {
	t.Helper()
	for index, arg := range args {
		if arg == flag {
			require.Less(t, index+1, len(args), "%s has no value", flag)
			return args[index+1]
		}
	}
	t.Fatalf("flag %s is absent from %s", flag, strings.Join(args, " "))
	return ""
}

func assertOMPReviewFixtureRuntimeRemoved(t *testing.T, logPath string) {
	t.Helper()
	start, overlay, _ := splitOMPReviewRPCRecords(t, readOMPReviewRPCRecords(t, logPath))
	sessionDir := flagValue(t, ompReviewProcessArgs(t, start.Args), "--session-dir")
	assert.Equal(t, filepath.Dir(filepath.Dir(sessionDir)), filepath.Dir(overlay.Path))
	assert.NoFileExists(t, overlay.Path)
	assert.NoDirExists(t, filepath.Dir(overlay.Path))
}

func ompReviewCommandTypes(commands []ompReviewRPCRecord) []string {
	types := make([]string, len(commands))
	for index, command := range commands {
		types[index] = command.Type
	}
	return types
}

// omp 18.1.x acknowledges a prompt with a bare success frame and no
// prompt_result; the lifecycle frames alone must settle the turn.
func TestOMPReviewBackend_AcceptsBarePromptAcknowledgement(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture executable uses a POSIX launcher")
	}
	projectDir, logPath := configureOMPReviewRPCFixture(t, "bare-ack")
	response, err := executeOMPReviewFixture(t, projectDir, 5*time.Second)
	require.NoError(t, err)
	require.NotNil(t, response)
	assert.Equal(t, "review output", response.Output)
	assert.False(t, response.TimedOut)
	assert.Equal(t, 0, response.ExitCode)
	assertOMPReviewFixtureRuntimeRemoved(t, logPath)
}
