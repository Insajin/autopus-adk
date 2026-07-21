package companionmanifest

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseSourceValidator_HistoricalTagsPreserveExactCoordinates(t *testing.T) {
	dir, sha := newMinimalSourceRepository(t)
	for _, phase := range []struct{ tag, name string }{
		{tag: "v0.50.69", name: "A0"},
		{tag: "v0.50.70", name: "A1"},
	} {
		runGit(t, dir, "tag", phase.tag)
		output, err := runReleaseSourceValidator(t, dir, phase.tag, sha)
		if err != nil {
			t.Fatalf("source validation %s failed: %v\n%s", phase.name, err, output)
		}
		if !strings.Contains(output, "release-phase="+phase.name) ||
			!strings.Contains(output, "source-commit="+sha) || len(sha) != 40 {
			t.Fatalf("validated %s output = %q", phase.name, output)
		}
	}
}

func TestReleaseSourceValidator_A2AcceptsAnnotatedIntegratedRepository(t *testing.T) {
	dir := cloneCurrentReleaseRepository(t)
	sha := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD"))
	runGit(t, dir, "tag", "-am", "A2 release candidate", "v0.50.71")
	output, err := runReleaseSourceValidator(t, dir, "v0.50.71", sha,
		"A2_A1_ANCESTOR_SHA="+strings.Repeat("0", 40),
		"A2_MAIN_ANCESTOR_SHA="+strings.Repeat("f", 40),
	)
	if err != nil {
		t.Fatalf("annotated integrated A2 rejected: %v\n%s", err, output)
	}
	if !strings.Contains(output, "release-phase=A2") || !strings.Contains(output, "source-commit="+sha) {
		t.Fatalf("validated A2 output = %q", output)
	}
}

func TestReleaseSourceValidator_A2RejectsLightweightTag(t *testing.T) {
	dir := cloneCurrentReleaseRepository(t)
	sha := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD"))
	runGit(t, dir, "tag", "v0.50.71")
	output, err := runReleaseSourceValidator(t, dir, "v0.50.71", sha)
	if err == nil || !strings.Contains(output, "A2 release tag must be annotated") {
		t.Fatalf("lightweight A2 result: %v\n%s", err, output)
	}
}

func TestReleaseSourceValidator_A2RejectsSourceWithoutPinnedAncestors(t *testing.T) {
	dir, sha := newMinimalSourceRepository(t)
	runGit(t, dir, "tag", "-am", "orphan A2", "v0.50.71")
	output, err := runReleaseSourceValidator(t, dir, "v0.50.71", sha)
	if err == nil || !strings.Contains(output, "does not contain the immutable A1 release") {
		t.Fatalf("ancestry-free A2 result: %v\n%s", err, output)
	}
}

func TestReleaseSourceValidator_A3AcceptsAnnotatedA2DescendantAndExactPins(t *testing.T) {
	dir := cloneCurrentReleaseRepository(t)
	sha := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD"))
	tree := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD^{tree}"))
	runGit(t, dir, "tag", "-am", "A3 release candidate", "v0.50.72")
	output, err := runReleaseSourceValidator(t, dir, "v0.50.72", sha,
		"COMPANION_SOURCE_PIN_REQUIRED=1",
		"COMPANION_APPROVED_SOURCE_COMMIT="+sha,
		"COMPANION_APPROVED_SOURCE_TREE="+tree,
	)
	if err != nil {
		t.Fatalf("annotated pinned A3 rejected: %v\n%s", err, output)
	}
	if !strings.Contains(output, "release-phase=A3") || !strings.Contains(output, "source-commit="+sha) {
		t.Fatalf("validated A3 output = %q", output)
	}
}

func TestReleaseSourceValidator_A3RejectsLightweightTagAndMissingA2(t *testing.T) {
	t.Run("lightweight", func(t *testing.T) {
		dir := cloneCurrentReleaseRepository(t)
		sha := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD"))
		runGit(t, dir, "tag", "v0.50.72")
		output, err := runReleaseSourceValidator(t, dir, "v0.50.72", sha)
		if err == nil || !strings.Contains(output, "A3 release tag must be annotated") {
			t.Fatalf("lightweight A3 result: %v\n%s", err, output)
		}
	})
	t.Run("missing_A2", func(t *testing.T) {
		dir, sha := newMinimalSourceRepository(t)
		runGit(t, dir, "tag", "-am", "orphan A3", "v0.50.72")
		output, err := runReleaseSourceValidator(t, dir, "v0.50.72", sha)
		if err == nil || !strings.Contains(output, "does not contain the immutable A2 release") {
			t.Fatalf("A2-free A3 result: %v\n%s", err, output)
		}
	})
}

