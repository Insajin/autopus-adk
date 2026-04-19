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

func TestKnowledgeSearcher_Search(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		query      string
		statusCode int
		response   any
		wantErr    string
		wantCount  int
	}{
		{
			name:       "successful search with results",
			query:      "deployment guide",
			statusCode: http.StatusOK,
			response: searchResponse{Data: []SearchResult{
				{ID: "1", Title: "Deploy Guide", Content: "How to deploy", RelevanceScore: 0.95},
				{ID: "2", Title: "CI/CD", Content: "Pipeline setup", RelevanceScore: 0.80},
			}},
			wantCount: 2,
		},
		{
			name:       "empty results",
			query:      "nonexistent topic",
			statusCode: http.StatusOK,
			response:   searchResponse{Data: []SearchResult{}},
			wantCount:  0,
		},
		{
			name:       "server error returns error",
			query:      "test",
			statusCode: http.StatusInternalServerError,
			response:   "internal error",
			wantErr:    "unexpected status 500",
		},
		{
			name:       "unauthorized returns error",
			query:      "test",
			statusCode: http.StatusUnauthorized,
			response:   "unauthorized",
			wantErr:    "unexpected status 401",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify request properties.
				assert.Equal(t, http.MethodPost, r.Method)
				assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
				assert.Contains(t, r.URL.Path, "/api/v1/workspaces/ws-123/knowledge/search")

				w.WriteHeader(tt.statusCode)
				_ = json.NewEncoder(w).Encode(tt.response)
			}))
			defer srv.Close()

			ks := NewKnowledgeSearcher(srv.URL, "test-token", "ws-123")
			results, err := ks.Search(context.Background(), tt.query)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Len(t, results, tt.wantCount)
			// Verify Score convenience field is populated.
			for _, r := range results {
				assert.Equal(t, r.RelevanceScore, r.Score)
			}
		})
	}
}

func TestKnowledgeSearcher_RequestBody(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the POST body contains the query.
		var req searchRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.Equal(t, "hello world", req.Query)
		assert.Equal(t, 10, req.Limit)

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(searchResponse{Data: []SearchResult{}})
	}))
	defer srv.Close()

	ks := NewKnowledgeSearcher(srv.URL, "tok", "ws-1")
	_, err := ks.Search(context.Background(), "hello world")
	require.NoError(t, err)
}

func TestKnowledgeSearcher_ContextCancelled(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(searchResponse{Data: []SearchResult{}})
	}))
	defer srv.Close()

	ks := NewKnowledgeSearcher(srv.URL, "tok", "ws-1")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := ks.Search(ctx, "test")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "request failed")
}

func TestKnowledgeSearcher_InvalidJSON(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not json"))
	}))
	defer srv.Close()

	ks := NewKnowledgeSearcher(srv.URL, "tok", "ws-1")
	_, err := ks.Search(context.Background(), "test")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode response")
}

func TestKnowledgeSearcher_SetAuthToken(t *testing.T) {
	t.Parallel()

	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(searchResponse{Data: []SearchResult{}})
	}))
	defer srv.Close()

	ks := NewKnowledgeSearcher(srv.URL, "old-token", "ws-1")
	ks.SetAuthToken("new-token")

	_, err := ks.Search(context.Background(), "test")
	require.NoError(t, err)
	assert.Equal(t, "Bearer new-token", gotAuth)
}
