//go:build darwin && cgo

package run

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSecureDesktopSpawn_ExecutesOnlyBoundCodeIdentity(t *testing.T) {
	t.Parallel()
	target := filepath.Join(t.TempDir(), "candidate")
	buildSecureSpawnFixture(t, target, secureSpawnFixtureSource)
	spec := secureSpawnFixtureSpec(t, target, []string{"echo"})

	result, err := runSecureDesktopCommand(context.Background(), spec, nil)
	require.NoError(t, err)
	assert.Equal(t, 0, result.exitCode)
	assert.Equal(t, "bound\n", string(result.stdout))
	assert.Empty(t, result.stderr)
}

func TestSecureDesktopSpawn_SubstitutedSentinelNeverRuns(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	target := filepath.Join(directory, "candidate")
	marker := filepath.Join(directory, "sentinel-executed")
	buildSecureSpawnFixture(t, target, secureSpawnFixtureSource)
	spec := secureSpawnFixtureSpec(t, target, []string{marker})

	require.NoError(t, os.Remove(target))
	buildSecureSpawnFixture(t, target, secureSpawnSentinelSource)
	_, err := runSecureDesktopCommand(context.Background(), spec, nil)
	assert.ErrorIs(t, err, errDesktopProviderUnavailable)
	assert.NoFileExists(t, marker)
}

func TestSecureDesktopSpawn_HardlinkedExecutableFailsIdentityAdmission(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	target := filepath.Join(directory, "candidate")
	buildSecureSpawnFixture(t, target, secureSpawnFixtureSource)
	require.NoError(t, os.Link(target, filepath.Join(directory, "candidate-alias")))
	info, _, _, err := snapshotDesktopExecutable(context.Background(), target, -1)
	require.NoError(t, err)
	_, err = desktopExecutableFileIdentity(info)
	assert.ErrorIs(t, err, errDesktopProviderUnavailable)
}

func TestSecureDesktopSpawn_PreservesStderrExitAndTimeoutCleanup(t *testing.T) {
	t.Parallel()
	shell := filepath.Join(t.TempDir(), "helper")
	buildSecureSpawnFixture(t, shell, secureSpawnFixtureSource)
	spec := secureSpawnFixtureSpec(t, shell, []string{"stderr"})
	result, err := runSecureDesktopCommand(context.Background(), spec, nil)
	require.NoError(t, err)
	assert.Equal(t, 7, result.exitCode)
	assert.Equal(t, "bounded-error", string(result.stderr))

	hanging := secureSpawnFixtureSpec(t, shell, []string{"hang"})
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err = runSecureDesktopCommand(ctx, hanging, nil)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Less(t, time.Since(started), 2*time.Second)
}

func TestSecureDesktopSpawn_RejectsBoundedOutputOverflow(t *testing.T) {
	t.Parallel()
	shell := filepath.Join(t.TempDir(), "helper")
	buildSecureSpawnFixture(t, shell, secureSpawnFixtureSource)
	spec := secureSpawnFixtureSpec(t, shell, []string{"overflow"})
	_, err := runSecureDesktopCommand(context.Background(), spec, nil)
	assert.Error(t, err)
}

func secureSpawnFixtureSpec(t *testing.T, command string, arguments []string) secureDesktopSpawnSpec {
	t.Helper()
	info, _, codeIdentity, err := snapshotDesktopExecutable(context.Background(), command, -1)
	require.NoError(t, err)
	fileIdentity, err := desktopExecutableFileIdentity(info)
	require.NoError(t, err)
	return secureDesktopSpawnSpec{command: command, arguments: arguments,
		environment: os.Environ(), codeIdentity: codeIdentity, fileIdentity: fileIdentity}
}

func buildSecureSpawnFixture(t *testing.T, target, source string) {
	t.Helper()
	sourcePath := filepath.Join(filepath.Dir(target), filepath.Base(target)+".go")
	require.NoError(t, os.WriteFile(sourcePath, []byte(source), 0o600))
	command := exec.Command("go", "build", "-o", target, sourcePath)
	output, err := command.CombinedOutput()
	require.NoError(t, err, string(output))
}

const secureSpawnFixtureSource = `package main
import ("bytes"; "os"; "os/exec"; "time")
func main() {
  if len(os.Args) != 2 { os.Exit(64) }
  switch os.Args[1] {
  case "echo": os.Stdout.Write([]byte("bound\n"))
  case "stderr": os.Stderr.Write([]byte("bounded-error")); os.Exit(7)
  case "overflow": os.Stdout.Write(bytes.Repeat([]byte("x"), 70000))
  case "hang": command := exec.Command(os.Args[0], "child"); _ = command.Run()
  case "child": time.Sleep(10 * time.Second)
  default: os.Exit(65)
  }
}
`

const secureSpawnSentinelSource = `package main
import "os"
func main() { if len(os.Args) == 2 { _ = os.WriteFile(os.Args[1], []byte("executed"), 0600) } }
`
