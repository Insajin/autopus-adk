package companionmanifest

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
)

const (
	publicKeyReceiptA17Tag     = "v0.50.88"
	publicKeyReceiptA17Version = "0.50.88"
)

var immutableA16LineagePins = map[string]string{
	"A16_COMMIT_SHA":                 "3e02c622af97f74873325ec65940c580e23c580a",
	"A16_TREE_SHA":                   "f041a63af99e2ba82bd6fb573272489fbd101e86",
	"A16_TAG_OBJECT_SHA":             "9fad744950e268fa500812cad0194fa5da47e369",
	"A16_CHECKSUMS_SHA256":           "7c19613a7f441264f90fb6b0c6e975c78cae3db250f2ed279fa44418a022dc7f",
	"A16_AMD64_ARCHIVE_SHA256":       "cc411eb9cc04476d280d272f0e52b7c1a40fad923439180b526a46c38be1b63e",
	"A16_ARM64_ARCHIVE_SHA256":       "b0880ef40f3089168be234a540cfaf1795e0e54168a4ead3f92275a13eb63012",
	"A16_LINUX_AMD64_ARCHIVE_SHA256": "8da3ec03967fa1b5911708716239bcaa9d0843069e65836f280f986b4cdd1aaa",
	"A16_LINUX_ARM64_ARCHIVE_SHA256": "9aeca632be6de54d3540e03ad99ba5dc520f3665f7f592bd12b689b844ea8bf3",
	"A16_AMD64_MANIFEST_SHA256":      "93943fafac83eb3090f10f8c48489a96b80d003c5c9f00dfaa2cd59eabf7be42",
	"A16_ARM64_MANIFEST_SHA256":      "4cc59ac1a3194df80a050e4159a6919a01e2bcc69efc575907aedf16aacc4057",
}

func TestReleasePublicKeyReceipt_A17PolicyPinsExactCoordinate(t *testing.T) {
	scripts := normalizedReleaseText(releaseScriptsText(t))
	if !exactA17TagVersionGuard(scripts) {
		t.Fatal("A17 release is not conjunctively restricted to tag v0.50.88 and version 0.50.88")
	}
	for _, required := range []string{
		"release_phase='A17'", "prior_phase='A16'", `prior_tree="$A16_TREE_SHA"`,
		`prior_linux_amd64_archive="$A16_LINUX_AMD64_ARCHIVE_SHA256"`,
		`prior_linux_arm64_archive="$A16_LINUX_ARM64_ARCHIVE_SHA256"`,
		"verify-public-key-lineage-assets.sh", `'.commit.tree.sha'`,
	} {
		if !strings.Contains(scripts, required) {
			t.Fatalf("A17 exact A16 predecessor contract missing %q", required)
		}
	}
}

func TestReleasePublicKeyReceipt_GoReleaserA17FixtureProducesPlatformArtifacts(t *testing.T) {
	tools := newExecutableLineageTools(t)
	evidence := produceGoReleaserFixtureEvidence(
		t, tools, publicKeyReceiptA17Tag, publicKeyReceiptA17Version, true,
	)
	for _, target := range executableLineageArchiveTargets {
		entries, err := decodeLineageArchiveFile(evidence.archives[target.key])
		if err != nil {
			t.Fatalf("decode A17 %s archive: %v", target.key, err)
		}
		if target.platform == "darwin" {
			manifest := entries["adk-companion-manifest.json"].data
			if !bytes.Contains(manifest, []byte(`"version":"0.50.88"`)) {
				t.Fatalf("A17 %s manifest does not carry the current version", target.key)
			}
			for _, name := range darwinLineageArchiveEvidenceNames {
				if len(entries[name].data) == 0 {
					t.Fatalf("A17 %s archive is missing %s", target.key, name)
				}
			}
			continue
		}
		for _, name := range darwinLineageArchiveEvidenceNames {
			if _, exists := entries[name]; exists {
				t.Fatalf("A17 %s Linux archive unexpectedly contains %s", target.key, name)
			}
		}
	}
}

