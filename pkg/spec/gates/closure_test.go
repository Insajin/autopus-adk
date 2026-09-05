package gates

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func closureTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, dir := range []string{"pkg/a", "pkg/a/deep", "pkg/b", ".git/objects"} {
		require.NoError(t, os.MkdirAll(filepath.Join(root, dir), 0o755))
	}
	writeFile(t, filepath.Join(root, "top.go"), "package top\n")
	writeFile(t, filepath.Join(root, "pkg", "a", "x.go"), "package a\n")
	writeFile(t, filepath.Join(root, "pkg", "a", "deep", "d.go"), "package deep\n")
	writeFile(t, filepath.Join(root, "pkg", "b", "y.go"), "package b\n")
	writeFile(t, filepath.Join(root, ".git", "objects", "blob.go"), "ignored\n")
	require.NoError(t, os.Symlink(filepath.Join(root, "pkg", "b", "y.go"), filepath.Join(root, "pkg", "a", "link.go")))
	return root
}

func paths(entries []InputEntry) []string {
	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		out = append(out, entry.Path)
	}
	return out
}

func TestResolveClosure_GlobSemantics(t *testing.T) {
	root := closureTree(t)

	inputs, _, err := ResolveClosure(root, []string{"*.go"}, nil)
	require.NoError(t, err)
	assert.Equal(t, []string{"top.go"}, paths(inputs), "a bare *.go only matches top-level files")

	inputs, _, err = ResolveClosure(root, []string{"**/*.go"}, nil)
	require.NoError(t, err)
	assert.Equal(t, []string{"pkg/a/deep/d.go", "pkg/a/x.go", "pkg/b/y.go", "top.go"}, paths(inputs),
		"** recurses, skips symlinks and .git, and sorts")

	inputs, _, err = ResolveClosure(root, []string{"pkg/a/*.go", "pkg/a/x.go"}, nil)
	require.NoError(t, err)
	assert.Equal(t, []string{"pkg/a/x.go"}, paths(inputs), "overlapping globs deduplicate")

	inputs, _, err = ResolveClosure(root, []string{"pkg/missing/*.go", "nope.go"}, nil)
	require.NoError(t, err)
	assert.Empty(t, inputs)
}

func TestResolveClosure_RejectsEscapingGlobs(t *testing.T) {
	root := closureTree(t)
	_, _, err := ResolveClosure(root, []string{"../*.go"}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "escapes the project root")
}

func TestResolveClosure_MissingDynamicDependency(t *testing.T) {
	root := closureTree(t)
	inputs, deps, err := ResolveClosure(root, []string{"top.go"}, []string{"go.sum", "top.go"})
	var missing *MissingInputError
	require.ErrorAs(t, err, &missing)
	assert.Equal(t, []string{"go.sum"}, missing.Paths)
	assert.Equal(t, []string{"top.go"}, paths(inputs), "resolved entries are still returned")
	assert.Equal(t, []string{"top.go"}, paths(deps))
}

func TestClosureSHA256_CanonicalForm(t *testing.T) {
	a := InputEntry{Path: "a.go", SHA256: "1111"}
	b := InputEntry{Path: "b.go", SHA256: "2222"}
	expected := sha256.Sum256([]byte("a.go\x001111\nb.go\x002222\n"))

	assert.Equal(t, hex.EncodeToString(expected[:]), ClosureSHA256([]InputEntry{b}, []InputEntry{a}),
		"entries are sorted by path regardless of group order")
	assert.Equal(t, ClosureSHA256([]InputEntry{a, b}), ClosureSHA256([]InputEntry{a, b}, []InputEntry{b}),
		"a dynamic dependency also matched by a glob counts once")
	assert.NotEqual(t, ClosureSHA256([]InputEntry{a, b}), ClosureSHA256([]InputEntry{a}))
}

