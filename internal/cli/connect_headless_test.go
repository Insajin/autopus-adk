package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/connect"
)

// captureEvents redirects EmitEvent output to a buffer and returns a cleanup func.
func captureEvents(t *testing.T) (*bytes.Buffer, func()) {
	t.Helper()
	buf := &bytes.Buffer{}
	orig := connect.SwapEmitWriter(buf)
	return buf, func() { connect.SwapEmitWriter(orig) }
}

// parseEvents parses NDJSON lines from buf into a slice of HeadlessEvent.
func parseEvents(t *testing.T, buf *bytes.Buffer) []connect.HeadlessEvent {
	t.Helper()
	var events []connect.HeadlessEvent
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}
		var ev connect.HeadlessEvent
		require.NoError(t, json.Unmarshal([]byte(line), &ev), "bad NDJSON line: %s", line)
		events = append(events, ev)
	}
	return events
}

// newTestConnectCmd wraps a cobra.Command for use in tests.
func newTestConnectCmd() *cobra.Command {
	return newConnectCmd()
}

// headlessTestServer creates a mock backend that handles all 3 headless steps.
func headlessTestServer(t *testing.T, workspaceID, workspaceName string) *httptest.Server {
	t.Helper()
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		// Step 1: device code
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/auth/device/code"):
			json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
				"device_code":      "dc_test",
				"user_code":        "TEST-CODE",
				"verification_uri": "https://verify.example.com",
				"expires_in":       300,
				"interval":         1, // 1s to keep tests fast
			})

		// Step 1: token poll
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/auth/device/token"):
			json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
				"access_token":  "server-tok-abc",
				"refresh_token": "refresh-abc",
				"token_type":    "Bearer",
				"expires_in":    3600,
			})

		// Step 2: workspace list
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/workspaces"):
			json.NewEncoder(w).Encode([]connect.Workspace{ //nolint:errcheck
				{ID: workspaceID, Name: workspaceName},
			})

		// Step 3: OpenAI device code
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "ai-oauth/device-code"):
			json.NewEncoder(w).Encode(connect.DeviceCodeResponse{ //nolint:errcheck
				DeviceCode:      "openai-dc",
				UserCode:        "OPEN-CODE",
				VerificationURI: "https://openai.example.com/device",
				ExpiresIn:       300,
				Interval:        1, // 1s to keep tests fast
			})

		// Step 3: OpenAI token poll
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "ai-oauth/device-token"):
			callCount++
			status := "pending"
			if callCount >= 2 {
				status = "completed"
			}
			json.NewEncoder(w).Encode(connect.DeviceTokenResponse{ //nolint:errcheck
				Status:      status,
				AccessToken: "openai-access-tok",
			})

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	return srv
}

func TestRunHeadlessConnect_HappyPath(t *testing.T) {
	buf, cleanup := captureEvents(t)
	defer cleanup()

	wsID := "ws-test-001"
	wsName := "Test Workspace"
	srv := headlessTestServer(t, wsID, wsName)
	defer srv.Close()

	cmd := newTestConnectCmd()
	cmd.SetContext(context.Background())

	err := runHeadlessConnect(cmd, srv.URL, wsID, 30*time.Second)
	require.NoError(t, err)

	events := parseEvents(t, buf)
	require.GreaterOrEqual(t, len(events), 6, "expected at least 6 events")

	// Verify event sequence.
	assert.Equal(t, "server_auth", events[0].Step)
	assert.Equal(t, "login_required", events[0].Action)
	assert.NotEmpty(t, events[0].URL)

	assert.Equal(t, "server_auth", events[1].Step)
	assert.Equal(t, "success", events[1].Status)

	assert.Equal(t, "workspace", events[2].Step)
	assert.Equal(t, "success", events[2].Status)
	assert.Equal(t, wsID, events[2].WorkspaceID)
	assert.Equal(t, wsName, events[2].WorkspaceName)

	assert.Equal(t, "openai_oauth", events[3].Step)
	assert.Equal(t, "login_required", events[3].Action)
	assert.NotEmpty(t, events[3].URL)

	// Find the final complete event.
	last := events[len(events)-1]
	assert.Equal(t, "complete", last.Step)
	assert.Equal(t, "success", last.Status)
	assert.Equal(t, wsID, last.WorkspaceID)
	assert.Equal(t, "openai", last.Provider)
}

func TestRunHeadlessConnect_WorkspaceNotFound(t *testing.T) {
	buf, cleanup := captureEvents(t)
	defer cleanup()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/auth/device/code"):
			json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
				"device_code": "dc_x", "user_code": "XX-00",
				"verification_uri": "https://v.example.com", "expires_in": 300, "interval": 1,
			})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/auth/device/token"):
			json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
				"access_token": "tok", "token_type": "Bearer",
			})
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/workspaces"):
			// Return a workspace with a different ID.
			json.NewEncoder(w).Encode([]connect.Workspace{ //nolint:errcheck
				{ID: "ws-other", Name: "Other"},
			})
		}
	}))
	defer srv.Close()

	cmd := newTestConnectCmd()
	cmd.SetContext(context.Background())

	err := runHeadlessConnect(cmd, srv.URL, "ws-nonexistent", 10*time.Second)
	assert.Error(t, err)

	events := parseEvents(t, buf)
	wsErrEvent := findEventByStep(events, "workspace")
	require.NotNil(t, wsErrEvent)
	assert.Equal(t, "error", wsErrEvent.Status)
	assert.Contains(t, wsErrEvent.Error, "not found")
}

func TestRunHeadlessConnect_OpenAIDeviceCodeFails(t *testing.T) {
	buf, cleanup := captureEvents(t)
	defer cleanup()

	wsID := "ws-001"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/auth/device/code"):
			json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
				"device_code": "dc_x", "user_code": "AB-00",
				"verification_uri": "https://v.example.com", "expires_in": 300, "interval": 1,
			})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/auth/device/token"):
			json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
				"access_token": "tok", "token_type": "Bearer",
			})
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/workspaces"):
			json.NewEncoder(w).Encode([]connect.Workspace{{ID: wsID, Name: "My WS"}}) //nolint:errcheck
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "ai-oauth/device-code"):
			// Simulate server not supporting device code flow.
			w.WriteHeader(http.StatusNotImplemented)
			w.Write([]byte("not supported")) //nolint:errcheck
		}
	}))
	defer srv.Close()

	cmd := newTestConnectCmd()
	cmd.SetContext(context.Background())

	err := runHeadlessConnect(cmd, srv.URL, wsID, 10*time.Second)
	assert.Error(t, err)

	events := parseEvents(t, buf)
	oauthErrEvent := findEventByStep(events, "openai_oauth")
	require.NotNil(t, oauthErrEvent)
	assert.Equal(t, "error", oauthErrEvent.Status)
}

// findEventByStep returns the first event with the given step and an error status.
func findEventByStep(events []connect.HeadlessEvent, step string) *connect.HeadlessEvent {
	for i := range events {
		if events[i].Step == step && events[i].Status == "error" {
			return &events[i]
		}
	}
	return nil
}
