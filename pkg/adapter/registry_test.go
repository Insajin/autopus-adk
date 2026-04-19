package adapter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/config"
)

// mockAdapter는 테스트용 PlatformAdapter 구현이다.
type mockAdapter struct {
	name     string
	detected bool
}

func (m *mockAdapter) Name() string                           { return m.name }
func (m *mockAdapter) Version() string                        { return "1.0.0" }
func (m *mockAdapter) CLIBinary() string                      { return m.name }
func (m *mockAdapter) Detect(_ context.Context) (bool, error) { return m.detected, nil }
func (m *mockAdapter) Generate(_ context.Context, _ *config.HarnessConfig) (*PlatformFiles, error) {
	return &PlatformFiles{}, nil
}
func (m *mockAdapter) Update(_ context.Context, _ *config.HarnessConfig) (*PlatformFiles, error) {
	return &PlatformFiles{}, nil
}
func (m *mockAdapter) Validate(_ context.Context) ([]ValidationError, error) { return nil, nil }
func (m *mockAdapter) Clean(_ context.Context) error                         { return nil }
func (m *mockAdapter) SupportsHooks() bool                                   { return false }
func (m *mockAdapter) InstallHooks(_ context.Context, _ []HookConfig, _ *PermissionSet) error {
	return nil
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	r.Register(&mockAdapter{name: "test"})

	a, err := r.Get("test")
	require.NoError(t, err)
	assert.Equal(t, "test", a.Name())
}

func TestRegistry_GetNotFound(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	_, err := r.Get("nonexistent")
	require.Error(t, err)
}

func TestRegistry_List(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	r.Register(&mockAdapter{name: "a"})
	r.Register(&mockAdapter{name: "b"})
	assert.Len(t, r.List(), 2)
}

func TestRegistry_DetectAll(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	r.Register(&mockAdapter{name: "detected", detected: true})
	r.Register(&mockAdapter{name: "not-detected", detected: false})

	detected := r.DetectAll(context.Background())
	assert.Len(t, detected, 1)
	assert.Equal(t, "detected", detected[0].Name())
}
