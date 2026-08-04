//go:build darwin || linux

package main

import (
	"errors"
	"os"
	"syscall"
)

func canaryFileIdentity(info os.FileInfo) (uint32, uint32, error) {
	if info == nil {
		return 0, 0, errors.New("release canary file identity is unavailable")
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, 0, errors.New("release canary owner identity is unavailable")
	}
	return stat.Uid, stat.Gid, nil

}

func canaryFileUID(info os.FileInfo) (uint32, error) {
	uid, _, err := canaryFileIdentity(info)
	return uid, err
}
