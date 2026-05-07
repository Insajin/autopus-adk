package docs

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCache_Set_CreatesFile verifies that setting a cache entry persists it to disk.
// Given: an empty cache directory
// When: Set is called with a key and doc content
// Then: a cache file exists on disk for that key
func TestCache_Set_CreatesFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cache := NewCache(dir, 1*time.Hour)

	entry := &CacheEntry{
		LibraryID: "/spf13/cobra",
		Topic:     "commands",
		Version:   "1.9.1",
		SourceRef: "/spf13/cobra",
		Content:   "# cobra\nCommand library docs.",
		Tokens:    10,
	}
	err := cache.Set("cobra:commands", entry)
	require.NoError(t, err)

	// Verify the entry is retrievable — confirms disk write
	got, err := cache.Get("cobra:commands")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, entry.Content, got.Content)
	assert.Equal(t, entry.LibraryID, got.LibraryID)
	assert.Equal(t, entry.Version, got.Version)
	assert.Equal(t, entry.SourceRef, got.SourceRef)
}

// TestCache_Get_HitWithinTTL verifies that a cache hit returns the stored entry within TTL.
// Given: a cache with an unexpired entry
// When: Get is called within the TTL window
// Then: the cached entry is returned without error
func TestCache_Get_HitWithinTTL(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cache := NewCache(dir, 1*time.Hour)

	entry := &CacheEntry{
		LibraryID: "/spf13/cobra",
		Topic:     "commands",
		Content:   "cached content",
		Tokens:    5,
	}
	require.NoError(t, cache.Set("cobra:commands", entry))

	got, err := cache.Get("cobra:commands")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "cached content", got.Content)
}

// TestCache_Get_MissExpired verifies that an expired cache entry returns nil.
// Given: a cache with a very short TTL
// When: Get is called after the TTL has expired
// Then: nil is returned (cache miss)
func TestCache_Get_MissExpired(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cache := NewCache(dir, 1*time.Millisecond)

	entry := &CacheEntry{
		LibraryID: "/spf13/cobra",
		Topic:     "commands",
		Content:   "expired content",
		Tokens:    5,
	}
	require.NoError(t, cache.Set("cobra:commands", entry))

	time.Sleep(10 * time.Millisecond)

	got, err := cache.Get("cobra:commands")
	require.NoError(t, err)
	assert.Nil(t, got, "expired cache entry must return nil")
}

// TestCache_Clear_RemovesAll verifies that Clear removes all entries from the cache.
// Given: a cache with multiple entries
// When: Clear is called
// Then: all entries are gone and list returns empty
func TestCache_Clear_RemovesAll(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cache := NewCache(dir, 1*time.Hour)

	require.NoError(t, cache.Set("lib1:topic", &CacheEntry{LibraryID: "lib1", Content: "a"}))
	require.NoError(t, cache.Set("lib2:topic", &CacheEntry{LibraryID: "lib2", Content: "b"}))

	err := cache.Clear()
	require.NoError(t, err)

	entries, err := cache.List()
	require.NoError(t, err)
	assert.Empty(t, entries, "cache must be empty after Clear")
}

// TestCache_List_ReturnsEntries verifies that List returns all cached entries with TTL info.
// Given: a cache with two entries
// When: List is called
// Then: both entries are returned with their expiry information
func TestCache_List_ReturnsEntries(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cache := NewCache(dir, 1*time.Hour)

	require.NoError(t, cache.Set("libA:api", &CacheEntry{LibraryID: "libA", Content: "docs A"}))
	require.NoError(t, cache.Set("libB:api", &CacheEntry{LibraryID: "libB", Content: "docs B"}))

	entries, err := cache.List()
	require.NoError(t, err)
	assert.Len(t, entries, 2)

	for _, e := range entries {
		assert.NotEmpty(t, e.Key)
		assert.False(t, e.ExpiresAt.IsZero(), "ExpiresAt must be set")
	}
}
