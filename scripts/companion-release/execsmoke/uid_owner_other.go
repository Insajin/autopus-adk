//go:build !darwin && !linux

package main

import (
	"errors"
	"os"
)

func canaryFileIdentity(os.FileInfo) (uint32, uint32, error) {
	return 0, 0, errors.New("release canary owner identity is unsupported")

}

func canaryFileUID(info os.FileInfo) (uint32, error) {
	uid, _, err := canaryFileIdentity(info)
	return uid, err
}
