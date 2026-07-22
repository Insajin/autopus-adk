//go:build !darwin && !linux

package main

import (
	"errors"
	"os/exec"
)

func configureProcessGroup(_ *exec.Cmd) error {
	return errors.New("isolated process groups are unsupported on this platform")
}

func killProcessGroup(_ int) error {
	return errors.New("isolated process groups are unsupported on this platform")
}
