//go:build !darwin

package selfupdate

func clearUpdateXattrs(_ string) error {
	return nil
}

func copyReplacementFile(sourcePath, targetPath string) error {
	return copyFile(sourcePath, targetPath)
}
