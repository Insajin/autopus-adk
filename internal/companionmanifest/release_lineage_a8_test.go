package companionmanifest

import (
	"strings"
	"testing"
)

func TestReleaseSourceValidator_A8AcceptsAnnotatedA7DescendantAndExactPins(t *testing.T) {
	dir := cloneCurrentReleaseRepository(t)
	sha := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD"))
	tree := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD^{tree}"))
	runGit(t, dir, "tag", "-am", "A8 release candidate", "v0.50.79")
	output, err := runReleaseSourceValidator(t, dir, "v0.50.79", sha,
		"COMPANION_SOURCE_PIN_REQUIRED=1",
		"COMPANION_APPROVED_SOURCE_COMMIT="+sha,
		"COMPANION_APPROVED_SOURCE_TREE="+tree,
	)
	if err != nil {
		t.Fatalf("annotated pinned A8 rejected: %v\n%s", err, output)
	}
	if !strings.Contains(output, "release-phase=A8") ||
		!strings.Contains(output, "source-commit="+sha) {
		t.Fatalf("validated A8 output = %q", output)
	}
}

func TestReleaseSourceValidator_A8RejectsInvalidIdentity(t *testing.T) {
	t.Run("lightweight", func(t *testing.T) {
		dir := cloneCurrentReleaseRepository(t)
		sha := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD"))
		runGit(t, dir, "tag", "v0.50.79")
		output, err := runReleaseSourceValidator(t, dir, "v0.50.79", sha)
		if err == nil || !strings.Contains(output, "A8 release tag must be annotated") {
			t.Fatalf("lightweight A8 result: %v\n%s", err, output)
		}
	})
	t.Run("missing_A7", func(t *testing.T) {
		dir, sha := newMinimalSourceRepository(t)
		runGit(t, dir, "tag", "-am", "orphan A8", "v0.50.79")
		output, err := runReleaseSourceValidator(t, dir, "v0.50.79", sha)
		if err == nil || !strings.Contains(output, "does not contain the immutable A7 release") {
			t.Fatalf("A7-free A8 result: %v\n%s", err, output)
		}
	})
	t.Run("unapproved_source", func(t *testing.T) {
		dir := cloneCurrentReleaseRepository(t)
		sha := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD"))
		tree := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD^{tree}"))
		runGit(t, dir, "tag", "-am", "A8 release candidate", "v0.50.79")
		output, err := runReleaseSourceValidator(t, dir, "v0.50.79", sha,
			"COMPANION_SOURCE_PIN_REQUIRED=1",
			"COMPANION_APPROVED_SOURCE_COMMIT="+strings.Repeat("a", 40),
			"COMPANION_APPROVED_SOURCE_TREE="+tree,
		)
		if err == nil ||
			!strings.Contains(output, "release commit differs from the approved exact source commit") {
			t.Fatalf("unapproved A8 source result: %v\n%s", err, output)
		}
	})
}

func TestLineageVerifier_A8PinsExactImmutableA7Evidence(t *testing.T) {
	pins := readReleaseFile(t, "scripts/companion-release/verify-public-key-lineage-pins.sh")
	for _, declaration := range []string{
		"A7_COMMIT_SHA='51de6030a69a8e36fcf7e5790ef157eff6fedf00'",
		"A7_TREE_SHA='3cd00b17bd8bd6aa8def213de1c5765c3611765d'",
		"A7_TAG_OBJECT_SHA='417a318fb6a11a720e2c4102e92e39ea9ed676e9'",
		"A7_CHECKSUMS_SHA256='322d2ef21dff55f02ca36944aba88ee5da92fdae6bcd16a89319f1697efb9733'",
		"A7_AMD64_ARCHIVE_SHA256='43018046ab37027b7fba3888d288961cb5abc136e478deaa9f878586bcce6629'",
		"A7_ARM64_ARCHIVE_SHA256='e72653fd3094537caa60398e2017d409796d7ceef88a7662ca93b6299e9d00ec'",
		"A7_AMD64_MANIFEST_SHA256='3f7c879c93dea0d119805987bef434b65c1a53684e80f78b5d9a0c9c2cd011d5'",
		"A7_ARM64_MANIFEST_SHA256='87ef2a30d6ee8c9abe9e679d597d0a4fbe9bb5cdee1266572476ad6a66aef975'",
	} {
		if strings.Count(pins, declaration) != 1 {
			t.Fatalf("A8 prior evidence pin drifted: %s", declaration)
		}
	}
	lineage := readReleaseFile(t, "scripts/companion-release/verify-public-key-lineage.sh")
	coordinates := readReleaseFile(t,
		"scripts/companion-release/verify-public-key-lineage-coordinates.sh")
	for _, required := range []string{
		"release_phase='A8' prior_phase='A7'",
		`prior_tree="$A7_TREE_SHA"`,
	} {
		if !strings.Contains(coordinates, required) {
			t.Fatalf("A8 lineage coordinate contract missing %q", required)
		}
	}
	for _, required := range []string{
		`[[ "$(jq -er '.commit.tree.sha' "$commit_json")" == "$prior_tree" ]]`,
		`[[ -f "$coordinates_helper" && ! -L "$coordinates_helper" ]]`,
		`source "$coordinates_helper"`,
	} {
		if !strings.Contains(lineage, required) {
			t.Fatalf("A8 lineage caller contract missing %q", required)
		}
	}
}
