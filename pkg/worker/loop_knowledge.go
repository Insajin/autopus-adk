package worker

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/insajin/autopus-adk/pkg/worker/knowledge"
)

var knowledgeWordCleaner = regexp.MustCompile(`[^a-zA-Z0-9._/-]+`)

func resolveMemoryAgentID(cfg LoopConfig) string {
	if _, err := uuid.Parse(cfg.MemoryAgentID); err == nil {
		return cfg.MemoryAgentID
	}
	if _, err := uuid.Parse(cfg.WorkerName); err == nil {
		return cfg.WorkerName
	}
	return ""
}

// populateMemory queries the agent memory API and returns formatted context.
// Returns empty string on failure or when searcher is nil (non-blocking).
func populateMemory(ctx context.Context, searcher *knowledge.MemorySearcher, agentID, description string) string {
	if searcher == nil || agentID == "" || description == "" {
		return ""
	}
	entries, err := searcher.GetContext(ctx, agentID, description)
	if err != nil {
		log.Printf("[worker] memory context failed: %v", err)
		return ""
	}
	if len(entries) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("## Agent Memory Context\n\n")
	for _, e := range entries {
		fmt.Fprintf(&b, "### %s [%s]\n%s\n\n", e.Title, e.Layer, e.Content)
	}
	return b.String()
}

// populateKnowledge searches the knowledge base and returns formatted context.
// Returns empty string on failure or when searcher is nil (non-blocking).
func populateKnowledge(ctx context.Context, searcher *knowledge.KnowledgeSearcher, description string) string {
	if searcher == nil || description == "" {
		return ""
	}

	results, err := searcher.Search(ctx, buildKnowledgeQuery(description))
	if err != nil {
		log.Printf("[worker] knowledge search failed: %v", err)
		return ""
	}
	if len(results) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("## Relevant Knowledge\n\n")
	for _, r := range results {
		header := fmt.Sprintf("### %s (score: %.2f", r.Title, r.Score)
		if r.FreshnessFactor > 0 {
			header += fmt.Sprintf(", freshness: %.1f", r.FreshnessFactor)
		}
		header += ")\n"
		b.WriteString(header)
		b.WriteString(r.Content)
		b.WriteByte('\n')
		if len(r.RelatedEntities) > 0 {
			b.WriteString("Related: ")
			for i, e := range r.RelatedEntities {
				if i > 0 {
					b.WriteString(", ")
				}
				b.WriteString(e.Name)
			}
			b.WriteByte('\n')
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func buildKnowledgeQuery(description string) string {
	normalized := strings.TrimSpace(strings.Join(strings.Fields(description), " "))
	if normalized == "" {
		return ""
	}
	if len(normalized) <= 120 {
		return normalized
	}

	words := strings.Fields(normalized)
	keywords := make([]string, 0, 10)
	seen := make(map[string]struct{}, 10)
	for _, word := range words {
		cleaned := strings.ToLower(knowledgeWordCleaner.ReplaceAllString(word, ""))
		if len(cleaned) < 3 {
			continue
		}
		if _, ok := seen[cleaned]; ok {
			continue
		}
		seen[cleaned] = struct{}{}
		keywords = append(keywords, cleaned)
		if len(keywords) == 10 {
			break
		}
	}
	if len(keywords) == 0 {
		if len(normalized) > 120 {
			return normalized[:120]
		}
		return normalized
	}
	return strings.Join(keywords, " ")
}

// truncateForMemory extracts a brief learning summary from task output.
// Limits content to ~500 chars to stay within memory entry size limits.
func truncateForMemory(description, output string) string {
	summary := fmt.Sprintf("Task: %s\nResult summary: %s", description, output)
	if len(summary) > 500 {
		return summary[:500]
	}
	return summary
}
