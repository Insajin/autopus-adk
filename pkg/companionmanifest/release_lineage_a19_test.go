package companionmanifest

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
)

const (
	publicKeyReceiptA19Tag     = "v0.50.90"
	publicKeyReceiptA19Version = "0.50.90"
)

var immutableA18LineagePins = map[string]string{
	"A18_RELEASE_ID":                 "359747783",
	"A18_COMMIT_SHA":                 "76f35d990e76511d169e239547d33bfedcea7948",
	"A18_TREE_SHA":                   "5e90442a2b6dec38ec0072117d24b1d5ecb1f1e1",
	"A18_TAG_OBJECT_SHA":             "ba18d42068693111ed78b4bc1ccc4891d0663334",
	"A18_CHECKSUMS_SHA256":           "027f4b6a1b7412f19aafa386c18b9b551096e99a3dfdd44e2ad7b550952eab30",
	"A18_AMD64_ARCHIVE_SHA256":       "4101bc35739f6dd428bdafe9e450624430690687dc0635e4e7dc97b62dd344a6",
	"A18_ARM64_ARCHIVE_SHA256":       "d89d77a8618136643f26c3920f0c5d8aa4f87d5c48f2e5d555f3caf072fc375c",
	"A18_LINUX_AMD64_ARCHIVE_SHA256": "79f54ed935a204c6f5a89a93ba8945e8ffe15557080a1dc25eac437a16398782",
	"A18_LINUX_ARM64_ARCHIVE_SHA256": "aa4834873453266c853213d940cde4b672f6c00de9672718fcb8004582076b5f",
	"A18_AMD64_MANIFEST_SHA256":      "71ef53e32e6e9fbfd1c03dd130412742f854bebb7958978ca8e2345e918b0a1e",
	"A18_ARM64_MANIFEST_SHA256":      "a554b5a75cfd5c5f04ed9c06a6a2e1ba49220c2025e0149b030ed89a68757c56",
}

func TestReleasePublicKeyReceipt_A19PolicyPinsExactCoordinate(t *testing.T) {
	scripts := normalizedReleaseText(releaseScriptsText(t))
	if !exactA19TagVersionGuard(scripts) {
		t.Fatal("A19 release is not conjunctively restricted to tag v0.50.90 and version 0.50.90")
	}
	for _, required := range []string{
		"release_phase='A19'", "prior_phase='A18'",
		`prior_release_id="$A18_RELEASE_ID"`, `prior_tree="$A18_TREE_SHA"`,
		`prior_linux_amd64_archive="$A18_LINUX_AMD64_ARCHIVE_SHA256"`,
		`prior_linux_arm64_archive="$A18_LINUX_ARM64_ARCHIVE_SHA256"`,
		"verify-public-key-lineage-assets.sh", `'.commit.tree.sha'`, `'.id'`,
	} {
		if !strings.Contains(scripts, required) {
			t.Fatalf("A19 exact A18 predecessor contract missing %q", required)
		}
	}
}

func TestReleasePublicKeyReceipt_GoReleaserA19FixtureProducesPlatformArtifacts(t *testing.T) {
	tools := newExecutableLineageTools(t)
	evidence := produceGoReleaserFixtureEvidence(
		t, tools, publicKeyReceiptA19Tag, publicKeyReceiptA19Version, true,
	)
	for _, target := range executableLineageArchiveTargets {
		entries, err := decodeLineageArchiveFile(evidence.archives[target.key])
		if err != nil {
			t.Fatalf("decode A19 %s archive: %v", target.key, err)
		}
		if target.platform == "darwin" {
			manifest := entries["adk-companion-manifest.json"].data
			if !bytes.Contains(manifest, []byte(`"version":"0.50.90"`)) {
				t.Fatalf("A19 %s manifest does not carry the current version", target.key)
			}
			for _, name := range darwinLineageArchiveEvidenceNames {
				if len(entries[name].data) == 0 {
					t.Fatalf("A19 %s archive is missing %s", target.key, name)
				}
			}
			continue
		}
		for _, name := range darwinLineageArchiveEvidenceNames {
			if _, exists := entries[name]; exists {
				t.Fatalf("A19 %s Linux archive unexpectedly contains %s", target.key, name)
			}
		}
	}
}

