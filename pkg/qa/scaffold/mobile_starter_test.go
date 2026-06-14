package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/insajin/autopus-adk/pkg/qa/journey"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestDetectJourneyStartersEmitsSingleMobileScriptedStarter(t *testing.T) {
	dir := t.TempDir()
	gradle := filepath.Join(dir, "android", "app", "build.gradle")
	require.NoError(t, os.MkdirAll(filepath.Dir(gradle), 0o755))
	require.NoError(t, os.WriteFile(gradle, []byte("apply plugin: 'com.android.application'\n"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "ios", "Runner.xcodeproj"), 0o755))

	starters := detectJourneyStarters(dir, false)

	var mobile []starterFile
	for _, starter := range starters {
		if starter.ID == "mobile-scripted-maestro" {
			mobile = append(mobile, starter)
		}
	}
	require.Len(t, mobile, 1)

	starter := mobile[0]
	assert.Contains(t, starter.Body, "Review before executing")

	var pack journey.Pack
	require.NoError(t, yaml.Unmarshal([]byte(starter.Body), &pack))
	assert.Equal(t, "maestro-scripted", pack.Adapter.ID)
	assert.Equal(t, "mobile", pack.Surface)
	assert.Contains(t, pack.Lanes, "mobile-scripted")
	assert.True(t, strings.HasSuffix(starter.RelPath, "mobile-scripted-maestro.yaml"))
	assert.NoError(t, journey.Validate(pack, dir))
}
