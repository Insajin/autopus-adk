package search_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/search"
)

func TestResolveLibrary_Success(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"/cobra-org/cobra","name":"cobra","description":"Cobra CLI library"}`))
	}))
	defer server.Close()

	client := search.NewContext7Client(search.WithContext7BaseURL(server.URL))
	id, err := client.ResolveLibrary("cobra")
	require.NoError(t, err)
	assert.NotEmpty(t, id)
	assert.Equal(t, "/cobra-org/cobra", id)
}

func TestResolveLibrary_NotFound(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"library not found"}`))
	}))
	defer server.Close()

	client := search.NewContext7Client(search.WithContext7BaseURL(server.URL))
	_, err := client.ResolveLibrary("nonexistent-lib-xyz")
	assert.Error(t, err)
}

func TestGetDocs_Success(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "cobra")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"content":"# Cobra Documentation\n\nCobra is a CLI library..."}`))
	}))
	defer server.Close()

	client := search.NewContext7Client(search.WithContext7BaseURL(server.URL))
	docs, err := client.GetDocs("/cobra-org/cobra", "getting started")
	require.NoError(t, err)
	assert.NotEmpty(t, docs)
	assert.Contains(t, docs, "Cobra")
}

func TestGetDocs_EmptyLibraryID(t *testing.T) {
	t.Parallel()

	client := search.NewContext7Client()
	_, err := client.GetDocs("", "topic")
	assert.Error(t, err)
}
