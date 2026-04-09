package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testServerReconnecter stubs ServerReconnecter for reconnection tests.
type testServerReconnecter struct {
	mu             sync.Mutex
	setAuthCalls   []string
	reconnectCalls int
	reconnectErr   error
}

func (m *testServerReconnecter) SetAuthToken(token string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.setAuthCalls = append(m.setAuthCalls, token)
}

func (m *testServerReconnecter) ReconnectTransport(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reconnectCalls++
	return m.reconnectErr
}

// newRefreshServer starts an httptest server that responds with the given token.
func newRefreshServer(t *testing.T, token string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"data": map[string]any{
				"access_token":  token,
				"refresh_token": "new-refresh",
				"expires_in":    3600,
			},
		})
	}))
}

// newCredStoreWithCreds returns a mock store pre-loaded with near-expiry credentials.
func newCredStoreWithCreds(refreshToken string) *mockCredentialStore {
	store := newMockCredStore()
	creds := &Credentials{
		AccessToken:  "old-token",
		RefreshToken: refreshToken,
		ExpiresAt:    time.Now().Add(1 * time.Minute), // within 5-min refresh window
	}
	data, _ := json.MarshalIndent(creds, "", "  ")
	store.data["autopus-worker"] = string(data)
	return store
}

// TestReconnector_SequenceOrder verifies that Reconnect executes in order:
// 1. Refresh token (ForceRefresh)
// 2. SetAuthToken with the new token
// 3. ReconnectTransport
func TestReconnector_SequenceOrder(t *testing.T) {
	t.Parallel()

	fakeSrv := newRefreshServer(t, "refreshed-token")
	defer fakeSrv.Close()

	store := newCredStoreWithCreds("old-refresh")
	srv := &testServerReconnecter{}
	refresher := NewTokenRefresher(fakeSrv.URL, store, func() {}, nil)
	r := NewReconnector(refresher, srv)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := r.Reconnect(ctx)
	require.NoError(t, err)

	// SetAuthToken must have been called with the refreshed token.
	srv.mu.Lock()
	defer srv.mu.Unlock()
	require.Len(t, srv.setAuthCalls, 1, "SetAuthToken must be called exactly once")
	assert.Equal(t, "refreshed-token", srv.setAuthCalls[0], "token must be the refreshed value")

	// ReconnectTransport must have executed.
	assert.Equal(t, 1, srv.reconnectCalls, "ReconnectTransport must be called exactly once")
}

// TestReconnector_DuplicatePrevention verifies that concurrent Reconnect calls
// deduplicate to at most one active execution (FR-AUTH-05 no double reconnect).
func TestReconnector_DuplicatePrevention(t *testing.T) {
	t.Parallel()

	var reconnectCount atomic.Int32
	fakeSrv := newRefreshServer(t, "dedup-token")
	defer fakeSrv.Close()

	store := newCredStoreWithCreds("old-refresh")

	// Slow reconnect to maximise goroutine overlap.
	slowSrv := &slowServerReconnecter{delay: 50 * time.Millisecond, counter: &reconnectCount}
	refresher := NewTokenRefresher(fakeSrv.URL, store, func() {}, nil)
	r := NewReconnector(refresher, slowSrv)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	const goroutines = 10
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			_ = r.Reconnect(ctx)
		}()
	}
	wg.Wait()

	// Only one ReconnectTransport call must have occurred.
	assert.Equal(t, int32(1), reconnectCount.Load(),
		"concurrent Reconnect calls must be deduplicated to one execution")
}

// TestReconnector_ErrorPropagation verifies that a ForceRefresh failure
// propagates without calling SetAuthToken or ReconnectTransport.
func TestReconnector_ErrorPropagation(t *testing.T) {
	t.Parallel()

	// Empty store → LoadCredentials fails → ForceRefresh returns error.
	store := newMockCredStore()
	srv := &testServerReconnecter{}
	refresher := NewTokenRefresher("http://localhost:0", store, func() {}, nil)
	r := NewReconnector(refresher, srv)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := r.Reconnect(ctx)

	require.Error(t, err, "refresh error must propagate to Reconnect caller")

	srv.mu.Lock()
	defer srv.mu.Unlock()
	assert.Empty(t, srv.setAuthCalls, "SetAuthToken must not be called on refresh error")
	assert.Equal(t, 0, srv.reconnectCalls, "ReconnectTransport must not be called on refresh error")
}

// TestReconnector_SkipsRefreshIfTokenValid verifies that the full reconnect
// sequence succeeds and delivers the new token when near-expiry is detected.
func TestReconnector_SkipsRefreshIfTokenValid(t *testing.T) {
	t.Parallel()

	fakeSrv := newRefreshServer(t, "final-token")
	defer fakeSrv.Close()

	store := newCredStoreWithCreds("refresh-tok")
	var gotToken atomic.Value
	captureSrv := &captureSrv{onSetAuth: func(tok string) { gotToken.Store(tok) }}
	refresher := NewTokenRefresher(fakeSrv.URL, store, func() {}, nil)
	r := NewReconnector(refresher, captureSrv)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := r.Reconnect(ctx)
	require.NoError(t, err)

	val := gotToken.Load()
	require.NotNil(t, val, "SetAuthToken must receive the new token")
	assert.Equal(t, "final-token", val.(string))
	assert.Equal(t, int32(1), captureSrv.reconnectCalls.Load(),
		"ReconnectTransport must be called once")
}

// --- test helpers ---

// slowServerReconnecter introduces a delay to exercise deduplication.
type slowServerReconnecter struct {
	delay   time.Duration
	counter *atomic.Int32
}

func (s *slowServerReconnecter) SetAuthToken(string) {}
func (s *slowServerReconnecter) ReconnectTransport(_ context.Context) error {
	time.Sleep(s.delay)
	s.counter.Add(1)
	return nil
}

// captureSrv captures SetAuthToken and counts ReconnectTransport calls.
type captureSrv struct {
	onSetAuth      func(string)
	reconnectCalls atomic.Int32
}

func (c *captureSrv) SetAuthToken(token string) {
	if c.onSetAuth != nil {
		c.onSetAuth(token)
	}
}

func (c *captureSrv) ReconnectTransport(_ context.Context) error {
	c.reconnectCalls.Add(1)
	return nil
}
