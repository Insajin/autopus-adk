package rulecond

// SPEC-STICKYRULE-001 exit-barrier oracle.
//
// REQ-STICKYRULE-FIRE-11 requires an enforced barrier, not an asserted one: an
// unrecovered Go panic terminates the process with exit status 2, and on
// UserPromptSubmit that status erases the user's prompt. A top-level deferred
// recover must convert the whole defect class into a silent no-op.
//
// The test seam this file requires is an unexported package variable:
//
//	// stickyPanicHook is a test seam. The sticky runtime invokes it during
//	// context assembly when non-nil.
//	var stickyPanicHook func()
//
// It stays unexported so the production surface carries no test-only API.

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// barrierRoot builds a minimal project root with one contained sticky rule.
func barrierRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	bodyRoot := filepath.Join(root, filepath.FromSlash(ClaudeRulesRelDir))
	require.NoError(t, os.MkdirAll(bodyRoot, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bodyRoot, "language-policy.md"),
		[]byte("---\nname: language-policy\n---\n# Language Policy\n\nBARRIER-MARKER\n"), 0o644))

	manifestPath := filepath.Join(root, filepath.FromSlash(ManifestRelPath))
	require.NoError(t, os.MkdirAll(filepath.Dir(manifestPath), 0o755))
	blob, err := json.Marshal(Manifest{
		Version: ManifestVersion,
		Sticky:  []StickyRule{{Name: "language-policy", Body: "language-policy.md"}},
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(manifestPath, blob, 0o644))
	return root
}

const barrierPayload = `{"hook_event_name":"UserPromptSubmit","session_id":"S-barrier","prompt":"hi"}`

// TestStickyFire_RecoverBarrierSuppressesAPanic is the S13 exit-barrier row.
// An injected panic during context assembly must produce no error, no partial
// stdout, and no diagnostic.
func TestStickyFire_RecoverBarrierSuppressesAPanic(t *testing.T) {
	root := barrierRoot(t)

	stickyPanicHook = func() { panic("injected fault during truncation") }
	t.Cleanup(func() { stickyPanicHook = nil })

	var stdout, stderr bytes.Buffer
	err := StickyFire(root, EventUserPromptSubmit, 1, strings.NewReader(barrierPayload), &stdout, &stderr)

	require.NoError(t, err,
		"a panic must not surface as an error; the caller would translate it into a non-zero exit")
	assert.Empty(t, stdout.String(),
		"the barrier discards partial stdout so no half-written payload reaches Claude Code")
	assert.Empty(t, stderr.String())
	assert.NotContains(t, stdout.String(), "BARRIER-MARKER")
}

// TestStickyFire_NoPanicHookStillInjects is the control: the barrier must not
// suppress the ordinary path.
func TestStickyFire_NoPanicHookStillInjects(t *testing.T) {
	root := barrierRoot(t)
	stickyPanicHook = nil

	var stdout, stderr bytes.Buffer
	require.NoError(t, StickyFire(root, EventUserPromptSubmit, 1,
		strings.NewReader(barrierPayload), &stdout, &stderr))

	assert.Contains(t, stdout.String(), "BARRIER-MARKER")
	assert.Contains(t, stdout.String(), EventUserPromptSubmit)
	assert.Empty(t, stderr.String())
}

// TestStickyEventConstant pins the platform contract: a bare additionalContext
// without this hookEventName is not recognized as structured hook output.
func TestStickyEventConstant(t *testing.T) {
	assert.Equal(t, "UserPromptSubmit", EventUserPromptSubmit)
	assert.Equal(t, ".autopus/runtime/sticky-rules",
		filepath.ToSlash(StickyStateDirRelPath))
}
