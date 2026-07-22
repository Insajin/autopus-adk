package companionmanifest

import (
	"strings"
	"testing"
)

const a14A13AncestorSHA = "2b7aa046bdb7861113dfa57b30489c11715582e9"

func TestReleaseSourceValidator_A14AcceptsAnnotatedA13DescendantAndExactPins(t *testing.T) {
	dir := cloneCurrentReleaseRepository(t)
	sha := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD"))
	tree := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD^{tree}"))
	runGit(t, dir, "tag", "-am", "A14 release candidate", "v0.50.85")
	output, err := runReleaseSourceValidator(t, dir, "v0.50.85", sha,
		"COMPANION_SOURCE_PIN_REQUIRED=1",
		"COMPANION_APPROVED_SOURCE_COMMIT="+sha,
		"COMPANION_APPROVED_SOURCE_TREE="+tree,
	)
	if err != nil {
		t.Fatalf("annotated pinned A14 rejected: %v\n%s", err, output)
	}
	if !strings.Contains(output, "release-phase=A14") ||
		!strings.Contains(output, "source-commit="+sha) {
		t.Fatalf("validated A14 output = %q", output)
	}
}

func TestReleaseSourceValidator_A14RejectsInvalidIdentity(t *testing.T) {
	t.Run("lightweight", func(t *testing.T) {
		dir := cloneCurrentReleaseRepository(t)
		sha := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD"))
		runGit(t, dir, "tag", "v0.50.85")
		output, err := runReleaseSourceValidator(t, dir, "v0.50.85", sha)
		if err == nil || !strings.Contains(output, "A14 release tag must be annotated") {
			t.Fatalf("lightweight A14 result: %v\n%s", err, output)
		}
	})
	t.Run("missing_A13", func(t *testing.T) {
		dir, sha := newMinimalSourceRepository(t)
		runGit(t, dir, "tag", "-am", "orphan A14", "v0.50.85")
		output, err := runReleaseSourceValidator(t, dir, "v0.50.85", sha)
		if err == nil || !strings.Contains(output, "does not contain the immutable A13 release") {
			t.Fatalf("A13-free A14 result: %v\n%s", err, output)
		}
	})
	t.Run("unapproved_source", func(t *testing.T) {
		dir := cloneCurrentReleaseRepository(t)
		sha := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD"))
		tree := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD^{tree}"))
		runGit(t, dir, "tag", "-am", "A14 release candidate", "v0.50.85")
		output, err := runReleaseSourceValidator(t, dir, "v0.50.85", sha,
			"COMPANION_SOURCE_PIN_REQUIRED=1",
			"COMPANION_APPROVED_SOURCE_COMMIT="+strings.Repeat("a", 40),
			"COMPANION_APPROVED_SOURCE_TREE="+tree,
		)
		if err == nil ||
			!strings.Contains(output, "release commit differs from the approved exact source commit") {
			t.Fatalf("unapproved A14 source result: %v\n%s", err, output)
		}
	})
}

func TestReleaseSourceValidator_A14PinsDirectA13Ancestor(t *testing.T) {
	source := readReleaseFile(t, "scripts/companion-release/validate-source.sh")
	declaration := "readonly A14_A13_ANCESTOR_SHA='" + a14A13AncestorSHA + "'"
	if strings.Count(source, declaration) != 1 {
		t.Fatalf("A14 immutable A13 ancestry pin drifted: %s", declaration)
	}
	for _, required := range []string{
		`git cat-file -t "refs/tags/$GITHUB_REF_NAME"`,
		`git merge-base --is-ancestor "$A14_A13_ANCESTOR_SHA" "$GITHUB_SHA"`,
		`[[ "$tag_object_type" == 'tag' ]]`,
		"COMPANION_APPROVED_SOURCE_COMMIT", "COMPANION_APPROVED_SOURCE_TREE",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("A14 source gate missing %q", required)
		}
	}
}

func TestLineageVerifier_A14PinsExactImmutableA13Evidence(t *testing.T) {
	pins := readReleaseFile(t, "scripts/companion-release/verify-public-key-lineage-pins.sh")
	for _, declaration := range []string{
		"A13_COMMIT_SHA='2b7aa046bdb7861113dfa57b30489c11715582e9'",
		"A13_TREE_SHA='95d1b00bcc1cb1bfcca3dd58e1e5e1b94575c367'",
		"A13_TAG_OBJECT_SHA='de34e9c1a2a06b27f57235c81a59d1da180eab6d'",
		"A13_CHECKSUMS_SHA256='8f00d3b42d71c9e71346bf62cd72f8e1428600cb0795f703d90de64b3b9ba14e'",
		"A13_AMD64_ARCHIVE_SHA256='fa60e03ecd39a5fa203be3cca3e8a7010e3af7854195f0e866ef80e7a0e82f0f'",
		"A13_ARM64_ARCHIVE_SHA256='f4ed0ef8d6f0274389ada5cebdeb87a2899bf34b7a11bd99318b5914775d84f1'",
		"A13_AMD64_MANIFEST_SHA256='ba6f3e92d4a1c0a1a52b7b17e484961cb8640944eae24856652ebe6192210931'",
		"A13_ARM64_MANIFEST_SHA256='22660fc029bbcb9ffe312964d9f674ba2587440dba48790e28fb4f35b19dcc69'",
	} {
		if strings.Count(pins, declaration) != 1 {
			t.Fatalf("A14 prior evidence pin drifted: %s", declaration)
		}
	}
	lineage := readReleaseFile(t, "scripts/companion-release/verify-public-key-lineage.sh")
	coordinates := readReleaseFile(t,
		"scripts/companion-release/verify-public-key-lineage-coordinates.sh")
	for _, required := range []string{
		"A13_REPOSITORY='Insajin/autopus-adk' A14_TAG='v0.50.85' A14_VERSION='0.50.85'",
		"release_phase='A14' prior_phase='A13'",
		`prior_evidence_source='immutable A13 GitHub release'`,
		`prior_tree="$A13_TREE_SHA"`,
		`prior_tag_object="$A13_TAG_OBJECT_SHA" prior_checksums="$A13_CHECKSUMS_SHA256"`,
	} {
		if !strings.Contains(coordinates, required) {
			t.Fatalf("A14 lineage coordinate contract missing %q", required)
		}
	}
	for _, required := range []string{
		`[[ "$(jq -er '.commit.tree.sha' "$commit_json")" == "$prior_tree" ]]`,
		`[[ -f "$coordinates_helper" && ! -L "$coordinates_helper" ]]`,
		`source "$coordinates_helper"`,
	} {
		if !strings.Contains(lineage, required) {
			t.Fatalf("A14 lineage caller contract missing %q", required)
		}
	}
}
