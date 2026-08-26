package omp

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
	}

	env := environmentMapOMP(mergeOMPProbeEnvironment(base, overrides))
	assert.Equal(t, "/usr/bin:/bin", env["PATH"])
	assert.Equal(t, "en_US.UTF-8", env["LANG"])
	assert.Equal(t, "/task/home", env["HOME"])
	assert.Equal(t, "/task/xdg-config", env["XDG_CONFIG_HOME"])
	assert.Equal(t, "/task/pi-agent", env["PI_CODING_AGENT_DIR"])
	for _, key := range []string{
		"OPENAI_API_KEY", "ANTHROPIC_API_KEY", "AWS_SECRET_ACCESS_KEY",
		"PI_CONFIG_FILES", "PI_PROVIDER_TOKEN",
	} {
		assert.NotContains(t, env, key)
	}
}

func environmentMapOMP(values []string) map[string]string {
	result := make(map[string]string, len(values))
	for _, value := range values {
		for index := range value {
			if value[index] == '=' {
				result[value[:index]] = value[index+1:]
				break
			}
		}
	}
	return result
}
