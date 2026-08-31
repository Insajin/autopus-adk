//go:build darwin || linux

package main

import (
	"errors"
	"fmt"
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

func run(arguments []string) error {
	config, err := parseRunArguments(arguments)
	if err != nil {
		return err
	}
	if os.Getuid() != 0 || os.Geteuid() != 0 {
		return errors.New("root authority is required")
	}
	if err := unix.Setgroups([]int{}); err != nil {
		return errors.New("clear supplementary groups")
	}
	if err := unix.Setgid(config.gid); err != nil {
		return errors.New("set dedicated group")
	}
	if err := unix.Setuid(config.uid); err != nil {
		return errors.New("set dedicated user")
	}
	groups, err := unix.Getgroups()
	groupsValid := len(groups) == 0 ||
		(len(groups) == 1 && groups[0] == config.gid)
	if err != nil || !groupsValid || os.Getuid() != config.uid ||
		os.Geteuid() != config.uid || os.Getgid() != config.gid || os.Getegid() != config.gid {
		return fmt.Errorf("dedicated identity verification failed: uid=%d euid=%d gid=%d egid=%d groups=%v groups_error=%v",
			os.Getuid(), os.Geteuid(), os.Getgid(), os.Getegid(), groups, err)
	}
	syscall.Umask(0o077)
	return syscall.Exec(config.arguments[0], config.arguments, os.Environ())
}
