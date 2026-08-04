//go:build darwin

package promptlayer

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	darwinOMPContextPromotionProcInfoCallPIDInfoV3 = 2
	darwinOMPContextPromotionPIDRegionPathInfoV3   = 8
	darwinOMPContextPromotionMaxPathV3             = 1024
)

type darwinOMPContextPromotionRegionInfoV3 struct {
	Protection            uint32
	MaxProtection         uint32
	Inheritance           uint32
	Flags                 uint32
	Offset                uint64
	Behavior              uint32
	UserWiredCount        uint32
	UserTag               uint32
	PagesResident         uint32
	PagesSharedNowPrivate uint32
	PagesSwappedOut       uint32
	PagesDirtied          uint32
	RefCount              uint32
	ShadowDepth           uint32
	ShareMode             uint32
	PrivatePagesResident  uint32
	SharedPagesResident   uint32
	ObjectID              uint32
	Depth                 uint32
	Address               uint64
	Size                  uint64
}

type darwinOMPContextPromotionVnodeStatV3 struct {
	Dev           uint32
	Mode          uint16
	Nlink         uint16
	Ino           uint64
	UID           uint32
	GID           uint32
	Atime         int64
	AtimeNsec     int64
	Mtime         int64
	MtimeNsec     int64
	Ctime         int64
	CtimeNsec     int64
	Birthtime     int64
	BirthtimeNsec int64
	Size          int64
	Blocks        int64
	BlockSize     int32
	Flags         uint32
	Generation    uint32
	Rdev          uint32
	Spare         [2]int64
}

type darwinOMPContextPromotionVnodeInfoV3 struct {
	Stat darwinOMPContextPromotionVnodeStatV3
	Type int32
	Pad  int32
	FSID [2]int32
}

type darwinOMPContextPromotionRegionPathInfoV3 struct {
	Region darwinOMPContextPromotionRegionInfoV3
	Vnode  struct {
		Info darwinOMPContextPromotionVnodeInfoV3
		Path [darwinOMPContextPromotionMaxPathV3]byte
	}
}

func currentOMPContextPromotionExecutableSHA256V3() (string, error) {
	live, path, err := currentDarwinOMPContextPromotionExecutableRegionV3()
	if err != nil {
		return "", err
	}
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_CLOEXEC|unix.O_NONBLOCK, 0)
	if err != nil {
		return "", errors.New("open OMP context promotion runtime executable")
	}
	var opened unix.Stat_t
	if unix.Fstat(fd, &opened) != nil || !sameDarwinOMPContextPromotionExecutableVnodeV3(opened, live) {
		_ = unix.Close(fd)
		return "", errors.New("OMP context promotion runtime executable is not the live process image")
	}
	digest, err := hashOMPContextPromotionExecutableFDV3(fd, filepath.Base(path), opened)
	if err != nil {
		return "", err
	}
	after, _, err := currentDarwinOMPContextPromotionExecutableRegionV3()
	if err != nil || !sameDarwinOMPContextPromotionVnodeV3(live, after) {
		return "", errors.New("OMP context promotion runtime executable image changed")
	}
	return digest, nil
}

func currentDarwinOMPContextPromotionExecutableRegionV3() (
	darwinOMPContextPromotionVnodeStatV3,
	string,
	error,
) {
	var info darwinOMPContextPromotionRegionPathInfoV3
	address := reflect.ValueOf(currentOMPContextPromotionExecutableSHA256V3).Pointer()
	//nolint:staticcheck // x/sys/unix has no libproc wrapper; live-image identity requires proc_pidinfo.
	written, _, errno := unix.Syscall6(
		unix.SYS_PROC_INFO,
		darwinOMPContextPromotionProcInfoCallPIDInfoV3,
		uintptr(os.Getpid()),
		darwinOMPContextPromotionPIDRegionPathInfoV3,
		address,
		uintptr(unsafe.Pointer(&info)),
		unsafe.Sizeof(info),
	)
	runtime.KeepAlive(&info)
	if errno != 0 || written != unsafe.Sizeof(info) {
		return darwinOMPContextPromotionVnodeStatV3{}, "", errors.New("inspect live OMP context promotion runtime executable")
	}
	path := unix.ByteSliceToString(info.Vnode.Path[:])
	if !filepath.IsAbs(path) || filepath.Clean(path) != path || !safeDarwinOMPContextPromotionVnodeV3(info.Vnode.Info.Stat) {
		return darwinOMPContextPromotionVnodeStatV3{}, "", errors.New("live OMP context promotion runtime executable identity is unsafe")
	}
	return info.Vnode.Info.Stat, path, nil
}

func safeDarwinOMPContextPromotionVnodeV3(stat darwinOMPContextPromotionVnodeStatV3) bool {
	mode := uint32(stat.Mode)
	return mode&unix.S_IFMT == unix.S_IFREG && mode&0o022 == 0 && stat.Size > 0 &&
		stat.Size <= ompContextPromotionExecutableMaxBytesV3
}

func sameDarwinOMPContextPromotionExecutableVnodeV3(
	opened unix.Stat_t,
	live darwinOMPContextPromotionVnodeStatV3,
) bool {
	return uint32(opened.Dev) == live.Dev && opened.Ino == live.Ino && opened.Size == live.Size &&
		opened.Mode == live.Mode && safeOMPContextPromotionExecutableStatV3(opened) &&
		safeDarwinOMPContextPromotionVnodeV3(live)
}

func sameDarwinOMPContextPromotionVnodeV3(
	left darwinOMPContextPromotionVnodeStatV3,
	right darwinOMPContextPromotionVnodeStatV3,
) bool {
	return left.Dev == right.Dev && left.Ino == right.Ino && left.Size == right.Size && left.Mode == right.Mode
}
