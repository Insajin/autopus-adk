package docs

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestContext7Client_ResolveLibrary_Success verifies that a library can be resolved by name.
// Given: a Context7 API server that returns a valid library ID
// When: ResolveLibrary is called with a known library name
// Then: the resolved library ID is returned without error
func TestContext7Client_ResolveLibrary_Success(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id": "/spf13/cobra", "name": "cobra", "version": "1.9.1"}`))
	}))
	defer srv.Close()

	client := NewContext7Client(srv.URL)
	result, err := client.ResolveLibrary("cobra")

	require.NoError(t, err)
	assert.Equal(t, "/spf13/cobra", result.ID)
	assert.Equal(t, "cobra", result.Name)
}

// TestContext7Client_ResolveLibrary_NotFound verifies that ErrLibraryNotFound is returned
// when the library cannot be resolved.
// Given: a Context7 API server that returns 404
// When: ResolveLibrary is called with an unknown library name
// Then: ErrLibraryNotFound is returned
func TestContext7Client_ResolveLibrary_NotFound(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := NewContext7Client(srv.URL)
	_, err := client.ResolveLibrary("nonexistent-lib-xyz")

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrLibraryNotFound)
}

// TestContext7Client_GetDocs_Success verifies that documentation is fetched by library ID and topic.
// Given: a Context7 API server that returns doc content
// When: GetDocs is called with a valid library ID and topic
// Then: documentation content is returned without error
func TestContext7Client_GetDocs_Success(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"content": "# cobra docs\nCommand creation API...", "tokens": 42}`))
	}))
	defer srv.Close()

	client := NewContext7Client(srv.URL)
	docs, err := client.GetDocs("/spf13/cobra", "commands")

	require.NoError(t, err)
	assert.NotEmpty(t, docs.Content)
	assert.Contains(t, docs.Content, "cobra")
	assert.Greater(t, docs.Tokens, 0)
}

// TestContext7Client_GetDocs_ServerError verifies that server-side errors are surfaced.
// Given: a Context7 API server that returns 500
// When: GetDocs is called
// Then: an error is returned
func TestContext7Client_GetDocs_ServerError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := NewContext7Client(srv.URL)
	_, err := client.GetDocs("/spf13/cobra", "commands")

	require.Error(t, err)
}

// TestContext7Client_GetDocs_QuotaExceeded verifies that quota exhaustion is classified explicitly.
func TestContext7Client_GetDocs_QuotaExceeded(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`quota exceeded`))
	}))
	defer srv.Close()

	client := NewContext7Client(srv.URL)
	_, err := client.GetDocs("/spf13/cobra", "commands")

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrQuotaExceeded)
}
