package orchestra

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/insajin/autopus-adk/pkg/terminal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSplitTrackedPane_CleansPartiallyCreatedPane(t *testing.T) {
	isolateSurfaceTracker(t)
	term := &paneCommitTerminal{
		splitID: "surface:91", splitErr: errors.New("split acknowledgement lost"),
	}

	paneID, err := splitTrackedPane(context.Background(), term, terminal.Horizontal)

	require.Error(t, err)
	assert.Equal(t, terminal.PaneID("surface:91"), paneID)
	assert.Equal(t, []string{"surface:91"}, term.closed)
	assert.NotContains(t, readTrackerRefs(surfaceTrackerFile(os.Getpid())), "surface:91")
}

func TestSplitTrackedPane_PartialCleanupFailureRemainsTracked(t *testing.T) {
	isolateSurfaceTracker(t)
	term := &paneCommitTerminal{
		splitID: "surface:94", splitErr: errors.New("split acknowledgement lost"),
		closeErr: errors.New("close failed"),
	}

	paneID, err := splitTrackedPane(context.Background(), term, terminal.Horizontal)

	require.Error(t, err)
	assert.Equal(t, terminal.PaneID("surface:94"), paneID)
	assert.Len(t, term.closed, closePaneSurfaceAttempts)
	assert.Contains(t, readTrackerRefs(surfaceTrackerFile(os.Getpid())), "surface:94",
		"a partial pane that cannot be closed must remain recoverable")
}

func TestInteractivePaneBackend_PartialSplitCleanupHasSingleOwner(t *testing.T) {
	isolateSurfaceTracker(t)
	term := &paneCommitTerminal{
		splitID: "surface:95", splitErr: errors.New("split acknowledgement lost"),
	}
	backend := NewInteractivePaneBackend(OrchestraConfig{Terminal: term})

	_, err := backend.Execute(context.Background(), ProviderRequest{
		Provider: "claude",
		Config:   ProviderConfig{Name: "claude", Binary: "claude"},
		Prompt:   "verify partial split cleanup",
	})

	require.Error(t, err)
	assert.Equal(t, []string{"surface:95"}, term.closed,
		"the split helper exclusively owns partial-pane cleanup")
	assert.NotContains(t, readTrackerRefs(surfaceTrackerFile(os.Getpid())), "surface:95")
}

func TestSplitProviderPanes_PartialSplitCleanupHasSingleOwner(t *testing.T) {
	isolateSurfaceTracker(t)
	term := &paneCommitTerminal{
		splitID: "surface:96", splitErr: errors.New("split acknowledgement lost"),
	}

	panes, failed, err := splitProviderPanes(context.Background(), OrchestraConfig{
		Terminal:  term,
		Providers: []ProviderConfig{{Name: "claude", Binary: "claude"}},
	})

	require.Error(t, err)
	assert.Nil(t, panes)
	assert.Nil(t, failed)
	assert.Equal(t, []string{"surface:96"}, term.closed,
		"the split helper exclusively owns partial-pane cleanup")
	assert.NotContains(t, readTrackerRefs(surfaceTrackerFile(os.Getpid())), "surface:96")
}

func TestPartialSplitCleanup_CoversRecreateAndWarmPool(t *testing.T) {
	t.Run("recreate preserves old pane", func(t *testing.T) {
		term := &paneCommitTerminal{
			splitID: "surface:92", splitErr: errors.New("split acknowledgement lost"),
		}
		old := paneInfo{
			paneID: "surface:10", provider: ProviderConfig{Name: "custom", Binary: "custom"},
		}

		got, err := recreatePane(context.Background(), OrchestraConfig{Terminal: term}, old, 2)

		require.Error(t, err)
		assert.Equal(t, old, got)
		assert.Contains(t, term.closed, "surface:92")
		assert.NotContains(t, term.closed, "surface:10")
	})

	t.Run("warm pool does not retain partial pane", func(t *testing.T) {
		term := &paneCommitTerminal{
			splitID: "surface:93", splitErr: errors.New("split acknowledgement lost"),
		}
		pool := NewWarmPool(term, 1)

		pool.Init(context.Background())

		assert.Zero(t, pool.Size())
		assert.Contains(t, term.closed, "surface:93")
	})
}
