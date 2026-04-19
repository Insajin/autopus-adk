// Package setup — HTTP-based coverage tests using httptest.
package setup

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFindWorkspaceByID_Found verifies workspace lookup by ID.
func TestFindWorkspaceByID_Found(t *testing.T) {
	t.Parallel()

	// Given: a server returning a list of workspaces
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/workspaces", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		data, _ := json.Marshal(map[string]any{
			"data": []Workspace{
				{ID: "ws-001", Name: "Alpha"},
				{ID: "ws-002", Name: "Beta"},
			},
		})
		_, _ = w.Write(data)
	}))
	defer srv.Close()

	// When: FindWorkspaceByID is called with existing ID
	ws, err := FindWorkspaceByID(srv.URL, "test-token", "ws-001")

	// Then: the matching workspace is returned
	require.NoError(t, err)
	require.NotNil(t, ws)
	assert.Equal(t, "ws-001", ws.ID)
	assert.Equal(t, "Alpha", ws.Name)
}

// TestFindWorkspaceByID_NotFound verifies error when ID not in list.
func TestFindWorkspaceByID_NotFound(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		data, _ := json.Marshal(map[string]any{
			"data": []Workspace{{ID: "ws-001", Name: "Alpha"}},
		})
		_, _ = w.Write(data)
	}))
	defer srv.Close()

	_, err := FindWorkspaceByID(srv.URL, "test-token", "ws-999")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ws-999")
}

// TestFindWorkspaceByID_ServerError verifies error propagation on HTTP failure.
func TestFindWorkspaceByID_ServerError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := FindWorkspaceByID(srv.URL, "test-token", "ws-001")
	require.Error(t, err)
}

// TestCreateWorkerAPIKey_Success verifies successful key creation.
func TestCreateWorkerAPIKey_Success(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Contains(t, r.URL.Path, "/worker-keys")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		data, _ := json.Marshal(map[string]any{
			"data": workerKeyResponse{
				ID:   "key-001",
				Name: "adk-worker",
				Key:  "acos_worker_testkey123",
			},
		})
		_, _ = w.Write(data)
	}))
	defer srv.Close()

	key, err := CreateWorkerAPIKey(srv.URL, "test-jwt", "ws-001")
	require.NoError(t, err)
	assert.Equal(t, "acos_worker_testkey123", key)
}

// TestCreateWorkerAPIKey_NonCreatedStatus verifies error on non-201 response.
func TestCreateWorkerAPIKey_NonCreatedStatus(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "forbidden", http.StatusForbidden)
	}))
	defer srv.Close()

	_, err := CreateWorkerAPIKey(srv.URL, "bad-jwt", "ws-001")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "403")
}

// TestCreateWorkerAPIKey_EmptyKey verifies error when backend returns empty key.
func TestCreateWorkerAPIKey_EmptyKey(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		data, _ := json.Marshal(map[string]any{
			"data": workerKeyResponse{ID: "key-001", Name: "adk-worker", Key: ""},
		})
		_, _ = w.Write(data)
	}))
	defer srv.Close()

	_, err := CreateWorkerAPIKey(srv.URL, "test-jwt", "ws-001")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty key")
}

// TestCreateWorkerAPIKey_InvalidURL verifies error on invalid backend URL.
func TestCreateWorkerAPIKey_InvalidURL(t *testing.T) {
	t.Parallel()

	_, err := CreateWorkerAPIKey("http://127.0.0.1:0", "test-jwt", "ws-001")
	require.Error(t, err)
}
