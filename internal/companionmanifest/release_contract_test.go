package companionmanifest

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestReleaseWorkflow_ExactPhasesProtectedEnvironmentAndImmutableActions(t *testing.T) {
	release := readReleaseFile(t, ".github/workflows/release.yaml")
	for _, required := range []string{
		"v0.50.69", "v0.50.70", "refs/tags/v0.50.69", "refs/tags/v0.50.70",
		"environment:", "adk-companion-release",
	} {
		if !strings.Contains(release, required) {
			t.Fatalf("release workflow missing %q", required)
		}
	}
	if strings.Contains(release, "- 'v*'") || strings.Contains(release, "- v*") {
		t.Fatal("arbitrary version tags can enter the protected release job")
	}
	immutable := regexp.MustCompile(`^[^@[:space:]]+@[0-9a-f]{40}$`)
	for _, name := range []string{
		".github/workflows/release.yaml", ".github/workflows/ci.yaml",
		".github/workflows/security.yml",
	} {
		var workflow struct {
			Jobs map[string]struct {
				Uses  string `yaml:"uses"`
				Steps []struct {
					Uses string `yaml:"uses"`
				} `yaml:"steps"`
			} `yaml:"jobs"`
		}
		if err := yaml.Unmarshal([]byte(readReleaseFile(t, name)), &workflow); err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for jobName, job := range workflow.Jobs {
			if job.Uses != "" && !strings.HasPrefix(job.Uses, "./") && !immutable.MatchString(job.Uses) {
				t.Fatalf("%s job %s uses mutable action %q", name, jobName, job.Uses)
			}
			for _, step := range job.Steps {
				if step.Uses != "" && !strings.HasPrefix(step.Uses, "./") && !immutable.MatchString(step.Uses) {
					t.Fatalf("%s uses mutable action %q", name, step.Uses)
				}
			}
		}
	}
	if !strings.Contains(release, `version: "v2.17.0"`) {
		t.Fatal("GoReleaser binary is not pinned to one exact version")
	}
}

func TestGoReleaser_TargetCommitishUsesValidatedFortyHexSourceCommit(t *testing.T) {
	config := readReleaseFile(t, ".goreleaser.yaml")
	if !strings.Contains(config, `target_commitish: "{{ .Env.COMPANION_SOURCE_COMMIT }}"`) {
		t.Fatal("GoReleaser target_commitish is not bound to the validated source commit")
	}
	workflow := readReleaseFile(t, ".github/workflows/release.yaml")
	for _, required := range []string{
		"scripts/companion-release/validate-source.sh",
		`COMPANION_SOURCE_COMMIT="${{ steps.release-source.outputs.source-commit }}"`,
	} {
		if !strings.Contains(workflow, required) {
			t.Fatalf("release source SHA contract missing %q", required)
		}
	}
}

func TestReleaseScripts_SourceFilesStayBelowLimit(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join(repositoryRoot(t), "scripts", "companion-release", "*.sh"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("find release scripts: %v", err)
	}
	for _, path := range paths {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if lines := strings.Count(string(data), "\n") + 1; lines > 300 {
			t.Fatalf("%s has %d lines, want <= 300", filepath.Base(path), lines)
		}
	}
}

func readReleaseFile(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repositoryRoot(t), filepath.FromSlash(name)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}
