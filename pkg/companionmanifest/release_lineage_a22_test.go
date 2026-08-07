package companionmanifest

import (
	"strings"
	"testing"
)

const (
	publicKeyReceiptA22Tag     = "v0.50.101"
	publicKeyReceiptA22Version = "0.50.101"
)

var immutableA21LineagePins = map[string]string{
	"A21_RELEASE_ID":                 "363679709",
	"A21_COMMIT_SHA":                 "b86fab067599f457261287552c5a9dd86460d7f4",
	"A21_TREE_SHA":                   "64d32460859b325e55dcf9959a8478567c8849a7",
	"A21_TAG_OBJECT_SHA":             "fdc0e94fbe2325ed47cf7a3fd1792028c1f3bb75",
	"A21_CHECKSUMS_SHA256":           "ae2933e24be05002d02497cc56fa4bb48c3b366941e8888b4874ed3445c9f892",
	"A21_AMD64_ARCHIVE_SHA256":       "027587eb7e7a73a33541ac4e9aaed1d36b446298ecf48ca78292ee87dc589f58",
	"A21_ARM64_ARCHIVE_SHA256":       "4364bb3e3db09df8ba63927c8f8f64978fd63d880831dfc154f4ac9a92484492",
	"A21_LINUX_AMD64_ARCHIVE_SHA256": "3058d0eec967a341c5eb9a176315f5093ba8a3763db6bb88d78df69ad2e4c961",
	"A21_LINUX_ARM64_ARCHIVE_SHA256": "a259b171cb5da9ad7ff6bae2d23f7e222297ff443456530928741bae118dbdc7",
	"A21_AMD64_MANIFEST_SHA256":      "4ceb4c9fd59e53295a7846c5d018cb2b65798d6aba6a91c9b3b5373c1342dde9",
	"A21_ARM64_MANIFEST_SHA256":      "5ab616a51d76c2606347caa0aa334467c21508e3f917307e264def17e58c5503",
}

func TestReleasePublicKeyReceipt_A22PolicyPinsExactA21Coordinate(t *testing.T) {
	scripts := normalizedReleaseText(releaseScriptsText(t))
	if !exactLineageTagVersionGuard(scripts, "101") {
		t.Fatal("A22 release is not conjunctively restricted to tag v0.50.101 and version 0.50.101")
	}
	for _, required := range []string{
		"release_phase='A22'", "prior_phase='A21'",
		`prior_release_id="$A21_RELEASE_ID"`, `prior_tree="$A21_TREE_SHA"`,
		`prior_linux_amd64_archive="$A21_LINUX_AMD64_ARCHIVE_SHA256"`,
		`prior_linux_arm64_archive="$A21_LINUX_ARM64_ARCHIVE_SHA256"`,
	} {
		if !strings.Contains(scripts, required) {
			t.Fatalf("A22 exact A21 predecessor contract missing %q", required)
		}
	}
}

func TestReleasePublicKeyReceipt_GoReleaserA21FixtureSealsA22DirectPredecessor(t *testing.T) {
	tools := newExecutableLineageTools(t)
	evidence := produceGoReleaserFixtureEvidence(
		t, tools, publicKeyReceiptA21Tag, publicKeyReceiptA21Version, true,
	)
	fixture := newExecutableLineageFixture(t, tools, evidence)
	output, err := fixture.run(t)
	if err != nil {
		t.Fatalf("valid A21 to A22 lineage failed: %v\n%s", err, output)
	}
	if !strings.Contains(output, "A22 exact A21 key record verified") {
		t.Fatalf("valid A22 lineage diagnostic = %q", output)
	}
}
