//go:build darwin || linux

package promptlayer

import "golang.org/x/sys/unix"

func validOMPContextPromotionDirectoryStatV2(stat unix.Stat_t) bool {
	return uint32(stat.Mode)&unix.S_IFMT == unix.S_IFDIR && uint64(stat.Nlink) > 0
}

func secureOMPContextPromotionRootStatV2(stat unix.Stat_t) bool {
	return validOMPContextPromotionDirectoryStatV2(stat) && uint32(stat.Mode)&0o022 == 0 &&
		uint64(stat.Uid) == uint64(unix.Geteuid())
}

func secureOMPContextPromotionMetadataDirectoryStatV2(stat unix.Stat_t) bool {
	return secureOMPContextPromotionRootStatV2(stat)
}

func secureOMPContextPromotionPrivateDirectoryStatV2(stat unix.Stat_t) bool {
	return validOMPContextPromotionDirectoryStatV2(stat) && uint32(stat.Mode)&0o7777 == 0o700 &&
		uint64(stat.Uid) == uint64(unix.Geteuid())
}

func secureOMPContextPromotionArtifactStatV2(stat unix.Stat_t, limit int64) bool {
	mode := uint32(stat.Mode)
	return mode&unix.S_IFMT == unix.S_IFREG && mode&0o7777 == 0o600 &&
		uint64(stat.Nlink) == 1 && uint64(stat.Uid) == uint64(unix.Geteuid()) &&
		stat.Size > 0 && stat.Size <= limit
}

func sameOMPContextPromotionUnixIdentityV2(left, right unix.Stat_t) bool {
	return uint64(left.Dev) == uint64(right.Dev) && uint64(left.Ino) == uint64(right.Ino)
}
