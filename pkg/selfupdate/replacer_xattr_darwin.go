//go:build darwin

package selfupdate

import (
	"errors"
	"fmt"
	"strings"

	"golang.org/x/sys/unix"
)

const maxPreservedXattrBytes = 16 << 20

var updateBlockingXattrs = [...]string{
	"com.apple.quarantine",
	"com.apple.provenance",
}

func clearUpdateXattrs(path string) error {
	return clearUpdateXattrsWith(path, unix.Removexattr)
}

func clearUpdateXattrsWith(
	path string,
	remove func(string, string) error,
) error {
	for _, name := range updateBlockingXattrs {
		if err := remove(path, name); err != nil {
			if errors.Is(err, unix.ENOATTR) {
				continue
			}
			if errors.Is(err, unix.ENOTSUP) {
				return nil
			}
			return fmt.Errorf("remove %s: %w", name, err)
		}
	}
	return nil
}

func copyReplacementFile(sourcePath, targetPath string) error {
	if err := copyFile(sourcePath, targetPath); err != nil {
		return err
	}
	return copyPreservedXattrs(sourcePath, targetPath)
}

func copyPreservedXattrs(sourcePath, targetPath string) error {
	names, err := listXattrNames(sourcePath)
	if err != nil {
		return err
	}
	for _, name := range names {
		if isUpdateBlockingXattr(name) {
			continue
		}
		value, err := readXattr(sourcePath, name)
		if errors.Is(err, unix.ENOATTR) {
			continue
		}
		if err != nil {
			return fmt.Errorf("read xattr %s: %w", name, err)
		}
		if err := unix.Setxattr(targetPath, name, value, 0); err != nil {
			return fmt.Errorf("preserve xattr %s: %w", name, err)
		}
	}
	return nil
}

func listXattrNames(path string) ([]string, error) {
	size, err := unix.Listxattr(path, nil)
	if errors.Is(err, unix.ENOTSUP) {
		return nil, nil
	}
	if err != nil || size == 0 {
		return nil, err
	}
	if err := validateXattrSize("name list", size); err != nil {
		return nil, err
	}
	for attempt := 0; attempt < 3; attempt++ {
		buffer := make([]byte, size)
		read, err := unix.Listxattr(path, buffer)
		if errors.Is(err, unix.ERANGE) {
			size, err = unix.Listxattr(path, nil)
			if err != nil {
				return nil, err
			}
			if err := validateXattrSize("name list", size); err != nil {
				return nil, err
			}
			continue
		}
		if err != nil {
			return nil, err
		}
		var names []string
		for _, name := range strings.Split(string(buffer[:read]), "\x00") {
			if name != "" {
				names = append(names, name)
			}
		}
		return names, nil
	}
	return nil, errors.New("xattr name list changed repeatedly")
}

func readXattr(path, name string) ([]byte, error) {
	size, err := unix.Getxattr(path, name, nil)
	if err != nil {
		return nil, err
	}
	if err := validateXattrSize("value", size); err != nil {
		return nil, err
	}
	for attempt := 0; attempt < 3; attempt++ {
		value := make([]byte, size)
		read, err := unix.Getxattr(path, name, value)
		if errors.Is(err, unix.ERANGE) {
			size, err = unix.Getxattr(path, name, nil)
			if err != nil {
				return nil, err
			}
			if err := validateXattrSize("value", size); err != nil {
				return nil, err
			}
			continue
		}
		if err != nil {
			return nil, err
		}
		return value[:read], nil
	}
	return nil, errors.New("xattr value changed repeatedly")
}

func validateXattrSize(kind string, size int) error {
	if size > maxPreservedXattrBytes {
		return fmt.Errorf("xattr %s exceeds %d bytes", kind, maxPreservedXattrBytes)
	}
	return nil
}

func isUpdateBlockingXattr(name string) bool {
	for _, blocked := range updateBlockingXattrs {
		if name == blocked {
			return true
		}
	}
	return false
}
