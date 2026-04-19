package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/platform"
	"github.com/insajin/autopus-adk/pkg/terminal"
)

type stubTerminal struct {
	name string
}

func (s stubTerminal) Name() string { return s.name }
func (s stubTerminal) CreateWorkspace(context.Context, string) error {
	return nil
}
func (s stubTerminal) SplitPane(context.Context, terminal.Direction) (terminal.PaneID, error) {
	return "", nil
}
func (s stubTerminal) SendCommand(context.Context, terminal.PaneID, string) error {
	return nil
}
func (s stubTerminal) SendLongText(context.Context, terminal.PaneID, string) error {
	return nil
}
func (s stubTerminal) Notify(context.Context, string) error { return nil }
func (s stubTerminal) ReadScreen(context.Context, terminal.PaneID, terminal.ReadScreenOpts) (string, error) {
	return "", nil
}
func (s stubTerminal) PipePaneStart(context.Context, terminal.PaneID, string) error {
	return nil
}
func (s stubTerminal) PipePaneStop(context.Context, terminal.PaneID) error {
	return nil
}
func (s stubTerminal) Close(context.Context, string) error { return nil }

func TestResolveCC21MonitorRuntime_Enabled(t *testing.T) {
	projectDir := t.TempDir()
	homeDir := t.TempDir()
	binDir := filepath.Join(t.TempDir(), "bin")
	require.NoError(t, os.MkdirAll(binDir, 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(homeDir, ".claude"), 0o755))

	cfg := config.DefaultFullConfig("test")
	cfg.Features.CC21 = config.CC21FeaturesConf{
		Enabled:                 true,
		MonitorEnabled:          true,
		MonitorPatternTimeoutMS: 1234,
	}
	require.NoError(t, config.Save(projectDir, cfg))

	// Touch the fake `claude` binary for exec.LookPath; the actual subprocess
	// call is bypassed via SetClaudeVersionForTest below to avoid flakes
	// under `go test -race ./...` CPU contention.
	claudePath := filepath.Join(binDir, "claude")
	require.NoError(t, os.WriteFile(claudePath, []byte("#!/bin/sh\necho 2.1.113\n"), 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(homeDir, ".claude", "settings.json"),
		[]byte(`{"hooks":{"Stop":[{"command":"autopus"}]}}`),
		0o644,
	))

	t.Setenv("AUTOPUS_PLATFORM", "claude-code")
	t.Setenv("HOME", homeDir)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Chdir(projectDir)

	t.Cleanup(platform.SetClaudeVersionForTest(func() (string, error) {
		return "2.1.113", nil
	}))

	harnessCfg, err := loadHarnessConfig()
	require.NoError(t, err)

	runtime := resolveCC21MonitorRuntime(stubTerminal{name: "cmux"}, harnessCfg)
	assert.True(t, runtime.Enabled)
	assert.True(t, runtime.HookMode)
	assert.Equal(t, 1234*time.Millisecond, runtime.PatternTimeout)
}

func TestResolveCC21MonitorRuntime_DisabledOutsideClaude(t *testing.T) {
	projectDir := t.TempDir()
	cfg := config.DefaultFullConfig("test")
	cfg.Features.CC21 = config.CC21FeaturesConf{
		Enabled:                 true,
		MonitorEnabled:          true,
		MonitorPatternTimeoutMS: 1234,
	}
	require.NoError(t, config.Save(projectDir, cfg))

	t.Setenv("AUTOPUS_PLATFORM", "codex")
	t.Chdir(projectDir)

	harnessCfg, err := loadHarnessConfig()
	require.NoError(t, err)

	runtime := resolveCC21MonitorRuntime(stubTerminal{name: "cmux"}, harnessCfg)
	assert.False(t, runtime.Enabled)
	assert.False(t, runtime.HookMode)
}
