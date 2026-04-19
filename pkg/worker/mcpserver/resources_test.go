package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResourceRegistry_ListResources(t *testing.T) {
	t.Parallel()

	r := NewResourceRegistry("http://localhost", "tok")
	resources := r.ListResources()
	assert.Len(t, resources, 4)

	uris := make([]string, len(resources))
	for i, res := range resources {
		uris[i] = res.URI
	}
	assert.Contains(t, uris, "autopus://status")
	assert.Contains(t, uris, "autopus://workspaces")
	assert.Contains(t, uris, "autopus://agents")
	assert.Contains(t, uris, "autopus://executions/")
}

func TestResourceRegistry_ListTemplates(t *testing.T) {
	t.Parallel()

	r := NewResourceRegistry("http://localhost", "tok")
	templates := r.ListTemplates()
	require.Len(t, templates, 1)
	assert.Equal(t, "autopus://executions/{id}", templates[0].URITemplate)
	assert.Equal(t, "execution_by_id", templates[0].Name)
}

func TestResourceRegistry_Get_FreshCache(t *testing.T) {
	t.Parallel()

	var fetchCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetchCount.Add(1)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer srv.Close()

	r := NewResourceRegistry(srv.URL, "tok")
	ctx := context.Background()

	// First call fetches from backend.
	data1, err := r.Get(ctx, "autopus://status")
	require.NoError(t, err)
	assert.NotNil(t, data1)
	assert.Equal(t, int32(1), fetchCount.Load())

	// Second call should use cache.
	data2, err := r.Get(ctx, "autopus://status")
	require.NoError(t, err)
	assert.Equal(t, data1, data2)
	assert.Equal(t, int32(1), fetchCount.Load(), "should serve from cache")
}

func TestResourceRegistry_Get_ExpiredCache(t *testing.T) {
	t.Parallel()

	var fetchCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetchCount.Add(1)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"v": fmt.Sprintf("%d", fetchCount.Load())})
	}))
	defer srv.Close()

	r := NewResourceRegistry(srv.URL, "tok")
	ctx := context.Background()

	// First fetch.
	_, err := r.Get(ctx, "autopus://status")
	require.NoError(t, err)
	assert.Equal(t, int32(1), fetchCount.Load())

	// Manually expire the cache entry.
	r.mu.Lock()
	entry := r.cache["autopus://status"]
	entry.fetchedAt = time.Now().Add(-2 * defaultTTL)
	r.mu.Unlock()

	// Should re-fetch.
	_, err = r.Get(ctx, "autopus://status")
	require.NoError(t, err)
	assert.Equal(t, int32(2), fetchCount.Load(), "should re-fetch expired entry")
}

func TestResourceRegistry_Get_StaleFallbackOnError(t *testing.T) {
	t.Parallel()

	callNum := atomic.Int32{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := callNum.Add(1)
		if n == 1 {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{"data": "cached"})
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	r := NewResourceRegistry(srv.URL, "tok")
	ctx := context.Background()

	// First fetch succeeds.
	data1, err := r.Get(ctx, "autopus://status")
	require.NoError(t, err)

	// Expire the cache.
	r.mu.Lock()
	r.cache["autopus://status"].fetchedAt = time.Now().Add(-2 * defaultTTL)
	r.mu.Unlock()

	// Second fetch fails — should return stale cache.
	data2, err := r.Get(ctx, "autopus://status")
	require.NoError(t, err)
	assert.Equal(t, data1, data2, "should return stale data on error")
}

func TestResourceRegistry_Get_UnknownResource(t *testing.T) {
	t.Parallel()

	r := NewResourceRegistry("http://localhost", "tok")
	_, err := r.Get(context.Background(), "autopus://unknown")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown resource")
}

func TestResourceRegistry_Get_DynamicExecution(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/executions/exec-42", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "exec-42"})
	}))
	defer srv.Close()

	r := NewResourceRegistry(srv.URL, "tok")
	data, err := r.Get(context.Background(), "autopus://executions/exec-42")
	require.NoError(t, err)

	m := data.(map[string]any)
	assert.Equal(t, "exec-42", m["id"])
}

func TestResourceRegistry_MatchFetcher(t *testing.T) {
	t.Parallel()

	r := NewResourceRegistry("http://localhost", "tok")

	tests := []struct {
		name  string
		uri   string
		found bool
	}{
		{"exact status", "autopus://status", true},
		{"exact workspaces", "autopus://workspaces", true},
		{"exact agents", "autopus://agents", true},
		{"prefix executions", "autopus://executions/123", true},
		{"unknown uri", "autopus://nope", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := r.matchFetcher(tt.uri)
			if tt.found {
				assert.NotNil(t, f)
			} else {
				assert.Nil(t, f)
			}
		})
	}
}

func TestResourceRegistry_FetchExecution_EmptyID(t *testing.T) {
	t.Parallel()

	r := NewResourceRegistry("http://localhost", "tok")
	_, err := r.fetchExecution(context.Background(), "autopus://executions/")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "execution id is required")
}

func TestCacheEntry_IsExpired(t *testing.T) {
	t.Parallel()

	fresh := &cacheEntry{fetchedAt: time.Now(), ttl: 30 * time.Second}
	assert.False(t, fresh.isExpired())

	expired := &cacheEntry{fetchedAt: time.Now().Add(-1 * time.Minute), ttl: 30 * time.Second}
	assert.True(t, expired.isExpired())
}
