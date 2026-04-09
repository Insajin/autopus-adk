// Package mcpserver_test tests MCP server configuration validation.
package mcpserver_test

import (
	"testing"

	"github.com/insajin/autopus-adk/pkg/worker/mcpserver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidateConfig_Valid verifies that a fully valid config passes validation.
func TestValidateConfig_Valid(t *testing.T) {
	t.Parallel()

	// Given: a well-formed MCP server config
	cfg := mcpserver.Config{
		BackendURL:  "https://api.example.com",
		AuthToken:   "tok_abc123",
		WorkspaceID: "ws-001",
		Transport:   "stdio",
	}

	// When: validating the config
	err := mcpserver.ValidateConfig(cfg)

	// Then: no error is returned
	require.NoError(t, err)
}

// TestValidateConfig_MissingRequired verifies that missing required fields return an error.
func TestValidateConfig_MissingRequired(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cfg     mcpserver.Config
		wantErr string
	}{
		{
			name: "missing BackendURL",
			cfg: mcpserver.Config{
				AuthToken:   "tok_abc",
				WorkspaceID: "ws-001",
				Transport:   "stdio",
			},
			wantErr: "BackendURL",
		},
		{
			name: "missing AuthToken",
			cfg: mcpserver.Config{
				BackendURL:  "https://api.example.com",
				WorkspaceID: "ws-001",
				Transport:   "stdio",
			},
			wantErr: "AuthToken",
		},
		{
			name: "missing WorkspaceID",
			cfg: mcpserver.Config{
				BackendURL: "https://api.example.com",
				AuthToken:  "tok_abc",
				Transport:  "stdio",
			},
			wantErr: "WorkspaceID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// When: validating an incomplete config
			err := mcpserver.ValidateConfig(tt.cfg)

			// Then: error mentions the missing field name
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr,
				"error message must name the missing required field")
		})
	}
}

// TestValidateConfig_InvalidType verifies that an invalid transport type is rejected.
func TestValidateConfig_InvalidType(t *testing.T) {
	t.Parallel()

	// Given: a config with an unrecognized transport type
	cfg := mcpserver.Config{
		BackendURL:  "https://api.example.com",
		AuthToken:   "tok_abc123",
		WorkspaceID: "ws-001",
		Transport:   "grpc", // invalid — only "stdio" and "sse" are valid
	}

	// When: validating the config
	err := mcpserver.ValidateConfig(cfg)

	// Then: error is returned indicating invalid transport type
	require.Error(t, err)
	assert.Contains(t, err.Error(), "transport", "error must mention the invalid field")
}

// TestDefaultConfig_HasDefaults verifies that DefaultConfig populates all optional fields.
func TestDefaultConfig_HasDefaults(t *testing.T) {
	t.Parallel()

	// When: calling DefaultConfig
	cfg := mcpserver.DefaultConfig()

	// Then: sensible defaults are set
	require.NotNil(t, cfg)
	assert.Equal(t, "stdio", cfg.Transport, "default transport must be stdio")
	assert.Equal(t, 8080, cfg.SSEPort, "default SSE port must be 8080")
	assert.Equal(t, "localhost", cfg.SSEHost, "default SSE host must be localhost")
	assert.Greater(t, cfg.MaxConnections, 0, "default MaxConnections must be positive")
}

// TestLoadConfig_FileNotFound verifies that LoadConfig returns an error for a missing file.
func TestLoadConfig_FileNotFound(t *testing.T) {
	t.Parallel()

	// Given: a path that does not exist
	// When: loading config from a non-existent file
	_, err := mcpserver.LoadConfig("/tmp/does-not-exist-autopus-mcp.json")

	// Then: an error is returned
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read config")
}

// TestNewMCPServerFromConfig_Valid verifies that a valid config produces an MCPServer.
func TestNewMCPServerFromConfig_Valid(t *testing.T) {
	t.Parallel()

	// Given: a valid config
	cfg := mcpserver.Config{
		BackendURL:  "https://api.example.com",
		AuthToken:   "tok_abc123",
		WorkspaceID: "ws-001",
		Transport:   "stdio",
	}

	// When: creating a server from config
	srv, err := mcpserver.NewMCPServerFromConfig(cfg)

	// Then: server is created without error
	require.NoError(t, err)
	assert.NotNil(t, srv)
}

// TestNewMCPServerFromConfig_InvalidConfig verifies that an invalid config returns an error.
func TestNewMCPServerFromConfig_InvalidConfig(t *testing.T) {
	t.Parallel()

	// Given: a config missing required fields
	cfg := mcpserver.Config{
		Transport: "stdio",
		// BackendURL, AuthToken, WorkspaceID all empty
	}

	// When: creating a server from an invalid config
	srv, err := mcpserver.NewMCPServerFromConfig(cfg)

	// Then: an error is returned and server is nil
	require.Error(t, err)
	assert.Nil(t, srv)
}
