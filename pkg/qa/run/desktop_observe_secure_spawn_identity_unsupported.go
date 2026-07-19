//go:build !darwin

package run

import "os"

func desktopExecutableFileIdentity(os.FileInfo) (desktopFileIdentity, error) {
	return desktopFileIdentity{}, errDesktopProviderUnavailable
}
