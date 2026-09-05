package gates

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// reuseTree builds a project root with a package glob target and a go.sum
// style dynamic dependency, records passing build evidence, and returns the
// root plus SPEC dir.
func reuseTree(t *testing.T, opts func(*RecordOptions)) (root, specDir string) {
	t.Helper()
	root = t.TempDir()
	specDir = filepath.Join(root, ".autopus", "specs", "SPEC-REUSE-001")
	require.NoError(t, os.MkdirAll(filepath.Join(root, "pkg", "a"), 0o755))
	require.NoError(t, os.MkdirAll(specDir, 0o755))
	writeFile(t, filepath.Join(root, "pkg", "a", "x.go"), "package a\n")
	writeFile(t, filepath.Join(root, "pkg", "a", "y.go"), "package a\n// y\n")
	writeFile(t, filepath.Join(root, "go.sum"), "example.com/dep v1.0.0 h1:abc\n")

	record := RecordOptions{
		SpecID:      "SPEC-REUSE-001",
		Gate:        GateBuild,
		Status:      StatusPass,
		InputGlobs:  []string{"pkg/a/*.go"},
		DynamicDeps: []string{"go.sum"},
		Command:     "go build ./pkg/a",
		Now:         decideNow.Add(-time.Hour),
	}
	if opts != nil {
		opts(&record)
	}
	receipt, err := BuildEvidence(root, record)
	require.NoError(t, err)
	_, err = WriteEvidence(specDir, receipt)
	require.NoError(t, err)
	return root, specDir
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func decideWithPrior(t *testing.T, root, specDir string, maxAge time.Duration) GateDecision {
	t.Helper()
	prior, err := LoadPriorEvidence(root, specDir)
	require.NoError(t, err)
	receipt := Decide(DecisionInput{
		SpecID:         "SPEC-REUSE-001",
		Classification: Classify([]string{"pkg/a/x.go"}, nil),
		Prior:          prior,
		Now:            decideNow,
		MaxAge:         maxAge,
	})
	return applicabilityOf(t, receipt, GateBuild)
}

func TestReuse_IdenticalClosureIsReusable(t *testing.T) {
	root, specDir := reuseTree(t, nil)
	build := decideWithPrior(t, root, specDir, 0)

	assert.Equal(t, Reusable, build.Applicability)
	assert.Equal(t, "exact-input evidence matches current tree", build.Reason)
	assert.Len(t, build.InputClosureSHA256, 64)
	require.NotNil(t, build.ReusedEvidence)
	assert.Equal(t, ".autopus/specs/SPEC-REUSE-001/gates/evidence-build.json", build.ReusedEvidence.Path)
	assert.Equal(t, StatusPass, build.ReusedEvidence.Status)
	assert.Equal(t, "2026-09-06T11:00:00Z", build.ReusedEvidence.ObservedAt)
}

func TestReuse_OneInputByteChangedIsRequired(t *testing.T) {
	root, specDir := reuseTree(t, nil)
	writeFile(t, filepath.Join(root, "pkg", "a", "x.go"), "package a\n\n")
	build := decideWithPrior(t, root, specDir, 0)

	assert.Equal(t, Required, build.Applicability)
	assert.Contains(t, build.Reason, ReasonClosureChanged)
	assert.Nil(t, build.ReusedEvidence)
}

func TestReuse_NewFileMatchingGlobIsRequired(t *testing.T) {
	root, specDir := reuseTree(t, nil)
	writeFile(t, filepath.Join(root, "pkg", "a", "z.go"), "package a\n")
	build := decideWithPrior(t, root, specDir, 0)

	assert.Equal(t, Required, build.Applicability)
	assert.Contains(t, build.Reason, ReasonClosureChanged)
}

func TestReuse_DynamicDependencyChangedIsRequired(t *testing.T) {
	root, specDir := reuseTree(t, nil)
	writeFile(t, filepath.Join(root, "go.sum"), "example.com/dep v1.0.1 h1:def\n")
	build := decideWithPrior(t, root, specDir, 0)

	assert.Equal(t, Required, build.Applicability)
	assert.Contains(t, build.Reason, ReasonClosureChanged)
}

func TestReuse_MissingDynamicDependencyIsRequired(t *testing.T) {
	root, specDir := reuseTree(t, nil)
	require.NoError(t, os.Remove(filepath.Join(root, "go.sum")))
	build := decideWithPrior(t, root, specDir, 0)

	assert.Equal(t, Required, build.Applicability)
	assert.Contains(t, build.Reason, ReasonMissingInput+" go.sum")
}

func TestReuse_PriorFailIsRequired(t *testing.T) {
	root, specDir := reuseTree(t, func(o *RecordOptions) { o.Status = StatusFail })
	build := decideWithPrior(t, root, specDir, 0)

	assert.Equal(t, Required, build.Applicability)
	assert.Contains(t, build.Reason, ReasonPriorFail)
}

func TestReuse_PriorPartialStatusIsRequired(t *testing.T) {
	root, specDir := reuseTree(t, func(o *RecordOptions) { o.Status = StatusPartial })
	build := decideWithPrior(t, root, specDir, 0)

	assert.Equal(t, Required, build.Applicability)
	assert.Contains(t, build.Reason, ReasonPriorPartial)
}

func TestReuse_IncompletePassIsRequired(t *testing.T) {
	root, specDir := reuseTree(t, func(o *RecordOptions) { o.Partial = true })
	build := decideWithPrior(t, root, specDir, 0)

	assert.Equal(t, Required, build.Applicability)
	assert.Contains(t, build.Reason, ReasonPriorPartial)
}

func TestReuse_OlderThanMaxAgeIsRequired(t *testing.T) {
	root, specDir := reuseTree(t, func(o *RecordOptions) { o.Now = decideNow.Add(-DefaultMaxAge - time.Minute) })
	build := decideWithPrior(t, root, specDir, 0)

	assert.Equal(t, Required, build.Applicability)
	assert.Contains(t, build.Reason, ReasonStale)
}

func TestReuse_MaxAgeBoundaryIsInclusive(t *testing.T) {
	root, specDir := reuseTree(t, func(o *RecordOptions) { o.Now = decideNow.Add(-2 * time.Hour) })
	assert.Equal(t, Reusable, decideWithPrior(t, root, specDir, 2*time.Hour).Applicability)
	assert.Equal(t, Required, decideWithPrior(t, root, specDir, 2*time.Hour-time.Second).Applicability)
}

func TestReuse_MissingPriorIsRequired(t *testing.T) {
	root, specDir := reuseTree(t, nil)
	require.NoError(t, os.Remove(EvidencePath(specDir, GateBuild)))
	build := decideWithPrior(t, root, specDir, 0)

	assert.Equal(t, Required, build.Applicability)
	assert.Contains(t, build.Reason, ReasonNoPriorEvidence)
}

func TestReuse_NeverOverridesNotApplicableOrBlocked(t *testing.T) {
	root, specDir := reuseTree(t, func(o *RecordOptions) { o.Gate = GateIntegration })
	prior, err := LoadPriorEvidence(root, specDir)
	require.NoError(t, err)
	receipt := Decide(DecisionInput{
		SpecID:         "SPEC-REUSE-001",
		Classification: Classify([]string{"pkg/a/x.go"}, nil),
		Prior:          prior,
		Now:            decideNow,
	})
	integration := applicabilityOf(t, receipt, GateIntegration)
	assert.Equal(t, NotApplicable, integration.Applicability)
	assert.Nil(t, integration.ReusedEvidence)
}
