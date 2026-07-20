package orchestra

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// echoProvider는 테스트용 echo 커맨드 프로바이더를 생성한다.
func echoProvider(name string) ProviderConfig {
	if runtime.GOOS == "windows" {
		return ProviderConfig{Name: name, Binary: "cmd", Args: []string{"/c", "echo hello"}}
	}
	return ProviderConfig{Name: name, Binary: "cat", Args: []string{}}
}

// sleepProvider는 타임아웃 테스트용 sleep 프로바이더를 생성한다.
func sleepProvider(name string) ProviderConfig {
	if runtime.GOOS == "windows" {
		return ProviderConfig{Name: name, Binary: "timeout", Args: []string{"/t", "10"}}
	}
	return ProviderConfig{Name: name, Binary: "sleep", Args: []string{"10"}}
}

func TestRunOrchestra_EmptyProviders(t *testing.T) {
	t.Parallel()
	cfg := OrchestraConfig{
		Strategy: StrategyConsensus,
		Prompt:   "test",
	}
	_, err := RunOrchestra(context.Background(), cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "providers")
}

func TestRunOrchestra_InvalidStrategy(t *testing.T) {
	t.Parallel()
	cfg := OrchestraConfig{
		Providers: []ProviderConfig{echoProvider("p1")},
		Strategy:  Strategy("invalid"),
		Prompt:    "test",
	}
	_, err := RunOrchestra(context.Background(), cfg)
	assert.Error(t, err)
}

func TestRunOrchestra_MissingBinary(t *testing.T) {
	t.Parallel()
	cfg := OrchestraConfig{
		Providers: []ProviderConfig{
			{Name: "nonexistent", Binary: "binary_that_does_not_exist_xyz", Args: []string{}},
		},
		Strategy: StrategyConsensus,
		Prompt:   "test",
	}
	_, err := RunOrchestra(context.Background(), cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "binary_that_does_not_exist_xyz")
}

func TestRunOrchestra_Consensus_WithCat(t *testing.T) {
	t.Parallel()
	// cat 명령어는 stdin을 그대로 출력한다
	cfg := OrchestraConfig{
		Providers: []ProviderConfig{
			echoProvider("p1"),
			echoProvider("p2"),
		},
		Strategy:       StrategyConsensus,
		Prompt:         "hello world",
		TimeoutSeconds: 10,
	}
	result, err := RunOrchestra(context.Background(), cfg)
	require.NoError(t, err)
	assert.Equal(t, StrategyConsensus, result.Strategy)
	assert.Len(t, result.Responses, 2)
	assert.NotEmpty(t, result.Summary)
}

func TestRunOrchestra_ConsensusUsesConfiguredThreshold(t *testing.T) {
	// This test replaces the package command factory, so it must not run in
	// parallel with other tests that use the same seam.
	originalNewCommand := newCommand
	t.Cleanup(func() { newCommand = originalNewCommand })
	outputs := map[string]string{
		"p1": "1. shared\n2. pair\n3. only-one\n",
		"p2": "1. shared\n2. pair\n",
		"p3": "1. shared\n",
	}
	newCommand = func(_ context.Context, _ string, args ...string) command {
		waitCh := make(chan error, 1)
		waitCh <- nil
		return &fakeCommand{
			waitCh: waitCh,
			startFn: func(cmd *fakeCommand) error {
				_, err := io.WriteString(cmd.stdout, outputs[args[0]])
				return err
			},
		}
	}
	binary, err := os.Executable()
	require.NoError(t, err)

	cfg := OrchestraConfig{
		Providers: []ProviderConfig{
			{Name: "p1", Binary: binary, Args: []string{"p1"}},
			{Name: "p2", Binary: binary, Args: []string{"p2"}},
			{Name: "p3", Binary: binary, Args: []string{"p3"}},
		},
		Strategy:           StrategyConsensus,
		Prompt:             "ignored by fixture",
		TimeoutSeconds:     10,
		ConsensusThreshold: 1.0,
	}

	result, err := RunOrchestra(context.Background(), cfg)
	require.NoError(t, err)
	assert.Contains(t, result.Merged, "✓ 1. shared")
	assert.Contains(t, result.Merged, "## 이견이 있는 내용")
	assert.Contains(t, result.Merged, "△ 2. pair [2/3]",
		"2/3 agreement must remain visible as dissent at threshold=1.0")
	assert.Contains(t, result.Merged, "△ 3. only-one [1/3]",
		"explicit thresholds must not delete minority evidence")
}

