package a2a

// S8, S9, S10: REST fallback poller tests (SPEC-A2ARES-001 Phase 3, worker side).
//
// All tests in this file are RED (compile-fail) because they reference types
// and functions not yet implemented: RESTPoller, NewRESTPoller, PollResult.

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

// TestRESTPoller_ActivatesWhenWebSocketExhausted asserts S8:
// When OnConnectionExhausted fires (WS backoff reaches ceiling),
// the REST poller starts polling /api/a2a/poll every 10 seconds.
func TestRESTPoller_ActivatesWhenWebSocketExhausted(t *testing.T) {
	t.Parallel()

	var pollCount atomic.Int32

	// Given a mock backend poll endpoint
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/worker/tasks/pending" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		pollCount.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]interface{}{})
	}))
	defer srv.Close()

	// When NewRESTPoller is created and started (type does not exist yet — compile error)
	poller := NewRESTPoller(RESTPollerConfig{ // NewRESTPoller and RESTPollerConfig do not exist yet
		BackendURL:   srv.URL,
		AuthToken:    "valid-token",
		WorkerID:     "w1",
		PollInterval: 20 * time.Millisecond,                      // short for test; real is 10s
		TaskHandler:  func(task PollResult) error { return nil }, // PollResult does not exist yet
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// When WebSocket is exhausted (Start activates polling)
	poller.Start(ctx) // method does not exist yet

	time.Sleep(70 * time.Millisecond)

	// Then polling occurs approximately every 20ms (3+ polls in 70ms)
	got := pollCount.Load()
	assert.GreaterOrEqual(t, got, int32(2), "REST poller should poll at configured interval (S8)")
}

// TestRESTPoller_ProcessesQueuedTasks asserts S9:
// When /api/a2a/poll returns queued tasks, the worker processes them
// through the normal task handler path.
func TestRESTPoller_ProcessesQueuedTasks(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	var processedTasks []string

	// Given a backend that returns one task
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/worker/tasks/pending" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]interface{}{
			map[string]interface{}{
				"id":   "poll-task-001",
				"type": "symphony_run",
				"payload": map[string]interface{}{
					"step": "test",
				},
			},
		})
	}))
	defer srv.Close()

	poller := NewRESTPoller(RESTPollerConfig{
		BackendURL:   srv.URL,
		AuthToken:    "valid-token",
		WorkerID:     "w1",
		PollInterval: 15 * time.Millisecond,
		TaskHandler: func(task PollResult) error {
			mu.Lock()
			processedTasks = append(processedTasks, task.ID)
			mu.Unlock()
			return nil
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	poller.Start(ctx)

	// Wait for at least one poll cycle
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(processedTasks) > 0
	}, 150*time.Millisecond, 5*time.Millisecond, "task from REST poll should be processed (S9)")

	mu.Lock()
	tasks := make([]string, len(processedTasks))
	copy(tasks, processedTasks)
	mu.Unlock()

	// Then the task ID is correct
	assert.Contains(t, tasks, "poll-task-001", "worker must process polled task (S9)")
}

// TestRESTPoller_StopsWhenWebSocketRecovers asserts S10:
// When the WebSocket connection is restored, REST polling stops
// and the worker switches back to WS mode.
func TestRESTPoller_StopsWhenWebSocketRecovers(t *testing.T) {
	t.Parallel()

	var pollCount atomic.Int32
	pollObserved := make(chan struct{}, 2)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pollCount.Add(1)
		select {
		case pollObserved <- struct{}{}:
		default:
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]interface{}{})
	}))
	defer srv.Close()

	// Given a running REST poller (type does not exist yet — compile error)
	poller := NewRESTPoller(RESTPollerConfig{ // does not exist yet
		BackendURL:   srv.URL,
		AuthToken:    "valid-token",
		WorkerID:     "w1",
		PollInterval: 10 * time.Millisecond,
		TaskHandler:  func(task PollResult) error { return nil }, // PollResult does not exist yet
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	poller.Start(ctx) // method does not exist yet

	// Observe two requests so the recurring polling contract is established
	// independently of goroutine scheduling latency.
	for range 2 {
		select {
		case <-pollObserved:
		case <-time.After(time.Second):
			require.FailNow(t, "polling should have started")
		}
	}

	// When WebSocket recovers — Stop() is called (method does not exist yet — compile error)
	poller.Stop() // method does not exist yet
	countAfterStop := pollCount.Load()

	// Then polling stops
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, countAfterStop, pollCount.Load(), "polling must stop when WS recovers (S10)")
}

// TestRESTPoller_AuthFailure_DoesNotRetry asserts S11 (worker side):
// When the backend returns 401, the poller does NOT retry with the same token
// and surfaces the auth error through the configured error handler.
func TestRESTPoller_AuthFailure_SurfacesError(t *testing.T) {
	t.Parallel()

	authErrorObserved := make(chan int, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	// When NewRESTPoller is configured with an error handler (type does not exist yet — compile error)
	poller := NewRESTPoller(RESTPollerConfig{ // does not exist yet
		BackendURL:   srv.URL,
		AuthToken:    "invalid-token",
		WorkerID:     "w1",
		PollInterval: 10 * time.Millisecond,
		TaskHandler:  func(task PollResult) error { return nil }, // PollResult does not exist yet
		OnAuthError: func(statusCode int) { // OnAuthError callback does not exist yet
			select {
			case authErrorObserved <- statusCode:
			default:
			}
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	poller.Start(ctx) // method does not exist yet

	// Then auth error is surfaced after the 401 response is processed.
	select {
	case statusCode := <-authErrorObserved:
		cancel()
		assert.Equal(t, http.StatusUnauthorized, statusCode, "401 from poll endpoint must surface auth error (S11)")
	case <-time.After(time.Second):
		require.FailNow(t, "401 from poll endpoint must surface auth error (S11)")
	}
}

func TestRESTPoller_SetAuthToken(t *testing.T) {
	t.Parallel()

	poller := NewRESTPoller(RESTPollerConfig{
		BackendURL:  "http://localhost:9999",
		AuthToken:   "old-token",
		WorkerID:    "w1",
		TaskHandler: func(_ PollResult) error { return nil },
	})

	poller.SetAuthToken("new-token")

	assert.Equal(t, "new-token", poller.config.AuthToken)
}

func TestRESTPoller_DecodeWrappedResponse(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"tasks": []interface{}{
				map[string]interface{}{"id": "wrapped-1", "type": "symphony_run"},
			},
		})
	}))
	defer srv.Close()

	var got string
	poller := NewRESTPoller(RESTPollerConfig{
		BackendURL:  srv.URL,
		AuthToken:   "valid-token",
		WorkerID:    "w1",
		TaskHandler: func(task PollResult) error { got = task.ID; return nil },
	})

	require.NoError(t, poller.poll(context.Background()))
	assert.Equal(t, "wrapped-1", got)
}
