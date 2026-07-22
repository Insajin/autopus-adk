//go:build !windows

package orchestra

import (
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type trackerOwnerInfo struct {
	mode os.FileMode
	uid  uint32
}

func (i trackerOwnerInfo) Name() string       { return "entry" }
func (i trackerOwnerInfo) Size() int64        { return 1 }
func (i trackerOwnerInfo) Mode() os.FileMode  { return i.mode }
func (i trackerOwnerInfo) ModTime() time.Time { return time.Time{} }
func (i trackerOwnerInfo) IsDir() bool        { return i.mode.IsDir() }
func (i trackerOwnerInfo) Sys() any           { return &syscall.Stat_t{Uid: i.uid} }

func TestTrackerFileInfoSecure_WrongOwnerFailsClosed(t *testing.T) {
	wrongUID := uint32(os.Getuid()) + 1
	info := trackerOwnerInfo{mode: 0o600, uid: wrongUID}

	assert.False(t, trackerFileInfoSecure(info))
}

func TestTrackerDirectoryInfoSecure_WrongOwnerFailsClosed(t *testing.T) {
	wrongUID := uint32(os.Getuid()) + 1
	info := trackerOwnerInfo{mode: os.ModeDir | 0o700, uid: wrongUID}

	assert.False(t, trackerDirectoryInfoSecure(info))
}
