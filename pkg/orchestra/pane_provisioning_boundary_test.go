package orchestra

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRunPaneOrchestra_PostSplitTempFailureDoesNotFallbackToSubprocess(t *testing.T) {
	isolateSurfaceTracker(t)
	const committedSurfaceRef = "surface:9101"
	provider, marker := newPaneBoundaryMarkerProvider(t, strings.Repeat("provider", 80), "")
	term := &paneCommitTerminal{splitID: committedSurfaceRef, screen: readyScreen}
	cfg := OrchestraConfig{
		Providers:      []ProviderConfig{provider},
		Strategy:       StrategyConsensus,
		Prompt:         "test provisioning cleanup",
		TimeoutSeconds: 3,
		Terminal:       term,
	}

	_, err := RunPaneOrchestra(context.Background(), cfg)

	assert.Error(t, err, "a failure after SplitPane commits must remain a pane-path error")
	assert.Equal(t, 1, term.splitCalls)
	assert.Contains(t, term.closed, committedSurfaceRef, "the split pane must be cleaned up")
	_, statErr := os.Stat(marker)
	assert.ErrorIs(t, statErr, os.ErrNotExist, "post-split temp-file failure must not execute subprocess")
	refs := readTrackerRefs(surfaceTrackerFile(os.Getpid()))
	assert.NotContains(t, refs, committedSurfaceRef, "successful cleanup must remove the tracker ref")
}