func TestReleaseSourceValidator_A3RejectsUnapprovedSourcePin(t *testing.T) {
	dir := cloneCurrentReleaseRepository(t)
	sha := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD"))
	tree := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD^{tree}"))
	runGit(t, dir, "tag", "-am", "A3 release candidate", "v0.50.72")
	output, err := runReleaseSourceValidator(t, dir, "v0.50.72", sha,
		"COMPANION_SOURCE_PIN_REQUIRED=1",
		"COMPANION_APPROVED_SOURCE_COMMIT="+strings.Repeat("a", 40),
		"COMPANION_APPROVED_SOURCE_TREE="+tree,
	)
	if err == nil || !strings.Contains(output, "release commit differs from the approved exact source commit") {
		t.Fatalf("unapproved A3 source result: %v\n%s", err, output)
	}
}

func TestReleaseSourceValidator_A4AcceptsAnnotatedA3DescendantAndExactPins(t *testing.T) {
	dir := cloneCurrentReleaseRepository(t)
	sha := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD"))
	tree := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD^{tree}"))
	runGit(t, dir, "tag", "-am", "A4 release candidate", "v0.50.73")
	output, err := runReleaseSourceValidator(t, dir, "v0.50.73", sha,
		"COMPANION_SOURCE_PIN_REQUIRED=1",
		"COMPANION_APPROVED_SOURCE_COMMIT="+sha,
		"COMPANION_APPROVED_SOURCE_TREE="+tree,
	)
	if err != nil {
		t.Fatalf("annotated pinned A4 rejected: %v\n%s", err, output)
	}
	if !strings.Contains(output, "release-phase=A4") || !strings.Contains(output, "source-commit="+sha) {
		t.Fatalf("validated A4 output = %q", output)
	}
}

func TestReleaseSourceValidator_A4RejectsLightweightTagAndMissingA3(t *testing.T) {
	t.Run("lightweight", func(t *testing.T) {
		dir := cloneCurrentReleaseRepository(t)
		sha := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD"))
		runGit(t, dir, "tag", "v0.50.73")
		output, err := runReleaseSourceValidator(t, dir, "v0.50.73", sha)
		if err == nil || !strings.Contains(output, "A4 release tag must be annotated") {
			t.Fatalf("lightweight A4 result: %v\n%s", err, output)
		}
	})
	t.Run("missing_A3", func(t *testing.T) {
		dir, sha := newMinimalSourceRepository(t)
		runGit(t, dir, "tag", "-am", "orphan A4", "v0.50.73")
		output, err := runReleaseSourceValidator(t, dir, "v0.50.73", sha)
		if err == nil || !strings.Contains(output, "does not contain the immutable A3 release") {
			t.Fatalf("A3-free A4 result: %v\n%s", err, output)
		}
	})
}

