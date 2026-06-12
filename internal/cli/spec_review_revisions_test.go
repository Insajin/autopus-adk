package cli

// SPEC-SPECREV-002: oracle tests for the --loop revision floor wiring
// (REQ-003) and the spec.Load error wrapping (REQ-005).

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/spec"
)

// S5 (REQ-003): the leaf helper applies the loop floor.
func TestLoopAwareMaxRevisions_AppliesFloor(t *testing.T) {
	t.Parallel()

	require.Equal(t, 5, loopModeMinRevisions)
	assert.Equal(t, 5, loopAwareMaxRevisions(2, true), "below floor is raised to 5")
	assert.Equal(t, 8, loopAwareMaxRevisions(8, true), "at/above floor is unchanged")
}

// S6 (REQ-003): without --loop the configured value is preserved.
func TestLoopAwareMaxRevisions_NoLoopUnchanged(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 2, loopAwareMaxRevisions(2, false))
}

// S11 (REQ-003): resolveSpecReviewMaxRevisions consumes both the gate config and
// the LoopMode flag, including the default fallback for a non-positive setting.
func TestResolveSpecReviewMaxRevisions_GateAndLoopMode(t *testing.T) {
	t.Parallel()

	require.Equal(t, 3, defaultMaxRevisions)
	assert.Equal(t, 5, resolveSpecReviewMaxRevisions(config.ReviewGateConf{MaxRevisions: 2}, true),
		"configured 2 raised to floor 5")
	assert.Equal(t, 5, resolveSpecReviewMaxRevisions(config.ReviewGateConf{MaxRevisions: 0}, true),
		"configured 0 falls back to default 3 then floor 5")
	assert.Equal(t, 2, resolveSpecReviewMaxRevisions(config.ReviewGateConf{MaxRevisions: 2}, false),
		"LoopMode false preserves configured 2")
}

// S9 (REQ-005): wrapSpecLoadError preserves the cause and SPEC ID without
// asserting an empty body.
func TestWrapSpecLoadError_PreservesCause(t *testing.T) {
	t.Parallel()

	errBoom := errors.New("parse spec.md: malformed frontmatter")
	wrapped := wrapSpecLoadError("SPEC-SPECREV-002", errBoom)

	require.Error(t, wrapped)
	msg := wrapped.Error()
	assert.Contains(t, msg, "SPEC-SPECREV-002")
	assert.Contains(t, msg, "parse spec.md: malformed frontmatter")
	assert.NotContains(t, msg, "본문이 비어있습니다")
	assert.True(t, errors.Is(wrapped, errBoom), "cause must be unwrappable")
}

// S10 (REQ-005): a real spec.Load failure (missing SPEC ID header) is reported
// with its actual cause once wrapped.
func TestWrapSpecLoadError_RealLoadFailure(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "spec.md"),
		[]byte("# 제목만 있고 SPEC ID 헤더가 없다\n\n본문\n"),
		0o644,
	))

	_, loadErr := spec.Load(dir)
	require.Error(t, loadErr)
	assert.Contains(t, loadErr.Error(), "SPEC ID를 찾을 수 없습니다")

	wrapped := wrapSpecLoadError("SPEC-SPECREV-002", loadErr)
	msg := wrapped.Error()
	assert.Contains(t, msg, "SPEC-SPECREV-002")
	assert.Contains(t, msg, loadErr.Error())
	assert.False(t, strings.Contains(msg, "본문이 비어있습니다"))
}
