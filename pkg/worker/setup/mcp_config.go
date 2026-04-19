package setup

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

// MCPServerConfig represents a single MCP server entry in the config.
type MCPServerConfig struct {
	Command   string            `json:"command,omitempty"`
	Args      []string          `json:"args,omitempty"`
	URL       string            `json:"url,omitempty"`
	Transport string            `json:"transport"`
	Headers   map[string]string `json:"headers,omitempty"`
}

// MCPConfig is the top-level worker-mcp.json structure.
type MCPConfig struct {
	MCPServers map[string]MCPServerConfig `json:"mcpServers"`
}

// MCPConfigOptions holds input parameters for MCP config generation.
type MCPConfigOptions struct {
	BackendURL  string // e.g., "https://api.autopus.co"
	AuthToken   string // Bearer token
	WorkspaceID string // workspace identifier
	OutputPath  string // where to write worker-mcp.json (default: ~/.config/autopus/worker-mcp.json)
}

// GenerateMCPConfig builds an MCPConfig from the given options.
func GenerateMCPConfig(opts MCPConfigOptions) (*MCPConfig, error) {
	if opts.BackendURL == "" {
		return nil, fmt.Errorf("BackendURL is required")
	}
	if opts.AuthToken == "" {
		return nil, fmt.Errorf("AuthToken is required")
	}

	headers := map[string]string{
		"Authorization": "Bearer " + opts.AuthToken,
	}
	if opts.WorkspaceID != "" {
		headers["X-Workspace-ID"] = opts.WorkspaceID
	}

	config := &MCPConfig{
		MCPServers: map[string]MCPServerConfig{
			"autopus": {
				URL:       opts.BackendURL + "/mcp/sse",
				Transport: "sse",
				Headers:   headers,
			},
		},
	}
	return config, nil
}

// WriteMCPConfig writes the config to the specified path with atomic write.
// The file is created with 0600 permissions since it contains auth tokens.
func WriteMCPConfig(config *MCPConfig, path string) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal mcp config: %w", err)
	}
	data = append(data, '\n')

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	// Atomic write: temp file + rename to avoid partial writes.
	tmp, err := os.CreateTemp(dir, "worker-mcp-*.json")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()

	if err := tmp.Chmod(0600); err != nil {
		_ = tmp.Close()
		removeIfExists(tmpPath)
		return fmt.Errorf("chmod temp file: %w", err)
	}

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		removeIfExists(tmpPath)
		return fmt.Errorf("write mcp config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		removeIfExists(tmpPath)
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		removeIfExists(tmpPath)
		return fmt.Errorf("rename mcp config: %w", err)
	}

	log.Printf("[setup] MCP config written to %s", path)
	return nil
}

// DefaultMCPConfigPath returns the default path for worker-mcp.json.
func DefaultMCPConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", "worker-mcp.json")
	}
	return filepath.Join(home, ".config", "autopus", "worker-mcp.json")
}

// LoadMCPConfig reads an existing MCP config from the given path.
func LoadMCPConfig(path string) (*MCPConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read mcp config: %w", err)
	}

	var config MCPConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("unmarshal mcp config: %w", err)
	}
	return &config, nil
}

func removeIfExists(path string) {
	_ = os.Remove(path)
}
