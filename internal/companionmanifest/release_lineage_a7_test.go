package companionmanifest

import (
	"strings"
	"testing"
)

func TestReleaseSourceValidator_A7AcceptsAnnotatedA6DescendantAndExactPins(t *testing.T) {
	dir := cloneCurrentReleaseRepository(t)
	sha := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD"))
	tree := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD^{tree}"))
	runGit(t, dir, "tag", "-am", "A7 release candidate", "v0.50.78")
	output, err := runReleaseSourceValidator(t, dir, "v0.50.78", sha,
		"COMPANION_SOURCE_PIN_REQUIRED=1",
		"COMPANION_APPROVED_SOURCE_COMMIT="+sha,
		"COMPANION_APPROVED_SOURCE_TREE="+tree,
	)
	if err != nil {
		t.Fatalf("annotated pinned A7 rejected: %v\n%s", err, output)
	}
	if !strings.Contains(output, "release-phase=A7") ||
		!strings.Contains(output, "source-commit="+sha) {
		t.Fatalf("validated A7 output = %q", output)
	}
}

func TestReleaseSourceValidator_A7RejectsInvalidIdentity(t *testing.T) {
	t.Run("lightweight", func(t *testing.T) {
		dir := cloneCurrentReleaseRepository(t)
		sha := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD"))
		runGit(t, dir, "tag", "v0.50.78")
		output, err := runReleaseSourceValidator(t, dir, "v0.50.78", sha)
		if err == nil || !strings.Contains(output, "A7 release tag must be annotated") {
			t.Fatalf("lightweight A7 result: %v\n%s", err, output)
		}
	})
	t.Run("missing_A6", func(t *testing.T) {
		dir, sha := newMinimalSourceRepository(t)
		runGit(t, dir, "tag", "-am", "orphan A7", "v0.50.78")
		output, err := runReleaseSourceValidator(t, dir, "v0.50.78", sha)
		if err == nil || !strings.Contains(output, "does not contain the immutable A6 release") {
			t.Fatalf("A6-free A7 result: %v\n%s", err, output)
		}
	})
	t.Run("unapproved_source", func(t *testing.T) {
		dir := cloneCurrentReleaseRepository(t)
		sha := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD"))
		tree := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD^{tree}"))
		runGit(t, dir, "tag", "-am", "A7 release candidate", "v0.50.78")
		output, err := runReleaseSourceValidator(t, dir, "v0.50.78", sha,
			"COMPANION_SOURCE_PIN_REQUIRED=1",
			"COMPANION_APPROVED_SOURCE_COMMIT="+strings.Repeat("a", 40),
			"COMPANION_APPROVED_SOURCE_TREE="+tree,
		)
		if err == nil ||
			!strings.Contains(output, "release commit differs from the approved exact source commit") {
			t.Fatalf("unapproved A7 source result: %v\n%s", err, output)
		}
	})
}
