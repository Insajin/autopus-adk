package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDispatcher_FetchesSchedules(t *testing.T) {
	t.Parallel()

	var gotPath, gotAuth string
	var mu sync.Mutex
	requested := make(chan struct{}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
		// Backend returns wrapped response: { success: true, data: [...] }
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"data":    []schedule{},
		})
		requested <- struct{}{}
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	d := NewDispatcher(srv.URL, "mytoken", "ws-42", time.UTC, func(string, string) {})
	done := make(chan struct{})
	go func() {
		d.Start(ctx)
		close(done)
	}()

	select {
	case <-requested:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for schedule request")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("dispatcher did not stop after cancellation")
	}

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, "/api/v1/workspaces/ws-42/schedules", gotPath)
	assert.Equal(t, "Bearer mytoken", gotAuth)
}

func TestDispatcher_TriggersMatchingSchedule(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Backend returns wrapped response: { success: true, data: [...] }
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"data":    []schedule{{ID: "s1", CronExpr: "* * * * *", TaskPayload: "payload1"}},
		})
	}))
	defer srv.Close()

	type triggerEvent struct {
		id      string
		payload string
	}
	triggered := make(chan triggerEvent, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	d := NewDispatcher(srv.URL, "tok", "ws", time.UTC, func(id, payload string) {
		triggered <- triggerEvent{id: id, payload: payload}
	})
	done := make(chan struct{})
	go func() {
		d.Start(ctx)
		close(done)
	}()

	select {
	case event := <-triggered:
		assert.Equal(t, "s1", event.id)
		assert.Equal(t, "payload1", event.payload)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for matching schedule trigger")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("dispatcher did not stop after cancellation")
	}
}

func TestDispatcher_Deduplication(t *testing.T) {
	t.Parallel()

	now := time.Now().In(time.UTC)
	cronExpr := minuteMatchingCron(now)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Backend returns wrapped response: { success: true, data: [...] }
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"data":    []schedule{{ID: "s1", CronExpr: cronExpr, TaskPayload: "p"}},
		})
	}))
	defer srv.Close()

	var count int
	var mu sync.Mutex
	d := NewDispatcher(srv.URL, "tok", "ws", time.UTC, func(string, string) {
		mu.Lock()
		count++
		mu.Unlock()
	})

	// Call tick twice within the same minute.
	ctx := context.Background()
	d.tick(ctx)
	d.tick(ctx)

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, 1, count, "should trigger only once per minute")
}

func TestDispatcher_TimezoneHandling(t *testing.T) {
	t.Parallel()

	// Use a fixed timezone offset.
	loc := time.FixedZone("TEST", 9*3600) // UTC+9
	now := time.Now().In(loc)
	cronExpr := minuteMatchingCron(now)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Backend returns wrapped response: { success: true, data: [...] }
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"data":    []schedule{{ID: "tz1", CronExpr: cronExpr, TaskPayload: "tz-payload"}},
		})
	}))
	defer srv.Close()

	var triggered bool
	var mu sync.Mutex
	d := NewDispatcher(srv.URL, "tok", "ws", loc, func(string, string) {
		mu.Lock()
		triggered = true
		mu.Unlock()
	})
	d.tick(context.Background())

	mu.Lock()
	defer mu.Unlock()
	assert.True(t, triggered, "should trigger when matching in the configured timezone")
}

func TestDispatcher_SetAuthToken_UpdatesHeader(t *testing.T) {
	t.Parallel()

	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"data":    []schedule{},
		})
	}))
	defer srv.Close()

	d := NewDispatcher(srv.URL, "old-token", "ws-42", time.UTC, func(string, string) {})
	d.SetAuthToken("new-token")

	_, err := d.fetchSchedules(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "Bearer new-token", gotAuth)
}

// minuteMatchingCron returns a cron expression that matches the given time's
// minute, hour, dom, month, and dow.
func minuteMatchingCron(t time.Time) string {
	return fmt.Sprintf("%d %d %d %d %d",
		t.Minute(), t.Hour(), t.Day(), int(t.Month()), int(t.Weekday()))
}
