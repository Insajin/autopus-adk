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
	for _, required := range []string{
		"scripts/companion-release/build-omp-context-candidate.sh",
		"go build -trimpath -buildvcs=false",
		"CGO_ENABLED=0",
		"GOOS=darwin",
		"GOARCH=arm64",
		"GOARM64=v8.0",
		"pkg/version.sourceCommit=${GITHUB_SHA}",
		"pkg/version.sourceTree=${COMPANION_SOURCE_TREE}",
	} {
		if !strings.Contains(release+"\n"+
			readReleaseFile(t, "scripts/companion-release/build-omp-context-candidate.sh"), required) {
			t.Fatalf("canonical candidate build contract missing %q", required)
		}
	}
}
