//go:build darwin || linux

package cli

import (
	"errors"
	"os"
	"syscall"
)

func workflowContextCanaryEffectiveUID() (uint32, error) {
	return uint32(os.Geteuid()), nil
}

func workflowContextCanaryFileUID(info os.FileInfo) (uint32, error) {
	if info == nil {
		return 0, errors.New("release canary file identity is unavailable")
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, errors.New("release canary owner identity is unavailable")
	}
	return stat.Uid, nil
}
