package setup

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// WorkerConfig holds the worker's persistent configuration.
type WorkerConfig struct {
	BackendURL        string   `yaml:"backend_url"`
	WorkspaceID       string   `yaml:"workspace_id"`
	Providers         []string `yaml:"providers"`
	WorkDir           string   `yaml:"work_dir"`
	KnowledgeDir      string   `yaml:"knowledge_dir"`       // directory to watch for knowledge sync (defaults to work_dir)
	KnowledgeSourceID string   `yaml:"knowledge_source_id"` // knowledge source ID for bridge binding
	MemoryAgentID     string   `yaml:"memory_agent_id"`     // agent UUID for memory context/write-back
	A2AURL            string   `yaml:"a2a_url"`
	Concurrency       int      `yaml:"concurrency"`
}

// DefaultWorkerConfigPath returns the default path for worker.yaml.
func DefaultWorkerConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", "worker.yaml")
	}
	return filepath.Join(home, ".config", "autopus", "worker.yaml")
}

// SaveWorkerConfig writes the config to ~/.config/autopus/worker.yaml.
func SaveWorkerConfig(cfg WorkerConfig) error {
	path := DefaultWorkerConfigPath()

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal worker config: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write worker config: %w", err)
	}
	return nil
}

// LoadWorkerConfig reads the worker config from the default path.
func LoadWorkerConfig() (*WorkerConfig, error) {
	return LoadWorkerConfigFrom(DefaultWorkerConfigPath())
}

// LoadWorkerConfigFrom reads the worker config from the given path.
func LoadWorkerConfigFrom(path string) (*WorkerConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read worker config: %w", err)
	}

	var cfg WorkerConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal worker config: %w", err)
	}
	return &cfg, nil
}
