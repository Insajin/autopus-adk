package a2a

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	ID                       string            `json:"id"`
	Type                     string            `json:"type"`
	Model                    string            `json:"model,omitempty"`
	PipelinePhases           []string          `json:"pipeline_phases,omitempty"`
	PipelineInstructions     map[string]string `json:"pipeline_instructions,omitempty"`
	PipelinePromptTemplates  map[string]string `json:"pipeline_prompt_templates,omitempty"`
	IterationBudget          *IterationBudget  `json:"iteration_budget,omitempty"`
	ControlPlaneCapabilities []string          `json:"control_plane_capabilities,omitempty"`
	ControlPlaneSignature    string            `json:"control_plane_signature,omitempty"`
	PolicySignature          string            `json:"policy_signature,omitempty"`
	Payload                  json.RawMessage   `json:"payload"`
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

// SetAuthToken updates the bearer token used by the poller.
func (p *RESTPoller) SetAuthToken(token string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.config.AuthToken = token
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
	p.mu.Lock()
	authToken := p.config.AuthToken
	p.mu.Unlock()

	pollURL := fmt.Sprintf("%s/api/worker/tasks/pending?worker_id=%s", p.config.BackendURL, url.QueryEscape(p.config.WorkerID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pollURL, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+authToken)

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

	tasks, err := decodePollTasks(resp.Body)
	if err != nil {
		return err
	}

	for _, task := range tasks {
		if p.config.TaskHandler != nil {
			if err := p.config.TaskHandler(task); err != nil {
				log.Printf("[rest-poller] task handler error for %s: %v", task.ID, err)
			}
		}
	}
	return nil
}

func decodePollTasks(body io.Reader) ([]PollResult, error) {
	payload, err := io.ReadAll(body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var wrapped pollResponse
	if err := json.Unmarshal(payload, &wrapped); err == nil && wrapped.Tasks != nil {
		return wrapped.Tasks, nil
	}

	var direct []PollResult
	if err := json.Unmarshal(payload, &direct); err == nil {
		return direct, nil
	}

	return nil, fmt.Errorf("decode response: unsupported poll response shape")
}
