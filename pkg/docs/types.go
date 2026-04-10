package docs

import (
	"errors"
	"time"
)

// DocResult represents a fetched documentation result.
type DocResult struct {
	LibraryName string // e.g., "cobra"
	Package     string // e.g., "github.com/spf13/cobra"
	Source      string // "context7", "scraper", or "cache"
	Content     string // documentation text content
	Tokens      int    // approximate token count
}

// CacheEntry represents a cached documentation entry on disk.
type CacheEntry struct {
	LibraryID string    // e.g., "/spf13/cobra"
	Topic     string    // e.g., "commands"
	Content   string    // documentation content
	Tokens    int       // approximate token count
	CachedAt  time.Time // when the entry was cached (for TTL)
}

// ListEntry represents a cache list item.
type ListEntry struct {
	Key       string    // cache key e.g. "cobra:commands"
	ExpiresAt time.Time // when the entry expires
}

// LibraryInfo is the result of resolving a library name.
type LibraryInfo struct {
	ID      string // Context7 library ID
	Name    string // library name
	Version string // version string
}

// DocContent is the result of fetching docs for a library.
type DocContent struct {
	Content string // documentation content
	Tokens  int    // token count
}

// ErrLibraryNotFound is returned when a library cannot be resolved.
var ErrLibraryNotFound = errors.New("library not found")

// ErrRateLimited is returned when the upstream documentation source rejects requests due to rate limits.
var ErrRateLimited = errors.New("rate limited")

// ErrQuotaExceeded is returned when the upstream documentation source reports quota exhaustion.
var ErrQuotaExceeded = errors.New("quota exceeded")

// DocFetcher is the interface for documentation sources (Context7, Scraper).
type DocFetcher interface {
	Fetch(library, topic string) (*DocResult, error)
}

// FetcherCache is the interface for the cache used by Fetcher.
type FetcherCache interface {
	Get(key string) (*DocResult, error)
	Set(key string, result *DocResult) error
}

// CalculateTokenBudget returns the per-library token budget based on the number of libraries.
// Adaptive schedule: 1→5000, 2→3000, 3→2500, 4-5→2000. Hard cap: total ≤ 10000.
func CalculateTokenBudget(libCount int) int {
	if libCount <= 0 {
		return 0
	}
	switch libCount {
	case 1:
		return 5000
	case 2:
		return 3000
	case 3:
		return 2500
	default:
		perLib := 2000
		// Ensure total does not exceed hard cap of 10000
		if perLib*libCount > 10000 {
			perLib = 10000 / libCount
		}
		return perLib
	}
}
