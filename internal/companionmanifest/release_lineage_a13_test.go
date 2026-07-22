package companionmanifest

import (
	"strings"
	"testing"
)

const a13A12AncestorSHA = "e6367b5375cd4cdf09cb1515877bc57323521364"

func TestReleaseSourceValidator_A13AcceptsAnnotatedA12DescendantAndExactPins(t *testing.T) {
	dir := cloneCurrentReleaseRepository(t)
	sha := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD"))
	tree := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD^{tree}"))
	runGit(t, dir, "tag", "-am", "A13 release candidate", "v0.50.84")
	output, err := runReleaseSourceValidator(t, dir, "v0.50.84", sha,
		"COMPANION_SOURCE_PIN_REQUIRED=1",
		"COMPANION_APPROVED_SOURCE_COMMIT="+sha,
		"COMPANION_APPROVED_SOURCE_TREE="+tree,
	)
	if err != nil {
		t.Fatalf("annotated pinned A13 rejected: %v\n%s", err, output)
	}
	if !strings.Contains(output, "release-phase=A13") ||
		!strings.Contains(output, "source-commit="+sha) {
		t.Fatalf("validated A13 output = %q", output)
	}
}

func TestReleaseSourceValidator_A13RejectsInvalidIdentity(t *testing.T) {
	t.Run("lightweight", func(t *testing.T) {
		dir := cloneCurrentReleaseRepository(t)
		sha := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD"))
		runGit(t, dir, "tag", "v0.50.84")
		output, err := runReleaseSourceValidator(t, dir, "v0.50.84", sha)
		if err == nil || !strings.Contains(output, "A13 release tag must be annotated") {
			t.Fatalf("lightweight A13 result: %v\n%s", err, output)
		}
	})
	t.Run("missing_A12", func(t *testing.T) {
		dir, sha := newMinimalSourceRepository(t)
		runGit(t, dir, "tag", "-am", "orphan A13", "v0.50.84")
		output, err := runReleaseSourceValidator(t, dir, "v0.50.84", sha)
		if err == nil || !strings.Contains(output, "does not contain the immutable A12 release") {
			t.Fatalf("A12-free A13 result: %v\n%s", err, output)
		}
	})
	t.Run("unapproved_source", func(t *testing.T) {
		dir := cloneCurrentReleaseRepository(t)
		sha := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD"))
		tree := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD^{tree}"))
		runGit(t, dir, "tag", "-am", "A13 release candidate", "v0.50.84")
		output, err := runReleaseSourceValidator(t, dir, "v0.50.84", sha,
			"COMPANION_SOURCE_PIN_REQUIRED=1",
			"COMPANION_APPROVED_SOURCE_COMMIT="+strings.Repeat("a", 40),
			"COMPANION_APPROVED_SOURCE_TREE="+tree,
		)
		if err == nil ||
			!strings.Contains(output, "release commit differs from the approved exact source commit") {
			t.Fatalf("unapproved A13 source result: %v\n%s", err, output)
		}
	})
}

func TestReleaseSourceValidator_A13PinsDirectA12Ancestor(t *testing.T) {
	source := readReleaseFile(t, "scripts/companion-release/validate-source.sh")
	declaration := "readonly A13_A12_ANCESTOR_SHA='" + a13A12AncestorSHA + "'"
	if strings.Count(source, declaration) != 1 {
		t.Fatalf("A13 immutable A12 ancestry pin drifted: %s", declaration)
	}
	for _, required := range []string{
		`git cat-file -t "refs/tags/$GITHUB_REF_NAME"`,
		`git merge-base --is-ancestor "$A13_A12_ANCESTOR_SHA" "$GITHUB_SHA"`,
		`[[ "$tag_object_type" == 'tag' ]]`,
		"COMPANION_APPROVED_SOURCE_COMMIT", "COMPANION_APPROVED_SOURCE_TREE",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("A13 source gate missing %q", required)
		}
	}
}

func TestLineageVerifier_A13PinsExactImmutableA12Evidence(t *testing.T) {
	pins := readReleaseFile(t, "scripts/companion-release/verify-public-key-lineage-pins.sh")
	for _, declaration := range []string{
		"A12_COMMIT_SHA='e6367b5375cd4cdf09cb1515877bc57323521364'",
		"A12_TREE_SHA='6c9a22e85d5a8c5f23c0d9e1bb41de270cab85a4'",
		"A12_TAG_OBJECT_SHA='080507fceb3b4bf31f0e0887e49013fd65645ac2'",
		"A12_CHECKSUMS_SHA256='7d871b077766f3a7dd6859427fa9b1333422312764820243d3bf7af5e935dee0'",
		"A12_AMD64_ARCHIVE_SHA256='da92acfa4e8f45a0abea90b0991ae87cc7fb345c4f1ca2c166a8626670df658b'",
		"A12_ARM64_ARCHIVE_SHA256='5b29fdb21b62f8933c1ff0608f9c1dca096be24649fd24ec40bcbe9ff72c4fcc'",
		"A12_AMD64_MANIFEST_SHA256='caa1145bc293a125495795914005429694e2a2b98a863d903a40575495ec250a'",
		"A12_ARM64_MANIFEST_SHA256='013e7b98bfea64783d932e787609d526d5157801788b90b13cc59990070ab20b'",
	} {
		if strings.Count(pins, declaration) != 1 {
			t.Fatalf("A13 prior evidence pin drifted: %s", declaration)
		}
	}
	lineage := readReleaseFile(t, "scripts/companion-release/verify-public-key-lineage.sh")
	coordinates := readReleaseFile(t,
		"scripts/companion-release/verify-public-key-lineage-coordinates.sh")
	for _, required := range []string{
		"A12_REPOSITORY='Insajin/autopus-adk' A13_TAG='v0.50.84' A13_VERSION='0.50.84'",
		"release_phase='A13' prior_phase='A12'",
		`prior_evidence_source='immutable A12 GitHub release'`,
		`prior_tree="$A12_TREE_SHA"`,
		`prior_tag_object="$A12_TAG_OBJECT_SHA" prior_checksums="$A12_CHECKSUMS_SHA256"`,
	} {
		if !strings.Contains(coordinates, required) {
			t.Fatalf("A13 lineage coordinate contract missing %q", required)
		}
	}
	for _, required := range []string{
		`[[ "$(jq -er '.commit.tree.sha' "$commit_json")" == "$prior_tree" ]]`,
		`[[ -f "$coordinates_helper" && ! -L "$coordinates_helper" ]]`,
		`source "$coordinates_helper"`,
	} {
		if !strings.Contains(lineage, required) {
			t.Fatalf("A13 lineage caller contract missing %q", required)
		}
	}
}
