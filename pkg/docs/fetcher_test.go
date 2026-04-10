package docs

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFetcher_Context7Primary verifies that Context7 is used as the primary source.
// Given: Context7 returns a valid result and scraper is not called
// When: Fetch is called for a library
// Then: the result source is "context7"
func TestFetcher_Context7Primary(t *testing.T) {
	t.Parallel()

	c7 := &stubContext7{result: &DocResult{LibraryName: "cobra", Source: "context7", Content: "cobra docs", Tokens: 100}}
	scraper := &stubScraper{err: errors.New("should not be called")}
	cache := newInMemoryCache()

	fetcher := NewFetcher(c7, scraper, cache)
	result, err := fetcher.Fetch("cobra", "commands")

	require.NoError(t, err)
	assert.Equal(t, "context7", result.Source)
	assert.NotEmpty(t, result.Content)
}

// TestFetcher_FallbackToScraper verifies that scraper is used when Context7 fails.
// Given: Context7 returns an error
// When: Fetch is called for a library
// Then: the result source is "scraper"
func TestFetcher_FallbackToScraper(t *testing.T) {
	t.Parallel()

	c7 := &stubContext7{err: errors.New("context7 unavailable")}
	scraper := &stubScraper{result: &DocResult{LibraryName: "cobra", Source: "scraper", Content: "scraped docs", Tokens: 50}}
	cache := newInMemoryCache()

	fetcher := NewFetcher(c7, scraper, cache)
	result, err := fetcher.Fetch("cobra", "commands")

	require.NoError(t, err)
	assert.Equal(t, "scraper", result.Source)
	assert.NotEmpty(t, result.Content)
}

// TestFetcher_FallbackToScraper_OnQuotaExceeded verifies that scraper fallback is used
// when Context7 reports quota exhaustion.
func TestFetcher_FallbackToScraper_OnQuotaExceeded(t *testing.T) {
	t.Parallel()

	c7 := &stubContext7{err: ErrQuotaExceeded}
	scraper := &stubScraper{result: &DocResult{LibraryName: "github.com/spf13/cobra", Source: "scraper", Content: "scraped docs", Tokens: 50}}
	cache := newInMemoryCache()

	fetcher := NewFetcher(c7, scraper, cache)
	result, err := fetcher.Fetch("github.com/spf13/cobra", "commands")

	require.NoError(t, err)
	assert.Equal(t, "scraper", result.Source)
	assert.Equal(t, "github.com/spf13/cobra", result.LibraryName)
}

// TestFetcher_FallbackToCache verifies that cache is used when both Context7 and scraper fail.
// Given: Context7 and scraper both return errors but cache has an entry
// When: Fetch is called for a library
// Then: the result source is "cache"
func TestFetcher_FallbackToCache(t *testing.T) {
	t.Parallel()

	c7 := &stubContext7{err: errors.New("context7 unavailable")}
	scraper := &stubScraper{err: errors.New("scraper unavailable")}
	cache := newInMemoryCache()
	cache.store["cobra:commands"] = &DocResult{LibraryName: "cobra", Source: "cache", Content: "cached docs", Tokens: 30}

	fetcher := NewFetcher(c7, scraper, cache)
	result, err := fetcher.Fetch("cobra", "commands")

	require.NoError(t, err)
	assert.Equal(t, "cache", result.Source)
}

// TestFetcher_TokenBudget_Single verifies that a single library gets ~5000 token budget.
// Given: fetcher is configured with 1 library to fetch
// When: token budget is calculated
// Then: budget is approximately 5000 tokens
func TestFetcher_TokenBudget_Single(t *testing.T) {
	t.Parallel()

	budget := CalculateTokenBudget(1)
	assert.GreaterOrEqual(t, budget, 4000, "single library budget must be at least 4000")
	assert.LessOrEqual(t, budget, 6000, "single library budget must not exceed 6000")
}

// TestFetcher_TokenBudget_Multiple verifies that 5 libraries each get ~2000 token budget.
// Given: fetcher is configured with 5 libraries to fetch
// When: token budget is calculated per library
// Then: per-library budget is approximately 2000 tokens
func TestFetcher_TokenBudget_Multiple(t *testing.T) {
	t.Parallel()

	budget := CalculateTokenBudget(5)
	assert.GreaterOrEqual(t, budget, 1500, "5-library per-lib budget must be at least 1500")
	assert.LessOrEqual(t, budget, 2500, "5-library per-lib budget must not exceed 2500")
}

// TestFetcher_TokenBudget_HardCap verifies that total tokens never exceed 10000.
// Given: multiple libraries are fetched
// When: total token usage is summed
// Then: total never exceeds 10000
func TestFetcher_TokenBudget_HardCap(t *testing.T) {
	t.Parallel()

	tests := []struct{ count int }{
		{1}, {2}, {3}, {4}, {5},
	}
	for _, tt := range tests {
		total := CalculateTokenBudget(tt.count) * tt.count
		assert.LessOrEqual(t, total, 10000, "total token budget must not exceed 10000 for %d libs", tt.count)
	}
}

// stubContext7 is a test double for Context7Client.
type stubContext7 struct {
	result *DocResult
	err    error
}

func (s *stubContext7) Fetch(library, topic string) (*DocResult, error) {
	return s.result, s.err
}

// stubScraper is a test double for Scraper.
type stubScraper struct {
	result *DocResult
	err    error
}

func (s *stubScraper) Fetch(library, topic string) (*DocResult, error) {
	return s.result, s.err
}

// inMemoryCache is a test double for Cache.
type inMemoryCache struct {
	store map[string]*DocResult
}

func newInMemoryCache() *inMemoryCache {
	return &inMemoryCache{store: make(map[string]*DocResult)}
}

func (c *inMemoryCache) Get(key string) (*DocResult, error) {
	v, ok := c.store[key]
	if !ok {
		return nil, nil
	}
	return v, nil
}

func (c *inMemoryCache) Set(key string, result *DocResult) error {
	c.store[key] = result
	return nil
}
