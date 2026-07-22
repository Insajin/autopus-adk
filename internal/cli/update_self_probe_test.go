package cli

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/processprobe"
)

func TestVerifyAndReplaceSelfUpdate_ProbeFailuresPreserveInstalledBinary(t *testing.T) {
	failures := []struct {
		name    string
		version string
		err     error
	}{
		{name: "timeout", err: context.DeadlineExceeded},
		{name: "wrong version", version: "0.50.86"},
		{name: "nonzero exit", err: errors.New("exit status 17")},
		{name: "signal", err: errors.New("signal: killed")},
		{name: "inherited pipe", err: exec.ErrWaitDelay},
	}

	for _, failure := range failures {
		t.Run(failure.name, func(t *testing.T) {
			targetPath := filepath.Join(t.TempDir(), "auto")
			require.NoError(t, os.WriteFile(targetPath, []byte("installed-version"), 0o755))
			replaceCalled := false

			err := verifyAndReplaceSelfUpdateWith(
				"/staging/auto",
				targetPath,
				"0.50.85",
				func(string) (string, error) { return failure.version, failure.err },
				func(string, string) error {
					replaceCalled = true
					return nil
				},
			)

			require.Error(t, err)
			assert.False(t, replaceCalled)
			installed, readErr := os.ReadFile(targetPath)
			require.NoError(t, readErr)
			assert.Equal(t, "installed-version", string(installed))
		})
	}
}

func TestVerifyAndReplaceSelfUpdate_ExactVersionReplacesBinary(t *testing.T) {
	var calls []string

	err := verifyAndReplaceSelfUpdateWith(
		"/staging/auto",
		"/installed/auto",
		"0.50.85",
		func(path string) (string, error) {
			calls = append(calls, "probe:"+path)
			return "0.50.85", nil
		},
		func(source, target string) error {
			calls = append(calls, "replace:"+source+":"+target)
			return nil
		},
	)

	require.NoError(t, err)
	assert.Equal(t, []string{
		"probe:/staging/auto",
		"replace:/staging/auto:/installed/auto",
	}, calls)
}

func TestVerifyAndReplaceSelfUpdate_OversizedOutputPreservesInstalledBinary(t *testing.T) {
	fixture := buildSelfUpdateProbeFixture(t)
	t.Setenv(selfUpdateProbeModeEnv, "oversized")
	targetPath := filepath.Join(t.TempDir(), "auto")
	require.NoError(t, os.WriteFile(targetPath, []byte("installed-version"), 0o755))
	replaceCalled := false

	err := verifyAndReplaceSelfUpdateWith(
		fixture,
		targetPath,
		"0.50.85",
		func(path string) (string, error) {
			return probeStagedSelfUpdateVersionWithTimeout(path, 10*time.Second)
		},
		func(string, string) error {
			replaceCalled = true
			return nil
		},
	)

	assert.ErrorIs(t, err, processprobe.ErrOutputLimit)
	assert.False(t, replaceCalled)
	installed, readErr := os.ReadFile(targetPath)
	require.NoError(t, readErr)
	assert.Equal(t, "installed-version", string(installed))
}

func TestProbeStagedSelfUpdateVersion_BoundsExecutionFailures(t *testing.T) {
	fixture := buildSelfUpdateProbeFixture(t)
	tests := []struct {
		name      string
		mode      string
		want      string
		wantError error
	}{
		{name: "success", mode: "success", want: "0.50.85"},
		{name: "nonzero", mode: "nonzero", wantError: errAnySelfUpdateProbe},
		{name: "signal", mode: "signal", wantError: errAnySelfUpdateProbe},
		{name: "inherited pipe", mode: "inherited-pipe", wantError: exec.ErrWaitDelay},
		{name: "oversized output", mode: "oversized", wantError: errAnySelfUpdateProbe},
		{name: "timeout", mode: "timeout", wantError: context.DeadlineExceeded},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv(selfUpdateProbeModeEnv, test.mode)
			timeout := 10 * time.Second
			if test.mode == "timeout" {
				timeout = 200 * time.Millisecond
			}
			started := time.Now()

			got, err := probeStagedSelfUpdateVersionWithTimeout(fixture, timeout)

			assert.Less(t, time.Since(started), timeout+2*time.Second)
			if test.wantError != nil {
				require.Error(t, err)
				if !errors.Is(test.wantError, errAnySelfUpdateProbe) {
					assert.ErrorIs(t, err, test.wantError)
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, test.want, got)
		})
	}
}

