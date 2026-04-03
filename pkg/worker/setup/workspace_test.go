package setup

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFetchWorkspaces_Success(t *testing.T) {
	t.Parallel()

	workspaces := []Workspace{
		{ID: "ws-1", Name: "Alpha"},
		{ID: "ws-2", Name: "Beta"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(workspaces)
	}))
	defer srv.Close()

	got, err := FetchWorkspaces(srv.URL, "test-token")
	require.NoError(t, err)
	assert.Len(t, got, 2)
	assert.Equal(t, "ws-1", got[0].ID)
	assert.Equal(t, "Beta", got[1].Name)
}

func TestFetchWorkspaces_ServerError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer srv.Close()

	_, err := FetchWorkspaces(srv.URL, "test-token")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestFetchWorkspaces_InvalidJSON(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not-json"))
	}))
	defer srv.Close()

	_, err := FetchWorkspaces(srv.URL, "test-token")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode")
}

func TestFetchWorkspaces_AuthHeader(t *testing.T) {
	t.Parallel()

	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]Workspace{})
	}))
	defer srv.Close()

	_, err := FetchWorkspaces(srv.URL, "my-secret-token")
	require.NoError(t, err)
	assert.Equal(t, "Bearer my-secret-token", gotAuth)
}

func TestSelectWorkspace_SingleAutoSelect(t *testing.T) {
	t.Parallel()

	ws := []Workspace{{ID: "ws-only", Name: "Only"}}
	got, err := SelectWorkspace(ws)
	require.NoError(t, err)
	assert.Equal(t, "ws-only", got.ID)
}

func TestSelectWorkspace_EmptyList(t *testing.T) {
	t.Parallel()

	_, err := SelectWorkspace([]Workspace{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no workspaces")
}

func TestFetchWorkspaces_WrappedResponse(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success":true,"data":[{"id":"ws-wrapped","name":"Wrapped"}]}`))
	}))
	defer srv.Close()

	got, err := FetchWorkspaces(srv.URL, "test-token")
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "ws-wrapped", got[0].ID)
}

func TestSelectWorkspace_MultipleWithValidInput(t *testing.T) {
	// Redirect os.Stdin to provide input
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = oldStdin })

	// Write "2\n" to select workspace 2
	go func() {
		w.Write([]byte("2\n"))
		w.Close()
	}()

	workspaces := []Workspace{
		{ID: "ws-1", Name: "Alpha"},
		{ID: "ws-2", Name: "Beta"},
		{ID: "ws-3", Name: "Gamma"},
	}
	got, err := SelectWorkspace(workspaces)
	require.NoError(t, err)
	assert.Equal(t, "ws-2", got.ID)
	assert.Equal(t, "Beta", got.Name)
}

func TestSelectWorkspace_OutOfRangeThenValid(t *testing.T) {
	// Redirect os.Stdin to provide out-of-range then valid input
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = oldStdin })

	// First "5" is out of range, then "1" is valid
	go func() {
		w.Write([]byte("5\n1\n"))
		w.Close()
	}()

	workspaces := []Workspace{
		{ID: "ws-1", Name: "Alpha"},
		{ID: "ws-2", Name: "Beta"},
	}
	got, err := SelectWorkspace(workspaces)
	require.NoError(t, err)
	assert.Equal(t, "ws-1", got.ID)
}

func TestSelectWorkspace_InvalidThenValid(t *testing.T) {
	// Test non-numeric input followed by valid
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = oldStdin })

	go func() {
		w.Write([]byte("abc\n1\n"))
		w.Close()
	}()

	workspaces := []Workspace{
		{ID: "ws-1", Name: "Alpha"},
		{ID: "ws-2", Name: "Beta"},
	}
	got, err := SelectWorkspace(workspaces)
	require.NoError(t, err)
	assert.Equal(t, "ws-1", got.ID)
}

func TestFetchWorkspaces_TrailingSlashURL(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/workspaces", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]Workspace{{ID: "ws-1", Name: "One"}})
	}))
	defer srv.Close()

	got, err := FetchWorkspaces(srv.URL+"/", "tok")
	require.NoError(t, err)
	assert.Len(t, got, 1)
}
