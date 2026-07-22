package companionmanifest

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

const releaseVectorSeed = "9d61b19deffd5a60ba844af492ec2cc44449c5697b326919703bac031cae7f60"

func TestProductionGoReleaserDarwinArchives_ContainLinkedCompanionEvidence(t *testing.T) {
	tools := newMockReleaseTools(t)
	archivePaths := runProductionGoReleaser(t, tools)
	archives := make(map[string]map[string]releaseArchiveEntry)
	for _, architecture := range []string{"amd64", "arm64"} {
		archives[architecture] = readReleaseArchive(t, archivePaths[architecture])
		if err := validateProductionDarwinArchive(
			archives[architecture], architecture, testReleasePrivateKey(t),
		); err != nil {
			t.Fatalf("%s production archive: %v", architecture, err)
		}
	}
	for _, name := range []string{
		"adk-companion-public-key-receipt.bundle/public-key-receipt.json",
		"adk-companion-public-key-receipt.bundle/public-key-receipt.sig",
	} {
		if !bytes.Equal(archives["amd64"][name].data, archives["arm64"][name].data) {
			t.Fatalf("Darwin archive receipt record differs for %s", name)
		}
	}
	assertExecutableArchiveWiringTamperingFails(t, tools, testReleasePrivateKey(t))
}

func TestMockedDarwinRelease_FailureRemovesTemporaryStaging(t *testing.T) {
	tools := newMockReleaseTools(t)
	artifactDir, output, err := runMockedRelease(t, tools, "arm64", "Invalid")
	if err == nil {
		t.Fatalf("rejected notarization was accepted\n%s", output)
	}
	if !bytes.Contains(output, []byte("notarization was not Accepted")) {
		t.Fatalf("mocked release failed before notarization oracle: %v\n%s", err, output)
	}
	entries, readErr := os.ReadDir(artifactDir)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if len(entries) != 1 || entries[0].Name() != "auto" {
		t.Fatalf("failed producer left outputs: %v", entries)
	}
	matches, globErr := filepath.Glob(filepath.Join(filepath.Dir(artifactDir), "tmp", "adk-companion-release.*"))
	if globErr != nil || len(matches) != 0 {
		t.Fatalf("temporary staging remains after failure: %v / %v", matches, globErr)
	}
}

type releaseArchiveEntry struct {
	data []byte
	mode int64
}

func readReleaseArchive(t *testing.T, archivePath string) map[string]releaseArchiveEntry {
	t.Helper()
	input, err := os.Open(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if closeErr := input.Close(); closeErr != nil {
			t.Errorf("close release archive: %v", closeErr)
		}
	}()
	entries, err := decodeReleaseArchive(input)
	if err != nil {
		t.Fatal(err)
	}
	return entries
}

func decodeReleaseArchive(input io.Reader) (map[string]releaseArchiveEntry, error) {
	gzipReader, err := gzip.NewReader(input)
	if err != nil {
		return nil, err
	}
	entries := make(map[string]releaseArchiveEntry)
	reader := tar.NewReader(gzipReader)
	for {
		header, nextErr := reader.Next()
		if nextErr == io.EOF {
			break
		}
		if nextErr != nil {
			_ = gzipReader.Close()
			return nil, nextErr
		}
		data, readErr := io.ReadAll(reader)
		if readErr != nil {
			_ = gzipReader.Close()
			return nil, readErr
		}
		entries[header.Name] = releaseArchiveEntry{data: data, mode: header.Mode}
	}
	if _, err := io.Copy(io.Discard, gzipReader); err != nil {
		_ = gzipReader.Close()
		return nil, err
	}
	if err := gzipReader.Close(); err != nil {
		return nil, err
	}
	return entries, nil
}

func testReleasePrivateKey(t *testing.T) ed25519.PrivateKey {
	t.Helper()
	seed, err := hex.DecodeString(releaseVectorSeed)
	if err != nil {
		t.Fatal(err)
	}
	return ed25519.NewKeyFromSeed(seed)
}

func buildReleaseBinary(t *testing.T, output, packagePath string) {
	t.Helper()
	command := exec.Command("go", "build", "-trimpath", "-o", output, packagePath)
	command.Dir = repositoryRoot(t)
	if result, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build %s: %v\n%s", packagePath, err, result)
	}
	if err := os.Chmod(output, 0o700); err != nil {
		t.Fatal(err)
	}
}

func writeExecutable(t *testing.T, path, source string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(source), 0o700); err != nil {
		t.Fatal(err)
	}
}

func encodedReleaseKey(t *testing.T) []byte {
	t.Helper()
	return []byte(base64.StdEncoding.EncodeToString(testReleasePrivateKey(t)))
}