var errAnySelfUpdateProbe = errors.New("any self-update probe error")

const selfUpdateProbeModeEnv = "AUTOPUS_SELF_UPDATE_PROBE_MODE"

func buildSelfUpdateProbeFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "main.go")
	binaryPath := filepath.Join(dir, "probe")
	if runtime.GOOS == "windows" {
		binaryPath += ".exe"
	}
	require.NoError(t, os.WriteFile(sourcePath, []byte(selfUpdateProbeFixtureSource), 0o600))
	build := exec.Command("go", "build", "-o", binaryPath, sourcePath)
	output, err := build.CombinedOutput()
	require.NoError(t, err, string(output))
	return binaryPath
}

const selfUpdateProbeFixtureSource = `package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

const modeEnv = "AUTOPUS_SELF_UPDATE_PROBE_MODE"

func main() {
	if len(os.Args) != 3 || os.Args[1] != "version" || os.Args[2] != "--short" {
		os.Exit(90)
	}
	switch os.Getenv(modeEnv) {
	case "success":
		fmt.Println("0.50.85")
	case "nonzero":
		os.Exit(17)
	case "signal":
		process, _ := os.FindProcess(os.Getpid())
		_ = process.Kill()
		time.Sleep(30 * time.Second)
	case "inherited-pipe":
		executable, err := os.Executable()
		if err != nil {
			os.Exit(91)
		}
		child := exec.Command(executable, "version", "--short")
		child.Env = withMode("pipe-child")
		child.Stdout = os.Stdout
		child.Stderr = os.Stderr
		if err := child.Start(); err != nil {
			os.Exit(92)
		}
		fmt.Println("0.50.85")
	case "oversized":
		fmt.Print(strings.Repeat("x", 4097))
	case "pipe-child", "timeout":
		time.Sleep(30 * time.Second)
	default:
		os.Exit(93)
	}
}

func withMode(mode string) []string {
	environment := make([]string, 0, len(os.Environ())+1)
	for _, item := range os.Environ() {
		if !strings.HasPrefix(item, modeEnv+"=") {
			environment = append(environment, item)
		}
	}
	return append(environment, modeEnv+"="+mode)
}
`

func TestNormalizeSelfUpdateVersionOutput_RequiresSingleExactVersion(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		want    string
		wantErr bool
	}{
		{name: "line feed", output: "0.50.85\n", want: "0.50.85"},
		{name: "windows line ending", output: "0.50.85\r\n", want: "0.50.85"},
		{name: "no line ending", output: "0.50.85", want: "0.50.85"},
		{name: "extra line", output: "0.50.85\nmalicious\n", wantErr: true},
		{name: "surrounding spaces", output: " 0.50.85 \n", wantErr: true},
		{name: "empty", output: "\n", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := normalizeSelfUpdateVersionOutput([]byte(test.output))
			if test.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, test.want, got)
		})
	}
}

func TestProbeStagedSelfUpdateVersion_UsesLiteralBinaryPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows executable names cannot contain shell metacharacters used by this fixture")
	}
	fixture := buildSelfUpdateProbeFixture(t)
	literalPath := filepath.Join(filepath.Dir(fixture), "auto; touch injected")
	require.NoError(t, os.Rename(fixture, literalPath))
	t.Setenv(selfUpdateProbeModeEnv, "success")

	version, err := probeStagedSelfUpdateVersion(literalPath)

	require.NoError(t, err)
	assert.Equal(t, "0.50.85", version)
	_, statErr := os.Stat(filepath.Join(filepath.Dir(fixture), "injected"))
	assert.ErrorIs(t, statErr, os.ErrNotExist)
}
