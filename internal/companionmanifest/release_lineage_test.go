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
		{name: "outside", tag: "v0.50.72", sha: headSHA, message: "outside the frozen A0/A1/A2 policy"},
	} {
		output, err := runReleaseSourceValidator(t, dir, test.tag, test.sha)
		if err == nil || !strings.Contains(output, test.message) {
			t.Fatalf("%s result: %v\n%s", test.name, err, output)
		}
	}
}

func TestLineageVerifier_A0BootstrapsWhileA1AndA2WithoutLiveEvidenceFailClosed(t *testing.T) {
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
		{name: "outside", tag: "v0.50.72", message: "outside the frozen A0/A1/A2 policy"},
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
