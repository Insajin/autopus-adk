package companionmanifest

import (
	"bytes"
	"sync"
	"sync/atomic"
	"testing"
)

type goReleaserLineageCache struct {
	version   string
	annotated bool
	once      sync.Once
	runs      atomic.Int32
	evidence  *goReleaserA0Evidence
}

var goReleaserLineageCaches = map[string]*goReleaserLineageCache{
	publicKeyReceiptA0Tag: {version: lineageA0Version},
	publicKeyReceiptA1Tag: {version: lineageA1Version, annotated: true},
	publicKeyReceiptA2Tag: {version: publicKeyReceiptA2Version, annotated: true},
	publicKeyReceiptA3Tag: {version: publicKeyReceiptA3Version, annotated: true},
	publicKeyReceiptA4Tag: {version: publicKeyReceiptA4Version, annotated: true},
	publicKeyReceiptA5Tag: {version: publicKeyReceiptA5Version, annotated: true},
	publicKeyReceiptA6Tag: {version: publicKeyReceiptA6Version, annotated: true},
}

func produceGoReleaserFixtureEvidence(
	t *testing.T,
	tools executableLineageTools,
	releaseTag, releaseVersion string,
	annotated bool,
) *goReleaserA0Evidence {
	t.Helper()
	requireExecutableLineageIntegration(t)
	cache, ok := goReleaserLineageCaches[releaseTag]
	if !ok || cache.version != releaseVersion || cache.annotated != annotated {
		t.Fatalf("unsupported GoReleaser lineage cache coordinate %s/%s annotated=%t",
			releaseTag, releaseVersion, annotated)
	}
	cache.once.Do(func() {
		cache.runs.Add(1)
		cache.evidence = produceUncachedGoReleaserFixtureEvidence(
			t, tools, releaseTag, releaseVersion, annotated,
		)
	})
	if cache.evidence == nil {
		t.Fatal("GoReleaser lineage cache produced no evidence")
	}
	return cloneGoReleaserEvidence(cache.evidence)
}

func cloneGoReleaserEvidence(source *goReleaserA0Evidence) *goReleaserA0Evidence {
	clone := *source
	clone.archives = make(map[string][]byte, len(source.archives))
	for architecture, archive := range source.archives {
		clone.archives[architecture] = bytes.Clone(archive)
	}
	clone.checksums = bytes.Clone(source.checksums)
	clone.receipt = bytes.Clone(source.receipt)
	clone.signature = bytes.Clone(source.signature)
	return &clone
}

func TestGoReleaserLineageFixtures_ProcessCacheBuildsEachCoordinateOnce(t *testing.T) {
	tools := newExecutableLineageTools(t)
	cases := []struct {
		tag       string
		version   string
		annotated bool
	}{
		{tag: publicKeyReceiptA0Tag, version: lineageA0Version},
		{tag: publicKeyReceiptA1Tag, version: lineageA1Version, annotated: true},
		{tag: publicKeyReceiptA2Tag, version: publicKeyReceiptA2Version, annotated: true},
		{tag: publicKeyReceiptA3Tag, version: publicKeyReceiptA3Version, annotated: true},
		{tag: publicKeyReceiptA4Tag, version: publicKeyReceiptA4Version, annotated: true},
		{tag: publicKeyReceiptA5Tag, version: publicKeyReceiptA5Version, annotated: true},
		{tag: publicKeyReceiptA6Tag, version: publicKeyReceiptA6Version, annotated: true},
	}
	for _, test := range cases {
		first := produceGoReleaserFixtureEvidence(t, tools, test.tag, test.version, test.annotated)
		second := produceGoReleaserFixtureEvidence(t, tools, test.tag, test.version, test.annotated)
		if first == second || !bytes.Equal(first.checksums, second.checksums) {
			t.Fatalf("GoReleaser lineage cache %s did not return independent equivalent evidence", test.tag)
		}
		if runs := goReleaserLineageCaches[test.tag].runs.Load(); runs != 1 {
			t.Fatalf("GoReleaser lineage cache %s runs = %d, want 1", test.tag, runs)
		}
	}
	if runs := executableLineageToolsBuildRuns.Load(); runs != 1 {
		t.Fatalf("executable lineage tool build runs = %d, want 1", runs)
	}
}

func TestGoReleaserLineageFixtures_RequireIntegrationTag(t *testing.T) {
	if executableLineageIntegrationEnabled {
		t.Skip("non-integration contract")
	}
	if runs := executableLineageToolsBuildRuns.Load(); runs != 0 {
		t.Fatalf("non-integration executable lineage tool build runs = %d, want 0", runs)
	}
	for tag, cache := range goReleaserLineageCaches {
		if runs := cache.runs.Load(); runs != 0 {
			t.Fatalf("non-integration GoReleaser lineage cache %s runs = %d, want 0", tag, runs)
		}
	}
}
