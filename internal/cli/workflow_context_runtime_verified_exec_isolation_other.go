//go:build !darwin && !linux

package cli

import (
	"errors"
	"os"
)

func workflowContextCanaryEffectiveUID() (uint32, error) {
	return 0, errors.New("release canary UID isolation is unsupported")
}

func workflowContextCanaryFileUID(os.FileInfo) (uint32, error) {
	return 0, errors.New("release canary owner identity is unsupported")
}
