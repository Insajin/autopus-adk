package companionmanifest

import (
	"strings"
	"testing"
)

const a12A11AncestorSHA = "a8558ccc36e04125de6b8d84c7ffc9e8ddb5a2c9"

func TestReleaseSourceValidator_A12AcceptsAnnotatedA11DescendantAndExactPins(t *testing.T) {
	dir := cloneCurrentReleaseRepository(t)
	sha := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD"))
	tree := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD^{tree}"))
	runGit(t, dir, "tag", "-am", "A12 release candidate", "v0.50.83")
	output, err := runReleaseSourceValidator(t, dir, "v0.50.83", sha,
		"COMPANION_SOURCE_PIN_REQUIRED=1",
		"COMPANION_APPROVED_SOURCE_COMMIT="+sha,
		"COMPANION_APPROVED_SOURCE_TREE="+tree,
	)
	if err != nil {
		t.Fatalf("annotated pinned A12 rejected: %v\n%s", err, output)
	}
	if !strings.Contains(output, "release-phase=A12") ||
		!strings.Contains(output, "source-commit="+sha) {
		t.Fatalf("validated A12 output = %q", output)
	}
}

func TestReleaseSourceValidator_A12RejectsInvalidIdentity(t *testing.T) {
	t.Run("lightweight", func(t *testing.T) {
		dir := cloneCurrentReleaseRepository(t)
		sha := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD"))
		runGit(t, dir, "tag", "v0.50.83")
		output, err := runReleaseSourceValidator(t, dir, "v0.50.83", sha)
		if err == nil || !strings.Contains(output, "A12 release tag must be annotated") {
			t.Fatalf("lightweight A12 result: %v\n%s", err, output)
		}
	})
	t.Run("missing_A11", func(t *testing.T) {
		dir, sha := newMinimalSourceRepository(t)
		runGit(t, dir, "tag", "-am", "orphan A12", "v0.50.83")
		output, err := runReleaseSourceValidator(t, dir, "v0.50.83", sha)
		if err == nil || !strings.Contains(output, "does not contain the immutable A11 release") {
			t.Fatalf("A11-free A12 result: %v\n%s", err, output)
		}
	})
	t.Run("unapproved_source", func(t *testing.T) {
		dir := cloneCurrentReleaseRepository(t)
		sha := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD"))
		tree := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD^{tree}"))
		runGit(t, dir, "tag", "-am", "A12 release candidate", "v0.50.83")
		output, err := runReleaseSourceValidator(t, dir, "v0.50.83", sha,
			"COMPANION_SOURCE_PIN_REQUIRED=1",
			"COMPANION_APPROVED_SOURCE_COMMIT="+strings.Repeat("a", 40),
			"COMPANION_APPROVED_SOURCE_TREE="+tree,
		)
		if err == nil ||
			!strings.Contains(output, "release commit differs from the approved exact source commit") {
			t.Fatalf("unapproved A12 source result: %v\n%s", err, output)
		}
	})
}

func TestReleaseSourceValidator_A12PinsDirectA11Ancestor(t *testing.T) {
	source := readReleaseFile(t, "scripts/companion-release/validate-source.sh")
	declaration := "readonly A12_A11_ANCESTOR_SHA='" + a12A11AncestorSHA + "'"
	if strings.Count(source, declaration) != 1 {
		t.Fatalf("A12 immutable A11 ancestry pin drifted: %s", declaration)
	}
	for _, required := range []string{
		`git cat-file -t "refs/tags/$GITHUB_REF_NAME"`,
		`git merge-base --is-ancestor "$A12_A11_ANCESTOR_SHA" "$GITHUB_SHA"`,
		`[[ "$tag_object_type" == 'tag' ]]`,
		"COMPANION_APPROVED_SOURCE_COMMIT", "COMPANION_APPROVED_SOURCE_TREE",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("A12 source gate missing %q", required)
		}
	}
}

func TestLineageVerifier_A12PinsExactImmutableA11Evidence(t *testing.T) {
	pins := readReleaseFile(t, "scripts/companion-release/verify-public-key-lineage-pins.sh")
	for _, declaration := range []string{
		"A11_COMMIT_SHA='a8558ccc36e04125de6b8d84c7ffc9e8ddb5a2c9'",
		"A11_TREE_SHA='9545ed7437e6dfd7573952586a31964061e30e2d'",
		"A11_TAG_OBJECT_SHA='c636f42a6e8dc65ef6500eb95dac4ef7d1faff9a'",
		"A11_CHECKSUMS_SHA256='a7973f9fa27d1e0ca1d1943adcfe5be0fa6807ba0517ff9066b2659fa6f4f01c'",
		"A11_AMD64_ARCHIVE_SHA256='f5825b4aff8ce84e6b18dfb0ae0249a432a1b247477c3a9e2cd14689a405d40d'",
		"A11_ARM64_ARCHIVE_SHA256='c913c51b396e01034e889f43ef4da68fcae851e7f7cba7f2b8ac60a2c4e00c66'",
		"A11_AMD64_MANIFEST_SHA256='5a036574b0cfe8fa62dfe3dde3d65d248ed225aa883c898caced3d55906b47ba'",
		"A11_ARM64_MANIFEST_SHA256='990b9f1cfb0768db4bb23719006320d845b72322fa9fddc2317ab75381b734ee'",
	} {
		if strings.Count(pins, declaration) != 1 {
			t.Fatalf("A12 prior evidence pin drifted: %s", declaration)
		}
	}
	lineage := readReleaseFile(t, "scripts/companion-release/verify-public-key-lineage.sh")
	coordinates := readReleaseFile(t,
		"scripts/companion-release/verify-public-key-lineage-coordinates.sh")
	for _, required := range []string{
		"A11_REPOSITORY='Insajin/autopus-adk' A12_TAG='v0.50.83' A12_VERSION='0.50.83'",
		"release_phase='A12' prior_phase='A11'",
		`prior_evidence_source='immutable A11 GitHub release'`,
		`prior_tree="$A11_TREE_SHA"`,
		`prior_tag_object="$A11_TAG_OBJECT_SHA" prior_checksums="$A11_CHECKSUMS_SHA256"`,
	} {
		if !strings.Contains(coordinates, required) {
			t.Fatalf("A12 lineage coordinate contract missing %q", required)
		}
	}
	for _, required := range []string{
		`[[ "$(jq -er '.commit.tree.sha' "$commit_json")" == "$prior_tree" ]]`,
		`[[ -f "$coordinates_helper" && ! -L "$coordinates_helper" ]]`,
		`source "$coordinates_helper"`,
	} {
		if !strings.Contains(lineage, required) {
			t.Fatalf("A12 lineage caller contract missing %q", required)
		}
	}
}
