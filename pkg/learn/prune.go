package learn

import "time"

// Prune removes entries older than the given number of days.
// Returns the number of entries removed.
// The read-modify-rewrite is serialized under store.mu so a concurrent
// Append cannot be lost when the kept entries are rewritten. Read and
// rewriteStore are unlocked primitives, safe to call while holding the mutex.
func Prune(store *Store, days int) (int, error) {
	store.mu.Lock()
	defer store.mu.Unlock()

	entries, err := store.Read()
	if err != nil {
		return 0, err
	}

	cutoff := time.Now().Add(-time.Duration(days) * 24 * time.Hour)
	var kept []LearningEntry
	pruned := 0
	for _, e := range entries {
		if e.Timestamp.Before(cutoff) || e.Timestamp.Equal(cutoff) {
			pruned++
		} else {
			kept = append(kept, e)
		}
	}

	if pruned == 0 {
		return 0, nil
	}

	// Rewrite file with kept entries
	return pruned, rewriteStore(store, kept)
}
