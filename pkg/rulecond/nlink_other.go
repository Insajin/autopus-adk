//go:build !unix

package rulecond

import "io/fs"

// hasMultipleLinks always reports false where the link count is not exposed
// through fs.FileInfo.Sys(). The hardlink vector needs local filesystem write
// access on the same volume and is not deliverable through a git clone, so the
// remaining path checks carry containment on these platforms.
func hasMultipleLinks(_ fs.FileInfo) bool {
	return false
}
