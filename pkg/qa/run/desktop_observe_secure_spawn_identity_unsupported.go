//go:build !darwin

package run

import "os"

type desktopFileIdentity struct{}

func desktopExecutableFileIdentity(os.FileInfo) (desktopFileIdentity, error) {
	return desktopFileIdentity{}, errDesktopProviderUnavailable
}
