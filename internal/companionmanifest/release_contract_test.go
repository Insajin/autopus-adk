package companionmanifest

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestReleaseWorkflow_ExactA14ProtectedEnvironmentAndImmutableActions(t *testing.T) {
	release := readReleaseFile(t, ".github/workflows/release.yaml")
	for _, required := range []string{
		"v0.50.85", "refs/tags/v0.50.85",
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
		"'v0.50.69'", "'v0.50.70'", "'v0.50.71'", "'v0.50.72'", "'v0.50.73'", "'v0.50.74'", "'v0.50.75'", "'v0.50.76'", "'v0.50.77'", "'v0.50.78'", "'v0.50.79'", "'v0.50.80'", "'v0.50.81'", "'v0.50.82'", "'v0.50.83'", "'v0.50.84'",
		"refs/tags/v0.50.69", "refs/tags/v0.50.70", "refs/tags/v0.50.71", "refs/tags/v0.50.72", "refs/tags/v0.50.73", "refs/tags/v0.50.74", "refs/tags/v0.50.75", "refs/tags/v0.50.76", "refs/tags/v0.50.77", "refs/tags/v0.50.78", "refs/tags/v0.50.79", "refs/tags/v0.50.80", "refs/tags/v0.50.81", "refs/tags/v0.50.82", "refs/tags/v0.50.83", "refs/tags/v0.50.84",
	} {
		if strings.Contains(release, forbidden) {
			t.Fatalf("historical tag %q can enter the A14 release workflow", forbidden)
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

func TestReleaseSourceValidator_A5PinsAnnotatedTagAndA4Ancestor(t *testing.T) {
	source := readReleaseFile(t, "scripts/companion-release/validate-source.sh")
	declaration := "readonly A5_A4_ANCESTOR_SHA='334b297f05942accbecdfa15b54e38e005c82f2d'"
	if strings.Count(source, declaration) != 1 {
		t.Fatalf("A5 immutable A4 ancestry pin drifted: %s", declaration)
	}
	for _, required := range []string{
		`git cat-file -t "refs/tags/$GITHUB_REF_NAME"`,
		`git merge-base --is-ancestor "$A5_A4_ANCESTOR_SHA" "$GITHUB_SHA"`,
		`[[ "$tag_object_type" == 'tag' ]]`,
		"COMPANION_APPROVED_SOURCE_COMMIT", "COMPANION_APPROVED_SOURCE_TREE",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("A5 source gate missing %q", required)
		}
	}
}

func TestReleaseSourceValidator_A6PinsAnnotatedTagAndA5Ancestor(t *testing.T) {
	source := readReleaseFile(t, "scripts/companion-release/validate-source.sh")
	declaration := "readonly A6_A5_ANCESTOR_SHA='b27252cb1148192a8ae1a95195c50e5f221453a4'"
	if strings.Count(source, declaration) != 1 {
		t.Fatalf("A6 immutable A5 ancestry pin drifted: %s", declaration)
	}
	for _, required := range []string{
		`git cat-file -t "refs/tags/$GITHUB_REF_NAME"`,
		`git merge-base --is-ancestor "$A6_A5_ANCESTOR_SHA" "$GITHUB_SHA"`,
		`[[ "$tag_object_type" == 'tag' ]]`,
		"COMPANION_APPROVED_SOURCE_COMMIT", "COMPANION_APPROVED_SOURCE_TREE",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("A6 source gate missing %q", required)
		}
	}
}

func TestReleaseSourceValidator_A7PinsAnnotatedTagAndA6Ancestor(t *testing.T) {
	source := readReleaseFile(t, "scripts/companion-release/validate-source.sh")
	declaration := "readonly A7_A6_ANCESTOR_SHA='902f1acfa91f1d0a2ac9471d5cd79117031a2599'"
	if strings.Count(source, declaration) != 1 {
		t.Fatalf("A7 immutable A6 ancestry pin drifted: %s", declaration)
	}
	for _, required := range []string{
		`git cat-file -t "refs/tags/$GITHUB_REF_NAME"`,
		`git merge-base --is-ancestor "$A7_A6_ANCESTOR_SHA" "$GITHUB_SHA"`,
		`[[ "$tag_object_type" == 'tag' ]]`,
		"COMPANION_APPROVED_SOURCE_COMMIT", "COMPANION_APPROVED_SOURCE_TREE",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("A7 source gate missing %q", required)
		}
	}
}

