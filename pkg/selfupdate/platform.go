package selfupdate

import "fmt"

// ArchiveName returns the GoReleaser archive filename for the given OS, architecture, and version.
// Format: autopus-adk_{version}_{os}_{arch}.tar.gz
func ArchiveName(goos, goarch, version string) string {
	return fmt.Sprintf("autopus-adk_%s_%s_%s.tar.gz", version, goos, goarch)
}
