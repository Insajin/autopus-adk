package workspace

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMultiWorkspace_AddGet(t *testing.T) {
	t.Parallel()

	mw := NewMultiWorkspace()
	conn := WorkspaceConn{
		WorkspaceID: "ws-1",
		ProjectDir:  "/projects/alpha",
		BackendURL:  "https://api.example.com",
		AuthToken:   "tok-123",
		Connected:   true,
	}

	mw.Add(conn)

	got, ok := mw.Get("ws-1")
	require.True(t, ok)
	assert.Equal(t, "ws-1", got.WorkspaceID)
	assert.Equal(t, "/projects/alpha", got.ProjectDir)
	assert.Equal(t, "https://api.example.com", got.BackendURL)
	assert.Equal(t, "tok-123", got.AuthToken)
	assert.True(t, got.Connected)
}

func TestMultiWorkspace_GetMissing(t *testing.T) {
	t.Parallel()

	mw := NewMultiWorkspace()
	_, ok := mw.Get("nonexistent")
	assert.False(t, ok)
}

func TestMultiWorkspace_AddReplacesExisting(t *testing.T) {
	t.Parallel()

	mw := NewMultiWorkspace()
	mw.Add(WorkspaceConn{WorkspaceID: "ws-1", ProjectDir: "/old"})
	mw.Add(WorkspaceConn{WorkspaceID: "ws-1", ProjectDir: "/new"})

	got, ok := mw.Get("ws-1")
	require.True(t, ok)
	assert.Equal(t, "/new", got.ProjectDir)
}

func TestMultiWorkspace_RouteTask(t *testing.T) {
	t.Parallel()

	mw := NewMultiWorkspace()
	mw.Add(WorkspaceConn{
		WorkspaceID: "ws-1",
		ProjectDir:  "/projects/alpha",
		Connected:   true,
	})

	dir, err := mw.RouteTask("ws-1")
	require.NoError(t, err)
	assert.Equal(t, "/projects/alpha", dir)
}

func TestMultiWorkspace_RouteTaskNotConnected(t *testing.T) {
	t.Parallel()

	mw := NewMultiWorkspace()
	mw.Add(WorkspaceConn{
		WorkspaceID: "ws-2",
		ProjectDir:  "/projects/beta",
		Connected:   false,
	})

	_, err := mw.RouteTask("ws-2")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

func TestMultiWorkspace_RouteTaskUnknown(t *testing.T) {
	t.Parallel()

	mw := NewMultiWorkspace()
	_, err := mw.RouteTask("unknown")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not registered")
}

func TestMultiWorkspace_List(t *testing.T) {
	t.Parallel()

	mw := NewMultiWorkspace()
	mw.Add(WorkspaceConn{WorkspaceID: "ws-a", ProjectDir: "/a"})
	mw.Add(WorkspaceConn{WorkspaceID: "ws-b", ProjectDir: "/b"})
	mw.Add(WorkspaceConn{WorkspaceID: "ws-c", ProjectDir: "/c"})

	list := mw.List()
	assert.Len(t, list, 3)

	ids := make(map[string]bool)
	for _, c := range list {
		ids[c.WorkspaceID] = true
	}
	assert.True(t, ids["ws-a"])
	assert.True(t, ids["ws-b"])
	assert.True(t, ids["ws-c"])
}

func TestMultiWorkspace_ListIsSnapshot(t *testing.T) {
	t.Parallel()

	mw := NewMultiWorkspace()
	mw.Add(WorkspaceConn{WorkspaceID: "ws-1", ProjectDir: "/orig"})

	snapshot := mw.List()
	require.Len(t, snapshot, 1)

	// Mutating the snapshot should not affect the manager.
	snapshot[0].ProjectDir = "/mutated"

	got, ok := mw.Get("ws-1")
	require.True(t, ok)
	assert.Equal(t, "/orig", got.ProjectDir)
}

func TestMultiWorkspace_Remove(t *testing.T) {
	t.Parallel()

	mw := NewMultiWorkspace()
	mw.Add(WorkspaceConn{WorkspaceID: "ws-1", ProjectDir: "/a"})

	mw.Remove("ws-1")

	_, ok := mw.Get("ws-1")
	assert.False(t, ok)
	assert.Empty(t, mw.List())
}

func TestMultiWorkspace_RemoveNonexistent(t *testing.T) {
	t.Parallel()

	mw := NewMultiWorkspace()
	// Should be a no-op, not panic.
	mw.Remove("does-not-exist")
}

func TestMultiWorkspace_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	const goroutines = 50
	mw := NewMultiWorkspace()

	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			wsID := "ws-concurrent"
			conn := WorkspaceConn{
				WorkspaceID: wsID,
				ProjectDir:  "/concurrent",
				Connected:   true,
			}
			mw.Add(conn)
			mw.Get(wsID)
			_, _ = mw.RouteTask(wsID)
			mw.List()
		}(i)
	}
	wg.Wait()

	// After all goroutines finish, the workspace should still be accessible.
	got, ok := mw.Get("ws-concurrent")
	assert.True(t, ok)
	assert.Equal(t, "/concurrent", got.ProjectDir)
}
