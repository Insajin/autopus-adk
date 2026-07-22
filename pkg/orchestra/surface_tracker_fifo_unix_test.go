//go:build !windows

package orchestra

import "golang.org/x/sys/unix"

func fifoSupported() bool { return true }

func createTrackerFIFO(path string) error {
	return unix.Mkfifo(path, 0o600)
}

func unblockTrackerFIFO(path string) {
	fd, err := unix.Open(path, unix.O_WRONLY|unix.O_NONBLOCK, 0)
	if err == nil {
		_ = unix.Close(fd)
	}
}
