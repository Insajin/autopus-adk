package companionmanifest

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
)

const (
	publicKeyReceiptA20Tag     = "v0.50.91"
	publicKeyReceiptA20Version = "0.50.91"
)

var immutableA19LineagePins = map[string]string{
	"A19_RELEASE_ID":                 "359989008",
	"A19_COMMIT_SHA":                 "5bc41dccc72f8244943fd9e862cba07a36bf09d3",
	"A19_TREE_SHA":                   "d7d26e5f66ac5d01556fb64eadd07407e00b5035",
	"A19_TAG_OBJECT_SHA":             "d4e2a495d194da97594e3eb6504f9ad3533b0dd8",
	"A19_CHECKSUMS_SHA256":           "3ac2b6563fd9800beed1cf682ef8d0af4a246041e31bed9eec4c62431ab269e4",
	"A19_AMD64_ARCHIVE_SHA256":       "8fe1e6b04ee107ebb3af2de1e0b3a272f42ab352f15e62ef1a2ab9e0b067c630",
	"A19_ARM64_ARCHIVE_SHA256":       "60c4fe79c83a4b58482808a6ea9937389b9fa5c27015521df76705db98fd42b0",
	"A19_LINUX_AMD64_ARCHIVE_SHA256": "34582ef39c7b54fb9327e295e6cbb50107e3689120cf74f75b1f21a01eecc1dc",
	"A19_LINUX_ARM64_ARCHIVE_SHA256": "11a00378365069b97804d6af40435645f26271550d1ab58e7776346ec7d4a879",
	"A19_AMD64_MANIFEST_SHA256":      "c5383760f8d914870c9a5db4d00116beeda68f03be0a0967b35e2b6ec2823c55",
	"A19_ARM64_MANIFEST_SHA256":      "41996a1a4b5c550cac02ac10f1e426a38f24555a0968fbc85807fad500f8dd9e",
}

func TestReleasePublicKeyReceipt_A20PolicyPinsExactCoordinate(t *testing.T) {
	scripts := normalizedReleaseText(releaseScriptsText(t))
	if !exactA20TagVersionGuard(scripts) {
		t.Fatal("A20 release is not conjunctively restricted to tag v0.50.91 and version 0.50.91")
	}
	for _, required := range []string{
		"release_phase='A20'", "prior_phase='A19'",
		`prior_release_id="$A19_RELEASE_ID"`, `prior_tree="$A19_TREE_SHA"`,
		`prior_linux_amd64_archive="$A19_LINUX_AMD64_ARCHIVE_SHA256"`,
		`prior_linux_arm64_archive="$A19_LINUX_ARM64_ARCHIVE_SHA256"`,
		"verify-public-key-lineage-assets.sh", `'.commit.tree.sha'`, `'.id'`,
	} {
		if !strings.Contains(scripts, required) {
			t.Fatalf("A20 exact A19 predecessor contract missing %q", required)
		}
	}
}

func TestReleasePublicKeyReceipt_GoReleaserA20FixtureProducesPlatformArtifacts(t *testing.T) {
	tools := newExecutableLineageTools(t)
	evidence := produceGoReleaserFixtureEvidence(
		t, tools, publicKeyReceiptA20Tag, publicKeyReceiptA20Version, true,
	)
	for _, target := range executableLineageArchiveTargets {
		entries, err := decodeLineageArchiveFile(evidence.archives[target.key])
		if err != nil {
			t.Fatalf("decode A20 %s archive: %v", target.key, err)
		}
		if target.platform == "darwin" {
			manifest := entries["adk-companion-manifest.json"].data
			if !bytes.Contains(manifest, []byte(`"version":"0.50.91"`)) {
				t.Fatalf("A20 %s manifest does not carry the current version", target.key)
			}
			for _, name := range darwinLineageArchiveEvidenceNames {
				if len(entries[name].data) == 0 {
					t.Fatalf("A20 %s archive is missing %s", target.key, name)
				}
			}
			continue
		}
		for _, name := range darwinLineageArchiveEvidenceNames {
			if _, exists := entries[name]; exists {
				t.Fatalf("A20 %s Linux archive unexpectedly contains %s", target.key, name)
			}
		}
	}
}

