//go:build windows

package cli

import "golang.org/x/sys/windows"

const windowsProcessStillActive = 259

func processAliveForTest(pid int) bool {
	handle, err := windows.OpenProcess(
		windows.PROCESS_QUERY_LIMITED_INFORMATION,
		false,
		uint32(pid),
	)
	if err != nil {
		return false
	}
	defer windows.CloseHandle(handle)

	var exitCode uint32
	if err := windows.GetExitCodeProcess(handle, &exitCode); err != nil {
		return false
	}
	return exitCode == windowsProcessStillActive
}
