package companionmanifest

import (
	"strings"
	"testing"
)

const a10A9AncestorSHA = "c9c4f49d48022eb0c8d72ee7b520136a4f21f176"

func TestReleaseSourceValidator_A10AcceptsAnnotatedA9DescendantAndExactPins(t *testing.T) {
	dir := cloneCurrentReleaseRepository(t)
	sha := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD"))
	tree := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD^{tree}"))
	runGit(t, dir, "tag", "-am", "A10 release candidate", "v0.50.81")
	output, err := runReleaseSourceValidator(t, dir, "v0.50.81", sha,
		"COMPANION_SOURCE_PIN_REQUIRED=1",
		"COMPANION_APPROVED_SOURCE_COMMIT="+sha,
		"COMPANION_APPROVED_SOURCE_TREE="+tree,
	)
	if err != nil {
		t.Fatalf("annotated pinned A10 rejected: %v\n%s", err, output)
	}
	if !strings.Contains(output, "release-phase=A10") ||
		!strings.Contains(output, "source-commit="+sha) {
		t.Fatalf("validated A10 output = %q", output)
	}
}

func TestReleaseSourceValidator_A10RejectsInvalidIdentity(t *testing.T) {
	t.Run("lightweight", func(t *testing.T) {
		dir := cloneCurrentReleaseRepository(t)
		sha := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD"))
		runGit(t, dir, "tag", "v0.50.81")
		output, err := runReleaseSourceValidator(t, dir, "v0.50.81", sha)
		if err == nil || !strings.Contains(output, "A10 release tag must be annotated") {
			t.Fatalf("lightweight A10 result: %v\n%s", err, output)
		}
	})
	t.Run("missing_A9", func(t *testing.T) {
		dir, sha := newMinimalSourceRepository(t)
		runGit(t, dir, "tag", "-am", "orphan A10", "v0.50.81")
		output, err := runReleaseSourceValidator(t, dir, "v0.50.81", sha)
		if err == nil || !strings.Contains(output, "does not contain the immutable A9 release") {
			t.Fatalf("A9-free A10 result: %v\n%s", err, output)
		}
	})
	t.Run("unapproved_source", func(t *testing.T) {
		dir := cloneCurrentReleaseRepository(t)
		sha := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD"))
		tree := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD^{tree}"))
		runGit(t, dir, "tag", "-am", "A10 release candidate", "v0.50.81")
		output, err := runReleaseSourceValidator(t, dir, "v0.50.81", sha,
			"COMPANION_SOURCE_PIN_REQUIRED=1",
			"COMPANION_APPROVED_SOURCE_COMMIT="+strings.Repeat("a", 40),
			"COMPANION_APPROVED_SOURCE_TREE="+tree,
		)
		if err == nil ||
			!strings.Contains(output, "release commit differs from the approved exact source commit") {
			t.Fatalf("unapproved A10 source result: %v\n%s", err, output)
		}
	})
}

func TestReleaseSourceValidator_A10PinsDirectA9Ancestor(t *testing.T) {
	source := readReleaseFile(t, "scripts/companion-release/validate-source.sh")
	declaration := "readonly A10_A9_ANCESTOR_SHA='" + a10A9AncestorSHA + "'"
	if strings.Count(source, declaration) != 1 {
		t.Fatalf("A10 immutable A9 ancestry pin drifted: %s", declaration)
	}
	for _, required := range []string{
		`git cat-file -t "refs/tags/$GITHUB_REF_NAME"`,
		`git merge-base --is-ancestor "$A10_A9_ANCESTOR_SHA" "$GITHUB_SHA"`,
		`[[ "$tag_object_type" == 'tag' ]]`,
		"COMPANION_APPROVED_SOURCE_COMMIT", "COMPANION_APPROVED_SOURCE_TREE",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("A10 source gate missing %q", required)
		}
	}
}

func TestLineageVerifier_A10PinsExactImmutableA9Evidence(t *testing.T) {
	pins := readReleaseFile(t, "scripts/companion-release/verify-public-key-lineage-pins.sh")
	for _, declaration := range []string{
		"A9_COMMIT_SHA='c9c4f49d48022eb0c8d72ee7b520136a4f21f176'",
		"A9_TREE_SHA='3a71fa56bd917f447a6b1705772b6ab99bbcfbc8'",
		"A9_TAG_OBJECT_SHA='b7d05fa76eed41b1dfb4eddbd9873525e0aac15f'",
		"A9_CHECKSUMS_SHA256='9ed1f99d22a761abb7953c70aab3c7de5ab0b7ec3524cf3798fcd3815c53bde7'",
		"A9_AMD64_ARCHIVE_SHA256='48f80577ff2ef40a843dab0a847895ca7b3877e7fb810a30d328cbe8a55fc51e'",
		"A9_ARM64_ARCHIVE_SHA256='503c338e1ce122e209b9e74bc883492317144b319b0713943bc299e57447024d'",
		"A9_AMD64_MANIFEST_SHA256='589f02503aa02338ed14d67b1eb6b31e2b96a9e83b47c99e5cd5a31b75ede9b7'",
		"A9_ARM64_MANIFEST_SHA256='ffdd6ccbecff2b8ea38bc5c5f65ff7f078b229bd4658f90d08bb5e801c184a7f'",
	} {
		if strings.Count(pins, declaration) != 1 {
			t.Fatalf("A10 prior evidence pin drifted: %s", declaration)
		}
	}
	lineage := readReleaseFile(t, "scripts/companion-release/verify-public-key-lineage.sh")
	coordinates := readReleaseFile(t,
		"scripts/companion-release/verify-public-key-lineage-coordinates.sh")
	for _, required := range []string{
		"release_phase='A10' prior_phase='A9'",
		`prior_evidence_source='immutable A9 GitHub release'`,
		`prior_tree="$A9_TREE_SHA"`,
		`prior_tag_object="$A9_TAG_OBJECT_SHA" prior_checksums="$A9_CHECKSUMS_SHA256"`,
	} {
		if !strings.Contains(coordinates, required) {
			t.Fatalf("A10 lineage coordinate contract missing %q", required)
		}
	}
	for _, required := range []string{
		`[[ "$(jq -er '.commit.tree.sha' "$commit_json")" == "$prior_tree" ]]`,
		`[[ -f "$coordinates_helper" && ! -L "$coordinates_helper" ]]`,
		`source "$coordinates_helper"`,
	} {
		if !strings.Contains(lineage, required) {
			t.Fatalf("A10 lineage caller contract missing %q", required)
		}
	}
}
