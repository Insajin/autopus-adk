//go:build darwin || linux

package cli

import (
	"errors"
	"os"
	"syscall"
)

type pipelineOMPExecutableOwnerIdentity struct {
	uid uint32
	gid uint32
}

func pipelineOMPExecutableOwner(info os.FileInfo) (pipelineOMPExecutableOwnerIdentity, error) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return pipelineOMPExecutableOwnerIdentity{}, errors.New("OMP pipeline executable ownership is unavailable")
	}
	return pipelineOMPExecutableOwnerIdentity{uid: stat.Uid, gid: stat.Gid}, nil
}
