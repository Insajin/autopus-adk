package knowledge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// KnowledgeSearcher queries the backend Knowledge Hub API.
// It uses workspace-scoped endpoints matching the Platform backend routes.
type KnowledgeSearcher struct {
	backendURL  string
	authToken   string
	workspaceID string
	client      *http.Client
}

// SearchResult represents a single knowledge search result.
// Fields align with the Platform's KnowledgeSearchResult response.
type SearchResult struct {
	ID             string  `json:"id"`
	Title          string  `json:"title"`
	Content        string  `json:"content"`
	Category       string  `json:"category"`
	RelevanceScore float64 `json:"relevance_score"`
	QualityScore   float64 `json:"quality_score"`
	SourceType     string  `json:"source_type"`

	// Graph RAG enrichment (optional — empty for workspaces without KG).
	GraphContext    *GraphContext `json:"graph_context,omitempty"`
	RelatedEntities []EntityBrief `json:"related_entities,omitempty"`
	ParentTitle     string        `json:"parent_title,omitempty"`
	FreshnessFactor float64       `json:"freshness_factor,omitempty"`

	// Score is a convenience alias for RelevanceScore used by populateKnowledge.
	Score float64 `json:"-"`
}

// GraphContext holds community detection context from the knowledge graph.
type GraphContext struct {
	CommunityID   string `json:"community_id,omitempty"`
	CommunityName string `json:"community_name,omitempty"`
}

// EntityBrief is a lightweight entity reference from the ontology graph.
type EntityBrief struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// searchRequest matches the Platform's SearchKnowledgeRequest body.
type searchRequest struct {
	Query string `json:"query"`
	Limit int    `json:"limit,omitempty"`
}

// searchResponse wraps the Platform's standard list response envelope.
type searchResponse struct {
	Data []SearchResult `json:"data"`
}

// NewKnowledgeSearcher creates a new KnowledgeSearcher with a 5-second timeout.
func NewKnowledgeSearcher(backendURL, authToken, workspaceID string) *KnowledgeSearcher {
	return &KnowledgeSearcher{
		backendURL:  backendURL,
		authToken:   authToken,
		workspaceID: workspaceID,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// Search queries the workspace-scoped Knowledge Hub search API.
// Endpoint: POST /api/v1/workspaces/{workspaceId}/knowledge/search
func (ks *KnowledgeSearcher) Search(ctx context.Context, query string) ([]SearchResult, error) {
	endpoint := fmt.Sprintf("%s/api/v1/workspaces/%s/knowledge/search",
		ks.backendURL, ks.workspaceID)

	body, err := json.Marshal(searchRequest{Query: query, Limit: 10})
	if err != nil {
		return nil, fmt.Errorf("knowledge search: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("knowledge search: create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+ks.authToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := ks.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("knowledge search: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("knowledge search: unexpected status %d", resp.StatusCode)
	}

	var envelope searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("knowledge search: decode response: %w", err)
	}

	// Populate the convenience Score field from RelevanceScore.
	for i := range envelope.Data {
		envelope.Data[i].Score = envelope.Data[i].RelevanceScore
	}

	return envelope.Data, nil
}