func TestBuildEvidence_RejectsInvalidInputs(t *testing.T) {
	root := closureTree(t)
	base := RecordOptions{SpecID: "S", Gate: GateBuild, Status: StatusPass, InputGlobs: []string{"top.go"}, Now: decideNow}

	cases := []struct {
		name   string
		mutate func(*RecordOptions)
		want   string
	}{
		{"unknown gate", func(o *RecordOptions) { o.Gate = "lint" }, "unknown gate"},
		{"invalid status", func(o *RecordOptions) { o.Status = "ok" }, "invalid status"},
		{"no inputs", func(o *RecordOptions) { o.InputGlobs = nil }, "at least one input glob"},
		{"unmatched glob", func(o *RecordOptions) { o.InputGlobs = []string{"pkg/none/*.go"} }, "matched no files"},
		{"missing dynamic dep", func(o *RecordOptions) { o.DynamicDeps = []string{"go.sum"} }, "missing input: go.sum"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts := base
			tc.mutate(&opts)
			_, err := BuildEvidence(root, opts)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestBuildEvidence_CompletenessAndBodyFreeReceipt(t *testing.T) {
	root := closureTree(t)
	receipt, err := BuildEvidence(root, RecordOptions{
		SpecID: "S", Gate: GateUnitTests, Status: StatusPass, Partial: true,
		InputGlobs: []string{"pkg/**/*.go"}, DynamicDeps: []string{"top.go"}, Command: " go test ./pkg/... ", Now: decideNow,
	})
	require.NoError(t, err)
	assert.False(t, receipt.Complete, "--partial marks a passing run incomplete")
	assert.Equal(t, "go test ./pkg/...", receipt.Command)
	assert.Equal(t, []string{"pkg/a/deep/d.go", "pkg/a/x.go", "pkg/b/y.go"}, paths(receipt.Inputs))
	assert.Equal(t, []string{"top.go"}, paths(receipt.DynamicDeps))
	assert.Equal(t, ClosureSHA256(receipt.Inputs, receipt.DynamicDeps), receipt.InputClosureSHA256)
	for _, entry := range append(receipt.Inputs, receipt.DynamicDeps...) {
		assert.Len(t, entry.SHA256, 64)
	}

	partial, err := BuildEvidence(root, RecordOptions{SpecID: "S", Gate: GateBuild, Status: StatusPartial, InputGlobs: []string{"top.go"}, Now: decideNow})
	require.NoError(t, err)
	assert.False(t, partial.Complete, "status partial is never complete")
}

func TestReceipts_RoundTripWithPrivatePermissions(t *testing.T) {
	root := closureTree(t)
	specDir := filepath.Join(root, ".autopus", "specs", "SPEC-IO-001")
	require.NoError(t, os.MkdirAll(specDir, 0o755))

	evidence, err := BuildEvidence(root, RecordOptions{SpecID: "SPEC-IO-001", Gate: GateBuild, Status: StatusPass, InputGlobs: []string{"top.go"}, Now: decideNow})
	require.NoError(t, err)
	evidencePath, err := WriteEvidence(specDir, evidence)
	require.NoError(t, err)
	info, err := os.Stat(evidencePath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	loaded, ok, err := ReadEvidence(specDir, GateBuild)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, evidence, loaded)
	_, ok, err = ReadEvidence(specDir, GateUnitTests)
	require.NoError(t, err)
	assert.False(t, ok)

	receipt := Decide(DecisionInput{SpecID: "SPEC-IO-001", Classification: Classify([]string{"top.go"}, nil), Now: decideNow})
	receipt.Decisions[0], receipt.Decisions[1] = receipt.Decisions[1], receipt.Decisions[0]
	applicabilityPath, err := WriteApplicability(specDir, receipt)
	require.NoError(t, err)
	assert.Equal(t, GateBuild, receipt.Decisions[0].Gate, "the caller's slice is not reordered")
	data, err := os.ReadFile(applicabilityPath)
	require.NoError(t, err)
	assert.Equal(t, ApplicabilityPath(specDir), applicabilityPath)
	assert.Contains(t, string(data), "\"schema\": \"autopus.gate-applicability.v1\"")

	reloaded, err := ReadApplicability(specDir)
	require.NoError(t, err)
	assert.Equal(t, GateRiskFirstProbe, reloaded.Decisions[0].Gate, "decisions are persisted in catalog order")
	assert.Equal(t, GateBuild, reloaded.Decisions[1].Gate)
}
