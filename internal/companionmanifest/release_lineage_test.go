package companionmanifest

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseSourceValidator_ExactTagAndFortyHexCommit(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init", "-q")
	runGit(t, dir, "config", "user.name", "Release Test")
	runGit(t, dir, "config", "user.email", "release-test@example.invalid")
	if err := os.WriteFile(filepath.Join(dir, "source"), []byte("source"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "source")
	runGit(t, dir, "commit", "-qm", "source")
	sha := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD"))
	runGit(t, dir, "tag", "v0.50.69")
	outputPath := filepath.Join(dir, "github-output")
	command := exec.Command("bash", filepath.Join(repositoryRoot(t), "scripts/companion-release/validate-source.sh"))
	command.Dir = dir
	command.Env = append(os.Environ(),
		"GITHUB_REF_NAME=v0.50.69", "GITHUB_REF_TYPE=tag", "GITHUB_SHA="+sha,
		"GITHUB_OUTPUT="+outputPath,
	)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("source validation failed: %v\n%s", err, output)
	}
	written, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(written), "source-commit="+sha) || len(sha) != 40 {
		t.Fatalf("validated source output = %q", written)
	}
	command = exec.Command("bash", filepath.Join(repositoryRoot(t), "scripts/companion-release/validate-source.sh"))
	command.Dir = dir
	command.Env = append(os.Environ(),
		"GITHUB_REF_NAME=v0.50.71", "GITHUB_REF_TYPE=tag", "GITHUB_SHA="+sha,
		"GITHUB_OUTPUT="+outputPath,
	)
	if err := command.Run(); err == nil {
		t.Fatal("out-of-policy release tag was accepted")
	}
}

func TestLineageVerifier_A0BootstrapsWhileA1WithoutLiveEvidenceFailsClosed(t *testing.T) {
	script := filepath.Join(repositoryRoot(t), "scripts/companion-release/verify-public-key-lineage.sh")
	cases := []struct {
		name    string
		tag     string
		wantOK  bool
		message string
	}{
		{name: "A0", tag: "v0.50.69", wantOK: true, message: "bootstrap accepted"},
		{name: "A1", tag: "v0.50.70", message: "missing GITHUB_TOKEN"},
		{name: "outside", tag: "v0.50.71", message: "outside the frozen A0/A1 policy"},
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
