package knowledge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// MemorySearcher queries the backend Agent Memory API.
type MemorySearcher struct {
	backendURL  string
	authToken   string
	workspaceID string
	client      *http.Client
	mu          sync.RWMutex
}

// MemoryEntry represents a single memory entry from the Platform.
type MemoryEntry struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Content string `json:"content"`
	Layer   string `json:"layer"`
	Source  string `json:"source,omitempty"`
}

// MemoryContextResponse is the API response for GET /memory/context.
type MemoryContextResponse struct {
	Entries    []MemoryEntry `json:"entries"`
	TokensUsed int           `json:"tokens_used"`
}

// commonWords are skipped during keyword extraction to reduce noise.
var commonWords = map[string]bool{
	"the": true, "a": true, "an": true, "and": true, "or": true,
	"in": true, "on": true, "at": true, "to": true, "for": true,
	"of": true, "with": true, "is": true, "are": true, "was": true,
	"be": true, "by": true, "from": true, "this": true, "that": true,
	"it": true, "as": true, "not": true, "but": true, "have": true,
}

// NewMemorySearcher creates a new MemorySearcher with a 5-second timeout.
func NewMemorySearcher(backendURL, authToken, workspaceID string) *MemorySearcher {
	return &MemorySearcher{
		backendURL:  backendURL,
		authToken:   authToken,
		workspaceID: workspaceID,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// extractKeywords returns the first 5 significant words from description,
// skipping common stop words.
func extractKeywords(description string) string {
	words := strings.Fields(description)
	var keywords []string
	for _, w := range words {
		lower := strings.ToLower(strings.Trim(w, ".,!?;:\"'()"))
		if lower == "" || commonWords[lower] {
			continue
		}
		keywords = append(keywords, lower)
		if len(keywords) == 5 {
			break
		}
	}
	return strings.Join(keywords, ",")
}

// GetContext queries GET /api/v1/workspaces/{workspaceId}/memory/context
// with agent_id, token_budget, top_k, and keywords derived from description.
func (ms *MemorySearcher) GetContext(ctx context.Context, agentID, description string) ([]MemoryEntry, error) {
	endpoint := fmt.Sprintf("%s/api/v1/workspaces/%s/memory/context",
		ms.backendURL, ms.workspaceID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("memory context: create request: %w", err)
	}

	q := req.URL.Query()
	q.Set("agent_id", agentID)
	q.Set("token_budget", "2000")
	q.Set("top_k", "10")
	if kw := extractKeywords(description); kw != "" {
		q.Set("keywords", kw)
	}
	req.URL.RawQuery = q.Encode()

	req.Header.Set("Authorization", "Bearer "+ms.getAuthToken())

	resp, err := ms.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("memory context: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("memory context: unexpected status %d", resp.StatusCode)
	}

	var result MemoryContextResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("memory context: decode response: %w", err)
	}

	return result.Entries, nil
}

// CreateMemoryRequest is the request body for POST /memory.
type CreateMemoryRequest struct {
	AgentID  string                 `json:"agent_id"`
	Title    string                 `json:"title"`
	Content  string                 `json:"content"`
	Source   string                 `json:"source"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// CreateMemory creates an L1 memory entry on the Platform.
// POST /api/v1/workspaces/{workspaceId}/memory
// Non-blocking by convention: returns error but caller should log and continue.
func (ms *MemorySearcher) CreateMemory(ctx context.Context, req CreateMemoryRequest) error {
	endpoint := fmt.Sprintf("%s/api/v1/workspaces/%s/memory",
		ms.backendURL, ms.workspaceID)

	encoded, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("memory write-back: marshal body: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(encoded))
	if err != nil {
		return fmt.Errorf("memory write-back: create request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+ms.getAuthToken())
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := ms.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("memory write-back: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("memory write-back: unexpected status %d", resp.StatusCode)
	}
	return nil
}

// SetAuthToken updates the bearer token used for memory API requests.
func (ms *MemorySearcher) SetAuthToken(token string) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.authToken = token
}

func (ms *MemorySearcher) getAuthToken() string {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	return ms.authToken
}
