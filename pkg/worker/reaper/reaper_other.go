//go:build !linux && !darwin && !freebsd && !openbsd && !netbsd

package reaper

// noopDetector is used on platforms without Unix process semantics.
type noopDetector struct{}

// DetectZombies always returns nil on non-Unix platforms.
func (d *noopDetector) DetectZombies() []int {
	return nil
}

// newDefaultDetector returns a no-op detector on non-Unix platforms.
func newDefaultDetector() ZombieDetector {
	return &noopDetector{}
}

// reapPID is a no-op on non-Unix platforms.
func reapPID(_ int) {}
