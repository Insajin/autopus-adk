package promptlayer

import (
	"bytes"
	"crypto/ed25519"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestOMPContextPromotionArtifactLoaderV2_ReadsOnlyFixedPrivateFiles(t *testing.T) {
	requireSecureOMPContextPromotionArtifactPlatformV2(t)
	fixture := newOMPContextPromotionV2Fixture(t)
	root := prepareOMPContextPromotionArtifactRootV2(t)
	writeOMPContextPromotionArtifactPairV2(t, root, fixture.reportBytes, fixture.attestationBytes)

	report, attestation, err := readOMPContextPromotionArtifactPairV2(root)
	if err != nil {
		t.Fatalf("read private artifact pair: %v", err)
	}
	if !bytes.Equal(report, fixture.reportBytes) || !bytes.Equal(attestation, fixture.attestationBytes) {
		t.Fatal("artifact pair bytes changed")
	}
	if _, err := loadVerifiedOMPContextPromotionV2At(root, fixture.now, fixture.expectation); err == nil {
		t.Fatal("test signer must not be admitted by the production trust root")
	}
}

func TestOMPContextPromotionArtifactLoaderV2_RejectsUnsafeFilesAndDirectories(t *testing.T) {
	requireSecureOMPContextPromotionArtifactPlatformV2(t)
	fixture := newOMPContextPromotionV2Fixture(t)

	t.Run("public file", func(t *testing.T) {
		root := prepareOMPContextPromotionArtifactRootV2(t)
		reportPath := filepath.Join(root, ompContextPromotionReportRelativePathV2)
		attestationPath := filepath.Join(root, ompContextPromotionAttestationRelativePathV2)
		if err := os.WriteFile(reportPath, fixture.reportBytes, 0o644); err != nil {
			t.Fatal(err)
		}
		writePrivateOMPContextPromotionArtifactV2(t, attestationPath, fixture.attestationBytes)
		if _, _, err := readOMPContextPromotionArtifactPairV2(root); err == nil {
			t.Fatal("public file accepted")
		}
	})

	t.Run("symlink file", func(t *testing.T) {
		root := prepareOMPContextPromotionArtifactRootV2(t)
		target := filepath.Join(root, "target")
		writePrivateOMPContextPromotionArtifactV2(t, target, fixture.reportBytes)
		reportPath := filepath.Join(root, ompContextPromotionReportRelativePathV2)
		if err := os.Symlink(target, reportPath); err != nil {
			t.Fatal(err)
		}
		writePrivateOMPContextPromotionArtifactV2(t,
			filepath.Join(root, ompContextPromotionAttestationRelativePathV2), fixture.attestationBytes)
		if _, _, err := readOMPContextPromotionArtifactPairV2(root); err == nil {
			t.Fatal("symlink file accepted")
		}
	})

	t.Run("hard linked file", func(t *testing.T) {
		root := prepareOMPContextPromotionArtifactRootV2(t)
		writeOMPContextPromotionArtifactPairV2(t, root, fixture.reportBytes, fixture.attestationBytes)
		reportPath := filepath.Join(root, ompContextPromotionReportRelativePathV2)
		if err := os.Link(reportPath, filepath.Join(root, "report-alias")); err != nil {
			t.Fatal(err)
		}
		if _, _, err := readOMPContextPromotionArtifactPairV2(root); err == nil {
			t.Fatal("hard linked file accepted")
		}
	})

	t.Run("public runtime directory", func(t *testing.T) {
		root := prepareOMPContextPromotionArtifactRootV2(t)
		writeOMPContextPromotionArtifactPairV2(t, root, fixture.reportBytes, fixture.attestationBytes)
		if err := os.Chmod(filepath.Join(root, ".autopus", "runtime"), 0o755); err != nil {
			t.Fatal(err)
		}
		if _, _, err := readOMPContextPromotionArtifactPairV2(root); err == nil {
			t.Fatal("public runtime directory accepted")
		}
	})

	t.Run("symlink root", func(t *testing.T) {
		root := prepareOMPContextPromotionArtifactRootV2(t)
		link := filepath.Join(t.TempDir(), "workspace")
		if err := os.Symlink(root, link); err != nil {
			t.Fatal(err)
		}
		if _, _, err := readOMPContextPromotionArtifactPairV2(link); err == nil {
			t.Fatal("symlink root accepted")
		}
	})
}

func TestOMPContextPromotionArtifactLoaderV2_RejectsDirectoryAndFileSwapAfterRootAnchor(t *testing.T) {
	requireSecureOMPContextPromotionArtifactPlatformV2(t)
	fixture := newOMPContextPromotionV2Fixture(t)

	t.Run("directory swap", func(t *testing.T) {
		root := prepareOMPContextPromotionArtifactRootV2(t)
		writeOMPContextPromotionArtifactPairV2(t, root, fixture.reportBytes, fixture.attestationBytes)
		called := false
		_, _, err := readOMPContextPromotionArtifactPairV2WithHook(root, func(stage string) error {
			if stage != "before_autopus_open" || called {
				return nil
			}
			called = true
			original := filepath.Join(root, ".autopus")
			moved := filepath.Join(root, ".autopus-moved")
			if err := os.Rename(original, moved); err != nil {
				return err
			}
			return os.Symlink(moved, original)
		})
		if err == nil || !called {
			t.Fatal("directory swap was accepted")
		}
	})

	t.Run("file swap", func(t *testing.T) {
		root := prepareOMPContextPromotionArtifactRootV2(t)
		writeOMPContextPromotionArtifactPairV2(t, root, fixture.reportBytes, fixture.attestationBytes)
		called := false
		_, _, err := readOMPContextPromotionArtifactPairV2WithHook(root, func(stage string) error {
			if stage != "before_report_open" || called {
				return nil
			}
			called = true
			reportPath := filepath.Join(root, ompContextPromotionReportRelativePathV2)
			moved := filepath.Join(root, ".autopus", "runtime", "omp-context", "report-moved")
			if err := os.Rename(reportPath, moved); err != nil {
				return err
			}
			return os.Symlink(moved, reportPath)
		})
		if err == nil || !called {
			t.Fatal("file swap was accepted")
		}
	})
}

func TestOMPContextPromotionTrustV2_ContainsCommittedRotationKeys(t *testing.T) {
	keys := committedOMPContextPromotionPublicKeysV2()
	for _, keyID := range []string{
		OMPContextPromotionKeyID2026Q3K1,
		OMPContextPromotionKeyID2026Q3K2,
	} {
		if len(keys[keyID]) != ed25519.PublicKeySize {
			t.Fatalf("production trust root %q is unavailable: %#v", keyID, keys)
		}
		keys[keyID][0] ^= 0xff
		if keys[keyID][0] == committedOMPContextPromotionPublicKeysV2()[keyID][0] {
			t.Fatalf("trust root %q returned mutable backing storage", keyID)
		}
	}
	if len(keys) != 2 {
		t.Fatalf("unexpected production trust root: %#v", keys)
	}
}

func requireSecureOMPContextPromotionArtifactPlatformV2(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("secure artifact loading fails closed off Darwin and Linux")
	}
}

func prepareOMPContextPromotionArtifactRootV2(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
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

func writeOMPContextPromotionArtifactPairV2(t *testing.T, root string, report, attestation []byte) {
	t.Helper()
	writePrivateOMPContextPromotionArtifactV2(t, filepath.Join(root, ompContextPromotionReportRelativePathV2), report)
	writePrivateOMPContextPromotionArtifactV2(t, filepath.Join(root, ompContextPromotionAttestationRelativePathV2), attestation)
}

func writePrivateOMPContextPromotionArtifactV2(t *testing.T, path string, body []byte) {
	t.Helper()
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
}
