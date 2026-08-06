package companionmanifest

import (
	"strings"
	"testing"
)

func TestGoReleaser_A22UsesCommitTimeTrimmedReproducibleBuilds(t *testing.T) {
	config := readReleaseFile(t, ".goreleaser.yaml")
	for _, required := range []string{
		"flags:\n      - -trimpath\n      - -buildvcs=false",
		`mod_timestamp: "{{ .CommitTimestamp }}"`,
		"github.com/insajin/autopus-adk/pkg/version.date={{.CommitDate}}",
		"github.com/insajin/autopus-adk/pkg/version.sourceCommit={{.Env.COMPANION_SOURCE_COMMIT}}",
		"github.com/insajin/autopus-adk/pkg/version.sourceTree={{.Env.COMPANION_SOURCE_TREE}}",
	} {
		if !strings.Contains(config, required) {
			t.Fatalf("reproducible GoReleaser config missing %q", required)
		}
	}
	if strings.Contains(config, "pkg/version.date={{.Date}}") {
		t.Fatal("release binary still embeds wall-clock release time")
	}
}

func TestReleaseWorkflow_A22BuildsCanonicalCandidateBeforeAuthority(t *testing.T) {
	release := readReleaseFile(t, ".github/workflows/release.yaml")
	preflight := readReleaseFile(t, ".github/workflows/companion-release-preflight.yml")
	runtime := readReleaseFile(t, "scripts/companion-release/prepare-release-runtime-lib.sh")
	builder := readReleaseFile(t, "scripts/companion-release/build-omp-context-candidate.sh")
	surface := strings.Join([]string{release, preflight, runtime, builder}, "\n")
	for _, required := range []string{
		"scripts/companion-release/build-omp-context-candidate.sh",
		`"${COMPANION_RELEASE_TAG:-}" == "$expected_tag"`,
		`COMPANION_RELEASE_TAG="$GITHUB_REF_NAME"`,
		"COMPANION_RELEASE_TAG: ${{ inputs.release_tag }}",
		`COMPANION_RELEASE_TAG="$release_tag"`,
		"go build -trimpath -buildvcs=false",
		"CGO_ENABLED=0",
		"GOOS=darwin",
		"GOARCH=arm64",
		"GOARM64=v8.0",
		"pkg/version.sourceCommit=${GITHUB_SHA}",
		"pkg/version.sourceTree=${COMPANION_SOURCE_TREE}",
		`TZ=UTC git show -s --date='format-local:%Y-%m-%dT%H:%M:%SZ'`,
		`short_commit=${GITHUB_SHA:0:8}`,
	} {
		if !strings.Contains(surface, required) {
			t.Fatalf("canonical candidate build contract missing %q", required)
		}
	}
	for _, obsolete := range []string{`--format=%cI`, `${GITHUB_SHA:0:7}`} {
		if strings.Contains(builder, obsolete) {
			t.Fatalf("canonical candidate builder retains GoReleaser-incompatible input %q", obsolete)
		}
	}
	if strings.Contains(preflight, "GITHUB_REF_NAME: ${{ inputs.release_tag }}") {
		t.Fatal("preflight attempts to override a reserved GitHub environment variable")
	}
}
