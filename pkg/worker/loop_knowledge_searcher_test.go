package worker

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/insajin/autopus-adk/pkg/worker/knowledge"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPopulateMemory_FormatsEntries(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(knowledge.MemoryContextResponse{
			Entries: []knowledge.MemoryEntry{
				{ID: "m1", Title: "Deploy", Content: "run the deploy", Layer: "L1"},
				{ID: "m2", Title: "Rollback", Content: "undo it", Layer: "L2"},
			},
		})
	}))
	defer srv.Close()

	searcher := knowledge.NewMemorySearcher(srv.URL, "tok", "ws-1")
	got := populateMemory(context.Background(), searcher, "agent-1", "deploy the service")

	assert.Contains(t, got, "## Agent Memory Context")
	assert.Contains(t, got, "### Deploy [L1]")
	assert.Contains(t, got, "run the deploy")
	assert.Contains(t, got, "### Rollback [L2]")
}

func TestPopulateMemory_EmptyEntriesReturnsEmpty(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(knowledge.MemoryContextResponse{Entries: []knowledge.MemoryEntry{}})
	}))
	defer srv.Close()

	searcher := knowledge.NewMemorySearcher(srv.URL, "tok", "ws-1")
	got := populateMemory(context.Background(), searcher, "agent-1", "deploy")
	assert.Equal(t, "", got)
}

func TestPopulateMemory_ServerErrorReturnsEmpty(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	searcher := knowledge.NewMemorySearcher(srv.URL, "tok", "ws-1")
	got := populateMemory(context.Background(), searcher, "agent-1", "deploy")
	assert.Equal(t, "", got)
}

func TestPopulateKnowledge_NilSearcherAndEmptyDescription(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "", populateKnowledge(context.Background(), nil, "anything"))
}

func TestPopulateKnowledge_FormatsResultsWithEnrichment(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Contains(t, r.URL.Path, "/api/v1/workspaces/ws-9/knowledge/search")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []knowledge.SearchResult{
				{
					ID:              "k1",
					Title:           "Runbook",
					Content:         "step one",
					RelevanceScore:  0.91,
					FreshnessFactor: 1.5,
					RelatedEntities: []knowledge.EntityBrief{
						{ID: "e1", Name: "Service A"},
						{ID: "e2", Name: "Service B"},
					},
				},
				{
					ID:             "k2",
					Title:          "Notes",
					Content:        "step two",
					RelevanceScore: 0.42,
				},
			},
		})
	}))
	defer srv.Close()

	searcher := knowledge.NewKnowledgeSearcher(srv.URL, "tok", "ws-9")
	got := populateKnowledge(context.Background(), searcher, "deploy the service safely")

	assert.Contains(t, got, "## Relevant Knowledge")
	assert.Contains(t, got, "### Runbook (score: 0.91, freshness: 1.5)")
	assert.Contains(t, got, "step one")
	assert.Contains(t, got, "Related: Service A, Service B")
	// Second result has no freshness/entities -> header omits freshness, no Related line.
	assert.Contains(t, got, "### Notes (score: 0.42)")
}

func TestPopulateKnowledge_EmptyResultsReturnsEmpty(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []knowledge.SearchResult{}})
	}))
	defer srv.Close()

	searcher := knowledge.NewKnowledgeSearcher(srv.URL, "tok", "ws-9")
	got := populateKnowledge(context.Background(), searcher, "deploy")
	assert.Equal(t, "", got)
}

func TestPopulateKnowledge_ServerErrorReturnsEmpty(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	searcher := knowledge.NewKnowledgeSearcher(srv.URL, "tok", "ws-9")
	got := populateKnowledge(context.Background(), searcher, "deploy")
	require.Equal(t, "", got)
}
