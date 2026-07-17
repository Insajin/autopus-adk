package learn

import "time"

// Prune removes entries older than the given number of days.
// Returns the number of entries removed.
func Prune(store *Store, days int) (int, error) {
	store.mu.Lock()
	defer store.mu.Unlock()

	entries, skips, err := store.ReadTolerant()
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

	if err := rewriteStore(store, kept, skips); err != nil {
		return 0, err
	}

	return pruned, nil
}
