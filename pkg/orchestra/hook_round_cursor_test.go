package orchestra

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHookSessionWriteRoundCursor(t *testing.T) {
	t.Parallel()

	session, err := NewHookSession("round-cursor-" + NewSessionID())
	require.NoError(t, err)
	defer session.Cleanup()

	require.NoError(t, session.WriteRoundCursor("claude", 2))
	path := filepath.Join(session.Dir(), "claude-round-cursor")
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "2", string(data))
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestHookSessionWriteRoundCursorRejectsUnsafeCoordinates(t *testing.T) {
	t.Parallel()

	session, err := NewHookSession("round-cursor-invalid-" + NewSessionID())
	require.NoError(t, err)
	defer session.Cleanup()

	assert.Error(t, session.WriteRoundCursor("../claude", 2))
	assert.Error(t, session.WriteRoundCursor("claude", 0))
}
