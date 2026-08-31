package capture

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestLocalCaptureDir_MatchesRunnerLayout pins the derivation against the real
// layout observed from a run: the manifest sits at
// `<runDir>/<journey>/manifest.json` while capture media stays under
// `<runDir>/_raw/<journey>/capture`.
func TestLocalCaptureDir_MatchesRunnerLayout(t *testing.T) {
	t.Parallel()

	runDir := filepath.Join(".autopus", "qa", "runs", "qa-1")
	manifestDir := filepath.Join(runDir, "browser-gui-explore")
	assert.Equal(t,
		filepath.Join(runDir, "_raw", "browser-gui-explore", "capture"),
		LocalCaptureDir(manifestDir))
	assert.Empty(t, LocalCaptureDir(""))
}
