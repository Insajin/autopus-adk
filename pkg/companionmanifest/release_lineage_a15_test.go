package companionmanifest

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
)

const (
	publicKeyReceiptA15Tag     = "v0.50.86"
	publicKeyReceiptA15Version = "0.50.86"
)

var immutableA14LineagePins = map[string]string{
	"A14_COMMIT_SHA":                 "4b8eb62200d253b46e022670c482e2f716a992a3",
	"A14_TREE_SHA":                   "fbdc83287982899c3d6bfe5fdf7b88494e76bcb0",
	"A14_TAG_OBJECT_SHA":             "f005dd935dbbcec8c60052adcfda6632fe8831e1",
	"A14_CHECKSUMS_SHA256":           "5bd11e327eab31c555f89298761e2d27bca2fadebfc3b7961cafb6a140539236",
	"A14_AMD64_ARCHIVE_SHA256":       "66834d509309cb09b84f78bb81a97e68a8d03434c9a37f239a2ae04677dbc10b",
	"A14_ARM64_ARCHIVE_SHA256":       "7fe10bc7b03b3df44f803622e3830e5e91f3ea12b47b706cf14f716b076b012e",
	"A14_LINUX_AMD64_ARCHIVE_SHA256": "187620011ce035f6bdb09f3f6d5b005f878463c3ba0fd805142cbd3e4f587698",
	"A14_LINUX_ARM64_ARCHIVE_SHA256": "654e42612a3f1ee670157cd461b3dff1270f2102b085984951975c0284356172",
	"A14_AMD64_MANIFEST_SHA256":      "4265d3f18c7aaab779a720216c2f1dfc9a486c01be898290d4f56be31102008e",
	"A14_ARM64_MANIFEST_SHA256":      "918c91d4bdee0c58e74e0068314d35463e094fef214986a550579bca08b2ef38",
}

func TestReleasePublicKeyReceipt_A15PolicyPinsExactCoordinate(t *testing.T) {
	scripts := normalizedReleaseText(releaseScriptsText(t))
	if !exactA15TagVersionGuard(scripts) {
		t.Fatal("A15 release is not conjunctively restricted to tag v0.50.86 and version 0.50.86")
	}
	for _, required := range []string{
		"release_phase='A15'", "prior_phase='A14'", `prior_tree="$A14_TREE_SHA"`,
		`prior_linux_amd64_archive="$A14_LINUX_AMD64_ARCHIVE_SHA256"`,
		`prior_linux_arm64_archive="$A14_LINUX_ARM64_ARCHIVE_SHA256"`,
		"verify-public-key-lineage-assets.sh", `'.commit.tree.sha'`,
	} {
		if !strings.Contains(scripts, required) {
			t.Fatalf("A15 exact A14 predecessor contract missing %q", required)
		}
	}
}

func TestReleasePublicKeyReceipt_GoReleaserA15FixtureProducesPlatformArtifacts(t *testing.T) {
	tools := newExecutableLineageTools(t)
	evidence := produceGoReleaserFixtureEvidence(
		t, tools, publicKeyReceiptA15Tag, publicKeyReceiptA15Version, true,
	)
	for _, target := range executableLineageArchiveTargets {
		entries, err := decodeLineageArchiveFile(evidence.archives[target.key])
		if err != nil {
			t.Fatalf("decode A15 %s archive: %v", target.key, err)
		}
		if target.platform == "darwin" {
			manifest := entries["adk-companion-manifest.json"].data
			if !bytes.Contains(manifest, []byte(`"version":"0.50.86"`)) {
				t.Fatalf("A15 %s manifest does not carry the current version", target.key)
			}
			for _, name := range darwinLineageArchiveEvidenceNames {
				if len(entries[name].data) == 0 {
					t.Fatalf("A15 %s archive is missing %s", target.key, name)
				}
			}
			continue
		}
		for _, name := range darwinLineageArchiveEvidenceNames {
			if _, exists := entries[name]; exists {
				t.Fatalf("A15 %s Linux archive unexpectedly contains %s", target.key, name)
			}
		}
	}
}

