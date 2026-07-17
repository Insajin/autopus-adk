package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- S10: --spec traversal rejection ---

func TestSyncVerifyS10TraversalRejected(t *testing.T) {
	// A non-workspace directory: validation must fire before any disk access.
	dir := t.TempDir()
	for _, bad := range []string{"../../etc", "SPEC-FOO/../../x", "/etc/passwd", "spec-foo"} {
		var buf bytes.Buffer
		_, err := executeSyncVerify(&buf, dir, bad, false)
		require.Error(t, err, "input %q must be rejected", bad)
		assert.Contains(t, err.Error(), "invalid --spec", "input %q", bad)
		// No workspace traversal happened: nothing was rendered.
		assert.Empty(t, buf.String(), "input %q produced output", bad)
		// No absolute path leaks into the error surface.
		for _, line := range strings.Split(err.Error(), "\n") {
			assert.False(t, strings.HasPrefix(strings.TrimSpace(line), "/"), "abs path leak: %q", line)
		}
	}
}

func TestSyncVerifyValidateSpecID(t *testing.T) {
	require.NoError(t, validateSpecID("SPEC-ADK-SYNC-VERIFY-001"))
	require.NoError(t, validateSpecID("SPEC-FOO-1"))
	for _, bad := range []string{"", "../../etc", "SPEC-foo", "SPEC-FOO/../x", "spec-1", "SPEC_FOO"} {
		assert.Error(t, validateSpecID(bad), "input %q", bad)
	}
}

// --- absolute-path / secret non-exposure ---

func TestSyncVerifyOutputHasNoAbsolutePaths(t *testing.T) {
	root := t.TempDir()
	initSyncRepo(t, root)
	modA := nestedRepo(t, root, "mod-a")
	syncWrite(t, root, ".autopus/project/product.md", "product\n")
	syncWrite(t, modA, "pkg/x.go", "package pkg\n")

	var buf bytes.Buffer
	_, err := executeSyncVerify(&buf, root, "", false)
	require.NoError(t, err)
	out := buf.String()

	assert.NotContains(t, out, root, "absolute meta-root path must not appear")
	for _, line := range strings.Split(out, "\n") {
		trimmed := strings.TrimSpace(line)
		assert.False(t, strings.HasPrefix(trimmed, "/"), "output line is an absolute path: %q", line)
	}
	// Sanity: the plan still uses workspace-relative repo labels.
	assert.Contains(t, out, "git -C mod-a add")
	assert.Contains(t, out, "git -C . add")
}
