//go:build darwin || linux

package main

import (
	"errors"
	"fmt"
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

const (
	releaseCanaryUID = 59999
	releaseCanaryGID = 20
)

func run(arguments []string) error {
	if os.Getuid() != 0 || os.Geteuid() != 0 || len(arguments) == 0 || arguments[0] == "" {
		return errors.New("root authority and an executable are required")
	}
	if err := unix.Setgroups([]int{}); err != nil {
		return errors.New("clear supplementary groups")
	}
	if err := unix.Setgid(releaseCanaryGID); err != nil {
		return errors.New("set dedicated group")
	}
	if err := unix.Setuid(releaseCanaryUID); err != nil {
		return errors.New("set dedicated user")
	}
	groups, err := unix.Getgroups()
	groupsValid := len(groups) == 0 ||
		(len(groups) == 1 && groups[0] == releaseCanaryGID)
	if err != nil || !groupsValid || os.Getuid() != releaseCanaryUID ||
		os.Geteuid() != releaseCanaryUID || os.Getgid() != releaseCanaryGID || os.Getegid() != releaseCanaryGID {
		return fmt.Errorf("dedicated identity verification failed: uid=%d euid=%d gid=%d egid=%d groups=%v groups_error=%v",
			os.Getuid(), os.Geteuid(), os.Getgid(), os.Getegid(), groups, err)
	}
	syscall.Umask(0o077)
	return syscall.Exec(arguments[0], arguments, os.Environ())
}
