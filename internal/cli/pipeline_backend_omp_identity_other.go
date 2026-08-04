//go:build !darwin && !linux

package cli

import "os"

type pipelineOMPExecutableOwnerIdentity struct{}

func pipelineOMPExecutableOwner(os.FileInfo) (pipelineOMPExecutableOwnerIdentity, error) {
	return pipelineOMPExecutableOwnerIdentity{}, nil
}
