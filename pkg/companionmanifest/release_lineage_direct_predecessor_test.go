package companionmanifest

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
	if prefix == "A7" || prefix == "A8" || prefix == "A9" || prefix == "A10" {
		replacements[prefix+"_TREE_SHA"] = fixture.pins.tree
	}
	return replacements
}

func immutableProductionLineagePin(name string) (string, bool) {
	for _, pins := range []map[string]string{
		immutableA0LineagePins, immutableA1LineagePins, immutableA2LineagePins,
		immutableA3LineagePins, immutableA4LineagePins, immutableA5LineagePins,
		immutableA6LineagePins, immutableA7LineagePins, immutableA8LineagePins,
		immutableA9LineagePins, immutableA10LineagePins,
	} {
		if value, ok := pins[name]; ok {
			return value, true
		}
	}
	return "", false
}
