//go:build !darwin || !cgo

package run

func secureDesktopSpawnSupported() bool { return false }

func startSecureDesktopProcess(secureDesktopSpawnSpec) (*secureDesktopProcess, error) {
	return nil, errDesktopProviderUnavailable
}

func secureDesktopKillProcessGroup(int) {}

func secureDesktopReapProcessGroup(int) error { return errDesktopProviderUnavailable }
