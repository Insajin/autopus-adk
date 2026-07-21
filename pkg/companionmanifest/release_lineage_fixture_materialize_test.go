package companionmanifest

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func (fixture *executableLineageFixture) writeEvidence(t *testing.T) {
	t.Helper()
	if err := os.RemoveAll(fixture.assetsDir); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(fixture.assetsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	assets := make([]executableLineageAsset, 0, 3)
	for _, architecture := range []string{"amd64", "arm64"} {
		name := fmt.Sprintf("autopus-adk_%s_darwin_%s.tar.gz",
			fixture.evidence.version, architecture)
		source := fixture.evidence.archives[architecture]
		target := filepath.Join(fixture.assetsDir, name)
		mutation := fixture.archiveMutation
		if mutation != nil &&
			(mutation.architecture == "" || mutation.architecture == architecture) {
			err := rewriteLineageArchiveTarget(source, target, mutation.entry,
				func(data []byte) ([]byte, bool) { return mutation.mutate(t, data) })
			if err != nil {
				t.Fatalf("rewrite %s lineage archive: %v", architecture, err)
			}
		} else if _, err := materializeLineageArchive(source, target, os.Link); err != nil {
			t.Fatalf("materialize %s lineage archive: %v", architecture, err)
		}
		digest, err := lineageArchiveFileDigest(target)
		if err != nil {
			t.Fatalf("digest %s lineage archive: %v", architecture, err)
		}
		if architecture == "amd64" && fixture.assetDigestOverride != "" {
			digest = fixture.assetDigestOverride
		} else {
			digest = "sha256:" + digest
		}
		assets = append(assets, executableLineageAsset{
			Name: name, State: "uploaded", Digest: digest,
		})
	}
	fixture.writeEvidenceMetadata(t, assets)
}

func (fixture *executableLineageFixture) writeEvidenceMetadata(
	t *testing.T,
	assets []executableLineageAsset,
) {
	t.Helper()
	checksumsName := "checksums.txt"
	if err := os.WriteFile(filepath.Join(fixture.assetsDir, checksumsName),
		fixture.checksums, 0o600); err != nil {
		t.Fatal(err)
	}
	assets = append(assets, executableLineageAsset{
		Name: checksumsName, State: "uploaded",
		Digest: "sha256:" + lineageDigest(fixture.checksums),
	})
	writeLineageJSON(t, fixture.releaseJSON, executableLineageRelease{
		TagName: fixture.releaseTag, TargetCommitish: fixture.targetCommit,
		Immutable: true, Assets: assets,
	})
	if fixture.tagObject == "" {
		writeLineageJSON(t, fixture.tagJSON, map[string]any{
			"object": map[string]string{"type": "commit", "sha": fixture.tagCommit},
		})
	} else {
		writeLineageJSON(t, fixture.tagJSON, map[string]any{
			"object": map[string]string{"type": "tag", "sha": fixture.tagObject},
		})
		writeLineageJSON(t, fixture.annotatedTagJSON, map[string]any{
			"tag":    fixture.releaseTag,
			"object": map[string]string{"type": "commit", "sha": fixture.tagCommit},
		})
	}
	writeLineageJSON(t, fixture.commitJSON, map[string]any{
		"sha": fixture.evidence.commit,
		"commit": map[string]any{
			"tree": map[string]string{"sha": fixture.evidence.pins.tree},
		},
	})
}
