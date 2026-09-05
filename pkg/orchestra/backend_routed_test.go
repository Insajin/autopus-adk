package orchestra

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type routedBackendRecorder struct {
	name     string
	requests []ProviderRequest
}

func (b *routedBackendRecorder) Execute(_ context.Context, req ProviderRequest) (*ProviderResponse, error) {
	b.requests = append(b.requests, req)
	return &ProviderResponse{Provider: req.Provider, ExecutedBackend: b.name}, nil
}

func (b *routedBackendRecorder) Name() string { return b.name }

type routedFreshBackend struct {
	*routedBackendRecorder
	fresh bool
}

func (b *routedFreshBackend) FreshExecutionPerRequest() bool { return b.fresh }

func TestRoutedBackendRoutesByProviderBackend(t *testing.T) {
	t.Parallel()

	base := &routedBackendRecorder{name: "subprocess"}
	omp := &routedBackendRecorder{name: "omp"}
	backend := NewRoutedBackend(base, map[string]ExecutionBackend{"omp": omp})

	ompResponse, err := backend.Execute(context.Background(), ProviderRequest{
		Provider: "claude",
		Config: ProviderConfig{
			Backend: "omp",
			Binary:  "must-not-run",
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "omp", ompResponse.ExecutedBackend)
	assert.Empty(t, base.requests)
	require.Len(t, omp.requests, 1)
	assert.Equal(t, "must-not-run", omp.requests[0].Config.Binary)

	baseResponse, err := backend.Execute(context.Background(), ProviderRequest{
		Provider: "codex",
		Config:   ProviderConfig{Binary: "codex"},
	})
	require.NoError(t, err)
	assert.Equal(t, "subprocess", baseResponse.ExecutedBackend)
	require.Len(t, base.requests, 1)
	assert.Len(t, omp.requests, 1)
	assert.Equal(t, "subprocess+omp", backend.Name())
}

func TestRoutedBackendNameSortsRouteKeys(t *testing.T) {
	t.Parallel()

	base := &routedBackendRecorder{name: "base"}
	route := &routedBackendRecorder{name: "route"}
	backend := NewRoutedBackend(base, map[string]ExecutionBackend{
		"zeta":  route,
		"alpha": route,
	})

	assert.Equal(t, "base+alpha+zeta", backend.Name())
}

func TestRoutedBackendDeclaresFreshnessOnlyWhenEveryDelegateIsFresh(t *testing.T) {
	t.Parallel()

	freshRoute := &routedFreshBackend{
		routedBackendRecorder: &routedBackendRecorder{name: "omp"},
		fresh:                 true,
	}
	staleRoute := &routedFreshBackend{
		routedBackendRecorder: &routedBackendRecorder{name: "omp"},
		fresh:                 false,
	}

	tests := []struct {
		name   string
		base   ExecutionBackend
		routes map[string]ExecutionBackend
		want   bool
	}{
		{
			name:   "fresh base and fresh route",
			base:   NewSubprocessBackendImpl(),
			routes: map[string]ExecutionBackend{"omp": freshRoute},
			want:   true,
		},
		{
			name:   "fresh base and stale route",
			base:   NewSubprocessBackendImpl(),
			routes: map[string]ExecutionBackend{"omp": staleRoute},
		},
		{
			name:   "fresh base and undeclared route",
			base:   NewSubprocessBackendImpl(),
			routes: map[string]ExecutionBackend{"omp": &routedBackendRecorder{name: "custom"}},
		},
		{
			name:   "undeclared base and fresh route",
			base:   &routedBackendRecorder{name: "custom"},
			routes: map[string]ExecutionBackend{"omp": freshRoute},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			backend := NewRoutedBackend(tt.base, tt.routes)
			assert.Equal(t, tt.want, pipelineBackendHasFreshExecutionSemantics(backend))
		})
	}
}

// A provider that names a backend with no registered route must fail closed
// instead of falling through to the CLI base backend.
func TestRoutedBackendFailsClosedForUnregisteredBackend(t *testing.T) {
	t.Parallel()

	base := &routedBackendRecorder{name: "subprocess"}
	backend := NewRoutedBackend(base, map[string]ExecutionBackend{})

	response, err := backend.Execute(context.Background(), ProviderRequest{
		Provider: "claude",
		Config:   ProviderConfig{Backend: "omp", Binary: "must-not-run"},
	})

	require.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), `provider claude backend "omp" is not available`)
	assert.Empty(t, base.requests)
}
