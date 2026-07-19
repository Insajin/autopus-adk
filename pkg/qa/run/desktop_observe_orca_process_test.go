package run

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/qa/desktopobserve"
)

func TestOrcaProductionResolver_ExplicitSelectionInvokesInstalledCLIOnly(t *testing.T) {
	directory := t.TempDir()
	callPath := filepath.Join(directory, "calls")
	executable := filepath.Join(directory, orcaExecutableName)
	require.NoError(t, os.WriteFile(executable, []byte(orcaProviderScript(t, callPath)), 0o700))
	t.Setenv("PATH", directory)

	runner := newProductionDesktopObservationRunner(Options{})
	request := desktopRunRequest(desktopobserve.RuntimeProviderOrca)
	request.Policy.AllowedNames = append(request.Policy.AllowedNames, "Disclosure")
	outcome, err := runner.Run(context.Background(), request)
	require.NoError(t, err)
	assert.Equal(t, desktopobserve.VerdictPassed, outcome.Verdict)
	calls, err := os.ReadFile(callPath)
	require.NoError(t, err)
	assert.Equal(t, strings.Join([]string{
		"computer capabilities --json",
		"computer permissions --json",
		"computer list-apps --json",
		"computer list-windows --app co.autopus.desktop --json",
		"computer get-app-state --app co.autopus.desktop --no-screenshot --json",
	}, "\n")+"\n", string(calls))
}

func TestProcessOrcaCommandExecutor_BoundsOutputTimeoutIdentityAndErrors(t *testing.T) {
	tests := []struct {
		name       string
		source     string
		mutate     bool
		timeout    time.Duration
		want       error
		maxElapsed time.Duration
	}{
		{name: "stderr", source: "#!/bin/sh\nprintf private-secret >&2\n", want: errDesktopProviderUnavailable},
		{name: "overflow", source: "#!/bin/sh\ni=0; while [ $i -lt 65537 ]; do printf x; i=$((i+1)); done\n",
			want: desktopobserve.ErrEnvelopeTooLarge},
		{name: "timeout", source: "#!/bin/sh\n/bin/sleep 5\n", timeout: 20 * time.Millisecond,
			want: context.DeadlineExceeded, maxElapsed: time.Second},
		{name: "identity drift", source: "#!/bin/sh\nprintf '{}\n'\n", mutate: true,
			want: errDesktopProviderUnavailable},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			directory := t.TempDir()
			path := filepath.Join(directory, "orca")
			require.NoError(t, os.WriteFile(path, []byte(test.source), 0o700))
			info, err := os.Stat(path)
			require.NoError(t, err)
			executor := &processOrcaCommandExecutor{executableInfo: info}
			if test.mutate {
				require.NoError(t, os.Rename(path, path+".old"))
				require.NoError(t, os.WriteFile(path, []byte(test.source), 0o700))
			}
			ctx := context.Background()
			cancel := func() {}
			if test.timeout > 0 {
				ctx, cancel = context.WithTimeout(ctx, test.timeout)
			}
			defer cancel()
			started := time.Now()
			body, err := executor.Run(ctx, path, []string{"computer", "capabilities", "--json"})
			assert.ErrorIs(t, err, test.want)
			assert.Empty(t, body)
			assert.NotContains(t, err.Error(), "private-secret")
			if test.maxElapsed > 0 {
				assert.Less(t, time.Since(started), test.maxElapsed)
			}
		})
	}
}

func TestOrcaProcessHelpers_RejectInvalidConstructionAndProjectEnvironment(t *testing.T) {
	_, err := newOrcaDesktopClient("")
	assert.ErrorIs(t, err, errDesktopProviderUnavailable)
	_, err = newOrcaDesktopClientWith("", &fakeOrcaCommandExecutor{}, &countingOrcaReader{})
	assert.ErrorIs(t, err, errDesktopProviderUnavailable)
	_, err = newOrcaDesktopClientWith("orca", nil, &countingOrcaReader{})
	assert.ErrorIs(t, err, errDesktopProviderUnavailable)
	_, err = newOrcaDesktopClientWith("orca", &fakeOrcaCommandExecutor{}, nil)
	assert.ErrorIs(t, err, errDesktopProviderUnavailable)

	t.Setenv("Q12_PRIVATE_SECRET", "do-not-project")
	environment := orcaProviderEnvironment()
	assert.NotContains(t, strings.Join(environment, "\n"), "Q12_PRIVATE_SECRET")
	assert.IsNonDecreasing(t, environment)

	output := &boundedOrcaOutput{}
	value := make([]byte, orcaMaxOutputBytes+1)
	n, err := output.Write(value)
	require.NoError(t, err)
	assert.Equal(t, len(value), n)
	assert.True(t, output.overflow)
	assert.Len(t, output.buffer.Bytes(), orcaMaxOutputBytes)
}

func TestOrcaDesktopClient_MethodOrderAliasesAndRandomFailureFailClosed(t *testing.T) {
	client, _ := newHermeticOrcaClient(t)
	_, err := client.Capabilities(context.Background())
	assert.ErrorIs(t, err, errDesktopProviderUnavailable)
	_, err = client.ListWindows(context.Background(), "other-app")
	assert.ErrorIs(t, err, desktopobserve.ErrMalformedEnvelope)
	_, err = client.GetState(context.Background(), "other-app", "other-window")
	assert.ErrorIs(t, err, desktopobserve.ErrMalformedEnvelope)

	client, _ = newHermeticOrcaClient(t)
	client.random = errorOrcaReader{}
	ctx := context.Background()
	_, err = client.Handshake(ctx)
	require.NoError(t, err)
	_, err = client.Permissions(ctx)
	require.NoError(t, err)
	_, err = client.ListApps(ctx)
	require.NoError(t, err)
	_, err = client.ListWindows(ctx, "autopus-desktop")
	require.NoError(t, err)
	projection, err := client.GetState(ctx, "autopus-desktop", "main-window")
	assert.ErrorIs(t, err, desktopobserve.ErrMalformedEnvelope)
	assert.Empty(t, projection)
}

type errorOrcaReader struct{}

func (errorOrcaReader) Read([]byte) (int, error) {
	return 0, errors.New("private random source failure")
}

func orcaProviderScript(t *testing.T, callPath string) string {
	t.Helper()
	responses := orcaTestResponses(false)
	var cases strings.Builder
	for arguments, response := range responses {
		require.NotContains(t, string(response), "'")
		fmt.Fprintf(&cases, "  '%s') printf '%%s\\n' '%s' ;;\n", arguments, response)
	}
	return fmt.Sprintf(`#!/bin/sh
printf '%%s\n' "$*" >> '%s'
case "$*" in
%s  *) exit 64 ;;
esac
`, callPath, cases.String())
}