func TestReleaseSourceValidator_RejectsCoordinateMismatchAndOutsidePolicy(t *testing.T) {
	dir := cloneCurrentReleaseRepository(t)
	taggedSHA := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD"))
	runGit(t, dir, "tag", "-am", "A2 release candidate", "v0.50.71")
	if err := os.WriteFile(filepath.Join(dir, "mismatch"), []byte("mismatch"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "mismatch")
	runGit(t, dir, "commit", "-qm", "mismatched head")
	headSHA := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD"))
	for _, test := range []struct{ name, tag, sha, message string }{
		{name: "head_tag", tag: "v0.50.71", sha: headSHA, message: "checked-out source, tag, and release commit differ"},
		{name: "github_sha", tag: "v0.50.71", sha: taggedSHA, message: "checked-out source, tag, and release commit differ"},
		{name: "failed_A6_tag_75", tag: "v0.50.75", sha: headSHA, message: "outside the frozen A0/A1/A2/A3/A4/A5/A6/A7/A8/A9/A10/A11 policy"},
		{name: "failed_A6_tag_76", tag: "v0.50.76", sha: headSHA, message: "outside the frozen A0/A1/A2/A3/A4/A5/A6/A7/A8/A9/A10/A11 policy"},
		{name: "outside", tag: "v0.50.83", sha: headSHA, message: "outside the frozen A0/A1/A2/A3/A4/A5/A6/A7/A8/A9/A10/A11 policy"},
	} {
		output, err := runReleaseSourceValidator(t, dir, test.tag, test.sha)
		if err == nil || !strings.Contains(output, test.message) {
			t.Fatalf("%s result: %v\n%s", test.name, err, output)
		}
	}
}

func TestLineageVerifier_A0BootstrapsWhileA1ThroughA11WithoutLiveEvidenceFailClosed(t *testing.T) {
	script := filepath.Join(repositoryRoot(t), "scripts/companion-release/verify-public-key-lineage.sh")
	cases := []struct {
		name    string
		tag     string
		wantOK  bool
		message string
	}{
		{name: "A0", tag: "v0.50.69", wantOK: true, message: "bootstrap accepted"},
		{name: "A1", tag: "v0.50.70", message: "missing GITHUB_TOKEN"},
		{name: "A2", tag: "v0.50.71", message: "missing GITHUB_TOKEN"},
		{name: "A3", tag: "v0.50.72", message: "missing GITHUB_TOKEN"},
		{name: "A4", tag: "v0.50.73", message: "missing GITHUB_TOKEN"},
		{name: "A5", tag: "v0.50.74", message: "missing GITHUB_TOKEN"},
		{name: "A6", tag: "v0.50.77", message: "missing GITHUB_TOKEN"},
		{name: "A7", tag: "v0.50.78", message: "missing GITHUB_TOKEN"},
		{name: "A8", tag: "v0.50.79", message: "missing GITHUB_TOKEN"},
		{name: "A9", tag: "v0.50.80", message: "missing GITHUB_TOKEN"},
		{name: "A10", tag: "v0.50.81", message: "missing GITHUB_TOKEN"},
		{name: "A11", tag: "v0.50.82", message: "missing GITHUB_TOKEN"},
		{name: "failed_A6_tag_75", tag: "v0.50.75", message: "outside the frozen A0/A1/A2/A3/A4/A5/A6/A7/A8/A9/A10/A11 policy"},
		{name: "failed_A6_tag_76", tag: "v0.50.76", message: "outside the frozen A0/A1/A2/A3/A4/A5/A6/A7/A8/A9/A10/A11 policy"},
		{name: "outside", tag: "v0.50.83", message: "outside the frozen A0/A1/A2/A3/A4/A5/A6/A7/A8/A9/A10/A11 policy"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			command := exec.Command("bash", script)
			command.Env = []string{"GITHUB_REF_NAME=" + test.tag, "PATH=" + os.Getenv("PATH")}
			output, err := command.CombinedOutput()
			if test.wantOK && err != nil {
				t.Fatalf("A0 bootstrap failed: %v\n%s", err, output)
			}
			if !test.wantOK && err == nil {
				t.Fatalf("%s unexpectedly passed\n%s", test.name, output)
			}
			if !strings.Contains(string(output), test.message) {
				t.Fatalf("%s output = %q, want %q", test.name, output, test.message)
			}
		})
	}
}

func newMinimalSourceRepository(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init", "-q")
	configureReleaseTestGit(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "source"), []byte("source"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "source")
	runGit(t, dir, "commit", "-qm", "source")
	return dir, strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD"))
}

func cloneCurrentReleaseRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "repository")
	runGit(t, root, "clone", "-q", "--no-hardlinks", "--no-tags", repositoryRoot(t), dir)
	configureReleaseTestGit(t, dir)
	return dir
}

func configureReleaseTestGit(t *testing.T, dir string) {
	t.Helper()
	runGit(t, dir, "config", "user.name", "Release Test")
	runGit(t, dir, "config", "user.email", "release-test@example.invalid")
}

func runReleaseSourceValidator(t *testing.T, dir, tag, sha string, extraEnvironment ...string) (string, error) {
	t.Helper()
	outputPath := filepath.Join(t.TempDir(), "github-output")
	command := exec.Command("bash", filepath.Join(repositoryRoot(t), "scripts/companion-release/validate-source.sh"))
	command.Dir = dir
	command.Env = append(os.Environ(),
		"GITHUB_REF_NAME="+tag, "GITHUB_REF_TYPE=tag", "GITHUB_SHA="+sha,
		"GITHUB_OUTPUT="+outputPath,
	)
	command.Env = append(command.Env, extraEnvironment...)
	output, err := command.CombinedOutput()
	written, _ := os.ReadFile(outputPath)
	return string(output) + string(written), err
}

func runGit(t *testing.T, dir string, arguments ...string) string {
	t.Helper()
	command := exec.Command("git", arguments...)
	command.Dir = dir
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", arguments, err, output)
	}
	return string(output)
}
