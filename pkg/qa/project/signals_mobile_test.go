package project

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMobileSignalsDetectAndroidAndIOS(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	gradle := filepath.Join(dir, "android", "app", "build.gradle")
	require.NoError(t, os.MkdirAll(filepath.Dir(gradle), 0o755))
	require.NoError(t, os.WriteFile(gradle, []byte("apply plugin: 'com.android.application'\n"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "ios", "Runner.xcodeproj"), 0o755))

	assert.True(t, HasAndroidSignals(dir))
	assert.True(t, HasIOSSignals(dir))
}

func TestMobileSignalsAbsentInEmptyProject(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	assert.False(t, HasAndroidSignals(dir))
	assert.False(t, HasIOSSignals(dir))
}
