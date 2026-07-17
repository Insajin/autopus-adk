package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- S6: --spec owned vs unrelated split ---

func TestSyncVerifyS6SpecOwnershipSplit(t *testing.T) {
	root := t.TempDir()
	initSyncRepo(t, root)
	modA := nestedRepo(t, root, "mod-a")
	// SPEC directory committed (clean) so only the two pkg files are dirty.
	syncWrite(t, modA, ".autopus/specs/SPEC-FOO-001/plan.md",
		"# plan\n\n- [ ] T1: implement pkg/foo.go\n")
	syncWrite(t, modA, ".autopus/specs/SPEC-FOO-001/spec.md", "# spec\n")
	syncGit(t, modA, "add", ".autopus/specs/SPEC-FOO-001")
	syncGit(t, modA, "commit", "-m", "spec")
	syncWrite(t, modA, "pkg/foo.go", "package pkg\n")
	syncWrite(t, modA, "pkg/unrelated.go", "package pkg\n")

	repos, err := collectDirty(root)
	require.NoError(t, err)
	owned, unrelated, err := splitSpecOwnership(repos, "SPEC-FOO-001", classifyWorkspace(repos))
	require.NoError(t, err)
	assert.Equal(t, []string{"mod-a/pkg/foo.go"}, owned)
	assert.Equal(t, []string{"mod-a/pkg/unrelated.go"}, unrelated)

	var buf bytes.Buffer
	n, err := executeSyncVerify(&buf, root, "SPEC-FOO-001", false)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, n, 1, "unrelated file triggers a mixing warning")
	out := buf.String()
	assert.Contains(t, out, "owned:")
	assert.Contains(t, out, "pkg/foo.go")
	assert.Contains(t, out, "unrelated:")
	assert.Contains(t, out, "pkg/unrelated.go")
	assert.Contains(t, out, "unrelated-mixing")
}

// --- S7: deterministic plan ordering ---

func TestSyncVerifyS7DeterministicOrdering(t *testing.T) {
	root := t.TempDir()
	initSyncRepo(t, root)
	// Create modules out of order to prove alphabetical sorting.
	for _, name := range []string{"mod-c", "mod-a", "mod-b"} {
		dir := nestedRepo(t, root, name)
		syncWrite(t, dir, "pkg/x.go", "package pkg\n")
	}
	syncWrite(t, root, "ARCHITECTURE.md", "# arch\n")

	var buf bytes.Buffer
	_, err := executeSyncVerify(&buf, root, "", false)
	require.NoError(t, err)
	out := buf.String()

	iA := strings.Index(out, "git -C mod-a add")
	iB := strings.Index(out, "git -C mod-b add")
	iC := strings.Index(out, "git -C mod-c add")
	iMeta := strings.Index(out, "git -C . add")
	require.True(t, iA >= 0 && iB >= 0 && iC >= 0 && iMeta >= 0, "all repo lines present:\n%s", out)
	assert.Less(t, iA, iB, "mod-a before mod-b")
	assert.Less(t, iB, iC, "mod-b before mod-c")
	assert.Less(t, iC, iMeta, "Phase A before Phase B meta")

	// Each module commit line is accompanied by a Lore reminder.
	assert.Equal(t, 4, strings.Count(out, loreReminderLine), "one reminder per commit step")
	assert.Contains(t, out, "Phase A — module commits:")
	assert.Contains(t, out, "Phase B — meta commit:")
}

// --- S9: exit contract (0 / 1 / 0) ---

func TestSyncVerifyS9ExitContract(t *testing.T) {
	// Violation fixture: staged + unstaged mixing in root.
	viol := t.TempDir()
	initSyncRepo(t, viol)
	nestedRepo(t, viol, "mod-a")
	syncWrite(t, viol, "ARCHITECTURE.md", "# arch\n")
	syncGit(t, viol, "add", "ARCHITECTURE.md")
	syncWrite(t, viol, ".autopus/project/tech.md", "tech\n")

	// Clean fixture: only unstaged changes, no boundary violations.
	clean := t.TempDir()
	initSyncRepo(t, clean)
	cleanMod := nestedRepo(t, clean, "mod-a")
	syncWrite(t, clean, ".autopus/project/product.md", "product\n")
	syncWrite(t, cleanMod, "src/app.ts", "export const x = 1\n")

	// 1) violation, no --strict -> exit 0
	var b1 bytes.Buffer
	n1, err1 := executeSyncVerify(&b1, viol, "", false)
	require.NoError(t, err1)
	assert.GreaterOrEqual(t, n1, 1)

	// 2) violation, --strict -> exit 1 (sentinel)
	var b2 bytes.Buffer
	n2, err2 := executeSyncVerify(&b2, viol, "", true)
	assert.True(t, errors.Is(err2, errSyncVerifyStrict), "strict + violation returns sentinel")
	assert.GreaterOrEqual(t, n2, 1)

	// 3) no violation, --strict -> exit 0
	var b3 bytes.Buffer
	n3, err3 := executeSyncVerify(&b3, clean, "", true)
	require.NoError(t, err3, "strict + no violation returns nil")
	assert.Equal(t, 0, n3)
}
