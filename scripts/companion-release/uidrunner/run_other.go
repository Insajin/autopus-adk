//go:build !darwin && !linux

package main

import "errors"

func run([]string) error {
	return errors.New("dedicated UID execution is unsupported on this platform")
}
