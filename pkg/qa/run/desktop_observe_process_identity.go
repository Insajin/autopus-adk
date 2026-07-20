package run

import (
	"context"
	"crypto/sha256"
	"io"
	"os"
)

const maxDesktopProviderExecutableBytes = int64(256 << 20)

type desktopExecutableDigest [sha256.Size]byte

func (transport *processDesktopEnvelopeTransport) verifyIdentity(ctx context.Context) error {
	if transport.command == "" || transport.artifactPath == "" || transport.executableInfo == nil {
		return errDesktopProviderUnavailable
	}
	executable, err := desktopLocalProviderExecutable(transport.artifactPath)
	if err != nil || executable != transport.command {
		return errDesktopProviderUnavailable
	}
	before, err := os.Lstat(executable)
	if err != nil || !sameDesktopExecutableInfo(transport.executableInfo, before) {
		return errDesktopProviderUnavailable
	}
	info, digest, codeIdentity, err := snapshotDesktopExecutable(
		ctx, executable, transport.executableInfo.Size(),
	)
	if err != nil || !sameDesktopExecutableInfo(transport.executableInfo, info) ||
		digest != transport.executableDigest || codeIdentity != transport.codeIdentity {
		return errDesktopProviderUnavailable
	}
	return nil
}

func snapshotDesktopExecutable(
	ctx context.Context,
	path string,
	expectedSize int64,
) (os.FileInfo, desktopExecutableDigest, desktopCodeIdentity, error) {
	if err := ctx.Err(); err != nil {
		return nil, desktopExecutableDigest{}, desktopCodeIdentity{}, err
	}
	before, err := os.Lstat(path)
	if err != nil || before.Mode()&os.ModeSymlink != 0 || !before.Mode().IsRegular() {
		return nil, desktopExecutableDigest{}, desktopCodeIdentity{}, errDesktopProviderUnavailable
	}
	if expectedSize < 0 {
		expectedSize = before.Size()
	}
	if expectedSize < 0 || expectedSize > maxDesktopProviderExecutableBytes || before.Size() != expectedSize {
		return nil, desktopExecutableDigest{}, desktopCodeIdentity{}, errDesktopProviderUnavailable
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, desktopExecutableDigest{}, desktopCodeIdentity{}, errDesktopProviderUnavailable
	}
	openedInfo, err := file.Stat()
	if err != nil || !sameDesktopExecutableInfo(before, openedInfo) {
		_ = file.Close()
		return nil, desktopExecutableDigest{}, desktopCodeIdentity{}, errDesktopProviderUnavailable
	}
	hash := sha256.New()
	reader := io.LimitReader(desktopContextReader{ctx: ctx, reader: file}, expectedSize+1)
	copied, copyErr := io.Copy(hash, reader)
	codeIdentity, identityErr := readDesktopCodeIdentity(file, expectedSize)
	closeErr := file.Close()
	if copyErr != nil || identityErr != nil || closeErr != nil || ctx.Err() != nil || copied != expectedSize {
		return nil, desktopExecutableDigest{}, desktopCodeIdentity{}, errDesktopProviderUnavailable
	}
	after, err := os.Lstat(path)
	if err != nil || !sameDesktopExecutableInfo(before, after) {
		return nil, desktopExecutableDigest{}, desktopCodeIdentity{}, errDesktopProviderUnavailable
	}
	var digest desktopExecutableDigest
	copy(digest[:], hash.Sum(nil))
	return after, digest, codeIdentity, nil
}

type desktopContextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (reader desktopContextReader) Read(value []byte) (int, error) {
	if err := reader.ctx.Err(); err != nil {
		return 0, err
	}
	count, err := reader.reader.Read(value)
	if contextErr := reader.ctx.Err(); contextErr != nil {
		return count, contextErr
	}
	return count, err
}

func sameDesktopExecutableInfo(expected, actual os.FileInfo) bool {
	return expected != nil && actual != nil && os.SameFile(expected, actual) &&
		expected.Mode() == actual.Mode() && expected.Size() == actual.Size() &&
		expected.ModTime().Equal(actual.ModTime())
}
