package selfupdate

// ReleaseInfo holds information about a GitHub release.
type ReleaseInfo struct {
	TagName     string // e.g. "v0.7.0"
	ArchiveURL  string // download URL for the archive
	ChecksumURL string // download URL for checksums.txt
	ArchiveName string // e.g. "autopus-adk_0.7.0_darwin_arm64.tar.gz"
}

// UpdateResult holds the outcome of a successful update.
type UpdateResult struct {
	PreviousVersion string // e.g. "0.6.0"
	NewVersion      string // e.g. "0.7.0"
	BinaryPath      string // path to the replaced binary
}
