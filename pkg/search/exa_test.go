package search_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/search"
)

func TestExaSearch_Success(t *testing.T) {
	t.Parallel()

	// 모의 Exa API 서버
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/search", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.NotEmpty(t, r.Header.Get("x-api-key"))

		resp := map[string]interface{}{
			"results": []map[string]interface{}{
				{
					"title":   "Go Best Practices",
					"url":     "https://example.com/go",
					"snippet": "Go 언어 모범 사례",
				},
				{
					"title":   "Go Testing",
					"url":     "https://example.com/testing",
					"snippet": "Go 테스트 작성법",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := search.NewExaClient("test-api-key", search.WithExaBaseURL(server.URL))
	results, err := client.Search("Go best practices", 5)
	require.NoError(t, err)
	require.Len(t, results, 2)

	assert.Equal(t, "Go Best Practices", results[0].Title)
	assert.Equal(t, "https://example.com/go", results[0].URL)
	assert.Equal(t, "Go 언어 모범 사례", results[0].Snippet)
}

func TestExaSearch_APIError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error": "Invalid API key"}`))
	}))
	defer server.Close()

	client := search.NewExaClient("invalid-key", search.WithExaBaseURL(server.URL))
	_, err := client.Search("test", 5)
	assert.Error(t, err)
}

func TestExaSearch_EmptyResults(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{"results": []interface{}{}}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := search.NewExaClient("test-key", search.WithExaBaseURL(server.URL))
	results, err := client.Search("nothing found", 5)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestExaClient_ReadsFromEnv(t *testing.T) {
	t.Parallel()

	t.Setenv("EXA_API_KEY", "env-api-key")

	client := search.NewExaClientFromEnv()
	assert.NotNil(t, client)
}
