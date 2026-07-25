package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/config"
)

func TestQualityProviderAtomicFailurePreservesConfigAndSkipsApply(t *testing.T) {
	dir := writeQualityTestConfig(t, "balanced")
	path := filepath.Join(dir, "autopus.yaml")
	before, err := os.ReadFile(path)
	require.NoError(t, err)

	originalRename := renameQualityConfig
	renameQualityConfig = func(string, string) error { return errors.New("rename failed") }
	t.Cleanup(func() { renameQualityConfig = originalRename })

	originalUpdater := qualityPlatformUpdater
	t.Cleanup(func() { qualityPlatformUpdater = originalUpdater })
	updateCalls := 0
	qualityPlatformUpdater = func(context.Context, string, string, *config.HarnessConfig) (bool, error) {
		updateCalls++
		return true, nil
	}

	root := NewRootCmd()
	root.SetOut(&bytes.Buffer{})
	root.SetArgs([]string{
		"--config", path,
		"quality", "provider", "claude", "ultra", "--apply",
	})
	err = root.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rename failed")

	after, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, before, after)
	assert.Zero(t, updateCalls)
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	for _, entry := range entries {
		assert.False(t, strings.HasPrefix(entry.Name(), ".autopus-quality-"), entry.Name())
	}
}

func TestQualityProviderAtomicStageFailuresPreserveConfigAndSkipApply(t *testing.T) {
	originalCreate := createTempQualityConfig
	originalChmod := chmodQualityConfig
	originalWrite := writeQualityConfig
	originalSync := syncQualityConfig
	originalClose := closeQualityConfig
	originalRename := renameQualityConfig
	originalUpdater := qualityPlatformUpdater
	restore := func() {
		createTempQualityConfig = originalCreate
		chmodQualityConfig = originalChmod
		writeQualityConfig = originalWrite
		syncQualityConfig = originalSync
		closeQualityConfig = originalClose
		renameQualityConfig = originalRename
		qualityPlatformUpdater = originalUpdater
	}
	t.Cleanup(restore)

	tests := []struct {
		name      string
		configure func()
	}{
		{
			name: "create temp",
			configure: func() {
				createTempQualityConfig = func(string, string) (*os.File, error) {
					return nil, errors.New("create failed")
				}
			},
		},
		{
			name: "write",
			configure: func() {
				writeQualityConfig = func(*os.File, []byte) (int, error) {
					return 0, errors.New("write failed")
				}
			},
		},
		{
			name: "sync",
			configure: func() {
				syncQualityConfig = func(*os.File) error {
					return errors.New("sync failed")
				}
			},
		},
		{
			name: "close",
			configure: func() {
				closeQualityConfig = func(file *os.File) error {
					_ = originalClose(file)
					return errors.New("close failed")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			restore()
			dir := writeQualityTestConfig(t, "balanced")
			path := filepath.Join(dir, "autopus.yaml")
			before, err := os.ReadFile(path)
			require.NoError(t, err)
			tt.configure()

			updateCalls := 0
			qualityPlatformUpdater = func(context.Context, string, string, *config.HarnessConfig) (bool, error) {
				updateCalls++
				return true, nil
			}

			root := NewRootCmd()
			root.SetOut(&bytes.Buffer{})
			root.SetArgs([]string{
				"--config", path,
				"quality", "provider", "claude", "ultra", "--apply",
			})
			err = root.Execute()
			require.Error(t, err)

			after, err := os.ReadFile(path)
			require.NoError(t, err)
			assert.Equal(t, before, after)
			assert.Zero(t, updateCalls)
			entries, err := os.ReadDir(dir)
			require.NoError(t, err)
			for _, entry := range entries {
				assert.False(t, strings.HasPrefix(entry.Name(), ".autopus-quality-"), entry.Name())
			}
		})
	}
}
