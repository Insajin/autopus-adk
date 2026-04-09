// Package mcpserver implements a JSON-RPC 2.0 MCP server over stdio.
package mcpserver

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config holds the MCP server configuration loaded from worker-mcp.json.
type Config struct {
	BackendURL     string   `json:"backend_url"`
	AuthToken      string   `json:"auth_token"`
	WorkspaceID    string   `json:"workspace_id"`
	Transport      string   `json:"transport,omitempty"`
	SSEPort        int      `json:"sse_port,omitempty"`
	SSEHost        string   `json:"sse_host,omitempty"`
	Tools          []string `json:"tools,omitempty"`
	MaxConnections int      `json:"max_connections,omitempty"`
}

// DefaultConfig returns a Config with all optional fields set to their defaults.
func DefaultConfig() *Config {
	// @AX:NOTE[AUTO]: magic constants — SSEPort=8080, MaxConnections=10 are defaults; override via config file for production deployments
	return &Config{
		Transport:      "stdio",
		SSEPort:        8080,
		SSEHost:        "localhost",
		MaxConnections: 10,
	}
}

// LoadConfig reads a JSON file at path and unmarshals it into a Config.
// Optional fields are filled with defaults before returning.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("mcpserver: read config %q: %w", path, err)
	}

	cfg := DefaultConfig()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("mcpserver: parse config %q: %w", path, err)
	}

	if err := ValidateConfig(*cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// ValidateConfig checks that cfg has all required fields and valid values.
// It returns a descriptive error naming the offending field when validation fails.
func ValidateConfig(cfg Config) error {
	if cfg.BackendURL == "" {
		return fmt.Errorf("mcpserver: BackendURL is required")
	}
	if cfg.AuthToken == "" {
		return fmt.Errorf("mcpserver: AuthToken is required")
	}
	if cfg.WorkspaceID == "" {
		return fmt.Errorf("mcpserver: WorkspaceID is required")
	}

	// Validate transport type when set.
	if t := cfg.Transport; t != "" && t != "stdio" && t != "sse" {
		return fmt.Errorf("mcpserver: invalid transport %q — valid values are \"stdio\", \"sse\"", t)
	}

	// Validate SSE port range when set.
	if cfg.SSEPort != 0 && (cfg.SSEPort < 1 || cfg.SSEPort > 65535) {
		return fmt.Errorf("mcpserver: SSEPort %d out of valid range 1-65535", cfg.SSEPort)
	}

	return nil
}
