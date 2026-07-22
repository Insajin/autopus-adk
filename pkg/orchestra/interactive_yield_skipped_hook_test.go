package orchestra

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReleaseYieldHookWaiters_SkippedReadyConsumesAbort(t *testing.T) {
	session, err := NewHookSession("yield-skipped-ready-" + NewSessionID())
	require.NoError(t, err)
	defer session.Cleanup()
	session.SetHookProviders(map[string]bool{"codex": true})
	require.NoError(t, session.writeArtifact(RoundSignalName("codex", 2, "ready"), nil, 0o600))
	panes := []paneInfo{{provider: ProviderConfig{Name: "codex"}, skipWait: true}}
	consumed := consumeYieldAborts(session, []string{"codex"}, 2)

	err = releaseYieldHookWaiters(context.Background(), session, panes, 2, time.Second)

	require.NoError(t, err)
	require.NoError(t, <-consumed)
	_, abortErr := session.statArtifact(RoundSignalName("codex", 2, "abort"))
	assert.True(t, errors.Is(abortErr, os.ErrNotExist))
}

func TestReleaseYieldHookWaiters_SkippedWithoutReadyIsIgnored(t *testing.T) {
	session, err := NewHookSession("yield-skipped-no-ready-" + NewSessionID())
	require.NoError(t, err)
	defer session.Cleanup()
	session.SetHookProviders(map[string]bool{"codex": true})
	panes := []paneInfo{{provider: ProviderConfig{Name: "codex"}, skipWait: true}}

	err = releaseYieldHookWaiters(context.Background(), session, panes, 2, 40*time.Millisecond)

	require.NoError(t, err)
	_, abortErr := session.statArtifact(RoundSignalName("codex", 2, "abort"))
	assert.True(t, errors.Is(abortErr, os.ErrNotExist))
}

func TestReleaseYieldHookWaiters_SkippedReadyStatErrorFailsClosed(t *testing.T) {
	root := t.TempDir()
	notDirectory := filepath.Join(root, "not-a-directory")
	require.NoError(t, os.WriteFile(notDirectory, []byte("x"), 0o600))
	session := &HookSession{sessionDir: notDirectory}
	session.SetHookProviders(map[string]bool{"codex": true})
	panes := []paneInfo{{provider: ProviderConfig{Name: "codex"}, skipWait: true}}

	err := releaseYieldHookWaiters(context.Background(), session, panes, 2, time.Second)

	require.Error(t, err)
	assert.ErrorContains(t, err, "inspect skipped hook ready")
}
