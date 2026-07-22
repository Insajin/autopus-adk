//go:build windows

package orchestra

import "errors"

func fifoSupported() bool { return false }

func createTrackerFIFO(string) error { return errors.New("FIFO unsupported") }

func unblockTrackerFIFO(string) {}
