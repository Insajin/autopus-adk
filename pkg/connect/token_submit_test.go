package connect_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/insajin/autopus-adk/pkg/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubmitToken_Success(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		token       string
		workspaceID string
		provider    string
	}{
		{
			name:        "submit OpenAI token",
			token:       "sk-test-abc123",
			workspaceID: "ws-001",
			provider:    "openai",
		},
		{
			name:        "submit Anthropic token",
			token:       "sk-ant-test",
			workspaceID: "ws-002",
			provider:    "anthropic",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Given: a mock server that accepts the token submission
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPost, r.Method)
				expectedPath := "/api/v1/workspaces/" + tt.workspaceID + "/ai-oauth/callback"
				assert.Equal(t, expectedPath, r.URL.Path)
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
				assert.Equal(t, "Bearer auth-token", r.Header.Get("Authorization"))

				body, err := io.ReadAll(r.Body)
				require.NoError(t, err)
				var payload map[string]string
				require.NoError(t, json.Unmarshal(body, &payload))
				assert.Equal(t, tt.provider, payload["provider"])
				assert.Equal(t, tt.token, payload["access_token"])

				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			client := connect.NewClient("auth-token").WithServerURL(srv.URL)
			req := connect.SubmitTokenRequest{
				ProviderToken: tt.token,
				WorkspaceID:   tt.workspaceID,
				Provider:      tt.provider,
			}

			// When: submitting the token
			err := client.SubmitToken(context.Background(), req)

			// Then: no error
			require.NoError(t, err)
		})
	}
}

func TestSubmitToken_ServerError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		statusCode int
		body       string
		wantErr    string
	}{
		{
			name:       "500 internal server error",
			statusCode: http.StatusInternalServerError,
			body:       "internal error",
			wantErr:    "submit token failed (500)",
		},
		{
			name:       "403 forbidden",
			statusCode: http.StatusForbidden,
			body:       "forbidden",
			wantErr:    "submit token failed (403)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Given: a server that returns an error status
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			client := connect.NewClient("auth-token").WithServerURL(srv.URL)
			req := connect.SubmitTokenRequest{
				ProviderToken: "sk-test",
				WorkspaceID:   "ws-001",
				Provider:      "openai",
			}

			// When: submitting the token
			err := client.SubmitToken(context.Background(), req)

			// Then: error should contain status code
			require.Error(t, err)
			assert.ErrorContains(t, err, tt.wantErr)
		})
	}
}

// SPEC-OAUTHUX-001 AC-003: SubmitToken request body must include nonce field with valid UUID v4
func TestSubmitToken_IncludesNonce(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		workspaceID string
		provider    string
	}{
		{
			name:        "openai token submission includes nonce",
			workspaceID: "ws-001",
			provider:    "openai",
		},
		{
			name:        "anthropic token submission includes nonce",
			workspaceID: "ws-002",
			provider:    "anthropic",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var capturedNonce string

			// Given: a server that captures the request body
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				require.NoError(t, err)
				var payload map[string]string
				require.NoError(t, json.Unmarshal(body, &payload))

				// Capture nonce from request payload
				capturedNonce = payload["nonce"]

				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			client := connect.NewClient("auth-token").WithServerURL(srv.URL)
			req := connect.SubmitTokenRequest{
				ProviderToken: "sk-test-token",
				WorkspaceID:   tt.workspaceID,
				Provider:      tt.provider,
			}

			// When: submitting the token
			err := client.SubmitToken(context.Background(), req)

			// Then: no error and nonce is a valid UUID v4
			require.NoError(t, err)
			// Fails until nonce field is added to SubmitToken payload
			require.NotEmpty(t, capturedNonce, "nonce field must be present in request body")
			// Validate UUID v4 format: xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx
			assert.Regexp(t, `^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`,
				capturedNonce, "nonce must be a valid UUID v4")
		})
	}
}

func TestSubmitToken_CancelledContext(t *testing.T) {
	t.Parallel()

	// Given: a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("request should not reach server")
	}))
	defer srv.Close()

	client := connect.NewClient("auth-token").WithServerURL(srv.URL)
	req := connect.SubmitTokenRequest{
		ProviderToken: "sk-test",
		WorkspaceID:   "ws-001",
		Provider:      "openai",
	}

	// When: submitting with cancelled context
	err := client.SubmitToken(ctx, req)

	// Then: context error should be returned
	require.Error(t, err)
}
