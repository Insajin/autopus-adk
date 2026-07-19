package companionmanifest

import (
	"strings"
	"testing"
)

func TestReleaseSourceValidator_A6AcceptsAnnotatedA5DescendantAndExactPins(t *testing.T) {
	dir := cloneCurrentReleaseRepository(t)
	sha := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD"))
	tree := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD^{tree}"))
	runGit(t, dir, "tag", "-am", "A6 release candidate", "v0.50.75")
	output, err := runReleaseSourceValidator(t, dir, "v0.50.75", sha,
		"COMPANION_SOURCE_PIN_REQUIRED=1",
		"COMPANION_APPROVED_SOURCE_COMMIT="+sha,
		"COMPANION_APPROVED_SOURCE_TREE="+tree,
	)
	if err != nil {
		t.Fatalf("annotated pinned A6 rejected: %v\n%s", err, output)
	}
	if !strings.Contains(output, "release-phase=A6") ||
		!strings.Contains(output, "source-commit="+sha) {
		t.Fatalf("validated A6 output = %q", output)
	}
}

func TestReleaseSourceValidator_A6RejectsInvalidIdentity(t *testing.T) {
	t.Run("lightweight", func(t *testing.T) {
		dir := cloneCurrentReleaseRepository(t)
		sha := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD"))
		runGit(t, dir, "tag", "v0.50.75")
		output, err := runReleaseSourceValidator(t, dir, "v0.50.75", sha)
		if err == nil || !strings.Contains(output, "A6 release tag must be annotated") {
			t.Fatalf("lightweight A6 result: %v\n%s", err, output)
		}
	})
	t.Run("missing_A5", func(t *testing.T) {
		dir, sha := newMinimalSourceRepository(t)
		runGit(t, dir, "tag", "-am", "orphan A6", "v0.50.75")
		output, err := runReleaseSourceValidator(t, dir, "v0.50.75", sha)
		if err == nil || !strings.Contains(output, "does not contain the immutable A5 release") {
			t.Fatalf("A5-free A6 result: %v\n%s", err, output)
		}
	})
	t.Run("unapproved_source", func(t *testing.T) {
		dir := cloneCurrentReleaseRepository(t)
		sha := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD"))
		tree := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD^{tree}"))
		runGit(t, dir, "tag", "-am", "A6 release candidate", "v0.50.75")
		output, err := runReleaseSourceValidator(t, dir, "v0.50.75", sha,
			"COMPANION_SOURCE_PIN_REQUIRED=1",
			"COMPANION_APPROVED_SOURCE_COMMIT="+strings.Repeat("a", 40),
			"COMPANION_APPROVED_SOURCE_TREE="+tree,
		)
		if err == nil ||
			!strings.Contains(output, "release commit differs from the approved exact source commit") {
			t.Fatalf("unapproved A6 source result: %v\n%s", err, output)
		}
	})
}
