package a2a

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// @AX:NOTE [AUTO] magic constants — pollInterval:10s, pollTimeout:30s control REST fallback latency; too short increases backend load
const (
	defaultPollInterval = 10 * time.Second
	defaultPollTimeout  = 30 * time.Second
)

// PollResult represents a single task returned from the REST poll endpoint.
type PollResult struct {
	ID      string          `json:"id"`
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// pollResponse is the envelope returned by /api/a2a/poll.
type pollResponse struct {
	Tasks []PollResult `json:"tasks"`
}

// RESTPollerConfig holds configuration for the REST fallback poller.
type RESTPollerConfig struct {
	BackendURL   string
	WorkerID     string
	AuthToken    string
	PollInterval time.Duration
	PollTimeout  time.Duration
	TaskHandler  func(task PollResult) error
	// OnAuthError is called when the poll endpoint returns 401.
	OnAuthError func(statusCode int)
}

// RESTPoller polls the backend REST endpoint as a fallback when WebSocket is unavailable.
type RESTPoller struct {
	config RESTPollerConfig
	client *http.Client
	cancel context.CancelFunc
	mu     sync.Mutex
	active bool
}

// NewRESTPoller creates a new RESTPoller with the given configuration.
func NewRESTPoller(config RESTPollerConfig) *RESTPoller {
	if config.PollInterval == 0 {
		config.PollInterval = defaultPollInterval
	}
	if config.PollTimeout == 0 {
		config.PollTimeout = defaultPollTimeout
	}
	return &RESTPoller{
		config: config,
		client: &http.Client{Timeout: config.PollTimeout},
	}
}

// @AX:ANCHOR [AUTO] fallback activation contract — called by messageLoop on connection exhaustion; Stop must be called when WebSocket recovers — fan_in: 3 (messageLoop, Server.Close, test)
// Start begins the polling loop in a goroutine. Polls until ctx is cancelled or Stop is called.
func (p *RESTPoller) Start(ctx context.Context) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.active {
		return
	}
	pollCtx, cancel := context.WithCancel(ctx)
	p.cancel = cancel
	p.active = true

	go p.loop(pollCtx)
}

// Stop cancels the polling goroutine (e.g., when WebSocket recovers).
func (p *RESTPoller) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.active {
		return
	}
	if p.cancel != nil {
		p.cancel()
	}
	p.active = false
}

func (p *RESTPoller) loop(ctx context.Context) {
	ticker := time.NewTicker(p.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := p.poll(ctx); err != nil {
				log.Printf("[rest-poller] poll error: %v", err)
			}
		}
	}
}

func (p *RESTPoller) poll(ctx context.Context) error {
	pollURL := fmt.Sprintf("%s/api/a2a/poll?worker_id=%s", p.config.BackendURL, url.QueryEscape(p.config.WorkerID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pollURL, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.config.AuthToken)

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		if p.config.OnAuthError != nil {
			p.config.OnAuthError(resp.StatusCode)
		}
		return fmt.Errorf("auth error: 401 unauthorized")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var result pollResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	for _, task := range result.Tasks {
		if p.config.TaskHandler != nil {
			if err := p.config.TaskHandler(task); err != nil {
				log.Printf("[rest-poller] task handler error for %s: %v", task.ID, err)
			}
		}
	}
	return nil
}