func TestReleasePublicKeyReceipt_GoReleaserA19FixtureSealsA20DirectPredecessor(t *testing.T) {
	tools := newExecutableLineageTools(t)
	evidence := produceGoReleaserFixtureEvidence(
		t, tools, publicKeyReceiptA19Tag, publicKeyReceiptA19Version, true,
	)
	t.Run("normal", func(t *testing.T) {
		fixture := newExecutableLineageFixture(t, tools, evidence)
		output, err := fixture.run(t)
		if err != nil {
			t.Fatalf("valid A19 to A20 lineage failed: %v\n%s", err, output)
		}
		if !strings.Contains(output, "A20 exact A19 key record verified") {
			t.Fatalf("valid A20 lineage diagnostic = %q", output)
		}
	})

	cases := []struct {
		name   string
		code   string
		tamper func(*testing.T, *executableLineageFixture)
	}{
		{name: "release_not_immutable", code: "A19 release is not immutable and final", tamper: tamperLineageReleaseImmutable},
		{name: "release_id", code: "A19 release ID differs from its pin", tamper: tamperA20ReleaseID},
		{name: "annotated_tag_object", code: "A19 annotated tag object differs from its pin", tamper: tamperA20TagObject},
		{name: "source_tree", code: "A19 source tree differs from its pin", tamper: tamperA20TreePin},
		{name: "archive_api_digest", code: "server digest differs", tamper: tamperA20ArchiveAPIDigest},
		{name: "darwin_archive_bytes", code: "prior_archive_digest_mismatch", tamper: tamperLineageArchiveBytes},
		{name: "darwin_archive_pin", code: "prior_archive_digest_mismatch", tamper: tamperA20DarwinArchivePin},
		{name: "linux_archive_bytes", code: "prior_archive_digest_mismatch", tamper: tamperA20LinuxArchiveBytes},
		{name: "linux_archive_pin", code: "prior_archive_digest_mismatch", tamper: tamperA20LinuxArchivePin},
		{name: "checksums_pin", code: "prior_checksums_bytes_mismatch", tamper: tamperLineageChecksums},
		{name: "manifest_pin", code: "prior_manifest_digest_mismatch", tamper: tamperA20ManifestPin},
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
				t.Fatalf("tampered A20 lineage passed\n%s", output)
			}
			if !strings.Contains(output, test.code) {
				t.Fatalf("A20 tamper diagnostic = %q, want %s", output, test.code)
			}
			secret := base64.StdEncoding.EncodeToString(fixture.privateKey)
			if strings.Contains(output, secret) || strings.Contains(output, fixture.token) {
				t.Fatal("A20 lineage diagnostic disclosed test credential material")
			}
		})
	}
}

func tamperA20ReleaseID(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.releaseID = "2"
}

func tamperA20TagObject(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.tagObject = differentHex(fixture.pins.tagObject, 40)
}

func tamperA20TreePin(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.tree = differentHex(fixture.pins.tree, 40)
}

func tamperA20ArchiveAPIDigest(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.assetDigestOverride = "sha256:" + differentHex(fixture.pins.amd64Archive, 64)
	fixture.writeEvidence(t)
}

func tamperA20DarwinArchivePin(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.amd64Archive = differentHex(fixture.pins.amd64Archive, 64)
}

func tamperA20LinuxArchivePin(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.linuxAMD64Archive = differentHex(fixture.pins.linuxAMD64Archive, 64)
}

func tamperA20LinuxArchiveBytes(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.archiveMutation = &lineageArchiveMutation{
		architecture: "linux_amd64", entry: "README.md",
		mutate: func(_ *testing.T, data []byte) ([]byte, bool) {
			return append(data, '\n'), true
		},
	}
	fixture.writeEvidence(t)
}

func tamperA20ManifestPin(t *testing.T, fixture *executableLineageFixture) {
	t.Helper()
	fixture.pins.arm64Manifest = differentHex(fixture.pins.arm64Manifest, 64)
}

func exactA20TagVersionGuard(source string) bool {
	return exactLineageTagVersionGuard(source, "91")
}
