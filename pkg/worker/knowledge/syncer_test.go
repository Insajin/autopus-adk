package knowledge

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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

	var received syncPayload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Contains(t, r.URL.Path, "/api/v1/workspaces/ws-123/knowledge/sources/src-1/bridge/push")

		err := json.NewDecoder(r.Body).Decode(&received)
		require.NoError(t, err)

		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	s := NewSyncer(srv.URL, "my-token", "ws-123", "src-1")
	err := s.SyncFile(context.Background(), filePath)
	require.NoError(t, err)

	assert.Equal(t, filePath, received.Path)
	assert.Equal(t, "content v1", received.Content)
	assert.NotEmpty(t, received.Hash)
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
