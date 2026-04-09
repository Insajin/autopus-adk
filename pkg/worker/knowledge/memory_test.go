package knowledge

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)


func TestMemorySearcher_GetContext(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "Bearer tok", r.Header.Get("Authorization"))
		assert.Contains(t, r.URL.Path, "/api/v1/workspaces/ws-1/memory/context")
		assert.Equal(t, "agent-1", r.URL.Query().Get("agent_id"))

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(MemoryContextResponse{
			Entries: []MemoryEntry{
				{ID: "m1", Title: "Deploy steps", Content: "Step 1...", Layer: "L1"},
				{ID: "m2", Title: "Rollback", Content: "How to rollback", Layer: "L2"},
			},
			TokensUsed: 42,
		})
	}))
	defer srv.Close()

	ms := NewMemorySearcher(srv.URL, "tok", "ws-1")
	entries, err := ms.GetContext(context.Background(), "agent-1", "deploy the service")

	require.NoError(t, err)
	assert.Len(t, entries, 2)
	assert.Equal(t, "m1", entries[0].ID)
	assert.Equal(t, "Deploy steps", entries[0].Title)
	assert.Equal(t, "L1", entries[0].Layer)
}

func TestMemorySearcher_GetContext_EmptyEntries(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(MemoryContextResponse{Entries: []MemoryEntry{}})
	}))
	defer srv.Close()

	ms := NewMemorySearcher(srv.URL, "tok", "ws-1")
	entries, err := ms.GetContext(context.Background(), "agent-1", "something")

	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestMemorySearcher_GetContext_ServerError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	ms := NewMemorySearcher(srv.URL, "tok", "ws-1")
	_, err := ms.GetContext(context.Background(), "agent-1", "test")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected status 500")
}

func TestMemorySearcher_GetContext_ContextCancelled(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(MemoryContextResponse{})
	}))
	defer srv.Close()

	ms := NewMemorySearcher(srv.URL, "tok", "ws-1")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := ms.GetContext(ctx, "agent-1", "test")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "request failed")
}

func TestMemorySearcher_CreateMemory(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "Bearer tok", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Contains(t, r.URL.Path, "/api/v1/workspaces/ws-1/memory")

		var req CreateMemoryRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.Equal(t, "agent-1", req.AgentID)
		assert.Equal(t, "Task learning: fix bug", req.Title)
		assert.Equal(t, "agent_learning", req.Source)

		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	ms := NewMemorySearcher(srv.URL, "tok", "ws-1")
	err := ms.CreateMemory(context.Background(), CreateMemoryRequest{
		AgentID: "agent-1",
		Title:   "Task learning: fix bug",
		Content: "Found that nil check was missing",
		Source:  "agent_learning",
	})

	require.NoError(t, err)
}

func TestMemorySearcher_CreateMemory_ServerError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	ms := NewMemorySearcher(srv.URL, "tok", "ws-1")
	err := ms.CreateMemory(context.Background(), CreateMemoryRequest{
		AgentID: "agent-1",
		Title:   "some title",
		Content: "some content",
		Source:  "agent_learning",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected status 500")
}

func TestExtractKeywords(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		description string
		want        string
	}{
		{
			name:        "extracts up to 5 meaningful words",
			description: "deploy the new service to production environment now",
			want:        "deploy,new,service,production,environment",
		},
		{
			name:        "skips common stop words",
			description: "the and or is are this that it",
			want:        "",
		},
		{
			name:        "fewer than 5 words",
			description: "rollback database",
			want:        "rollback,database",
		},
		{
			name:        "strips punctuation",
			description: "fix: broken, pipeline! now.",
			want:        "fix,broken,pipeline,now",
		},
		{
			name:        "empty description",
			description: "",
			want:        "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := extractKeywords(tt.description)
			assert.Equal(t, tt.want, got)
		})
	}
}
