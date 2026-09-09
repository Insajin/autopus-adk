//go:build !darwin && !linux

package ompprobe

import "errors"

var errExportUnsupported = errors.New("probe export is unsupported on this platform")

// The exporter's guarantees are descriptor-based: openat with O_NOFOLLOW on
// every component, fstat on the same descriptor that is read, exclusive
// creation with POSIX ownership. Without those primitives there is no way to
// keep the promise, so unsupported platforms fail loudly instead of copying
// bytes on weaker terms.

func readProbeSource(ExportOptions, exportPlan) ([]byte, error) {
	return nil, errExportUnsupported
}

func publishProbeRecords(ExportOptions, []byte) error {
	return errExportUnsupported
}
