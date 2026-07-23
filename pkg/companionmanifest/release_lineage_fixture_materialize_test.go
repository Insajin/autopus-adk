package companionmanifest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func (fixture *executableLineageFixture) writeEvidence(t *testing.T) {
	t.Helper()
	fixture.writeEvidenceWithChecksumReseal(t, false)
}

func (fixture *executableLineageFixture) writeResealedEvidence(t *testing.T) {
	t.Helper()
	fixture.writeEvidenceWithChecksumReseal(t, true)
}

func (fixture *executableLineageFixture) writeEvidenceWithChecksumReseal(
	t *testing.T,
	reseal bool,
) {
	t.Helper()
	if err := os.RemoveAll(fixture.assetsDir); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(fixture.assetsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	assets := fixture.materializeLineageArchiveAssets(t)
	if reseal {
		fixture.resealLineageArchiveChecksums(t)
	}
	fixture.writeEvidenceMetadata(t, assets)
}

func (fixture *executableLineageFixture) resealLineageArchiveChecksums(t *testing.T) {
	t.Helper()
	// Preserve the outer checksum layer so inner trust-claim tampering is exercised.
	lines := strings.Split(string(fixture.checksums), "\n")
	for _, target := range executableLineageArchiveTargets {
		name := target.archiveName(fixture.evidence.version)
		digest, err := lineageArchiveFileDigest(filepath.Join(fixture.assetsDir, name))
		if err != nil {
			t.Fatalf("digest resealed %s lineage archive: %v", target.key, err)
		}
		matches := 0
		for index, line := range lines {
			fields := strings.Fields(line)
			if len(fields) == 2 && fields[1] == name {
				lines[index] = digest + "  " + name
				matches++
			}
		}
		if matches != 1 {
			t.Fatalf("checksum entries for %s = %d, want 1", name, matches)
		}
	}
	fixture.checksums = []byte(strings.Join(lines, "\n"))
	fixture.pins.checksums = lineageDigest(fixture.checksums)
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