func TestReleasePublicKeyReceipt_GoReleaserA18FixtureSealsA19DirectPredecessor(t *testing.T) {
	tools := newExecutableLineageTools(t)
	evidence := produceGoReleaserFixtureEvidence(
		t, tools, publicKeyReceiptA18Tag, publicKeyReceiptA18Version, true,
	)
	t.Run("normal", func(t *testing.T) {
		fixture := newExecutableLineageFixture(t, tools, evidence)
		output, err := fixture.run(t)
		if err != nil {
			t.Fatalf("valid A18 to A19 lineage failed: %v\n%s", err, output)
		}
		if !strings.Contains(output, "A19 exact A18 key record verified") {
			t.Fatalf("valid A19 lineage diagnostic = %q", output)
		}
	})

	cases := []struct {
		name   string
		code   string
		tamper func(*testing.T, *executableLineageFixture)
	}{
		{name: "release_not_immutable", code: "A18 release is not immutable and final", tamper: tamperLineageReleaseImmutable},
		{name: "release_id", code: "A18 release ID differs from its pin", tamper: tamperA19ReleaseID},
		{name: "annotated_tag_object", code: "A18 annotated tag object differs from its pin", tamper: tamperA19TagObject},
		{name: "source_tree", code: "A18 source tree differs from its pin", tamper: tamperA19TreePin},
		{name: "archive_api_digest", code: "server digest differs", tamper: tamperA19ArchiveAPIDigest},
		{name: "darwin_archive_bytes", code: "prior_archive_digest_mismatch", tamper: tamperLineageArchiveBytes},
		{name: "darwin_archive_pin", code: "prior_archive_digest_mismatch", tamper: tamperA19DarwinArchivePin},
		{name: "linux_archive_bytes", code: "prior_archive_digest_mismatch", tamper: tamperA19LinuxArchiveBytes},
		{name: "linux_archive_pin", code: "prior_archive_digest_mismatch", tamper: tamperA19LinuxArchivePin},
		{name: "checksums_pin", code: "prior_checksums_bytes_mismatch", tamper: tamperLineageChecksums},
		{name: "manifest_pin", code: "prior_manifest_digest_mismatch", tamper: tamperA19ManifestPin},
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
				t.Fatalf("tampered A19 lineage passed\n%s", output)
			}
			if !strings.Contains(output, test.code) {
				t.Fatalf("A19 tamper diagnostic = %q, want %s", output, test.code)
			}
			secret := base64.StdEncoding.EncodeToString(fixture.privateKey)
			if strings.Contains(output, secret) || strings.Contains(output, fixture.token) {
				t.Fatal("A19 lineage diagnostic disclosed test credential material")
			}
		})
	}
}

func tamperA19ReleaseID(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.releaseID = "2"
}

func tamperA19TagObject(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.tagObject = differentHex(fixture.pins.tagObject, 40)
}

func tamperA19TreePin(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.tree = differentHex(fixture.pins.tree, 40)
}

func tamperA19ArchiveAPIDigest(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.assetDigestOverride = "sha256:" + differentHex(fixture.pins.amd64Archive, 64)
	fixture.writeEvidence(t)
}

func tamperA19DarwinArchivePin(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.amd64Archive = differentHex(fixture.pins.amd64Archive, 64)
}

func tamperA19LinuxArchivePin(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.linuxAMD64Archive = differentHex(fixture.pins.linuxAMD64Archive, 64)
}

func tamperA19LinuxArchiveBytes(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.archiveMutation = &lineageArchiveMutation{
		architecture: "linux_amd64", entry: "README.md",
		mutate: func(_ *testing.T, data []byte) ([]byte, bool) {
			return append(data, '\n'), true
		},
	}
	fixture.writeEvidence(t)
}

func tamperA19ManifestPin(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.arm64Manifest = differentHex(fixture.pins.arm64Manifest, 64)
}

func exactA19TagVersionGuard(source string) bool {
	return exactLineageTagVersionGuard(source, "90")
}
