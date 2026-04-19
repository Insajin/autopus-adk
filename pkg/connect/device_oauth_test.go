package connect_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/connect"
)

func TestRequestOpenAIDeviceCode_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Contains(t, r.URL.Path, "/ai-oauth/device-code")
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		resp := connect.DeviceCodeResponse{
			DeviceCode:      "dc_abc123",
			UserCode:        "ABCD-1234",
			VerificationURI: "https://auth.example.com/device",
			ExpiresIn:       300,
			Interval:        5,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp) //nolint:errcheck
	}))
	defer srv.Close()

	ctx := context.Background()
	result, err := connect.RequestOpenAIDeviceCode(ctx, srv.URL, "test-token", "ws-001")

	require.NoError(t, err)
	assert.Equal(t, "dc_abc123", result.DeviceCode)
	assert.Equal(t, "ABCD-1234", result.UserCode)
	assert.Equal(t, "https://auth.example.com/device", result.VerificationURI)
	assert.Equal(t, 300, result.ExpiresIn)
	assert.Equal(t, 5, result.Interval)
}

func TestRequestOpenAIDeviceCode_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotImplemented)
		_, _ = w.Write([]byte("not implemented")) //nolint:errcheck
	}))
	defer srv.Close()

	ctx := context.Background()
	result, err := connect.RequestOpenAIDeviceCode(ctx, srv.URL, "tok", "ws-001")

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "server does not support device code flow")
}

func TestRequestOpenAIDeviceCode_WrappedResponse(t *testing.T) {
	// Verify that the standard { success, data } envelope is handled.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inner := connect.DeviceCodeResponse{
			DeviceCode:      "dc_wrapped",
			UserCode:        "WRAP-0001",
			VerificationURI: "https://verify.example.com",
			ExpiresIn:       600,
			Interval:        10,
		}
		innerJSON, _ := json.Marshal(inner)
		envelope := map[string]interface{}{
			"success": true,
			"data":    json.RawMessage(innerJSON),
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(envelope) //nolint:errcheck
	}))
	defer srv.Close()

	ctx := context.Background()
	result, err := connect.RequestOpenAIDeviceCode(ctx, srv.URL, "tok", "ws-002")

	require.NoError(t, err)
	assert.Equal(t, "dc_wrapped", result.DeviceCode)
}

func TestPollOpenAIDeviceToken_CompletesOnSecondPoll(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		var status string
		var token string
		if callCount < 2 {
			status = "pending"
		} else {
			status = "completed"
			token = "access-token-xyz"
		}
		resp := connect.DeviceTokenResponse{
			Status:      status,
			AccessToken: token,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp) //nolint:errcheck
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Use interval=1 to avoid 5s default wait in tests.
	result, err := connect.PollOpenAIDeviceToken(ctx, srv.URL, "tok", "ws-001", "dc_abc", 1)

	require.NoError(t, err)
	assert.Equal(t, "completed", result.Status)
	assert.Equal(t, "access-token-xyz", result.AccessToken)
	assert.Equal(t, 2, callCount)
}

func TestPollOpenAIDeviceToken_Expired(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := connect.DeviceTokenResponse{Status: "expired"}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp) //nolint:errcheck
	}))
	defer srv.Close()

	ctx := context.Background()
	// Use interval=1 to avoid 5s default wait in tests.
	result, err := connect.PollOpenAIDeviceToken(ctx, srv.URL, "tok", "ws-001", "dc_expired", 1)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "device code expired")
}

func TestPollOpenAIDeviceToken_ContextCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := connect.DeviceTokenResponse{Status: "pending"}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp) //nolint:errcheck
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := connect.PollOpenAIDeviceToken(ctx, srv.URL, "tok", "ws-001", "dc_pending", 0)
	assert.Error(t, err)
}
