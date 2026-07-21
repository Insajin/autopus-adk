package companionmanifest

import (
	"strings"
	"testing"
)

const a11A10AncestorSHA = "54536edc09c37a634532c2c9b51e62869d393db4"

func TestReleaseSourceValidator_A11AcceptsAnnotatedA10DescendantAndExactPins(t *testing.T) {
	dir := cloneCurrentReleaseRepository(t)
	sha := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD"))
	tree := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD^{tree}"))
	runGit(t, dir, "tag", "-am", "A11 release candidate", "v0.50.82")
	output, err := runReleaseSourceValidator(t, dir, "v0.50.82", sha,
		"COMPANION_SOURCE_PIN_REQUIRED=1",
		"COMPANION_APPROVED_SOURCE_COMMIT="+sha,
		"COMPANION_APPROVED_SOURCE_TREE="+tree,
	)
	if err != nil {
		t.Fatalf("annotated pinned A11 rejected: %v\n%s", err, output)
	}
	if !strings.Contains(output, "release-phase=A11") ||
		!strings.Contains(output, "source-commit="+sha) {
		t.Fatalf("validated A11 output = %q", output)
	}
}

func TestReleaseSourceValidator_A11RejectsInvalidIdentity(t *testing.T) {
	t.Run("lightweight", func(t *testing.T) {
		dir := cloneCurrentReleaseRepository(t)
		sha := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD"))
		runGit(t, dir, "tag", "v0.50.82")
		output, err := runReleaseSourceValidator(t, dir, "v0.50.82", sha)
		if err == nil || !strings.Contains(output, "A11 release tag must be annotated") {
			t.Fatalf("lightweight A11 result: %v\n%s", err, output)
		}
	})
	t.Run("missing_A10", func(t *testing.T) {
		dir, sha := newMinimalSourceRepository(t)
		runGit(t, dir, "tag", "-am", "orphan A11", "v0.50.82")
		output, err := runReleaseSourceValidator(t, dir, "v0.50.82", sha)
		if err == nil || !strings.Contains(output, "does not contain the immutable A10 release") {
			t.Fatalf("A10-free A11 result: %v\n%s", err, output)
		}
	})
	t.Run("unapproved_source", func(t *testing.T) {
		dir := cloneCurrentReleaseRepository(t)
		sha := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD"))
		tree := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD^{tree}"))
		runGit(t, dir, "tag", "-am", "A11 release candidate", "v0.50.82")
		output, err := runReleaseSourceValidator(t, dir, "v0.50.82", sha,
			"COMPANION_SOURCE_PIN_REQUIRED=1",
			"COMPANION_APPROVED_SOURCE_COMMIT="+strings.Repeat("a", 40),
			"COMPANION_APPROVED_SOURCE_TREE="+tree,
		)
		if err == nil ||
			!strings.Contains(output, "release commit differs from the approved exact source commit") {
			t.Fatalf("unapproved A11 source result: %v\n%s", err, output)
		}
	})
}

func TestReleaseSourceValidator_A11PinsDirectA10Ancestor(t *testing.T) {
	source := readReleaseFile(t, "scripts/companion-release/validate-source.sh")
	declaration := "readonly A11_A10_ANCESTOR_SHA='" + a11A10AncestorSHA + "'"
	if strings.Count(source, declaration) != 1 {
		t.Fatalf("A11 immutable A10 ancestry pin drifted: %s", declaration)
	}
	for _, required := range []string{
		`git cat-file -t "refs/tags/$GITHUB_REF_NAME"`,
		`git merge-base --is-ancestor "$A11_A10_ANCESTOR_SHA" "$GITHUB_SHA"`,
		`[[ "$tag_object_type" == 'tag' ]]`,
		"COMPANION_APPROVED_SOURCE_COMMIT", "COMPANION_APPROVED_SOURCE_TREE",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("A11 source gate missing %q", required)
		}
	}
}

func TestLineageVerifier_A11PinsExactImmutableA10Evidence(t *testing.T) {
	pins := readReleaseFile(t, "scripts/companion-release/verify-public-key-lineage-pins.sh")
	for _, declaration := range []string{
		"A10_COMMIT_SHA='54536edc09c37a634532c2c9b51e62869d393db4'",
		"A10_TREE_SHA='e9a30f4530e06c9b62933e7bf97e0056faed259c'",
		"A10_TAG_OBJECT_SHA='8b37fccb57255fc24003dc3af2700334f4a8d3c4'",
		"A10_CHECKSUMS_SHA256='2e97c1f3c8d0cba0f93dd83c724c71eaa4966c79d4812a6a9cf034144c7b178d'",
		"A10_AMD64_ARCHIVE_SHA256='b745eaddd8c70cb415aca42901213ffeb3c1d567f9b889e87a4a895ecfda8134'",
		"A10_ARM64_ARCHIVE_SHA256='71a40ee709f34fb29bb562cde4587e2da1db1d6e8bc300d0edb4cfe63f8bec3c'",
		"A10_AMD64_MANIFEST_SHA256='98b38d8d59c5d146234e5a5f9bae26e80f8af0f699ac23e3f9fed5e59b32321e'",
		"A10_ARM64_MANIFEST_SHA256='976aa2bbeedd4e32b522373f6bf75a93b15f6813c4373c638c27d2cb98e4f00a'",
	} {
		if strings.Count(pins, declaration) != 1 {
			t.Fatalf("A11 prior evidence pin drifted: %s", declaration)
		}
	}
	lineage := readReleaseFile(t, "scripts/companion-release/verify-public-key-lineage.sh")
	for _, required := range []string{
		"release_phase='A11' prior_phase='A10'",
		`prior_evidence_source='immutable A10 GitHub release'`,
		`prior_tree="$A10_TREE_SHA"`,
		`prior_tag_object="$A10_TAG_OBJECT_SHA" prior_checksums="$A10_CHECKSUMS_SHA256"`,
		`[[ "$(jq -er '.commit.tree.sha' "$commit_json")" == "$prior_tree" ]]`,
	} {
		if !strings.Contains(lineage, required) {
			t.Fatalf("A11 lineage contract missing %q", required)
		}
	}
}
