package cli

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSyncCmdWiring proves `auto sync verify` is reachable through the real root
// command and that the parent `sync` is help-only with no bare behavior.
func TestSyncCmdWiring(t *testing.T) {
	root := t.TempDir()
	initSyncRepo(t, root)
	modA := nestedRepo(t, root, "mod-a")
	syncWrite(t, modA, "pkg/x.go", "package pkg\n")

	cmd := NewRootCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"sync", "verify", "--dir", root})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, buf.String(), "git -C mod-a add")
	assert.Contains(t, buf.String(), "read-only (no git mutations)")

	// Parent "sync" alone renders help and produces no commit plan.
	parent := NewRootCmd()
	var pbuf bytes.Buffer
	parent.SetOut(&pbuf)
	parent.SetErr(&pbuf)
	parent.SetArgs([]string{"sync"})
	require.NoError(t, parent.Execute())
	assert.NotContains(t, pbuf.String(), "Phase A — module commits")
}

// TestClassifySpecLocationBranches exercises every ownership-vs-location branch.
func TestClassifySpecLocationBranches(t *testing.T) {
	// Ambiguous (no referenced module) is suppressed.
	assert.Empty(t, classifySpecLocation("SPEC-X-1", ".", nil))
	// Root SPEC bound to a single module -> misplacement.
	assert.Contains(t, classifySpecLocation("SPEC-X-1", ".", []string{"mod-a"}), "misplacement")
	// Module SPEC referencing a different single module -> location-mismatch.
	w := classifySpecLocation("SPEC-X-1", "mod-a", []string{"mod-b"})
	assert.Contains(t, w, "location-mismatch")
	assert.Contains(t, w, "mod-b/.autopus/specs/")
	// Module SPEC correctly placed in its own module -> no warning.
	assert.Empty(t, classifySpecLocation("SPEC-X-1", "mod-a", []string{"mod-a"}))
	// Cross-module SPEC correctly at root -> no warning.
	assert.Empty(t, classifySpecLocation("SPEC-X-1", ".", []string{"mod-a", "mod-b"}))
	// Cross-module SPEC stranded in a module -> location-mismatch.
	assert.Contains(t, classifySpecLocation("SPEC-X-1", "mod-a", []string{"mod-a", "mod-b"}), "cross-module")
}

// TestSyncVerifyModuleCarriesRootMeta covers the reverse misplacement direction:
// a nested module holding a root-scoped project-context doc.
func TestSyncVerifyModuleCarriesRootMeta(t *testing.T) {
	root := t.TempDir()
	initSyncRepo(t, root)
	modA := nestedRepo(t, root, "mod-a")
	syncWrite(t, modA, ".autopus/project/stray.md", "stray\n")

	var buf bytes.Buffer
	n, err := executeSyncVerify(&buf, root, "", false)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, n, 1)
	assert.Contains(t, buf.String(), "root-scoped meta docs")
	assert.Contains(t, buf.String(), "mod-a")
}

// TestSyncVerifySpecNotFound returns a distinct (non-"invalid") error.
func TestSyncVerifySpecNotFound(t *testing.T) {
	root := t.TempDir()
	initSyncRepo(t, root)
	nestedRepo(t, root, "mod-a")

	var buf bytes.Buffer
	_, err := executeSyncVerify(&buf, root, "SPEC-DOES-NOT-EXIST", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
	assert.NotContains(t, err.Error(), "invalid --spec")
}
