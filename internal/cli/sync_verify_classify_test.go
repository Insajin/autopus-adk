package cli

import (
	"bytes"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- S2: Phase classification set ---

func TestSyncVerifyS2PhaseSets(t *testing.T) {
	root := t.TempDir()
	initSyncRepo(t, root)
	modA := nestedRepo(t, root, "mod-a")
	syncWrite(t, root, ".autopus/project/product.md", "product\n")
	syncWrite(t, root, "autopus.yaml", "version: 1\n")
	syncWrite(t, root, "CHANGELOG.md", "# changelog\n")
	syncWrite(t, modA, "src/app.ts", "export const x = 1\n")

	repos, err := collectDirty(root)
	require.NoError(t, err)
	phaseA, phaseB := classifyPhases(repos)

	expectedB := []string{".autopus/project/product.md", "CHANGELOG.md", "autopus.yaml"}
	sort.Strings(expectedB)
	assert.Equal(t, expectedB, phaseB.Files, "Phase B is exactly the root-tracked set")

	require.Len(t, phaseA, 1)
	assert.Equal(t, "mod-a", phaseA[0].RepoPath)
	assert.Equal(t, []string{"src/app.ts"}, phaseA[0].Files)

	// No root file is misclassified into Phase A.
	for _, g := range phaseA {
		for _, f := range g.Files {
			assert.NotContains(t, expectedB, f)
		}
	}
}

// --- S3: cross-boundary misplacement (root SPEC -> single module) ---

func TestSyncVerifyS3Misplacement(t *testing.T) {
	root := t.TempDir()
	initSyncRepo(t, root)
	nestedRepo(t, root, "mod-a")
	syncWrite(t, root, ".autopus/specs/SPEC-FOO-001/plan.md",
		"# plan\n\n- [ ] T1: implement mod-a/pkg/foo.go handler\n")

	var buf bytes.Buffer
	n, err := executeSyncVerify(&buf, root, "", false)
	require.NoError(t, err, "no --strict means exit 0")
	assert.GreaterOrEqual(t, n, 1)

	out := buf.String()
	assert.Contains(t, out, "SPEC-FOO-001")
	assert.Contains(t, out, "root")
	assert.Contains(t, out, "mod-a/.autopus/specs/")
}

// --- S4: SPEC location vs code-path module mismatch (module SPEC -> cross-module) ---

func TestSyncVerifyS4LocationMismatch(t *testing.T) {
	root := t.TempDir()
	initSyncRepo(t, root)
	modA := nestedRepo(t, root, "mod-a")
	nestedRepo(t, root, "mod-b")
	syncWrite(t, modA, ".autopus/specs/SPEC-BAR-001/plan.md",
		"# plan\n\n- [ ] T1: mod-a/pkg/a.go\n- [ ] T2: mod-b/pkg/b.go\n")

	var buf bytes.Buffer
	_, err := executeSyncVerify(&buf, root, "", false)
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "SPEC-BAR-001")
	assert.Contains(t, out, "cross-module")
	assert.Contains(t, out, ".autopus/specs/")
}

// --- S5: unrelated-file mixing (staged + unstaged coexist) ---

func TestSyncVerifyS5MixedStaging(t *testing.T) {
	root := t.TempDir()
	initSyncRepo(t, root)
	nestedRepo(t, root, "mod-a")
	syncWrite(t, root, "ARCHITECTURE.md", "# arch\n")
	syncGit(t, root, "add", "ARCHITECTURE.md")
	syncWrite(t, root, ".autopus/project/tech.md", "tech\n")

	var buf bytes.Buffer
	n, err := executeSyncVerify(&buf, root, "", false)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, n, 1)

	out := buf.String()
	assert.Contains(t, out, "staged and unstaged")
	assert.Contains(t, out, "ARCHITECTURE.md")
	assert.Contains(t, out, ".autopus/project/tech.md")
}
