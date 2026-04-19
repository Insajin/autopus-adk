package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultTestProfileCapabilities(t *testing.T) {
	t.Parallel()

	assert.Equal(t, []string{"standalone"}, DefaultTestProfileCapabilities("standalone"))
	assert.Equal(
		t,
		[]string{"standalone", "local", "docker", "db", "redis", "backend-server", "frontend-server", "network"},
		DefaultTestProfileCapabilities("local"),
	)
	assert.Equal(
		t,
		[]string{"standalone", "ci", "db", "redis", "backend-server", "frontend-server", "network"},
		DefaultTestProfileCapabilities("ci"),
	)
	assert.Equal(
		t,
		[]string{"standalone", "prod", "db", "redis", "backend-server", "frontend-server", "network"},
		DefaultTestProfileCapabilities("prod"),
	)
}

func TestHarnessConfig_AvailableTestCapabilities_MergesConfigAdditions(t *testing.T) {
	t.Parallel()

	cfg := &HarnessConfig{
		Profiles: ProfilesConf{
			Test: TestProfileConf{
				Capabilities: map[string][]string{
					"standalone": []string{"docker", "standalone"},
				},
			},
		},
	}

	assert.Equal(t, []string{"standalone", "docker"}, cfg.AvailableTestCapabilities("standalone"))
}

func TestIsValidTestProfile(t *testing.T) {
	t.Parallel()

	assert.True(t, IsValidTestProfile("standalone"))
	assert.True(t, IsValidTestProfile("LOCAL"))
	assert.False(t, IsValidTestProfile("preview"))
}
