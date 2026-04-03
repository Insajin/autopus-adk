package setup

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestDeviceCode_Success(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/v1/auth/device/code", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(DeviceCode{
			DeviceCode:      "dev-123",
			UserCode:        "ABCD-EFGH",
			VerificationURI: "https://auth.example.com/device",
			ExpiresIn:       600,
			Interval:        5,
		})
	}))
	defer srv.Close()

	dc, err := RequestDeviceCode(srv.URL, "test-verifier")
	require.NoError(t, err)
	assert.Equal(t, "dev-123", dc.DeviceCode)
	assert.Equal(t, "ABCD-EFGH", dc.UserCode)
}

func TestRequestDeviceCode_ServerError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	}))
	defer srv.Close()

	_, err := RequestDeviceCode(srv.URL, "test-verifier")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestRequestDeviceCode_InvalidJSON(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not-json"))
	}))
	defer srv.Close()

	_, err := RequestDeviceCode(srv.URL, "test-verifier")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode")
}

func TestRequestDeviceCode_WrappedResponse(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success":true,"data":{"device_code":"wrapped-dc","user_code":"WRAP-CODE","verification_uri":"https://auth.example.com","expires_in":600,"interval":5}}`))
	}))
	defer srv.Close()

	dc, err := RequestDeviceCode(srv.URL, "test-verifier")
	require.NoError(t, err)
	assert.Equal(t, "wrapped-dc", dc.DeviceCode)
}

func TestRequestDeviceCode_TrailingSlashURL(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/auth/device/code", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(DeviceCode{DeviceCode: "dc-slash"})
	}))
	defer srv.Close()

	dc, err := RequestDeviceCode(srv.URL+"/", "verifier")
	require.NoError(t, err)
	assert.Equal(t, "dc-slash", dc.DeviceCode)
}

func TestTryTokenExchange_Success(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(TokenResponse{
			AccessToken:  "new-tok",
			RefreshToken: "new-ref",
			ExpiresIn:    3600,
			TokenType:    "Bearer",
		})
	}))
	defer srv.Close()

	token, status, err := tryTokenExchange(srv.URL, "dev-code", "verifier")
	require.NoError(t, err)
	assert.Equal(t, pollDone, status)
	assert.Equal(t, "new-tok", token.AccessToken)
}

func TestTryTokenExchange_Pending(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"authorization_pending"}`))
	}))
	defer srv.Close()

	token, status, err := tryTokenExchange(srv.URL, "dev-code", "verifier")
	require.NoError(t, err)
	assert.Equal(t, pollPending, status)
	assert.Nil(t, token)
}

func TestTryTokenExchange_SlowDown(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error":"slow_down"}`))
	}))
	defer srv.Close()

	token, status, err := tryTokenExchange(srv.URL, "dev-code", "verifier")
	require.NoError(t, err)
	assert.Equal(t, pollSlowDown, status)
	assert.Nil(t, token)
}

func TestTryTokenExchange_ServerError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	}))
	defer srv.Close()

	_, _, err := tryTokenExchange(srv.URL, "dev-code", "verifier")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestTryTokenExchange_WrappedResponse(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success":true,"data":{"access_token":"wrapped-tok","refresh_token":"ref","expires_in":3600,"token_type":"Bearer"}}`))
	}))
	defer srv.Close()

	token, status, err := tryTokenExchange(srv.URL, "dev-code", "verifier")
	require.NoError(t, err)
	assert.Equal(t, pollDone, status)
	assert.Equal(t, "wrapped-tok", token.AccessToken)
}

func TestTryTokenExchange_BadRequestUnknownError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"expired_token"}`))
	}))
	defer srv.Close()

	_, _, err := tryTokenExchange(srv.URL, "dev-code", "verifier")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "400")
}

func TestPollForToken_ContextCancelled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := PollForToken(ctx, "http://localhost", "dev-code", "verifier", 1)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestPollForToken_SuccessAfterPending(t *testing.T) {
	t.Parallel()

	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount <= 1 {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error":"authorization_pending"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(TokenResponse{
			AccessToken:  "polled-tok",
			RefreshToken: "polled-ref",
			ExpiresIn:    3600,
			TokenType:    "Bearer",
		})
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	token, err := PollForToken(ctx, srv.URL, "dev-code", "verifier", 1)
	require.NoError(t, err)
	assert.Equal(t, "polled-tok", token.AccessToken)
	assert.GreaterOrEqual(t, callCount, 2)
}

func TestPollForToken_DefaultInterval(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := PollForToken(ctx, "http://localhost", "dev-code", "verifier", 0)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestPollForToken_SlowDownThenSuccess(t *testing.T) {
	t.Parallel()

	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":"slow_down"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(TokenResponse{
			AccessToken: "slow-tok",
			ExpiresIn:   3600,
			TokenType:   "Bearer",
		})
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	token, err := PollForToken(ctx, srv.URL, "dev-code", "verifier", 1)
	require.NoError(t, err)
	assert.Equal(t, "slow-tok", token.AccessToken)
}

func TestPollForToken_ServerError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("error"))
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := PollForToken(ctx, srv.URL, "dev-code", "verifier", 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}