func TestReleaseSourceValidator_A8PinsAnnotatedTagAndA7Ancestor(t *testing.T) {
	source := readReleaseFile(t, "scripts/companion-release/validate-source.sh")
	declaration := "readonly A8_A7_ANCESTOR_SHA='51de6030a69a8e36fcf7e5790ef157eff6fedf00'"
	if strings.Count(source, declaration) != 1 {
		t.Fatalf("A8 immutable A7 ancestry pin drifted: %s", declaration)
	}
	for _, required := range []string{
		`git cat-file -t "refs/tags/$GITHUB_REF_NAME"`,
		`git merge-base --is-ancestor "$A8_A7_ANCESTOR_SHA" "$GITHUB_SHA"`,
		`[[ "$tag_object_type" == 'tag' ]]`,
		"COMPANION_APPROVED_SOURCE_COMMIT", "COMPANION_APPROVED_SOURCE_TREE",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("A8 source gate missing %q", required)
		}
	}
}

func TestReleaseSourceValidator_A9PinsAnnotatedTagAndA8Ancestor(t *testing.T) {
	source := readReleaseFile(t, "scripts/companion-release/validate-source.sh")
	declaration := "readonly A9_A8_ANCESTOR_SHA='dd0c2759ed5435d4634011e349caad62ea3df414'"
	if strings.Count(source, declaration) != 1 {
		t.Fatalf("A9 immutable A8 ancestry pin drifted: %s", declaration)
	}
	for _, required := range []string{
		`git cat-file -t "refs/tags/$GITHUB_REF_NAME"`,
		`git merge-base --is-ancestor "$A9_A8_ANCESTOR_SHA" "$GITHUB_SHA"`,
		`[[ "$tag_object_type" == 'tag' ]]`,
		"COMPANION_APPROVED_SOURCE_COMMIT", "COMPANION_APPROVED_SOURCE_TREE",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("A9 source gate missing %q", required)
		}
	}
}

func TestReleaseWorkflow_HomebrewFormulaBridgeRunsAfterPublishBeforeCleanup(t *testing.T) {
	release := readReleaseFile(t, ".github/workflows/release.yaml")
	releaseIndex := strings.Index(release, "goreleaser release --clean")
	signingCleanupIndex := strings.Index(release, "name: Remove release signing credentials")
	evidenceIndex := strings.Index(release, "scripts/companion-release/verify-current-release.sh")
	tokenIndex := strings.Index(release, "name: Create Homebrew tap token")
	bridgeIndex := strings.Index(release, "scripts/companion-release/publish-homebrew-formula-bridge.sh")
	cleanupIndex := strings.Index(release, "Remove release credentials and keychain")
	if releaseIndex < 0 || signingCleanupIndex <= releaseIndex ||
		evidenceIndex <= signingCleanupIndex || tokenIndex <= evidenceIndex ||
		bridgeIndex <= tokenIndex || cleanupIndex <= bridgeIndex {
		t.Fatalf("Homebrew ordering release=%d signing-cleanup=%d evidence=%d token=%d bridge=%d cleanup=%d",
			releaseIndex, signingCleanupIndex, evidenceIndex, tokenIndex, bridgeIndex, cleanupIndex)
	}
	for _, exact := range []string{
		"GITHUB_REF_NAME='v0.50.85'",
		"COMPANION_VERSION='0.50.85'",
		"COMPANION_CHECKSUMS_PATH: ${{ steps.release-evidence.outputs.checksums-path }}",
		`COMPANION_CHECKSUMS_PATH="$COMPANION_CHECKSUMS_PATH"`,
		`HOMEBREW_TAP_TOKEN="$HOMEBREW_TAP_TOKEN"`,
	} {
		if !strings.Contains(release, exact) {
			t.Fatalf("Homebrew formula bridge missing exact input %q", exact)
		}
	}
	if strings.Contains(release, "COMPANION_CHECKSUMS_PATH='dist/checksums.txt'") {
		t.Fatal("Homebrew publication can consume unverified local checksums")
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
