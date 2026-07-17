package companionmanifest

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestReleaseWorkflow_ExactA4ProtectedEnvironmentAndImmutableActions(t *testing.T) {
	release := readReleaseFile(t, ".github/workflows/release.yaml")
	for _, required := range []string{
		"v0.50.73", "refs/tags/v0.50.73",
		"environment:", "adk-companion-release",
	} {
		if !strings.Contains(release, required) {
			t.Fatalf("release workflow missing %q", required)
		}
	}
	if strings.Contains(release, "- 'v*'") || strings.Contains(release, "- v*") {
		t.Fatal("arbitrary version tags can enter the protected release job")
	}
	for _, forbidden := range []string{
		"'v0.50.69'", "'v0.50.70'", "'v0.50.71'", "'v0.50.72'",
		"refs/tags/v0.50.69", "refs/tags/v0.50.70", "refs/tags/v0.50.71", "refs/tags/v0.50.72",
	} {
		if strings.Contains(release, forbidden) {
			t.Fatalf("historical tag %q can enter the A4 release workflow", forbidden)
		}
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

func TestReleaseSourceValidator_A2PinsAnnotatedTagAndBothAncestors(t *testing.T) {
	source := readReleaseFile(t, "scripts/companion-release/validate-source.sh")
	for _, declaration := range []string{
		"readonly A2_A1_ANCESTOR_SHA='e25e8be02b55b9385f58919c30ad1ccf92179030'",
		"readonly A2_MAIN_ANCESTOR_SHA='acb735cca0ef120cfed0d01863de09535310b5a3'",
	} {
		if strings.Count(source, declaration) != 1 {
			t.Fatalf("A2 immutable ancestry pin drifted: %s", declaration)
		}
	}
	for _, required := range []string{
		`git cat-file -t "refs/tags/$GITHUB_REF_NAME"`,
		`git merge-base --is-ancestor "$A2_A1_ANCESTOR_SHA" "$GITHUB_SHA"`,
		`git merge-base --is-ancestor "$A2_MAIN_ANCESTOR_SHA" "$GITHUB_SHA"`,
		`[[ "$tag_object_type" == 'tag' ]]`,
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("A2 source gate missing %q", required)
		}
	}
}

func TestReleaseSourceValidator_A3PinsAnnotatedTagAndA2Ancestor(t *testing.T) {
	source := readReleaseFile(t, "scripts/companion-release/validate-source.sh")
	declaration := "readonly A3_A2_ANCESTOR_SHA='7b5b52822b0cda75bf6c971f5f1c2a713881008c'"
	if strings.Count(source, declaration) != 1 {
		t.Fatalf("A3 immutable A2 ancestry pin drifted: %s", declaration)
	}
	for _, required := range []string{
		`git cat-file -t "refs/tags/$GITHUB_REF_NAME"`,
		`git merge-base --is-ancestor "$A3_A2_ANCESTOR_SHA" "$GITHUB_SHA"`,
		`[[ "$tag_object_type" == 'tag' ]]`,
		"COMPANION_APPROVED_SOURCE_COMMIT",
		"COMPANION_APPROVED_SOURCE_TREE",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("A3 source gate missing %q", required)
		}
	}
}

func TestReleaseSourceValidator_A4PinsAnnotatedTagAndA3Ancestor(t *testing.T) {
	source := readReleaseFile(t, "scripts/companion-release/validate-source.sh")
	declaration := "readonly A4_A3_ANCESTOR_SHA='ba5509b692a43dc8a70e0bd6173acb56166ed67f'"
	if strings.Count(source, declaration) != 1 {
		t.Fatalf("A4 immutable A3 ancestry pin drifted: %s", declaration)
	}
	for _, required := range []string{
		`git cat-file -t "refs/tags/$GITHUB_REF_NAME"`,
		`git merge-base --is-ancestor "$A4_A3_ANCESTOR_SHA" "$GITHUB_SHA"`,
		`[[ "$tag_object_type" == 'tag' ]]`,
		"COMPANION_APPROVED_SOURCE_COMMIT", "COMPANION_APPROVED_SOURCE_TREE",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("A4 source gate missing %q", required)
		}
	}
}

func TestReleaseWorkflow_HomebrewFormulaBridgeRunsAfterPublishBeforeCleanup(t *testing.T) {
	release := readReleaseFile(t, ".github/workflows/release.yaml")
	releaseIndex := strings.Index(release, "goreleaser release --clean")
	bridgeIndex := strings.Index(release, "scripts/companion-release/publish-homebrew-formula-bridge.sh")
	cleanupIndex := strings.Index(release, "Remove release credentials and keychain")
	if releaseIndex < 0 || bridgeIndex <= releaseIndex || cleanupIndex <= bridgeIndex {
		t.Fatalf("Homebrew bridge ordering release=%d bridge=%d cleanup=%d", releaseIndex, bridgeIndex, cleanupIndex)
	}
	for _, exact := range []string{
		"GITHUB_REF_NAME='v0.50.73'",
		"COMPANION_VERSION='0.50.73'",
		"COMPANION_CHECKSUMS_PATH='dist/checksums.txt'",
		`HOMEBREW_TAP_TOKEN="$HOMEBREW_TAP_TOKEN"`,
	} {
		if !strings.Contains(release, exact) {
			t.Fatalf("Homebrew formula bridge missing exact input %q", exact)
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
