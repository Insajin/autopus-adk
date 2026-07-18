package companionmanifest

import (
	"strings"
	"testing"
)

func TestReleaseSourceValidator_A5AcceptsAnnotatedA4DescendantAndExactPins(t *testing.T) {
	dir := cloneCurrentReleaseRepository(t)
	sha := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD"))
	tree := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD^{tree}"))
	runGit(t, dir, "tag", "-am", "A5 release candidate", "v0.50.74")
	output, err := runReleaseSourceValidator(t, dir, "v0.50.74", sha,
		"COMPANION_SOURCE_PIN_REQUIRED=1",
		"COMPANION_APPROVED_SOURCE_COMMIT="+sha,
		"COMPANION_APPROVED_SOURCE_TREE="+tree,
	)
	if err != nil {
		t.Fatalf("annotated pinned A5 rejected: %v\n%s", err, output)
	}
	if !strings.Contains(output, "release-phase=A5") ||
		!strings.Contains(output, "source-commit="+sha) {
		t.Fatalf("validated A5 output = %q", output)
	}
}

func TestReleaseSourceValidator_A5RejectsInvalidIdentity(t *testing.T) {
	t.Run("lightweight", func(t *testing.T) {
		dir := cloneCurrentReleaseRepository(t)
		sha := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD"))
		runGit(t, dir, "tag", "v0.50.74")
		output, err := runReleaseSourceValidator(t, dir, "v0.50.74", sha)
		if err == nil || !strings.Contains(output, "A5 release tag must be annotated") {
			t.Fatalf("lightweight A5 result: %v\n%s", err, output)
		}
	})
	t.Run("missing_A4", func(t *testing.T) {
		dir, sha := newMinimalSourceRepository(t)
		runGit(t, dir, "tag", "-am", "orphan A5", "v0.50.74")
		output, err := runReleaseSourceValidator(t, dir, "v0.50.74", sha)
		if err == nil || !strings.Contains(output, "does not contain the immutable A4 release") {
			t.Fatalf("A4-free A5 result: %v\n%s", err, output)
		}
	})
	t.Run("unapproved_source", func(t *testing.T) {
		dir := cloneCurrentReleaseRepository(t)
		sha := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD"))
		tree := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD^{tree}"))
		runGit(t, dir, "tag", "-am", "A5 release candidate", "v0.50.74")
		output, err := runReleaseSourceValidator(t, dir, "v0.50.74", sha,
			"COMPANION_SOURCE_PIN_REQUIRED=1",
			"COMPANION_APPROVED_SOURCE_COMMIT="+strings.Repeat("a", 40),
			"COMPANION_APPROVED_SOURCE_TREE="+tree,
		)
		if err == nil ||
			!strings.Contains(output, "release commit differs from the approved exact source commit") {
			t.Fatalf("unapproved A5 source result: %v\n%s", err, output)
		}
	})
}
