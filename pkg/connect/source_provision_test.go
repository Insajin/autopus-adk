package connect

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProvisionBridgeSource_Success(t *testing.T) {
	t.Parallel()

	step := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		switch step {
		case 0:
			// POST /knowledge/sources
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Contains(t, r.URL.Path, "/knowledge/sources")

			var req createSourceRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "bridge_sync", req.Type)
			assert.Contains(t, req.Name, "ADK Worker")

			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(apiEnvelope{
				Success: true,
				Data:    json.RawMessage(`{"id":"src-001","name":"ADK Worker","type":"bridge_sync"}`),
			})
			step++

		case 1:
			// PUT /bridge/binding
			assert.Equal(t, http.MethodPut, r.Method)
			assert.Contains(t, r.URL.Path, "/bridge/binding")

			var req bridgeBindingRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "1.0", req.ManifestVersion)
			assert.Equal(t, "push", req.SyncMode)
			assert.Equal(t, "/tmp/work", req.WorkspaceRoot)

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
			step++
		}
	}))
	defer srv.Close()

	client := NewClient("test-token").WithServerURL(srv.URL)
	sourceID, err := client.ProvisionBridgeSource(context.Background(), "ws-123", "/tmp/work")

	require.NoError(t, err)
	assert.Equal(t, "src-001", sourceID)
	assert.Equal(t, 2, step, "both API calls should have been made")
}

func TestProvisionBridgeSource_CreateFails(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("no access"))
	}))
	defer srv.Close()

	client := NewClient("tok").WithServerURL(srv.URL)
	_, err := client.ProvisionBridgeSource(context.Background(), "ws-1", "/tmp")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "create")
}

func TestProvisionBridgeSource_BindingFails(t *testing.T) {
	t.Parallel()

	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			// Source creation succeeds.
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(apiEnvelope{
				Success: true,
				Data:    json.RawMessage(`{"id":"src-002","name":"test","type":"bridge_sync"}`),
			})
			return
		}
		// Binding fails.
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("binding error"))
	}))
	defer srv.Close()

	client := NewClient("tok").WithServerURL(srv.URL)
	_, err := client.ProvisionBridgeSource(context.Background(), "ws-1", "/tmp")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "binding")
}
