package worker

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/insajin/autopus-adk/pkg/worker/knowledge"
)

// populateKnowledge searches the knowledge base and returns formatted context.
// Returns empty string on failure or when searcher is nil (non-blocking).
func populateKnowledge(ctx context.Context, searcher *knowledge.KnowledgeSearcher, description string) string {
	if searcher == nil || description == "" {
		return ""
	}

	results, err := searcher.Search(ctx, description)
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
		fmt.Fprintf(&b, "### %s (score: %.2f)\n%s\n\n", r.Title, r.Score, r.Content)
	}
	return b.String()
}

// startKnowledgeWatcher creates and starts a file watcher that syncs changes
// to the knowledge backend. Runs in background until context is cancelled.
// skipSyncPatterns contains file patterns that should not be synced.
var skipSyncPatterns = []string{
	"audit.jsonl",
	".git/",
	"node_modules/",
	".DS_Store",
}

func shouldSkipSync(path string) bool {
	for _, pat := range skipSyncPatterns {
		if strings.Contains(path, pat) {
			return true
		}
	}
	return false
}

func startKnowledgeWatcher(ctx context.Context, syncer *knowledge.Syncer, dir string) *knowledge.FileWatcher {
	watcher := knowledge.NewFileWatcher(dir, 0, func(path string) {
		if shouldSkipSync(path) {
			return
		}
		if err := syncer.SyncFile(ctx, path); err != nil {
			log.Printf("[worker] knowledge sync failed for %s: %v", path, err)
		}
	}, nil)
	go func() {
		if err := watcher.Start(ctx); err != nil && ctx.Err() == nil {
			log.Printf("[worker] knowledge watcher stopped: %v", err)
		}
	}()
	return watcher
}
