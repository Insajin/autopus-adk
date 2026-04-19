package connect_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/insajin/autopus-adk/pkg/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListWorkspaces_Success(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		response  []connect.Workspace
		wantCount int
	}{
		{
			name: "single workspace",
			response: []connect.Workspace{
				{ID: "ws-001", Name: "Production", Description: "Main workspace"},
			},
			wantCount: 1,
		},
		{
			name: "multiple workspaces",
			response: []connect.Workspace{
				{ID: "ws-001", Name: "Production"},
				{ID: "ws-002", Name: "Staging"},
				{ID: "ws-003", Name: "Development"},
			},
			wantCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Given: a mock server that returns workspace data
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/api/v1/workspaces", r.URL.Path)
				assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(tt.response)
			}))
			defer srv.Close()

			client := connect.NewClient("test-token").WithServerURL(srv.URL)

			// When: listing workspaces
			workspaces, err := client.ListWorkspaces(context.Background())

			// Then: correct workspaces should be returned
			require.NoError(t, err)
			assert.Len(t, workspaces, tt.wantCount)
			assert.Equal(t, tt.response[0].ID, workspaces[0].ID)
			assert.Equal(t, tt.response[0].Name, workspaces[0].Name)
		})
	}
}

func TestListWorkspaces_Unauthorized(t *testing.T) {
	t.Parallel()

	// Given: a server that returns 401
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	client := connect.NewClient("bad-token").WithServerURL(srv.URL)

	// When: listing workspaces
	_, err := client.ListWorkspaces(context.Background())

	// Then: unauthorized error should be returned
	require.Error(t, err)
	assert.ErrorContains(t, err, "unauthorized")
}

func TestListWorkspaces_EmptyResponse(t *testing.T) {
	t.Parallel()

	// Given: a server that returns empty array
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]connect.Workspace{})
	}))
	defer srv.Close()

	client := connect.NewClient("test-token").WithServerURL(srv.URL)

	// When: listing workspaces
	workspaces, err := client.ListWorkspaces(context.Background())

	// Then: empty slice returned without error
	require.NoError(t, err)
	assert.Empty(t, workspaces)
}

func TestListWorkspaces_ServerError(t *testing.T) {
	t.Parallel()

	// Given: a server that returns 500
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal error"))
	}))
	defer srv.Close()

	client := connect.NewClient("test-token").WithServerURL(srv.URL)

	// When: listing workspaces
	_, err := client.ListWorkspaces(context.Background())

	// Then: error with status code should be returned
	require.Error(t, err)
	assert.ErrorContains(t, err, "500")
}

func TestListWorkspaces_CancelledContext(t *testing.T) {
	t.Parallel()

	// Given: a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("request should not reach server")
	}))
	defer srv.Close()

	client := connect.NewClient("test-token").WithServerURL(srv.URL)

	// When: listing workspaces with cancelled context
	_, err := client.ListWorkspaces(ctx)

	// Then: context cancelled error
	require.Error(t, err)
}

func TestListWorkspaces_InvalidJSON(t *testing.T) {
	t.Parallel()

	// Given: a server that returns invalid JSON with 200 status
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not valid json"))
	}))
	defer srv.Close()

	client := connect.NewClient("test-token").WithServerURL(srv.URL)

	// When: listing workspaces
	_, err := client.ListWorkspaces(context.Background())

	// Then: decode error should be returned
	require.Error(t, err)
	assert.ErrorContains(t, err, "decode workspaces")
}

func TestNewClient_SetsAuthToken(t *testing.T) {
	t.Parallel()

	// Given: a mock server checking auth header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer my-secret-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]connect.Workspace{})
	}))
	defer srv.Close()

	// When: creating a client with a specific token
	client := connect.NewClient("my-secret-token").WithServerURL(srv.URL)
	_, err := client.ListWorkspaces(context.Background())

	// Then: requests should include the auth header
	require.NoError(t, err)
}
