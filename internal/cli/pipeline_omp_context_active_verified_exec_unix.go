//go:build darwin || linux

package cli

import (
	"crypto/sha256"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"golang.org/x/sys/unix"
)

// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-OMP-004: build-selected sandbox-mode contract shared by process startup, version probes, and platform tests.
// @AX:REASON [AUTO]: callers depend on exact command, parent FD, private-root, and Darwin-only invariants before inherited execution.
func configurePipelineOMPVerifiedExecSandboxMode(
	command *pipelineOMPVerifiedExecCommand,
	mode pipelineOMPActiveSandboxMode,
	requirePrivate bool,
) error {
	if mode == pipelineOMPActiveSandboxManaged {
		return nil
	}
	if mode != pipelineOMPActiveSandboxInheritedParent || runtime.GOOS != "darwin" ||
		command == nil || command.cmd == nil || command.parentFD == nil || command.expected.info == nil ||
		command.cmd.Path != command.parentFD.Name() || len(command.cmd.Args) == 0 ||
		command.cmd.Args[0] != command.cmd.Path || len(command.cmd.ExtraFiles) != 0 {
		return errors.New("inherited parent sandbox verified OMP command is unsafe")
	}
	if requirePrivate && !pipelineOMPVerifiedPrivateExecutableRoot(command.cmd.Path) {
		return errors.New("inherited parent sandbox private OMP root is unsafe")
	}
	command.inheritedDarwinPath = true
	command.inheritedDarwinPrivate = requirePrivate
	return nil
}

func pipelineOMPVerifiedPrivateExecutableRoot(path string) bool {
	root := filepath.Dir(path)
	resolved, resolveErr := filepath.EvalSymlinks(root)
	info, statErr := os.Lstat(root)
	if resolveErr != nil || statErr != nil || filepath.Clean(resolved) != filepath.Clean(root) ||
		!info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0o700 {
		return false
	}
	owner, ownerErr := pipelineOMPExecutableOwner(info)
	return ownerErr == nil && owner.uid == uint32(os.Geteuid())
}

func pipelineOMPVerifiedExecUsesDarwinPtrace(command *pipelineOMPVerifiedExecCommand) bool {
	return command != nil && command.cmd != nil && command.cmd.SysProcAttr != nil && command.cmd.SysProcAttr.Ptrace
}

// @AX:WARN [AUTO]: executable FD verification contains 8 if branches.
// @AX:REASON [AUTO]: no-follow open, pre/post identity, ownership, digest, mutation detection, and rewind checks must fail closed together.
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
