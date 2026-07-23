package companionmanifest

import (
	"strings"
	"testing"
)

func directPredecessorPinReplacements(fixture *executableLineageFixture) map[string]string {
	prefix := ""
	switch fixture.currentTag {
	case publicKeyReceiptA2Tag:
		prefix = "A1"
	case publicKeyReceiptA3Tag:
		prefix = "A2"
	case publicKeyReceiptA4Tag:
		prefix = "A3"
	case publicKeyReceiptA5Tag:
		prefix = "A4"
	case publicKeyReceiptA6Tag:
		prefix = "A5"
	case publicKeyReceiptA7Tag:
		prefix = "A6"
	case publicKeyReceiptA8Tag:
		prefix = "A7"
	case publicKeyReceiptA9Tag:
		prefix = "A8"
	case publicKeyReceiptA10Tag:
		prefix = "A9"
	case publicKeyReceiptA11Tag:
		prefix = "A10"
	case publicKeyReceiptA12Tag:
		prefix = "A11"
	case publicKeyReceiptA13Tag:
		prefix = "A12"
	case publicKeyReceiptA14Tag:
		prefix = "A13"
	case publicKeyReceiptA15Tag:
		prefix = "A14"
	default:
		return nil
	}
	replacements := map[string]string{
		prefix + "_COMMIT_SHA":            fixture.pins.commit,
		prefix + "_TAG_OBJECT_SHA":        fixture.pins.tagObject,
		prefix + "_CHECKSUMS_SHA256":      fixture.pins.checksums,
		prefix + "_AMD64_ARCHIVE_SHA256":  fixture.pins.amd64Archive,
		prefix + "_ARM64_ARCHIVE_SHA256":  fixture.pins.arm64Archive,
		prefix + "_AMD64_MANIFEST_SHA256": fixture.pins.amd64Manifest,
		prefix + "_ARM64_MANIFEST_SHA256": fixture.pins.arm64Manifest,
	}
	if prefix == "A14" {
		replacements[prefix+"_LINUX_AMD64_ARCHIVE_SHA256"] = fixture.pins.linuxAMD64Archive
		replacements[prefix+"_LINUX_ARM64_ARCHIVE_SHA256"] = fixture.pins.linuxARM64Archive
	}
	if prefix == "A7" || prefix == "A8" || prefix == "A9" || prefix == "A10" || prefix == "A11" || prefix == "A12" || prefix == "A13" || prefix == "A14" {
		replacements[prefix+"_TREE_SHA"] = fixture.pins.tree
	}
	return replacements
}

func immutableProductionLineagePin(name string) (string, bool) {
	for _, pins := range []map[string]string{
		immutableA0LineagePins, immutableA1LineagePins, immutableA2LineagePins,
		immutableA3LineagePins, immutableA4LineagePins, immutableA5LineagePins,
		immutableA6LineagePins, immutableA7LineagePins, immutableA8LineagePins,
		immutableA9LineagePins, immutableA10LineagePins, immutableA11LineagePins,
		immutableA12LineagePins, immutableA13LineagePins, immutableA14LineagePins,
	} {
		if value, ok := pins[name]; ok {
			return value, true
		}
	}
	return "", false
}

func TestDirectPredecessorPinReplacements_LinuxPinsBeginAtA14(t *testing.T) {
	pins := executableLineagePins{
		linuxAMD64Archive: "linux-amd64", linuxARM64Archive: "linux-arm64",
	}
	for _, tag := range []string{
		publicKeyReceiptA2Tag, publicKeyReceiptA3Tag, publicKeyReceiptA4Tag,
		publicKeyReceiptA5Tag, publicKeyReceiptA6Tag, publicKeyReceiptA7Tag,
		publicKeyReceiptA8Tag, publicKeyReceiptA9Tag, publicKeyReceiptA10Tag,
		publicKeyReceiptA11Tag, publicKeyReceiptA12Tag, publicKeyReceiptA13Tag,
		publicKeyReceiptA14Tag,
	} {
		replacements := directPredecessorPinReplacements(&executableLineageFixture{
			currentTag: tag, pins: pins,
		})
		for name := range replacements {
			if strings.Contains(name, "LINUX") {
				t.Fatalf("legacy predecessor %s unexpectedly replaces %s", tag, name)
			}
		}
	}
	replacements := directPredecessorPinReplacements(&executableLineageFixture{
		currentTag: publicKeyReceiptA15Tag, pins: pins,
	})
	if replacements["A14_LINUX_AMD64_ARCHIVE_SHA256"] != "linux-amd64" ||
		replacements["A14_LINUX_ARM64_ARCHIVE_SHA256"] != "linux-arm64" {
		t.Fatalf("A15 direct predecessor Linux replacements = %#v", replacements)
	}
}
