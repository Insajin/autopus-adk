package knowledge

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSyncer_ComputeHash(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	content := []byte("hello world")
	require.NoError(t, os.WriteFile(filePath, content, 0644))

	s := NewSyncer("http://unused", "tok", "ws1", "src1")
	hash, err := s.ComputeHash(filePath)
	require.NoError(t, err)

	expected := sha256.Sum256(content)
	assert.Equal(t, hex.EncodeToString(expected[:]), hash)
}

func TestSyncer_ComputeHash_FileNotFound(t *testing.T) {
	t.Parallel()

	s := NewSyncer("http://unused", "tok", "ws1", "src1")
	_, err := s.ComputeHash("/nonexistent/file.txt")
	require.Error(t, err)
}

func TestSyncer_SyncFile_UploadsChangedFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	filePath := filepath.Join(dir, "data.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("content v1"), 0644))

	var receivedPath string
	var receivedHash string
	var receivedContent string
	var receivedModifiedAt string
	var receivedMimeType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))
		assert.Contains(t, r.URL.Path, "/api/v1/workspaces/ws-123/knowledge/sources/src-1/bridge/push")
		assert.Contains(t, r.Header.Get("Content-Type"), "multipart/form-data;")

		reader, err := r.MultipartReader()
		require.NoError(t, err)

		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				break
			}
			require.NoError(t, err)

			data, readErr := io.ReadAll(part)
			require.NoError(t, readErr)

			switch part.FormName() {
			case "path":
				receivedPath = string(data)
			case "file_hash":
				receivedHash = string(data)
			case "modified_at":
				receivedModifiedAt = string(data)
			case "mime_type":
				receivedMimeType = string(data)
			case "file":
				receivedContent = string(data)
				assert.Equal(t, filepath.Base(filePath), part.FileName())
			}
		}

		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	s := NewSyncer(srv.URL, "my-token", "ws-123", "src-1")
	err := s.SyncFile(context.Background(), filePath)
	require.NoError(t, err)

	assert.Equal(t, filePath, receivedPath)
	assert.Equal(t, "content v1", receivedContent)
	assert.NotEmpty(t, receivedHash)
	assert.NotEmpty(t, receivedModifiedAt)
	assert.Equal(t, mime.TypeByExtension(filepath.Ext(filePath)), receivedMimeType)
}

func TestSyncer_SyncFile_SkipsUnchanged(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	filePath := filepath.Join(dir, "stable.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("no change"), 0644))

	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := NewSyncer(srv.URL, "tok", "ws1", "src1")
	ctx := context.Background()

	// First sync uploads.
	require.NoError(t, s.SyncFile(ctx, filePath))
	assert.Equal(t, 1, callCount)

	// Second sync with same content skips.
	require.NoError(t, s.SyncFile(ctx, filePath))
	assert.Equal(t, 1, callCount, "should not upload unchanged file")
}

func TestSyncer_SyncFile_UploadsAfterChange(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	filePath := filepath.Join(dir, "evolving.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("v1"), 0644))

	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := NewSyncer(srv.URL, "tok", "ws1", "src1")
	ctx := context.Background()

	require.NoError(t, s.SyncFile(ctx, filePath))
	assert.Equal(t, 1, callCount)

	// Change file content.
	require.NoError(t, os.WriteFile(filePath, []byte("v2"), 0644))
	require.NoError(t, s.SyncFile(ctx, filePath))
	assert.Equal(t, 2, callCount, "should upload after content change")
}

func TestSyncer_SyncFile_ServerError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	filePath := filepath.Join(dir, "err.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("data"), 0644))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	s := NewSyncer(srv.URL, "tok", "ws1", "src1")
	err := s.SyncFile(context.Background(), filePath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected status 500")
}
