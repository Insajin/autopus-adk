package omp

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOMPProbeEnvironment_DropsCredentialsAndHostPIValues(t *testing.T) {
	base := []string{
		"PATH=/usr/bin:/bin",
		"LANG=en_US.UTF-8",
		"HOME=/host/home",
		"OPENAI_API_KEY=secret-openai",
		"ANTHROPIC_API_KEY=secret-anthropic",
		"AWS_SECRET_ACCESS_KEY=secret-aws",
		"PI_CONFIG_FILES=/host/pi.yml",
		"PI_PROVIDER_TOKEN=secret-pi",
	}
	overrides := []string{
		"HOME=/task/home",
		"XDG_CONFIG_HOME=/task/xdg-config",
		"PI_CODING_AGENT_DIR=/task/pi-agent",
		"PI_CONFIG_FILES=/task/config.yml",
	}

	env := environmentMapOMP(mergeOMPProbeEnvironment(base, overrides))
	assert.Equal(t, "/usr/bin:/bin", env["PATH"])
	assert.Equal(t, "en_US.UTF-8", env["LANG"])
	assert.Equal(t, "/task/home", env["HOME"])
	assert.Equal(t, "/task/xdg-config", env["XDG_CONFIG_HOME"])
	assert.Equal(t, "/task/pi-agent", env["PI_CODING_AGENT_DIR"])
	assert.Equal(t, "/task/config.yml", env["PI_CONFIG_FILES"])
	for _, key := range []string{"OPENAI_API_KEY", "ANTHROPIC_API_KEY", "AWS_SECRET_ACCESS_KEY", "PI_PROVIDER_TOKEN"} {
		assert.NotContains(t, env, key)
	}
}

func TestOMPReadiness_InvalidIdentityStopsProbesAndDropsRawVersion(t *testing.T) {
	runner := newReadinessFakeRunner(t.TempDir())
	runner.outputs["version"] = []byte("hostile-wrapper 9.1\n")

	report := ProbeOMPReadiness(context.Background(), readinessOptions(t, runner))
	assert.Empty(t, report.Version, "unverified identity output must not enter the receipt")
	assert.Len(t, runner.calls, 1, "identity failure must stop every subsequent subprocess")
	assert.False(t, capabilityByID(t, report, "identity.version").Supported)
	for _, capability := range report.Capabilities[1:] {
		assert.False(t, capability.Supported)
		assert.Equal(t, "identity_unverified", capability.Reason)
	}
	assert.Equal(t, "identity_unverified", report.CatalogReason)
}

func TestOMPReadiness_DefaultRunnerUsesCanonicalTargetAndFailsClosedOnBehavioralRPCError(t *testing.T) {
	skipWithoutPOSIXShellOMP(t)
	binDir := t.TempDir()
	good := filepath.Join(binDir, "omp-good")
	bad := filepath.Join(binDir, "omp-bad")
	link := filepath.Join(binDir, "omp")
	rpcMarker := filepath.Join(binDir, "rpc-called")
	goodScript := "#!/bin/sh\n" +
		"[ -z \"$OPENAI_API_KEY\" ] || exit 81\n" +
		"[ -z \"$PI_PROVIDER_TOKEN\" ] || exit 82\n" +
		"[ \"$HOME\" != '/host/home' ] && [ -n \"$HOME\" ] || exit 83\n" +
		"[ -n \"$XDG_CONFIG_HOME\" ] && [ -n \"$XDG_CACHE_HOME\" ] || exit 84\n" +
		"[ \"$PI_CODING_AGENT_DIR\" != '/host/pi-agent' ] && [ -n \"$PI_CODING_AGENT_DIR\" ] || exit 85\n" +
		"[ \"$PI_CONFIG_FILES\" != '/host/pi.yml' ] && [ -f \"$PI_CONFIG_FILES\" ] || exit 86\n" +
		"case \"$*\" in\n" +
		"  *--version*) /bin/rm -f '" + link + "'; /bin/ln -s '" + bad + "' '" + link + "'; printf 'omp/17.1.8\\n' ;;\n" +
		"  *--help*) printf '%s\\n' '--mode <interactive|rpc> --no-session --cwd <path> --model <provider/model>' ;;\n" +
		"  *'config get'*) printf '%s\\n' '[\".agents/skills\"]' ;;\n" +
		"  *'models --json'*) printf '%s\\n' '{\"models\":[]}' ;;\n" +
		"  *'--mode rpc'*) : > '" + rpcMarker + "'; exit 93 ;;\n" +
		"esac\n"
	require.NoError(t, os.WriteFile(good, []byte(goodScript), 0o755))
	require.NoError(t, os.WriteFile(bad, []byte("#!/bin/sh\nprintf 'not-omp\\n'\n"), 0o755))
	require.NoError(t, os.Symlink(good, link))
	t.Setenv("PATH", binDir+":/usr/bin:/bin")
	t.Setenv("HOME", "/host/home")
	t.Setenv("OPENAI_API_KEY", "must-not-cross")
	t.Setenv("PI_PROVIDER_TOKEN", "must-not-cross")
	t.Setenv("PI_CODING_AGENT_DIR", "/host/pi-agent")
	t.Setenv("PI_CONFIG_FILES", "/host/pi.yml")

	report := ProbeOMPReadiness(context.Background(), OMPReadinessOptions{
		Root: rootForOMPReadinessTest(t), Timeout: time.Second, MaxOutput: 4096,
	})
	assert.Equal(t, "omp/17.1.8", report.Version)
	assert.True(t, capabilityByID(t, report, "launch.rpc").Supported,
		"the canonical resolved target must survive the PATH symlink replacement")
	for _, id := range []string{"rpc.command_discovery", "rpc.tool_events", "rpc.terminal"} {
		capability := capabilityByID(t, report, id)
		assert.False(t, capability.Supported)
		assert.Equal(t, "exit_nonzero", capability.Reason)
	}
	assert.FileExists(t, rpcMarker, "doctor readiness must execute the isolated behavioral RPC turn")
}

func TestOMPRPCCapabilities_PartialEventsNeverBecomeReady(t *testing.T) {
	for _, reason := range []string{"timeout", "exit_nonzero"} {
		t.Run(reason, func(t *testing.T) {
			capabilities := evaluateOMPRPCCapabilities(ompProbeResult{
				output: []byte("{\"type\":\"available_commands_update\"}\n"), reason: reason,
			})
			assert.False(t, capabilities[0].Supported)
			assert.Equal(t, "event_observed_partial_"+reason, capabilities[0].Reason)
		})
	}
}

func environmentMapOMP(values []string) map[string]string {
	result := make(map[string]string, len(values))
	for _, value := range values {
		key, current, found := cutEnvOMP(value)
		if found {
			result[key] = current
		}
	}
	return result
}

func cutEnvOMP(value string) (string, string, bool) {
	for index := range value {
		if value[index] == '=' {
			return value[:index], value[index+1:], true
		}
	}
	return "", "", false
}

func rootForOMPReadinessTest(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}
