package promptlayer

import (
	"crypto/ed25519"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestOMPContextPromotionArtifactLoaderV2_ReadsOnlyFixedPrivateFiles(t *testing.T) {
	fixture := newOMPContextPromotionV2Fixture(t)
	root := prepareOMPContextPromotionArtifactRootV2(t)
	reportPath := filepath.Join(root, ompContextPromotionReportRelativePathV2)
	attestationPath := filepath.Join(root, ompContextPromotionAttestationRelativePathV2)
	writePrivateOMPContextPromotionArtifactV2(t, reportPath, fixture.reportBytes)
	writePrivateOMPContextPromotionArtifactV2(t, attestationPath, fixture.attestationBytes)

	resolvedReport, resolvedAttestation, err := resolveOMPContextPromotionArtifactPathsV2(root)
	if err != nil {
		t.Fatalf("resolve artifact paths: %v", err)
	}
	if resolvedReport != reportPath || resolvedAttestation != attestationPath {
		t.Fatalf("unexpected fixed paths: %q %q", resolvedReport, resolvedAttestation)
	}
	if body, err := readOMPContextPromotionArtifactFileV2(reportPath, ompContextPromotionReportMaxBytesV1); err != nil || string(body) != string(fixture.reportBytes) {
		t.Fatalf("read private report: %v", err)
	}
	if _, err := LoadVerifiedOMPContextPromotionV2(root, fixture.now, fixture.expectation); err == nil {
		t.Fatal("test signer must not be admitted by the production trust root")
	}
}

func TestOMPContextPromotionArtifactLoaderV2_RejectsUnsafeFilesAndDirectories(t *testing.T) {
	t.Run("public file", func(t *testing.T) {
		root := prepareOMPContextPromotionArtifactRootV2(t)
		path := filepath.Join(root, ompContextPromotionReportRelativePathV2)
		if err := os.WriteFile(path, []byte(`{}`), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := readOMPContextPromotionArtifactFileV2(path, 1024); err == nil {
			t.Fatal("public file accepted")
		}
	})

	t.Run("symlink file", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("symlink semantics differ")
		}
		root := prepareOMPContextPromotionArtifactRootV2(t)
		target := filepath.Join(root, "target")
		if err := os.WriteFile(target, []byte(`{}`), 0o600); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(root, ompContextPromotionReportRelativePathV2)
		if err := os.Symlink(target, path); err != nil {
			t.Fatal(err)
		}
		if _, err := readOMPContextPromotionArtifactFileV2(path, 1024); err == nil {
			t.Fatal("symlink file accepted")
		}
	})

	t.Run("public runtime directory", func(t *testing.T) {
		root := prepareOMPContextPromotionArtifactRootV2(t)
		path := filepath.Join(root, ".autopus", "runtime")
		if err := os.Chmod(path, 0o755); err != nil {
			t.Fatal(err)
		}
		if _, _, err := resolveOMPContextPromotionArtifactPathsV2(root); err == nil {
			t.Fatal("public runtime directory accepted")
		}
	})

	t.Run("symlink root", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("symlink semantics differ")
		}
		root := prepareOMPContextPromotionArtifactRootV2(t)
		link := filepath.Join(t.TempDir(), "workspace")
		if err := os.Symlink(root, link); err != nil {
			t.Fatal(err)
		}
		if _, _, err := resolveOMPContextPromotionArtifactPathsV2(link); err == nil {
			t.Fatal("symlink root accepted")
		}
	})
}

func TestOMPContextPromotionTrustV2_ContainsOnlyCommittedPublicKey(t *testing.T) {
	keys := committedOMPContextPromotionPublicKeysV2()
	if len(keys) != 1 || len(keys[OMPContextPromotionKeyID2026Q3K1]) != ed25519.PublicKeySize {
		t.Fatalf("unexpected production trust root: %#v", keys)
	}
	keys[OMPContextPromotionKeyID2026Q3K1][0] ^= 0xff
	if keys[OMPContextPromotionKeyID2026Q3K1][0] == committedOMPContextPromotionPublicKeysV2()[OMPContextPromotionKeyID2026Q3K1][0] {
		t.Fatal("trust root returned mutable backing storage")
	}
}

func prepareOMPContextPromotionArtifactRootV2(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".autopus"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, ".autopus", "runtime"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, ".autopus", "runtime", "omp-context"), 0o700); err != nil {
		t.Fatal(err)
	}
	return root
}

func writePrivateOMPContextPromotionArtifactV2(t *testing.T, path string, body []byte) {
	t.Helper()
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
}
