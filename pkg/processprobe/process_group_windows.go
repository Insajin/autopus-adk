//go:build windows

package processprobe

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const windowsProbeTerminationCode = 1

func init() {
	outputCommand = outputWindowsCommand
	limitedOutputCommand = outputLimitedWindowsCommand
}

func configureProcessGroup(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= windows.CREATE_SUSPENDED
}

func terminateProcessGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}

func outputWindowsCommand(cmd *exec.Cmd) ([]byte, error) {
	if cmd.Stdout != nil {
		return nil, errors.New("exec: Stdout already set")
	}

	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	captureStderr := cmd.Stderr == nil
	var stderr windowsProbeStderr
	if captureStderr {
		cmd.Stderr = &stderr
	}

	err := runWindowsProbeCommand(cmd, func() bool { return false })
	if err != nil && captureStderr {
		if exitErr := (*exec.ExitError)(nil); errors.As(err, &exitErr) {
			exitErr.Stderr = stderr.Bytes()
		}
	}
	return stdout.Bytes(), err
}

func outputLimitedWindowsCommand(cmd *exec.Cmd, outputExceeded func() bool) error {
	return runWindowsProbeCommand(cmd, outputExceeded)
}

func runWindowsProbeCommand(cmd *exec.Cmd, outputExceeded func() bool) error {
	job, err := newWindowsProbeJob()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return joinCleanupError(err, job.close())
	}
	if err := job.assign(cmd); err != nil {
		cleanupErr := abortWindowsProbe(cmd, job, false)
		return errors.Join(err, cleanupErr)
	}
	if err := resumeWindowsProcess(cmd.Process.Pid); err != nil {
		cleanupErr := abortWindowsProbe(cmd, job, true)
		return errors.Join(err, cleanupErr)
	}

	waitErr := cmd.Wait()
	if waitErr != nil || outputExceeded() {
		if waitErr == nil {
			waitErr = ErrOutputLimit
		}
		return joinCleanupError(waitErr, job.terminateAndClose())
	}
	return job.releaseAndClose()
}

type windowsProbeJob struct {
	handle windows.Handle
}

func newWindowsProbeJob() (*windowsProbeJob, error) {
	handle, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, fmt.Errorf("create probe job object: %w", err)
	}
	job := &windowsProbeJob{handle: handle}
	if err := job.setKillOnClose(true); err != nil {
		return nil, errors.Join(err, job.close())
	}
	return job, nil
}

func (job *windowsProbeJob) assign(cmd *exec.Cmd) error {
	var assignErr error
	err := cmd.Process.WithHandle(func(handle uintptr) {
		assignErr = windows.AssignProcessToJobObject(job.handle, windows.Handle(handle))
	})
	if err != nil {
		return fmt.Errorf("acquire probe process handle: %w", err)
	}
	if assignErr != nil {
		return fmt.Errorf("assign probe process to job object: %w", assignErr)
	}
	return nil
}

func (job *windowsProbeJob) releaseAndClose() error {
	if err := job.setKillOnClose(false); err != nil {
		return errors.Join(err, job.terminateAndClose())
	}
	return job.close()
}

func (job *windowsProbeJob) terminateAndClose() error {
	terminateErr := windows.TerminateJobObject(job.handle, windowsProbeTerminationCode)
	return errors.Join(terminateErr, job.close())
}

func (job *windowsProbeJob) setKillOnClose(enabled bool) error {
	var info windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION
	if enabled {
		info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	}
	_, err := windows.SetInformationJobObject(
		job.handle,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
	)
	if err != nil {
		return fmt.Errorf("configure probe job object: %w", err)
	}
	return nil
}

func (job *windowsProbeJob) close() error {
	if job.handle == 0 {
		return nil
	}
	err := windows.CloseHandle(job.handle)
	job.handle = 0
	if err != nil {
		return fmt.Errorf("close probe job object: %w", err)
	}
	return nil
}

func abortWindowsProbe(cmd *exec.Cmd, job *windowsProbeJob, assigned bool) error {
	var terminateErr error
	if assigned {
		terminateErr = windows.TerminateJobObject(job.handle, windowsProbeTerminationCode)
	} else if cmd.Process != nil {
		terminateErr = cmd.Process.Kill()
	}
	_ = cmd.Wait()
	return errors.Join(terminateErr, job.close())
}

func joinCleanupError(primary, cleanup error) error {
	if cleanup == nil {
		return primary
	}
	return errors.Join(primary, cleanup)
}

func resumeWindowsProcess(pid int) error {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
	if err != nil {
		return fmt.Errorf("snapshot probe process threads: %w", err)
	}
	defer windows.CloseHandle(snapshot)

	entry := windows.ThreadEntry32{Size: uint32(unsafe.Sizeof(windows.ThreadEntry32{}))}
	if err := windows.Thread32First(snapshot, &entry); err != nil {
		return fmt.Errorf("enumerate probe process threads: %w", err)
	}
	for {
		if entry.OwnerProcessID == uint32(pid) {
			if err := resumeWindowsThread(entry.ThreadID); err != nil {
				return err
			}
			return nil
		}
		if err := windows.Thread32Next(snapshot, &entry); err != nil {
			if errors.Is(err, windows.ERROR_NO_MORE_FILES) {
				break
			}
			return fmt.Errorf("enumerate probe process threads: %w", err)
		}
	}
	return fmt.Errorf("resume probe process %d: primary thread not found", pid)
}

func resumeWindowsThread(threadID uint32) error {
	thread, err := windows.OpenThread(windows.THREAD_SUSPEND_RESUME, false, threadID)
	if err != nil {
		return fmt.Errorf("open probe process thread: %w", err)
	}
	defer windows.CloseHandle(thread)
	if _, err := windows.ResumeThread(thread); err != nil {
		return fmt.Errorf("resume probe process thread: %w", err)
	}
	return nil
}

type windowsProbeStderr struct {
	prefix  []byte
	suffix  []byte
	dropped int
}

func (buffer *windowsProbeStderr) Write(data []byte) (int, error) {
	written := len(data)
	if remaining := (32 << 10) - len(buffer.prefix); remaining > 0 {
		if remaining > len(data) {
			remaining = len(data)
		}
		buffer.prefix = append(buffer.prefix, data[:remaining]...)
		data = data[remaining:]
	}
	if len(data) == 0 {
		return written, nil
	}

	buffer.dropped += len(data)
	const suffixLimit = 32 << 10
	if len(data) >= suffixLimit {
		buffer.suffix = append(buffer.suffix[:0], data[len(data)-suffixLimit:]...)
		return written, nil
	}
	if overflow := len(buffer.suffix) + len(data) - suffixLimit; overflow > 0 {
		copy(buffer.suffix, buffer.suffix[overflow:])
		buffer.suffix = buffer.suffix[:len(buffer.suffix)-overflow]
	}
	buffer.suffix = append(buffer.suffix, data...)
	return written, nil
}

func (buffer *windowsProbeStderr) Bytes() []byte {
	if buffer.dropped == 0 {
		return bytes.Clone(buffer.prefix)
	}
	omitted := buffer.dropped - len(buffer.suffix)
	if omitted <= 0 {
		return append(bytes.Clone(buffer.prefix), buffer.suffix...)
	}
	result := bytes.Clone(buffer.prefix)
	result = fmt.Appendf(result, "\n... omitting %d bytes ...\n", omitted)
	return append(result, buffer.suffix...)
}
