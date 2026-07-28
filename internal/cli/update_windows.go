//go:build windows

package cli

// isWritable on Windows always returns true — elevated permission handling is out of scope.
func isWritable(_ string) bool {
	return true
}
