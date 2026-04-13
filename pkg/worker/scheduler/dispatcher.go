package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"
)

const (
	fetchInterval = 60 * time.Second
	httpTimeout   = 10 * time.Second
	maxBodyBytes  = 10 << 20 // 10MB
)

// schedule represents a single schedule entry from the backend API.
type schedule struct {
	ID              string `json:"id"`
	CronExpr        string `json:"cron_expression"`
	TaskPayload     string `json:"task_payload"`
	TargetAgentType string `json:"target_agent_type"`
}

// Dispatcher fetches schedules from the backend and triggers local execution.
type Dispatcher struct {
	backendURL  string
	authToken   string
	workspaceID string
	location    *time.Location
	lastTrigger map[string]time.Time
	onTrigger   func(scheduleID, taskPayload string)
	client      *http.Client
	mu          sync.RWMutex
}

// NewDispatcher creates a dispatcher that evaluates cron schedules in the given timezone.
func NewDispatcher(
	backendURL, authToken, workspaceID string,
	loc *time.Location,
	onTrigger func(string, string),
) *Dispatcher {
	return &Dispatcher{
		backendURL:  backendURL,
		authToken:   authToken,
		workspaceID: workspaceID,
		location:    loc,
		lastTrigger: make(map[string]time.Time),
		onTrigger:   onTrigger,
		client:      &http.Client{Timeout: httpTimeout},
	}
}

// Start runs the dispatch loop, fetching schedules every 60s and triggering matches.
func (d *Dispatcher) Start(ctx context.Context) {
	// Run immediately on start, then every fetchInterval.
	ticker := time.NewTicker(fetchInterval)
	defer ticker.Stop()

	d.tick(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.tick(ctx)
		}
	}
}

func (d *Dispatcher) tick(ctx context.Context) {
	schedules, err := d.fetchSchedules(ctx)
	if err != nil {
		log.Printf("[scheduler] fetch error: %v", err)
		return
	}

	// Build a set of active schedule IDs for pruning.
	activeIDs := make(map[string]struct{}, len(schedules))
	for _, s := range schedules {
		activeIDs[s.ID] = struct{}{}
	}
	// Prune stale entries from lastTrigger.
	for id := range d.lastTrigger {
		if _, ok := activeIDs[id]; !ok {
			delete(d.lastTrigger, id)
		}
	}

	now := time.Now().In(d.location)
	// Truncate to the current minute for dedup comparison.
	minuteKey := now.Truncate(time.Minute)

	for _, s := range schedules {
		if s.CronExpr == "" {
			continue // Skip schedules with no cron expression configured.
		}
		expr, err := ParseCron(s.CronExpr)
		if err != nil {
			log.Printf("[scheduler] invalid cron %q for schedule %s: %v", s.CronExpr, s.ID, err)
			continue
		}
		if !expr.Match(now) {
			continue
		}
		if last, ok := d.lastTrigger[s.ID]; ok && last.Equal(minuteKey) {
			continue // already triggered this minute
		}
		d.lastTrigger[s.ID] = minuteKey
		d.onTrigger(s.ID, s.TaskPayload)
	}
}

func (d *Dispatcher) fetchSchedules(ctx context.Context) ([]schedule, error) {
	url := fmt.Sprintf("%s/api/v1/workspaces/%s/schedules", d.backendURL, d.workspaceID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+d.getAuthToken())

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	limited := io.LimitReader(resp.Body, maxBodyBytes)

	// Backend returns { success: true, data: [...] } wrapper.
	var wrapper struct {
		Success bool       `json:"success"`
		Data    []schedule `json:"data"`
	}
	if err := json.NewDecoder(limited).Decode(&wrapper); err != nil {
		return nil, fmt.Errorf("decode schedules: %w", err)
	}
	return wrapper.Data, nil
}

// SetAuthToken updates the bearer token used for schedule fetches.
func (d *Dispatcher) SetAuthToken(token string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.authToken = token
}

func (d *Dispatcher) getAuthToken() string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.authToken
}
