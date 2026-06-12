package skillevolve

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCandidateHasSourceHashAllThreeSources asserts the helper finds a sha256 hash
// from top-level SourceHashes, Provenance.SourceHashes, and SourceFailures.
func TestCandidateHasSourceHashAllThreeSources(t *testing.T) {
	t.Parallel()

	// SourceHashes field.
	assert.True(t, candidateHasSourceHash(CandidateBundle{
		SourceHashes: []string{"sha256:abc123"},
	}))
	// Provenance.SourceHashes field.
	assert.True(t, candidateHasSourceHash(CandidateBundle{
		Provenance: CandidateProvenance{SourceHashes: []string{"sha256:def456"}},
	}))
	// SourceFailures.Hash field.
	assert.True(t, candidateHasSourceHash(CandidateBundle{
		SourceFailures: []SourceFailure{{Hash: "sha256:ghi789"}},
	}))
	// No sha256 prefix — must return false.
	assert.False(t, candidateHasSourceHash(CandidateBundle{
		SourceHashes: []string{"md5:abc"},
	}))
	// Empty candidate.
	assert.False(t, candidateHasSourceHash(CandidateBundle{}))
}

// TestCanonicalEvidencePathResolvesExistingFile asserts the function returns the
// cleaned absolute path for a real file and false for empty input.
func TestCanonicalEvidencePathResolvesExistingFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	file := filepath.Join(dir, "evidence.json")
	require.NoError(t, os.WriteFile(file, []byte("{}"), 0o644))

	abs, ok := canonicalEvidencePath(file)
	assert.True(t, ok)
	// Use EvalSymlinks to handle macOS /var -> /private/var symlink.
	expectedAbs, err := filepath.EvalSymlinks(file)
	require.NoError(t, err)
	assert.Equal(t, filepath.Clean(expectedAbs), abs)

	// Empty input must return false.
	_, ok2 := canonicalEvidencePath("")
	assert.False(t, ok2)
}

// TestStripEvidenceFragmentRemovesHashPart asserts fragment stripping on various
// URI-like paths.
func TestStripEvidenceFragmentRemovesHashPart(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input string
		want  string
	}{
		{".autopus/qa/evidence/run.json#scenario-1", ".autopus/qa/evidence/run.json"},
		{"plain/path", "plain/path"},
		{"", ""},
		{"#fragment-only", ""},
		{"path#", "path"},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			assert.Equal(t, tc.want, stripEvidenceFragment(tc.input))
		})
	}
}

// TestHasAffectedOutputMappingAllThreeSources asserts the helper finds an output
// mapping from AffectedRefs, ProposedFiles, and Provenance.AffectedSourceOfTruths.
func TestHasAffectedOutputMappingAllThreeSources(t *testing.T) {
	t.Parallel()

	// AffectedRefs.
	assert.True(t, hasAffectedOutputMapping(CandidateBundle{AffectedRefs: []string{"content/skills/foo.md"}}))
	// ProposedFiles.
	assert.True(t, hasAffectedOutputMapping(CandidateBundle{ProposedFiles: []ProposedFile{{Path: "content/skills/bar.md"}}}))
	// Provenance.AffectedSourceOfTruths.
	assert.True(t, hasAffectedOutputMapping(CandidateBundle{
		Provenance: CandidateProvenance{AffectedSourceOfTruths: []string{"content/skills/baz.md"}},
	}))
	// All empty — must return false.
	assert.False(t, hasAffectedOutputMapping(CandidateBundle{}))
}

// TestHasAffectedAcceptanceMappingAllFourSources asserts the helper finds an
// acceptance mapping from the four possible locations.
func TestHasAffectedAcceptanceMappingAllFourSources(t *testing.T) {
	t.Parallel()

	// AffectedAcceptanceIDs.
	assert.True(t, hasAffectedAcceptanceMapping(CandidateBundle{AffectedAcceptanceIDs: []string{"AC-001"}}))
	// ReplayPlan.AcceptanceRefs.
	assert.True(t, hasAffectedAcceptanceMapping(CandidateBundle{ReplayPlan: ReplayPlan{AcceptanceRefs: []string{"AC-002"}}}))
	// Provenance.AffectedAcceptanceIDs.
	assert.True(t, hasAffectedAcceptanceMapping(CandidateBundle{
		Provenance: CandidateProvenance{AffectedAcceptanceIDs: []string{"AC-003"}},
	}))
	// ReplayPlan.MustChecks with AcceptanceRef.
	assert.True(t, hasAffectedAcceptanceMapping(CandidateBundle{
		ReplayPlan: ReplayPlan{MustChecks: []ReplayCheckRef{{AcceptanceRef: "AC-004"}}},
	}))
	// All empty — must return false.
	assert.False(t, hasAffectedAcceptanceMapping(CandidateBundle{}))
}

// TestStatusOrUnknownFallback asserts empty/whitespace returns "unknown" and non-empty
// passes through.
func TestStatusOrUnknownFallback(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "unknown", statusOrUnknown(""))
	assert.Equal(t, "unknown", statusOrUnknown("   "))
	assert.Equal(t, "passed", statusOrUnknown("passed"))
	assert.Equal(t, "failed", statusOrUnknown("  failed  "))
}

// TestPathWithinProjectForWriteNewPath asserts a non-existent path inside the project
// dir is accepted, and a path outside is rejected.
func TestPathWithinProjectForWriteNewPath(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Existing file inside the project.
	f := filepath.Join(dir, "content", "skills", "foo.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(f), 0o755))
	require.NoError(t, os.WriteFile(f, []byte("x"), 0o644))
	assert.True(t, pathWithinProjectForWrite(dir, f))

	// Non-existent path inside the project (parent exists).
	newFile := filepath.Join(dir, "content", "skills", "new.md")
	assert.True(t, pathWithinProjectForWrite(dir, newFile))

	// Path outside the project.
	outside := filepath.Join(t.TempDir(), "outside.md")
	assert.False(t, pathWithinProjectForWrite(dir, outside))

	// Empty inputs are permissive.
	assert.True(t, pathWithinProjectForWrite("", "anything"))
	assert.True(t, pathWithinProjectForWrite("anything", ""))
}

// TestEvidenceRefMatchesProjectPathSamePath asserts identical clean paths match.
func TestEvidenceRefMatchesProjectPathSamePath(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	file := filepath.Join(dir, "run-index.json")
	require.NoError(t, os.WriteFile(file, []byte("{}"), 0o644))

	// Identical path — must match.
	assert.True(t, evidenceRefMatchesProjectPath(dir, file, file))
	// Fragment on one side must still match the base path.
	assert.True(t, evidenceRefMatchesProjectPath(dir, file+"#fragment", file))
	// Empty ref or target — must not match.
	assert.False(t, evidenceRefMatchesProjectPath(dir, "", file))
	assert.False(t, evidenceRefMatchesProjectPath(dir, file, ""))
}
