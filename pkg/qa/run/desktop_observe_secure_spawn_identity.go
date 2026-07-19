package run

import (
	"crypto/sha1" // #nosec G505 -- CodeDirectory may advertise legacy SHA-1; it is only an identity comparator.
	"crypto/sha256"
	"crypto/sha512"
	"debug/macho"
	"encoding/binary"
	"errors"
	"io"
	"runtime"
)

const (
	desktopCodeSignatureCommand = uint32(0x1d)
	desktopEmbeddedSignature    = uint32(0xfade0cc0)
	desktopCodeDirectory        = uint32(0xfade0c02)
	maxDesktopCodeDirectories   = 7
	maxDesktopSignatureEntries  = uint32(64)
	maxDesktopSignatureBytes    = uint32(16 << 20)
)

type desktopCodeDirectoryDigest [sha1.Size]byte

type desktopCodeIdentity struct {
	digests [maxDesktopCodeDirectories]desktopCodeDirectoryDigest
	count   uint8
}

func readDesktopCodeIdentity(reader io.ReaderAt, size int64) (desktopCodeIdentity, error) {
	if reader == nil || size <= 0 || size > maxDesktopProviderExecutableBytes {
		return desktopCodeIdentity{}, errDesktopProviderUnavailable
	}
	file, base, err := currentDesktopMachO(reader)
	if err != nil {
		return desktopCodeIdentity{}, errDesktopProviderUnavailable
	}
	defer file.Close()
	offset, length, err := desktopCodeSignatureRange(file)
	if err != nil || length == 0 || length > maxDesktopSignatureBytes ||
		base > uint64(size) || uint64(offset) > uint64(size)-base ||
		uint64(length) > uint64(size)-base-uint64(offset) {
		return desktopCodeIdentity{}, errDesktopProviderUnavailable
	}
	blob := make([]byte, length)
	if _, err := reader.ReadAt(blob, int64(base)+int64(offset)); err != nil {
		return desktopCodeIdentity{}, errDesktopProviderUnavailable
	}
	return parseDesktopEmbeddedSignature(blob)
}

func currentDesktopMachO(reader io.ReaderAt) (*macho.File, uint64, error) {
	fat, err := macho.NewFatFile(reader)
	if err == nil {
		for index := range fat.Arches {
			arch := &fat.Arches[index]
			if arch.Cpu == currentDesktopCPU() {
				return arch.File, uint64(arch.Offset), nil
			}
		}
		_ = fat.Close()
		return nil, 0, errDesktopProviderUnavailable
	}
	if !errors.Is(err, macho.ErrNotFat) {
		return nil, 0, err
	}
	file, err := macho.NewFile(reader)
	return file, 0, err
}

func currentDesktopCPU() macho.Cpu {
	switch runtime.GOARCH {
	case "amd64":
		return macho.CpuAmd64
	case "arm64":
		return macho.CpuArm64
	default:
		return 0
	}
}

func desktopCodeSignatureRange(file *macho.File) (uint32, uint32, error) {
	for _, load := range file.Loads {
		raw := load.Raw()
		if len(raw) >= 16 && file.ByteOrder.Uint32(raw[:4]) == desktopCodeSignatureCommand {
			return file.ByteOrder.Uint32(raw[8:12]), file.ByteOrder.Uint32(raw[12:16]), nil
		}
	}
	return 0, 0, errDesktopProviderUnavailable
}

func parseDesktopEmbeddedSignature(blob []byte) (desktopCodeIdentity, error) {
	if len(blob) < 12 || binary.BigEndian.Uint32(blob[:4]) != desktopEmbeddedSignature {
		return desktopCodeIdentity{}, errDesktopProviderUnavailable
	}
	length := binary.BigEndian.Uint32(blob[4:8])
	count := binary.BigEndian.Uint32(blob[8:12])
	if length > uint32(len(blob)) || count > maxDesktopSignatureEntries || 12+count*8 > length {
		return desktopCodeIdentity{}, errDesktopProviderUnavailable
	}
	var identity desktopCodeIdentity
	for index := uint32(0); index < count; index++ {
		entry := 12 + index*8
		slot := binary.BigEndian.Uint32(blob[entry : entry+4])
		if slot != 0 && (slot < 0x1000 || slot > 0x1005) {
			continue
		}
		offset := binary.BigEndian.Uint32(blob[entry+4 : entry+8])
		if offset > length-8 || binary.BigEndian.Uint32(blob[offset:offset+4]) != desktopCodeDirectory {
			return desktopCodeIdentity{}, errDesktopProviderUnavailable
		}
		directoryLength := binary.BigEndian.Uint32(blob[offset+4 : offset+8])
		if directoryLength < 44 || directoryLength > length-offset {
			return desktopCodeIdentity{}, errDesktopProviderUnavailable
		}
		digest, ok := digestDesktopCodeDirectory(blob[offset : offset+directoryLength])
		if !ok {
			continue
		}
		if !identity.contains(digest) {
			if identity.count >= maxDesktopCodeDirectories {
				return desktopCodeIdentity{}, errDesktopProviderUnavailable
			}
			identity.digests[identity.count] = digest
			identity.count++
		}
	}
	if identity.count == 0 {
		return desktopCodeIdentity{}, errDesktopProviderUnavailable
	}
	return identity, nil
}

func digestDesktopCodeDirectory(directory []byte) (desktopCodeDirectoryDigest, bool) {
	var digest desktopCodeDirectoryDigest
	switch directory[37] {
	case 1:
		value := sha1.Sum(directory) // #nosec G401 -- compared with Apple's signed CodeDirectory identity only.
		copy(digest[:], value[:])
	case 2, 3:
		value := sha256.Sum256(directory)
		copy(digest[:], value[:sha1.Size])
	case 4:
		value := sha512.Sum384(directory)
		copy(digest[:], value[:sha1.Size])
	default:
		return desktopCodeDirectoryDigest{}, false
	}
	return digest, true
}

func (identity desktopCodeIdentity) contains(candidate desktopCodeDirectoryDigest) bool {
	for index := uint8(0); index < identity.count; index++ {
		if identity.digests[index] == candidate {
			return true
		}
	}
	return false
}