func TestReleasePublicKeyReceipt_GoReleaserA14FixtureSealsA15DirectPredecessor(t *testing.T) {
	tools := newExecutableLineageTools(t)
	evidence := produceGoReleaserFixtureEvidence(
		t, tools, publicKeyReceiptA14Tag, publicKeyReceiptA14Version, true,
	)
	t.Run("normal", func(t *testing.T) {
		fixture := newExecutableLineageFixture(t, tools, evidence)
		output, err := fixture.run(t)
		if err != nil {
			t.Fatalf("valid A14 to A15 lineage failed: %v\n%s", err, output)
		}
		if !strings.Contains(output, "A15 exact A14 key record verified") {
			t.Fatalf("valid A15 lineage diagnostic = %q", output)
		}
	})

	cases := []struct {
		name   string
		code   string
		tamper func(*testing.T, *executableLineageFixture)
	}{
		{name: "release_not_immutable", code: "A14 release is not immutable and final", tamper: tamperLineageReleaseImmutable},
		{name: "annotated_tag_object", code: "A14 annotated tag object differs from its pin", tamper: tamperA15TagObject},
		{name: "source_tree", code: "A14 source tree differs from its pin", tamper: tamperA15TreePin},
		{name: "archive_api_digest", code: "server digest differs", tamper: tamperA15ArchiveAPIDigest},
		{name: "darwin_archive_bytes", code: "prior_archive_digest_mismatch", tamper: tamperLineageArchiveBytes},
		{name: "darwin_archive_pin", code: "prior_archive_digest_mismatch", tamper: tamperA15DarwinArchivePin},
		{name: "linux_archive_bytes", code: "prior_archive_digest_mismatch", tamper: tamperA15LinuxArchiveBytes},
		{name: "linux_archive_pin", code: "prior_archive_digest_mismatch", tamper: tamperA15LinuxArchivePin},
		{name: "checksums_pin", code: "prior_checksums_bytes_mismatch", tamper: tamperLineageChecksums},
		{name: "manifest_pin", code: "prior_manifest_digest_mismatch", tamper: tamperA15ManifestPin},
		{name: "receipt_continuity", code: "prior_receipt_bytes_mismatch", tamper: tamperLineageReceiptPin},
		{name: "signature_continuity", code: "prior_signature_bytes_mismatch", tamper: tamperLineageSignaturePin},
		{name: "record_continuity", code: "prior_record_digest_mismatch", tamper: tamperLineageRecordPin},
		{name: "commit_pin", code: "prior_release_identity_mismatch", tamper: tamperLineageCommitPin},
		{name: "tag_commit", code: "prior_release_identity_mismatch", tamper: tamperLineageTagCommit},
		{name: "target_commit", code: "prior_release_identity_mismatch", tamper: tamperLineageTargetCommit},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			fixture := newExecutableLineageFixture(t, tools, evidence)
			test.tamper(t, fixture)
			output, err := fixture.run(t)
			if err == nil {
				t.Fatalf("tampered A15 lineage passed\n%s", output)
			}
			if !strings.Contains(output, test.code) {
				t.Fatalf("A15 tamper diagnostic = %q, want %s", output, test.code)
			}
			secret := base64.StdEncoding.EncodeToString(fixture.privateKey)
			if strings.Contains(output, secret) || strings.Contains(output, fixture.token) {
				t.Fatal("A15 lineage diagnostic disclosed test credential material")
			}
		})
	}
}

func tamperA15TagObject(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.tagObject = differentHex(fixture.pins.tagObject, 40)
}

func tamperA15TreePin(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.tree = differentHex(fixture.pins.tree, 40)
}

func tamperA15ArchiveAPIDigest(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.assetDigestOverride = "sha256:" + differentHex(fixture.pins.amd64Archive, 64)
	fixture.writeEvidence(t)
}

func tamperA15DarwinArchivePin(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.amd64Archive = differentHex(fixture.pins.amd64Archive, 64)
}

func tamperA15LinuxArchivePin(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.linuxAMD64Archive = differentHex(fixture.pins.linuxAMD64Archive, 64)
}

func tamperA15LinuxArchiveBytes(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.archiveMutation = &lineageArchiveMutation{
		architecture: "linux_amd64", entry: "README.md",
		mutate: func(_ *testing.T, data []byte) ([]byte, bool) {
			return append(data, '\n'), true
		},
	}
	fixture.writeEvidence(t)
}

func tamperA15ManifestPin(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.arm64Manifest = differentHex(fixture.pins.arm64Manifest, 64)
}

func exactA15TagVersionGuard(source string) bool {
	return exactLineageTagVersionGuard(source, "86")
}
