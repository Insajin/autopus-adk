package selfupdate

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestReleaseInfo_Fields verifies that the ReleaseInfo struct has the expected
// fields and they can be set and read correctly.
func TestReleaseInfo_Fields(t *testing.T) {
	t.Parallel()

	// Given/When: a ReleaseInfo is constructed with all fields
	info := ReleaseInfo{
		TagName:     "v0.7.0",
		ArchiveURL:  "https://example.com/autopus-adk_0.7.0_darwin_arm64.tar.gz",
		ChecksumURL: "https://example.com/checksums.txt",
		ArchiveName: "autopus-adk_0.7.0_darwin_arm64.tar.gz",
	}

	// Then: all fields are accessible and correct
	assert.Equal(t, "v0.7.0", info.TagName)
	assert.NotEmpty(t, info.ArchiveURL)
	assert.NotEmpty(t, info.ChecksumURL)
	assert.NotEmpty(t, info.ArchiveName)
}

// TestUpdateResult_Fields verifies that the UpdateResult struct has the expected
// fields for reporting the outcome of an update operation.
func TestUpdateResult_Fields(t *testing.T) {
	t.Parallel()

	// Given/When: an UpdateResult is constructed
	result := UpdateResult{
		PreviousVersion: "0.6.0",
		NewVersion:      "0.7.0",
		BinaryPath:      "/usr/local/bin/autopus-adk",
	}

	// Then: all fields are accessible and correct
	assert.Equal(t, "0.6.0", result.PreviousVersion)
	assert.Equal(t, "0.7.0", result.NewVersion)
	assert.Equal(t, "/usr/local/bin/autopus-adk", result.BinaryPath)
}
