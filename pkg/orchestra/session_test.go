package orchestra

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSessionID_Format(t *testing.T) {
	t.Parallel()

	id := NewSessionID()
	assert.Contains(t, id, "orch-")
	assert.Greater(t, len(id), 15) // orch- + timestamp + hex
}

func TestNewSessionID_Unique(t *testing.T) {
	t.Parallel()

	id1 := NewSessionID()
	id2 := NewSessionID()
	assert.NotEqual(t, id1, id2)
}

func TestSaveAndLoadSession(t *testing.T) {
	t.Parallel()

	session := OrchestraSession{
		ID:    "test-save-load-" + NewSessionID(),
		Panes: map[string]string{"claude": "surface:1", "gemini": "surface:2"},
		Providers: []SessionProviderConfig{
			{Name: "claude", Binary: "claude"},
			{Name: "gemini", Binary: "gemini"},
		},
		Rounds: [][]SessionProviderResponse{
			{
				{Provider: "claude", Output: "hello", DurationMs: 100, TimedOut: false},
				{Provider: "gemini", Output: "world", DurationMs: 200, TimedOut: false},
			},
		},
		CreatedAt: time.Now().Truncate(time.Second),
	}

	err := SaveSession(session)
	require.NoError(t, err)
	defer RemoveSession(session.ID)

	loaded, err := LoadSession(session.ID)
	require.NoError(t, err)

	assert.Equal(t, session.ID, loaded.ID)
	assert.Equal(t, session.Panes, loaded.Panes)
	assert.Len(t, loaded.Providers, 2)
	assert.Equal(t, "claude", loaded.Providers[0].Name)
	assert.Len(t, loaded.Rounds, 1)
	assert.Len(t, loaded.Rounds[0], 2)
	assert.Equal(t, "hello", loaded.Rounds[0][0].Output)
}

func TestLoadSession_NotFound(t *testing.T) {
	t.Parallel()

	_, err := LoadSession("nonexistent-session-id-12345")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "read session")
}

func TestRemoveSession_Idempotent(t *testing.T) {
	t.Parallel()

	// Removing a non-existent session should return nil (idempotent).
	err := RemoveSession("nonexistent-cleanup-id-67890")
	assert.NoError(t, err)
}

func TestRemoveSession_AfterSave(t *testing.T) {
	t.Parallel()

	session := OrchestraSession{
		ID:        "test-remove-" + NewSessionID(),
		Panes:     map[string]string{},
		CreatedAt: time.Now(),
	}

	require.NoError(t, SaveSession(session))

	// Should load successfully
	_, err := LoadSession(session.ID)
	require.NoError(t, err)

	// Remove
	require.NoError(t, RemoveSession(session.ID))

	// Should fail to load after removal
	_, err = LoadSession(session.ID)
	assert.Error(t, err)
}
