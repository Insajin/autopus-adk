//go:build darwin || linux

package promptlayer

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"

	"golang.org/x/sys/unix"
)

const ompContextPromotionExecutableMaxBytesV3 = 512 << 20

func hashOMPContextPromotionExecutableFDV3(fd int, name string, opened unix.Stat_t) (string, error) {
	file := os.NewFile(uintptr(fd), name)
	if file == nil {
		_ = unix.Close(fd)
		return "", errors.New("adopt OMP context promotion runtime executable")
	}
	defer file.Close()
	if !safeOMPContextPromotionExecutableStatV3(opened) {
		return "", errors.New("OMP context promotion runtime executable identity is unsafe")
	}
	hash := sha256.New()
	written, err := io.Copy(hash, io.LimitReader(file, ompContextPromotionExecutableMaxBytesV3+1))
	if err != nil || written != opened.Size || written > ompContextPromotionExecutableMaxBytesV3 {
		return "", errors.New("read OMP context promotion runtime executable")
	}
	var after unix.Stat_t
	if unix.Fstat(fd, &after) != nil || !sameOMPContextPromotionUnixIdentityV2(opened, after) || after.Size != opened.Size {
		return "", errors.New("OMP context promotion runtime executable changed")
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil)), nil
}

func safeOMPContextPromotionExecutableStatV3(stat unix.Stat_t) bool {
	mode := uint32(stat.Mode)
	return mode&unix.S_IFMT == unix.S_IFREG && mode&0o022 == 0 && stat.Size > 0 &&
		stat.Size <= ompContextPromotionExecutableMaxBytesV3
}
