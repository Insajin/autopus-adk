package companionmanifest

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

type executableLineageArchiveTarget struct {
	key          string
	platform     string
	architecture string
}

var executableLineageArchiveTargets = []executableLineageArchiveTarget{
	{key: "amd64", platform: "darwin", architecture: "amd64"},
	{key: "arm64", platform: "darwin", architecture: "arm64"},
	{key: "linux_amd64", platform: "linux", architecture: "amd64"},
	{key: "linux_arm64", platform: "linux", architecture: "arm64"},
}

var darwinLineageArchiveEvidenceNames = []string{
	"adk-companion-manifest.json",
	"adk-companion-manifest.sig",
	"adk-companion-darwin-receipt.json",
	lineageBundleName + "/public-key-receipt.json",
	lineageBundleName + "/public-key-receipt.sig",
}

func (target executableLineageArchiveTarget) archiveName(version string) string {
	return fmt.Sprintf("autopus-adk_%s_%s_%s.tar.gz",
		version, target.platform, target.architecture)
}

func cacheGoReleaserLineageArchives(
	t *testing.T,
	repositoryRoot, cacheDirectory string,
	evidence *goReleaserA0Evidence,
) {
	t.Helper()
	for _, target := range executableLineageArchiveTargets {
		name := target.archiveName(evidence.version)
		sourceArchive := filepath.Join(repositoryRoot, "dist", name)
		cachedArchive := filepath.Join(cacheDirectory, name)
		if _, err := materializeLineageArchive(sourceArchive, cachedArchive, os.Link); err != nil {
			t.Fatalf("cache exact %s archive: %v", target.key, err)
		}
		evidence.archives[target.key] = cachedArchive
		if target.platform == "darwin" {
			captureDarwinLineageArchiveEvidence(t, evidence, target, cachedArchive)
		}
	}
}

func captureDarwinLineageArchiveEvidence(
	t *testing.T,
	evidence *goReleaserA0Evidence,
	target executableLineageArchiveTarget,
	archive string,
) {
	t.Helper()
	entries, err := decodeLineageArchiveFile(archive)
	if err != nil {
		t.Fatalf("decode exact %s archive: %v", target.key, err)
	}
	manifest := entries["adk-companion-manifest.json"].data
	switch target.key {
	case "amd64":
		evidence.receipt = entries[lineageBundleName+"/public-key-receipt.json"].data
		evidence.signature = entries[lineageBundleName+"/public-key-receipt.sig"].data
		evidence.pins.amd64Manifest = lineageDigest(manifest)
	case "arm64":
		evidence.pins.arm64Manifest = lineageDigest(manifest)
	default:
		t.Fatalf("unexpected Darwin lineage target %s", target.key)
	}
}

func captureGoReleaserArchivePins(
	t *testing.T,
	evidence *goReleaserA0Evidence,
	pins *executableLineagePins,
) {
	t.Helper()
	for _, target := range executableLineageArchiveTargets {
		digest, err := lineageArchiveFileDigest(evidence.archives[target.key])
		if err != nil {
			t.Fatalf("digest exact %s archive: %v", target.key, err)
		}
		setExecutableLineageArchivePin(t, pins, target.key, digest)
	}
}

func setExecutableLineageArchivePin(
	t *testing.T,
	pins *executableLineagePins,
	target, digest string,
) {
	t.Helper()
	switch target {
	case "amd64":
		pins.amd64Archive = digest
	case "arm64":
		pins.arm64Archive = digest
	case "linux_amd64":
		pins.linuxAMD64Archive = digest
	case "linux_arm64":
		pins.linuxARM64Archive = digest
	default:
		t.Fatalf("unknown executable lineage archive target %s", target)
	}
}

func (fixture *executableLineageFixture) materializeLineageArchiveAssets(
	t *testing.T,
) []executableLineageAsset {
	t.Helper()
	assets := make([]executableLineageAsset, 0, len(executableLineageArchiveTargets))
	for _, target := range executableLineageArchiveTargets {
		name := target.archiveName(fixture.evidence.version)
		source := fixture.evidence.archives[target.key]
		if source == "" {
			t.Fatalf("lineage evidence is missing %s archive", target.key)
		}
		path := filepath.Join(fixture.assetsDir, name)
		fixture.materializeLineageArchiveTarget(t, target, source, path)
		digest, err := lineageArchiveFileDigest(path)
		if err != nil {
			t.Fatalf("digest %s lineage archive: %v", target.key, err)
		}
		if target.key == "amd64" && fixture.assetDigestOverride != "" {
			digest = fixture.assetDigestOverride
		} else {
			digest = "sha256:" + digest
		}
		assets = append(assets, executableLineageAsset{
			Name: name, State: "uploaded", Digest: digest,
		})
	}
	return assets
}

func (fixture *executableLineageFixture) materializeLineageArchiveTarget(
	t *testing.T,
	target executableLineageArchiveTarget,
	source, destination string,
) {
	t.Helper()
	mutation := fixture.archiveMutation
	if lineageArchiveMutationMatches(mutation, target) {
		err := rewriteLineageArchiveTarget(source, destination, mutation.entry,
			func(data []byte) ([]byte, bool) { return mutation.mutate(t, data) })
		if err != nil {
			t.Fatalf("rewrite %s lineage archive: %v", target.key, err)
		}
		return
	}
	if _, err := materializeLineageArchive(source, destination, os.Link); err != nil {
		t.Fatalf("materialize %s lineage archive: %v", target.key, err)
	}
}

func lineageArchiveMutationMatches(
	mutation *lineageArchiveMutation,
	target executableLineageArchiveTarget,
) bool {
	if mutation == nil {
		return false
	}
	if mutation.architecture == "" {
		return target.platform == "darwin"
	}
	return mutation.architecture == target.key
}

func TestExecutableLineageArchiveTargets_PreserveDarwinKeysAndSeparateLinux(t *testing.T) {
	want := []executableLineageArchiveTarget{
		{key: "amd64", platform: "darwin", architecture: "amd64"},
		{key: "arm64", platform: "darwin", architecture: "arm64"},
		{key: "linux_amd64", platform: "linux", architecture: "amd64"},
		{key: "linux_arm64", platform: "linux", architecture: "arm64"},
	}
	if fmt.Sprint(executableLineageArchiveTargets) != fmt.Sprint(want) {
		t.Fatalf("executable lineage targets = %#v, want %#v",
			executableLineageArchiveTargets, want)
	}
}

func TestLineageArchiveMutationMatches_DefaultRemainsDarwinOnly(t *testing.T) {
	mutation := &lineageArchiveMutation{}
	for _, target := range executableLineageArchiveTargets {
		got := lineageArchiveMutationMatches(mutation, target)
		if got != (target.platform == "darwin") {
			t.Fatalf("default mutation match for %s = %t", target.key, got)
		}
	}
}
