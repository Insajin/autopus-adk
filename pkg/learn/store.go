package learn

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Store manages learning entries in a JSONL file.
type Store struct {
	path string // path to pipeline.jsonl
	mu   sync.Mutex
}

// NewStore creates a store rooted at dir, ensuring .autopus/learnings/ exists.
func NewStore(dir string) (*Store, error) {
	learningsDir := filepath.Join(dir, ".autopus", "learnings")
	if err := os.MkdirAll(learningsDir, 0o755); err != nil {
		return nil, fmt.Errorf("create learnings dir: %w", err)
	}
	return &Store{
		path: filepath.Join(learningsDir, "pipeline.jsonl"),
	}, nil
}

// Append adds a new entry to the JSONL file.
// It acquires the store mutex so that an O_APPEND write cannot interleave with
// a concurrent read-modify-rewrite (UpdateReuseCount/Prune) and get lost.
func (s *Store) Append(entry LearningEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.appendUnlocked(entry)
}

// appendUnlocked writes a single entry without acquiring s.mu. Callers that
// already hold the store mutex (AppendAtomic) MUST use this primitive instead
// of Append to avoid re-entrant locking on the non-reentrant sync.Mutex.
func (s *Store) appendUnlocked(entry LearningEntry) error {
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal entry: %w", err)
	}

	f, err := os.OpenFile(s.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write entry: %w", err)
	}
	return nil
}

// Read reads all entries from the JSONL file.
// Returns empty slice if the file does not exist.
func (s *Store) Read() ([]LearningEntry, error) {
	f, err := os.Open(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return []LearningEntry{}, nil
		}
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	var entries []LearningEntry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var entry LearningEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			return nil, fmt.Errorf("unmarshal entry: %w", err)
		}
		entries = append(entries, entry)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan file: %w", err)
	}
	return entries, nil
}

// NextID generates the next L-{NNN} ID based on existing entries.
func (s *Store) NextID() (string, error) {
	entries, err := s.Read()
	if err != nil {
		return "", err
	}

	maxNum := 0
	for _, e := range entries {
		if strings.HasPrefix(e.ID, "L-") {
			numStr := e.ID[2:]
			if n, err := strconv.Atoi(numStr); err == nil && n > maxNum {
				maxNum = n
			}
		}
	}
	return fmt.Sprintf("L-%03d", maxNum+1), nil
}

// AppendAtomic atomically generates an ID and appends a new learning entry.
// It holds a mutex to prevent race conditions between NextID and Append.
func (s *Store) AppendAtomic(entryType EntryType, opts RecordOpts) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	id, err := s.NextID()
	if err != nil {
		return fmt.Errorf("next id: %w", err)
	}

	entry := LearningEntry{
		ID:         id,
		Timestamp:  time.Now(),
		Type:       entryType,
		Phase:      opts.Phase,
		SpecID:     opts.SpecID,
		Files:      opts.Files,
		Packages:   opts.Packages,
		Pattern:    opts.Pattern,
		Resolution: opts.Resolution,
		Severity:   opts.Severity,
	}
	// s.mu is already held: call the unlocked primitive, not Append.
	return s.appendUnlocked(entry)
}

// UpdateReuseCount increments reuse_count for the entry with the given ID.
// The whole read-modify-rewrite is serialized under s.mu so a concurrent
// Append/AppendAtomic cannot be truncated away by the rewrite. Read is the
// unlocked primitive, so it is safe to call while holding the mutex.
func (s *Store) UpdateReuseCount(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	entries, err := s.Read()
	if err != nil {
		return err
	}

	found := false
	for i := range entries {
		if entries[i].ID == id {
			entries[i].ReuseCount++
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("entry not found: %s", id)
	}

	// Rewrite entire file
	f, err := os.Create(s.path)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	for _, e := range entries {
		data, err := json.Marshal(e)
		if err != nil {
			return fmt.Errorf("marshal entry: %w", err)
		}
		if _, err := f.Write(append(data, '\n')); err != nil {
			return fmt.Errorf("write entry: %w", err)
		}
	}
	return nil
}
