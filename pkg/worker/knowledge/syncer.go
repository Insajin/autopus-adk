package knowledge

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Syncer uploads changed files to the Knowledge Hub backend using SHA256-based
// incremental diffing via the workspace-scoped bridge sync endpoint.
// Conflict resolution is Last-Write-Wins.
type Syncer struct {
	backendURL  string
	authToken   string
	workspaceID string
	sourceID    string // knowledge source ID for bridge binding
	client      *http.Client

	mu     sync.Mutex
	hashes map[string]string // path -> sha256 hex
}

// NewSyncer creates a Syncer for the given backend, workspace, and source.
// sourceID identifies the knowledge source bound to this bridge connection.
func NewSyncer(backendURL, authToken, workspaceID, sourceID string) *Syncer {
	return &Syncer{
		backendURL:  backendURL,
		authToken:   authToken,
		workspaceID: workspaceID,
		sourceID:    sourceID,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		hashes: make(map[string]string),
	}
}

// SyncFile computes the file's SHA256, and if it differs from the last known
// hash, uploads the file content to the backend.
func (s *Syncer) SyncFile(ctx context.Context, path string) error {
	hash, err := s.ComputeHash(path)
	if err != nil {
		return fmt.Errorf("sync file: compute hash: %w", err)
	}

	s.mu.Lock()
	prev, known := s.hashes[path]
	if known && prev == hash {
		s.mu.Unlock()
		return nil // unchanged
	}
	s.hashes[path] = hash
	s.mu.Unlock()

	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("sync file: read: %w", err)
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	if err := writer.WriteField("path", path); err != nil {
		return fmt.Errorf("sync file: write path field: %w", err)
	}
	if err := writer.WriteField("file_hash", hash); err != nil {
		return fmt.Errorf("sync file: write hash field: %w", err)
	}

	if info, statErr := os.Stat(path); statErr == nil {
		if err := writer.WriteField("modified_at", info.ModTime().UTC().Format(time.RFC3339Nano)); err != nil {
			return fmt.Errorf("sync file: write modified_at field: %w", err)
		}
	}

	mimeType := mime.TypeByExtension(filepath.Ext(path))
	if mimeType != "" {
		if err := writer.WriteField("mime_type", mimeType); err != nil {
			return fmt.Errorf("sync file: write mime_type field: %w", err)
		}
	}

	fileWriter, err := writer.CreateFormFile("file", filepath.Base(path))
	if err != nil {
		return fmt.Errorf("sync file: create form file: %w", err)
	}
	if _, err := fileWriter.Write(content); err != nil {
		return fmt.Errorf("sync file: write file content: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("sync file: close multipart writer: %w", err)
	}

	endpoint := fmt.Sprintf("%s/api/v1/workspaces/%s/knowledge/sources/%s/bridge/push",
		s.backendURL, s.workspaceID, s.sourceID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body.Bytes()))
	if err != nil {
		return fmt.Errorf("sync file: create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.authToken)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("sync file: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("sync file: unexpected status %d", resp.StatusCode)
	}

	return nil
}

// ComputeHash returns the SHA256 hex digest of the file at the given path.
func (s *Syncer) ComputeHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
