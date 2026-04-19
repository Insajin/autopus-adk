// Package audit provides a rotating log writer for audit trails.
package audit

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const maxRotatedFiles = 5

// RotatingWriter implements io.Writer with size-based rotation.
type RotatingWriter struct {
	path    string
	maxSize int64
	maxAge  time.Duration
	mu      sync.Mutex
	file    *os.File
	size    int64
}

// NewRotatingWriter opens or creates the log file at path. It rotates when
// the file exceeds maxSize and deletes rotated files older than maxAge.
func NewRotatingWriter(path string, maxSize int64, maxAge time.Duration) (*RotatingWriter, error) {
	if maxSize <= 0 {
		maxSize = 10 * 1024 * 1024 // 10 MB
	}
	if maxAge <= 0 {
		maxAge = 7 * 24 * time.Hour // 7 days
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create log dir: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open log file: %w", err)
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("stat log file: %w", err)
	}
	return &RotatingWriter{
		path:    path,
		maxSize: maxSize,
		maxAge:  maxAge,
		file:    f,
		size:    info.Size(),
	}, nil
}

// Write appends p to the log file. It is safe for concurrent use.
// If the write would exceed maxSize, the file is rotated first.
func (w *RotatingWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file == nil {
		return 0, fmt.Errorf("writer is closed")
	}

	if w.size+int64(len(p)) > w.maxSize {
		if err := w.rotate(); err != nil {
			return 0, fmt.Errorf("rotate: %w", err)
		}
	}
	n, err = w.file.Write(p)
	w.size += int64(n)
	return n, err
}

// Close closes the underlying file.
func (w *RotatingWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file != nil {
		err := w.file.Close()
		w.file = nil
		return err
	}
	return nil
}

// StartCleanup runs a background goroutine that deletes rotated files older
// than maxAge. It checks every hour and stops when ctx is cancelled.
func (w *RotatingWriter) StartCleanup(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.cleanup()
		}
	}
}

// rotate shifts existing rotated files and renames the current file.
func (w *RotatingWriter) rotate() error {
	if err := w.file.Close(); err != nil {
		return fmt.Errorf("close current log: %w", err)
	}

	// Shift .4→delete, .3→.4, .2→.3, .1→.2, current→.1
	for i := maxRotatedFiles; i >= 1; i-- {
		src := w.rotatedName(i)
		if i == maxRotatedFiles {
			removeIfExists(src)
			continue
		}
		dst := w.rotatedName(i + 1)
		// Ignore errors — file may not exist yet.
		if err := os.Rename(src, dst); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("rotate log %s -> %s: %w", src, dst, err)
		}
	}
	if err := os.Rename(w.path, w.rotatedName(1)); err != nil && !os.IsNotExist(err) {
		// Best-effort: if rename fails, truncate instead.
		f, ferr := os.Create(w.path)
		if ferr != nil {
			return fmt.Errorf("fallback create: %w", ferr)
		}
		w.file = f
		w.size = 0
		return nil
	}

	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("open new log: %w", err)
	}
	w.file = f
	w.size = 0
	return nil
}

// cleanup removes rotated files older than maxAge.
func (w *RotatingWriter) cleanup() {
	cutoff := time.Now().Add(-w.maxAge)
	for i := 1; i <= maxRotatedFiles; i++ {
		name := w.rotatedName(i)
		info, err := os.Stat(name)
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			removeIfExists(name)
		}
	}
}

func (w *RotatingWriter) rotatedName(index int) string {
	return fmt.Sprintf("%s.%d", w.path, index)
}

func removeIfExists(path string) {
	_ = os.Remove(path)
}
