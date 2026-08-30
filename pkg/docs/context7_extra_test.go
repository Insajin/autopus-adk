package docs

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestContext7Client_Fetch_Success verifies the Fetch method resolves and fetches in one call.
// Given: a server that handles both resolve and get-docs endpoints
// When: Fetch is called with a library name and topic
// Then: a DocResult is returned with source "context7"
func TestContext7Client_Fetch_Success(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if r.URL.Path == "/libraries" {
			_, _ = w.Write([]byte(`{"id": "/spf13/cobra", "name": "cobra", "version": "1.9.1"}`))
		} else {
			_, _ = w.Write([]byte(`{"content": "cobra command docs", "tokens": 30}`))
		}
	}))
	defer srv.Close()

	client := NewContext7Client(srv.URL)
	result, err := client.Fetch("cobra", "commands")

	require.NoError(t, err)
	assert.Equal(t, "context7", result.Source)
	assert.Equal(t, "cobra", result.LibraryName)
	assert.NotEmpty(t, result.Content)
}

// TestContext7Client_Fetch_NotFound verifies ErrLibraryNotFound propagates from Fetch.
// Given: a server that returns 404 for the resolve endpoint
// When: Fetch is called with an unknown library
// Then: ErrLibraryNotFound is returned
func TestContext7Client_Fetch_NotFound(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := NewContext7Client(srv.URL)
	_, err := client.Fetch("unknown-lib-xyz", "api")

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrLibraryNotFound)
}

// TestContext7Client_Fetch_ResolveError verifies that non-404 resolve errors are wrapped.
// Given: a server that returns 500 for the resolve endpoint
// When: Fetch is called
// Then: an error is returned (not ErrLibraryNotFound)
func TestContext7Client_Fetch_ResolveError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := NewContext7Client(srv.URL)
	_, err := client.Fetch("cobra", "api")

	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrLibraryNotFound)
}

// TestContext7Client_GetDocs_EmptyLibraryID verifies that empty libraryID is rejected.
// Given: a valid Context7 client
// When: GetDocs is called with an empty libraryID
// Then: an error is returned immediately
func TestContext7Client_GetDocs_EmptyLibraryID(t *testing.T) {
	t.Parallel()

	client := NewContext7Client("http://localhost:9999")
	_, err := client.GetDocs("", "commands")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "libraryID")
}

// TestContext7Client_NewContext7Client_DefaultURL verifies that empty baseURL uses the default.
// Given: NewContext7Client called with empty string
// When: the client is created
// Then: the client is non-nil (default URL is set internally)
func TestContext7Client_NewContext7Client_DefaultURL(t *testing.T) {
	t.Parallel()

	client := NewContext7Client("")
	assert.NotNil(t, client)
}

func TestNewContext7Client_OwnsIndependentTransport(t *testing.T) {
	t.Parallel()

	first := NewContext7Client("http://example.test")
	second := NewContext7Client("http://example.test")

	require.NotNil(t, first.http.Transport)
	require.NotNil(t, second.http.Transport)
	assert.NotSame(t, http.DefaultTransport, first.http.Transport)
	assert.NotSame(t, first.http.Transport, second.http.Transport)
}
