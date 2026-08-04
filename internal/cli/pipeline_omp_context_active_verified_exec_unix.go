//go:build darwin || linux

package cli

import (
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"os"
	"os/exec"

	"golang.org/x/sys/unix"
)

type pipelineOMPVerifiedExecCommand struct {
	cmd                  *exec.Cmd
	parentFD             *os.File
	expected             pipelineOMPExecutableIdentity
	gateContext          context.Context
	afterFirstDarwinStop func()
}

func openPipelineOMPVerifiedExecutable(
	path string,
	expected pipelineOMPExecutableIdentity,
) (*os.File, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_CLOEXEC|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, errors.New("open verified managed active OMP executable")
	}
	file := os.NewFile(uintptr(fd), path)
	if file == nil {
		_ = unix.Close(fd)
		return nil, errors.New("bind verified managed active OMP executable")
	}
	failed := true
	defer func() {
		if failed {
			_ = file.Close()
		}
	}()
	before, err := file.Stat()
	if err != nil || expected.info == nil || !before.Mode().IsRegular() || before.Mode()&0o111 == 0 ||
		!os.SameFile(expected.info, before) || before.Size() != expected.size || before.Mode() != expected.mode ||
		before.ModTime().UnixNano() != expected.modTime {
		return nil, errors.New("managed active OMP executable FD identity mismatch")
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return nil, errors.New("hash managed active OMP executable FD")
	}
	after, statErr := file.Stat()
	if statErr != nil {
		return nil, errors.New("restat managed active OMP executable FD")
	}
	beforeOwner, beforeOwnerErr := pipelineOMPExecutableOwner(before)
	afterOwner, afterOwnerErr := pipelineOMPExecutableOwner(after)
	var digest [sha256.Size]byte
	copy(digest[:], hash.Sum(nil))
	if beforeOwnerErr != nil || afterOwnerErr != nil || !os.SameFile(before, after) ||
		after.Size() != before.Size() || after.Mode() != before.Mode() ||
		!after.ModTime().Equal(before.ModTime()) || beforeOwner != afterOwner ||
		beforeOwner != expected.owner || digest != expected.digest {
		return nil, errors.New("managed active OMP executable FD changed while verifying")
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, errors.New("rewind verified managed active OMP executable FD")
	}
	failed = false
	return file, nil
}

func (command *pipelineOMPVerifiedExecCommand) Close() error {
	if command == nil || command.parentFD == nil {
		return nil
	}
	err := command.parentFD.Close()
	command.parentFD = nil
	return err
}
