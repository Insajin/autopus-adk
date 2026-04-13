package setup

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestSaveAndLoadWorkerConfig_RoundTrip(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "worker.yaml")

	original := WorkerConfig{
		BackendURL:        "https://api.autopus.co",
		WorkspaceID:       "ws-123",
		Providers:         []string{"claude", "codex"},
		WorkDir:           "/tmp/autopus-work",
		WorktreeIsolation: true,
		KnowledgeDir:      "/tmp/autopus-work",
		KnowledgeSourceID: "src-123",
		MemoryAgentID:     "11111111-2222-4333-8444-555555555555",
		A2AURL:            "https://a2a.autopus.co",
		Concurrency:       3,
	}

	data, err := yaml.Marshal(original)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0600))

	loaded, err := LoadWorkerConfigFrom(path)
	require.NoError(t, err)

	assert.Equal(t, original.BackendURL, loaded.BackendURL)
	assert.Equal(t, original.WorkspaceID, loaded.WorkspaceID)
	assert.Equal(t, original.Providers, loaded.Providers)
	assert.Equal(t, original.WorkDir, loaded.WorkDir)
	assert.Equal(t, original.WorktreeIsolation, loaded.WorktreeIsolation)
	assert.Equal(t, original.KnowledgeDir, loaded.KnowledgeDir)
	assert.Equal(t, original.KnowledgeSourceID, loaded.KnowledgeSourceID)
	assert.Equal(t, original.MemoryAgentID, loaded.MemoryAgentID)
	assert.Equal(t, original.A2AURL, loaded.A2AURL)
	assert.Equal(t, original.Concurrency, loaded.Concurrency)
}

func TestLoadWorkerConfigFrom_MissingFile(t *testing.T) {
	t.Parallel()

	_, err := LoadWorkerConfigFrom("/nonexistent/worker.yaml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read worker config")
}

func TestLoadWorkerConfigFrom_InvalidYAML(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "worker.yaml")
	require.NoError(t, os.WriteFile(path, []byte(":::invalid"), 0600))

	_, err := LoadWorkerConfigFrom(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal worker config")
}

func TestDefaultWorkerConfigPath(t *testing.T) {
	t.Parallel()

	path := DefaultWorkerConfigPath()
	assert.Contains(t, path, "worker.yaml")
	assert.Contains(t, path, ".config")
	assert.Contains(t, path, "autopus")
}

func TestSaveWorkerConfig_WritesToDisk(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	cfg := WorkerConfig{
		BackendURL:        "https://api.autopus.co",
		WorkspaceID:       "ws-test",
		Providers:         []string{"claude"},
		WorkDir:           "/tmp/work",
		WorktreeIsolation: true,
		KnowledgeDir:      "/tmp/work",
		KnowledgeSourceID: "src-001",
		A2AURL:            "https://a2a.autopus.co",
		Concurrency:       2,
	}
	err := SaveWorkerConfig(cfg)
	require.NoError(t, err)

	loaded, err := LoadWorkerConfig()
	require.NoError(t, err)
	assert.Equal(t, cfg.BackendURL, loaded.BackendURL)
	assert.Equal(t, cfg.WorkspaceID, loaded.WorkspaceID)
	assert.Equal(t, cfg.Providers, loaded.Providers)
	assert.Equal(t, cfg.WorktreeIsolation, loaded.WorktreeIsolation)
	assert.Equal(t, cfg.KnowledgeSourceID, loaded.KnowledgeSourceID)
}

func TestLoadWorkerConfig_NoFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	_, err := LoadWorkerConfig()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read worker config")
}

func TestSaveWorkerConfig_ReadOnlyDir(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Create config dir then make a file where worker.yaml should go
	dir := filepath.Join(tmp, ".config", "autopus")
	require.NoError(t, os.MkdirAll(dir, 0700))
	// Create worker.yaml as a directory to force write failure
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "worker.yaml"), 0700))

	err := SaveWorkerConfig(WorkerConfig{BackendURL: "https://test.co"})
	require.Error(t, err)
}

func TestLoadWorkerConfigFrom_EmptyProviders(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "worker.yaml")

	cfg := WorkerConfig{
		BackendURL: "https://api.autopus.co",
	}

	data, err := yaml.Marshal(cfg)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0600))

	loaded, err := LoadWorkerConfigFrom(path)
	require.NoError(t, err)
	assert.Empty(t, loaded.Providers)
}
