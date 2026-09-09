//go:build darwin || linux

package ompprobe

import (
	"io"
	"os"

	"golang.org/x/sys/unix"
)

// readProbeSource opens the record file exactly once and reads only that
// descriptor. The path is never re-opened after inspection, so replacing the
// name between check and use cannot change which object is read: a swap after
// our open leaves the attacker holding a different inode than the one we keep.
func readProbeSource(options ExportOptions, plan exportPlan) ([]byte, error) {
	rootFD, err := openProbeDirectory(options.SourceRoot)
	if err != nil {
		return nil, err
	}
	directoryFD, err := descendProbeDirectories(rootFD, plan.directories)
	if err != nil {
		return nil, err
	}
	defer func() { _ = unix.Close(directoryFD) }()

	fd, err := unix.Openat(directoryFD, plan.file, probeSourceFileFlags, 0)
	if err != nil {
		return nil, errExportSource
	}
	file := os.NewFile(uintptr(fd), plan.file)
	if file == nil {
		_ = unix.Close(fd)
		return nil, errExportSource
	}
	defer func() { _ = file.Close() }()

	var opened, onPath unix.Stat_t
	if unix.Fstat(fd, &opened) != nil ||
		unix.Fstatat(directoryFD, plan.file, &onPath, unix.AT_SYMLINK_NOFOLLOW) != nil ||
		!acceptedProbeSourceStat(opened, options.SourceUID) ||
		!acceptedProbeSourceStat(onPath, options.SourceUID) ||
		!sameProbeFile(opened, onPath) {
		return nil, errExportSource
	}
	body, readErr := io.ReadAll(io.LimitReader(file, opened.Size+1))
	var afterRead unix.Stat_t
	if readErr != nil || int64(len(body)) != opened.Size ||
		unix.Fstat(fd, &afterRead) != nil || !sameProbeFile(opened, afterRead) ||
		afterRead.Size != opened.Size {
		clear(body)
		return nil, errExportRead
	}
	return body, nil
}

// publishProbeRecords creates the retained file exclusively, gives it to the
// runner, writes the re-serialized records and verifies the result on the same
// descriptor. An existing destination is never opened, truncated or replaced.
func publishProbeRecords(options ExportOptions, payload []byte) error {
	directoryFD, err := openProbeDirectory(options.DestinationRoot)
	if err != nil {
		return err
	}
	defer func() { _ = unix.Close(directoryFD) }()
	var directoryStat unix.Stat_t
	if unix.Fstat(directoryFD, &directoryStat) != nil ||
		!trustedProbeDestinationStat(directoryStat, options.OwnerUID) {
		return errExportDestination
	}
	fd, err := unix.Openat(directoryFD, options.DestinationName, probeDestinationFlags, 0o600)
	if err != nil {
		return errExportPublish
	}
	var created unix.Stat_t
	if unix.Fstat(fd, &created) != nil {
		// Exclusive creation just succeeded, so the name is ours to remove
		// even though its identity could not be captured.
		_ = unix.Close(fd)
		_ = unix.Unlinkat(directoryFD, options.DestinationName, 0)
		return errExportPublish
	}
	if writeErr := writeProbeRecordFile(fd, options, payload); writeErr != nil {
		_ = unix.Close(fd)
		discardProbeDestination(directoryFD, options.DestinationName, created)
		return writeErr
	}
	if unix.Close(fd) != nil {
		discardProbeDestination(directoryFD, options.DestinationName, created)
		return errExportPublish
	}
	return nil
}

func writeProbeRecordFile(fd int, options ExportOptions, payload []byte) error {
	if unix.Fchown(fd, int(options.OwnerUID), int(options.OwnerGID)) != nil {
		return errExportPublish
	}
	if unix.Fchmod(fd, 0o600) != nil {
		return errExportPublish
	}
	for written := 0; written < len(payload); {
		count, err := unix.Write(fd, payload[written:])
		if err != nil || count <= 0 {
			return errExportPublish
		}
		written += count
	}
	if unix.Fsync(fd) != nil {
		return errExportPublish
	}
	var published unix.Stat_t
	if unix.Fstat(fd, &published) != nil ||
		!publishedProbeDestinationStat(published, options, int64(len(payload))) {
		return errExportPublish
	}
	return nil
}

// discardProbeDestination removes only the file this exporter created: the
// name must still resolve, without following symlinks, to that exact inode.
// A file someone else placed at the name in the meantime is left alone.
func discardProbeDestination(directoryFD int, name string, created unix.Stat_t) {
	var onPath unix.Stat_t
	if unix.Fstatat(directoryFD, name, &onPath, unix.AT_SYMLINK_NOFOLLOW) != nil ||
		!sameProbeFile(created, onPath) {
		return
	}
	_ = unix.Unlinkat(directoryFD, name, 0)
}
