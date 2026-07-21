package companionmanifest

import (
	"strings"
	"testing"
)

func TestReleaseSourceValidator_A9AcceptsAnnotatedA8DescendantAndExactPins(t *testing.T) {
	dir := cloneCurrentReleaseRepository(t)
	sha := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD"))
	tree := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD^{tree}"))
	runGit(t, dir, "tag", "-am", "A9 release candidate", "v0.50.80")
	output, err := runReleaseSourceValidator(t, dir, "v0.50.80", sha,
		"COMPANION_SOURCE_PIN_REQUIRED=1",
		"COMPANION_APPROVED_SOURCE_COMMIT="+sha,
		"COMPANION_APPROVED_SOURCE_TREE="+tree,
	)
	if err != nil {
		t.Fatalf("annotated pinned A9 rejected: %v\n%s", err, output)
	}
	if !strings.Contains(output, "release-phase=A9") ||
		!strings.Contains(output, "source-commit="+sha) {
		t.Fatalf("validated A9 output = %q", output)
	}
}

func TestReleaseSourceValidator_A9RejectsInvalidIdentity(t *testing.T) {
	t.Run("lightweight", func(t *testing.T) {
		dir := cloneCurrentReleaseRepository(t)
		sha := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD"))
		runGit(t, dir, "tag", "v0.50.80")
		output, err := runReleaseSourceValidator(t, dir, "v0.50.80", sha)
		if err == nil || !strings.Contains(output, "A9 release tag must be annotated") {
			t.Fatalf("lightweight A9 result: %v\n%s", err, output)
		}
	})
	t.Run("missing_A8", func(t *testing.T) {
		dir, sha := newMinimalSourceRepository(t)
		runGit(t, dir, "tag", "-am", "orphan A9", "v0.50.80")
		output, err := runReleaseSourceValidator(t, dir, "v0.50.80", sha)
		if err == nil || !strings.Contains(output, "does not contain the immutable A8 release") {
			t.Fatalf("A8-free A9 result: %v\n%s", err, output)
		}
	})
	t.Run("unapproved_source", func(t *testing.T) {
		dir := cloneCurrentReleaseRepository(t)
		sha := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD"))
		tree := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD^{tree}"))
		runGit(t, dir, "tag", "-am", "A9 release candidate", "v0.50.80")
		output, err := runReleaseSourceValidator(t, dir, "v0.50.80", sha,
			"COMPANION_SOURCE_PIN_REQUIRED=1",
			"COMPANION_APPROVED_SOURCE_COMMIT="+strings.Repeat("a", 40),
			"COMPANION_APPROVED_SOURCE_TREE="+tree,
		)
		if err == nil ||
			!strings.Contains(output, "release commit differs from the approved exact source commit") {
			t.Fatalf("unapproved A9 source result: %v\n%s", err, output)
		}
	})
}

func TestLineageVerifier_A9PinsExactImmutableA8Evidence(t *testing.T) {
	pins := readReleaseFile(t, "scripts/companion-release/verify-public-key-lineage-pins.sh")
	for _, declaration := range []string{
		"A8_COMMIT_SHA='dd0c2759ed5435d4634011e349caad62ea3df414'",
		"A8_TREE_SHA='4325913ba332c583dd573ccf9248b38497d76926'",
		"A8_TAG_OBJECT_SHA='8c6dcef91407e3321704014559cfd929d14768d0'",
		"A8_CHECKSUMS_SHA256='1d0bdbfe50f85c381fde11c334c97a1b783dcfa4e12e0c4023152f38119a0bcd'",
		"A8_AMD64_ARCHIVE_SHA256='19e317cdabc9dde976ca772d9ddbbf693b444dd44eefa70c8d0313a32de89a9b'",
		"A8_ARM64_ARCHIVE_SHA256='41e29ae1c3c48dd6e3e5f4dfe8076472704d00a7d479b5cc8a90f53c0af6ef31'",
		"A8_AMD64_MANIFEST_SHA256='c5ac37874bac5de87152e781bd82a17c7705894f24be81657ccc907f15ba1f65'",
		"A8_ARM64_MANIFEST_SHA256='ebcf563c11f0836be2b2bd4423ea315283eeec12cfa200d479e1a56f5909f5f1'",
	} {
		if strings.Count(pins, declaration) != 1 {
			t.Fatalf("A9 prior evidence pin drifted: %s", declaration)
		}
	}
	lineage := readReleaseFile(t, "scripts/companion-release/verify-public-key-lineage.sh")
	coordinates := readReleaseFile(t,
		"scripts/companion-release/verify-public-key-lineage-coordinates.sh")
	for _, required := range []string{
		"release_phase='A9' prior_phase='A8'",
		`prior_tree="$A8_TREE_SHA"`,
	} {
		if !strings.Contains(coordinates, required) {
			t.Fatalf("A9 lineage coordinate contract missing %q", required)
		}
	}
	for _, required := range []string{
		`[[ "$(jq -er '.commit.tree.sha' "$commit_json")" == "$prior_tree" ]]`,
		`[[ -f "$coordinates_helper" && ! -L "$coordinates_helper" ]]`,
		`source "$coordinates_helper"`,
	} {
		if !strings.Contains(lineage, required) {
			t.Fatalf("A9 lineage caller contract missing %q", required)
		}
	}
}