func TestReleasePublicKeyReceipt_GoReleaserA16FixtureSealsA17DirectPredecessor(t *testing.T) {
	tools := newExecutableLineageTools(t)
	evidence := produceGoReleaserFixtureEvidence(
		t, tools, publicKeyReceiptA16Tag, publicKeyReceiptA16Version, true,
	)
	t.Run("normal", func(t *testing.T) {
		fixture := newExecutableLineageFixture(t, tools, evidence)
		output, err := fixture.run(t)
		if err != nil {
			t.Fatalf("valid A16 to A17 lineage failed: %v\n%s", err, output)
		}
		if !strings.Contains(output, "A17 exact A16 key record verified") {
			t.Fatalf("valid A17 lineage diagnostic = %q", output)
		}
	})

	cases := []struct {
		name   string
		code   string
		tamper func(*testing.T, *executableLineageFixture)
	}{
		{name: "release_not_immutable", code: "A16 release is not immutable and final", tamper: tamperLineageReleaseImmutable},
		{name: "annotated_tag_object", code: "A16 annotated tag object differs from its pin", tamper: tamperA17TagObject},
		{name: "source_tree", code: "A16 source tree differs from its pin", tamper: tamperA17TreePin},
		{name: "archive_api_digest", code: "server digest differs", tamper: tamperA17ArchiveAPIDigest},
		{name: "darwin_archive_bytes", code: "prior_archive_digest_mismatch", tamper: tamperLineageArchiveBytes},
		{name: "darwin_archive_pin", code: "prior_archive_digest_mismatch", tamper: tamperA17DarwinArchivePin},
		{name: "linux_archive_bytes", code: "prior_archive_digest_mismatch", tamper: tamperA17LinuxArchiveBytes},
		{name: "linux_archive_pin", code: "prior_archive_digest_mismatch", tamper: tamperA17LinuxArchivePin},
		{name: "checksums_pin", code: "prior_checksums_bytes_mismatch", tamper: tamperLineageChecksums},
		{name: "manifest_pin", code: "prior_manifest_digest_mismatch", tamper: tamperA17ManifestPin},
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
				t.Fatalf("tampered A17 lineage passed\n%s", output)
			}
			if !strings.Contains(output, test.code) {
				t.Fatalf("A17 tamper diagnostic = %q, want %s", output, test.code)
			}
			secret := base64.StdEncoding.EncodeToString(fixture.privateKey)
			if strings.Contains(output, secret) || strings.Contains(output, fixture.token) {
				t.Fatal("A17 lineage diagnostic disclosed test credential material")
			}
		})
	}
}

func tamperA17TagObject(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.tagObject = differentHex(fixture.pins.tagObject, 40)
}

func tamperA17TreePin(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.tree = differentHex(fixture.pins.tree, 40)
}

func tamperA17ArchiveAPIDigest(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.assetDigestOverride = "sha256:" + differentHex(fixture.pins.amd64Archive, 64)
	fixture.writeEvidence(t)
}

func tamperA17DarwinArchivePin(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.amd64Archive = differentHex(fixture.pins.amd64Archive, 64)
}

func tamperA17LinuxArchivePin(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.linuxAMD64Archive = differentHex(fixture.pins.linuxAMD64Archive, 64)
}

func tamperA17LinuxArchiveBytes(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.archiveMutation = &lineageArchiveMutation{
		architecture: "linux_amd64", entry: "README.md",
		mutate: func(_ *testing.T, data []byte) ([]byte, bool) {
			return append(data, '\n'), true
		},
	}
	fixture.writeEvidence(t)
}

func tamperA17ManifestPin(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.arm64Manifest = differentHex(fixture.pins.arm64Manifest, 64)
}

func exactA17TagVersionGuard(source string) bool {
	return exactLineageTagVersionGuard(source, "88")
}