func TestRunOrchestra_Pipeline_WithCat(t *testing.T) {
	t.Parallel()
	cfg := OrchestraConfig{
		Providers: []ProviderConfig{
			echoProvider("stage1"),
			echoProvider("stage2"),
			echoProvider("stage3"),
		},
		Strategy:       StrategyPipeline,
		Prompt:         "pipeline input",
		TimeoutSeconds: 10,
	}
	result, err := RunOrchestra(context.Background(), cfg)
	require.NoError(t, err)
	assert.Equal(t, StrategyPipeline, result.Strategy)
	assert.Len(t, result.Responses, 3)
	assert.Contains(t, result.Summary, "파이프라인")
	assert.Contains(t, result.Summary, "3단계")
}

func TestRunOrchestra_Fastest_WithCat(t *testing.T) {
	t.Parallel()
	cfg := OrchestraConfig{
		Providers: []ProviderConfig{
			echoProvider("fast1"),
			echoProvider("fast2"),
		},
		Strategy:       StrategyFastest,
		Prompt:         "fastest test",
		TimeoutSeconds: 10,
	}
	result, err := RunOrchestra(context.Background(), cfg)
	require.NoError(t, err)
	assert.Equal(t, StrategyFastest, result.Strategy)
	// fastest는 첫 번째 응답만 반환
	assert.Len(t, result.Responses, 1)
	assert.Equal(t, 2, result.DispatchCount)
	assert.ElementsMatch(t, []string{"fast1", "fast2"}, result.AttemptedProviders)
	assert.Contains(t, result.Summary, "최속 응답")
}

func TestRunOrchestra_Timeout(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("Windows에서는 sleep 타임아웃 테스트를 건너뜁니다")
	}

	cfg := OrchestraConfig{
		Providers: []ProviderConfig{
			sleepProvider("slow"),
		},
		Strategy:       StrategyFastest,
		Prompt:         "timeout test",
		TimeoutSeconds: 1, // 1초 타임아웃
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := RunOrchestra(ctx, cfg)
	// 타임아웃이나 오류가 발생해야 한다
	assert.Error(t, err)
}

func TestRunOrchestra_Debate_WithCat(t *testing.T) {
	t.Parallel()
	judge := typedJudgeProvider(t, "judge")
	cfg := OrchestraConfig{
		Providers: []ProviderConfig{
			echoProvider("debater1"),
			echoProvider("debater2"),
		},
		Strategy:       StrategyDebate,
		Prompt:         "debate topic",
		TimeoutSeconds: 10,
		JudgeProvider:  judge.Name,
		JudgeConfig:    &judge,
	}
	result, err := RunOrchestra(context.Background(), cfg)
	require.NoError(t, err)
	assert.Equal(t, StrategyDebate, result.Strategy)
	assert.Contains(t, result.Summary, "판정")
	assert.Equal(t, JudgePassed, result.JudgeStatus)
}

func TestRunOrchestra_DefaultTimeout(t *testing.T) {
	t.Parallel()
	// TimeoutSeconds가 0이면 기본값 120이 사용된다
	cfg := OrchestraConfig{
		Providers: []ProviderConfig{
			echoProvider("p1"),
		},
		Strategy:       StrategyConsensus,
		Prompt:         "test",
		TimeoutSeconds: 0,
	}
	result, err := RunOrchestra(context.Background(), cfg)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func writeOutputProvider(t *testing.T, dir, name, output string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	script := "#!/bin/sh\ncat >/dev/null\nprintf '%s' " + shellSingleQuote(output) + "\n"
	require.NoError(t, os.WriteFile(path, []byte(script), 0o755))
	return path
}

func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
