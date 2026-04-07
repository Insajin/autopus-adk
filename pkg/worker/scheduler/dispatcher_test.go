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
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	d := NewDispatcher(srv.URL, "mytoken", "ws-42", time.UTC, func(string, string) {})
	d.Start(ctx)

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, "/api/v1/workspaces/ws-42/schedules", gotPath)
	assert.Equal(t, "Bearer mytoken", gotAuth)
}

func TestDispatcher_TriggersMatchingSchedule(t *testing.T) {
	t.Parallel()

	now := time.Now().In(time.UTC)
	cronExpr := minuteMatchingCron(now)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Backend returns wrapped response: { success: true, data: [...] }
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"data":    []schedule{{ID: "s1", CronExpr: cronExpr, TaskPayload: "payload1"}},
		})
	}))
	defer srv.Close()

	var triggered sync.Map
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	d := NewDispatcher(srv.URL, "tok", "ws", time.UTC, func(id, payload string) {
		triggered.Store(id, payload)
	})
	d.Start(ctx)

	val, ok := triggered.Load("s1")
	require.True(t, ok, "schedule s1 should have been triggered")
	assert.Equal(t, "payload1", val)
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

// minuteMatchingCron returns a cron expression that matches the given time's
// minute, hour, dom, month, and dow.
func minuteMatchingCron(t time.Time) string {
	return fmt.Sprintf("%d %d %d %d %d",
		t.Minute(), t.Hour(), t.Day(), int(t.Month()), int(t.Weekday()))
}
