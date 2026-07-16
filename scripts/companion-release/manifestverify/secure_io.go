package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"os"
)

const maximumSigningKeyBytes = 4096

type regularFile struct {
	file   *os.File
	path   string
	opened os.FileInfo
}

func readRegularFile(path string, maximum int64) ([]byte, error) {
	source, err := openRegularFile(path)
	if err != nil {
		return nil, err
	}
	defer source.file.Close()
	size := source.opened.Size()
	if size < 1 || size > maximum {
		return nil, errors.New("invalid regular file size")
	}
	data, readErr := io.ReadAll(io.LimitReader(source.file, maximum+1))
	identityErr := source.verifyAfterRead()
	if readErr != nil || identityErr != nil || int64(len(data)) != size {
		return nil, errors.New("read regular file")
	}
	return data, nil
}

func hashRegularFile(path string) (string, error) {
	source, err := openRegularFile(path)
	if err != nil || source.opened.Size() < 1 {
		if source != nil {
			source.file.Close()
		}
		return "", errors.New("invalid artifact")
	}
	defer source.file.Close()
	hash := sha256.New()
	written, copyErr := io.Copy(hash, source.file)
	identityErr := source.verifyAfterRead()
	if copyErr != nil || identityErr != nil || written != source.opened.Size() {
		return "", errors.New("digest artifact")
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil)), nil
}

func openRegularFile(path string) (*regularFile, error) {
	before, err := os.Lstat(path)
	if err != nil || !before.Mode().IsRegular() {
		return nil, errors.New("invalid path")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	opened, openErr := file.Stat()
	after, pathErr := os.Lstat(path)
	if openErr != nil || pathErr != nil || !opened.Mode().IsRegular() ||
		!after.Mode().IsRegular() || !os.SameFile(before, opened) ||
		!os.SameFile(opened, after) || before.Size() != opened.Size() ||
		after.Size() != opened.Size() {
		file.Close()
		return nil, errors.New("file identity changed before read")
	}
	return &regularFile{file: file, path: path, opened: opened}, nil
}

func (source *regularFile) verifyAfterRead() error {
	descriptor, descriptorErr := source.file.Stat()
	path, pathErr := os.Lstat(source.path)
	if descriptorErr != nil || pathErr != nil || !descriptor.Mode().IsRegular() ||
		!path.Mode().IsRegular() || !os.SameFile(source.opened, descriptor) ||
		!os.SameFile(descriptor, path) || descriptor.Size() != source.opened.Size() ||
		path.Size() != source.opened.Size() {
		return errors.New("file identity changed after read")
	}
	return nil
}

func publicKeySHA256FromPrivateKey(path string) (string, error) {
	encoded, err := readRegularFile(path, maximumSigningKeyBytes)
	if err != nil {
		return "", errors.New("read release signing key")
	}
	defer clear(encoded)
	trimmed := bytes.TrimSpace(encoded)
	encoding := base64.StdEncoding.Strict()
	decoded := make([]byte, encoding.DecodedLen(len(trimmed)))
	defer clear(decoded)
	decodedLength, err := encoding.Decode(decoded, trimmed)
	if err != nil || decodedLength != ed25519.PrivateKeySize {
		return "", errors.New("invalid release signing key")
	}
	privateKey := ed25519.PrivateKey(decoded[:decodedLength])
	seed := privateKey.Seed()
	normalized := ed25519.NewKeyFromSeed(seed)
	clear(seed)
	defer clear(normalized)
	if subtle.ConstantTimeCompare(privateKey, normalized) != 1 {
		return "", errors.New("inconsistent release signing key")
	}
	digest := sha256.Sum256(normalized[ed25519.SeedSize:])
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}

func sameDigest(data []byte, expected string) bool {
	digest := sha256.Sum256(data)
	actual := "sha256:" + hex.EncodeToString(digest[:])
	return subtle.ConstantTimeCompare([]byte(actual), []byte(expected)) == 1
}
