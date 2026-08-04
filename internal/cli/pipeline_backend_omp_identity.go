package cli

import (
	"crypto/sha256"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type pipelineOMPExecutableIdentity struct {
	info    os.FileInfo
	size    int64
	mode    os.FileMode
	modTime int64
	digest  [sha256.Size]byte
	owner   pipelineOMPExecutableOwnerIdentity
}

// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-OMP-004: executable canonicalization pins the external OMP process identity.
// @AX:REASON [AUTO]: Path resolution, regular-file checks, execute bits, and captured metadata must agree before spawn.
func canonicalPipelineOMPExecutable(path string) (string, pipelineOMPExecutableIdentity, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", pipelineOMPExecutableIdentity{}, errors.New("OMP pipeline executable path is invalid")
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", pipelineOMPExecutableIdentity{}, errors.New("OMP pipeline executable identity is unavailable")
	}
	identity, err := capturePipelineOMPExecutableIdentity(resolved)
	return filepath.Clean(resolved), identity, err
}

func verifyPipelineOMPExecutable(path string, expected pipelineOMPExecutableIdentity) error {
	observed, err := capturePipelineOMPExecutableIdentity(path)
	if err != nil || expected.info == nil || !os.SameFile(expected.info, observed.info) ||
		observed.size != expected.size || observed.mode != expected.mode || observed.modTime != expected.modTime ||
		observed.digest != expected.digest || observed.owner != expected.owner {
		return errors.New("OMP pipeline executable identity changed before launch")
	}
	return nil
}

// @AX:WARN [AUTO]: Executable capture has cyclomatic complexity 16 across file mode, digest, identity, and owner checks.
// @AX:REASON [AUTO]: The same regular file and owner must survive open, hashing, stat, and close before it can be trusted for spawn.
func capturePipelineOMPExecutableIdentity(path string) (pipelineOMPExecutableIdentity, error) {
	before, err := os.Lstat(path)
	if err != nil || !before.Mode().IsRegular() || before.Mode()&0o111 == 0 || before.Mode()&os.ModeSymlink != 0 {
		return pipelineOMPExecutableIdentity{}, errors.New("OMP pipeline executable identity is unsafe")
	}
	file, err := os.Open(path)
	if err != nil {
		return pipelineOMPExecutableIdentity{}, errors.New("OMP pipeline executable identity is unreadable")
	}
	hash := sha256.New()
	_, copyErr := io.Copy(hash, file)
	after, statErr := file.Stat()
	closeErr := file.Close()
	if copyErr != nil || statErr != nil || closeErr != nil || !os.SameFile(before, after) ||
		after.Size() != before.Size() || after.Mode() != before.Mode() || !after.ModTime().Equal(before.ModTime()) {
		return pipelineOMPExecutableIdentity{}, errors.New("OMP pipeline executable identity changed while hashing")
	}
	beforeOwner, beforeOwnerErr := pipelineOMPExecutableOwner(before)
	afterOwner, afterOwnerErr := pipelineOMPExecutableOwner(after)
	if beforeOwnerErr != nil || afterOwnerErr != nil || beforeOwner != afterOwner {
		return pipelineOMPExecutableIdentity{}, errors.New("OMP pipeline executable ownership changed while hashing")
	}
	var digest [sha256.Size]byte
	copy(digest[:], hash.Sum(nil))
	return pipelineOMPExecutableIdentity{
		info: before, size: before.Size(), mode: before.Mode(), modTime: before.ModTime().UnixNano(), digest: digest,
		owner: beforeOwner,
	}, nil
}

func materializePipelineOMPExecutable(
	source string,
	expected pipelineOMPExecutableIdentity,
	runtimeRoot string,
) (string, error) {
	if err := verifyPipelineOMPExecutable(source, expected); err != nil {
		return "", err
	}
	input, err := os.Open(source)
	if err != nil {
		return "", errors.New("open verified OMP pipeline executable")
	}
	defer input.Close()
	destination := filepath.Join(runtimeRoot, "omp-verified")
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o700)
	if err != nil {
		return "", errors.New("create private OMP pipeline executable")
	}
	_, copyErr := io.Copy(output, input)
	syncErr := output.Sync()
	closeErr := output.Close()
	if copyErr != nil || syncErr != nil || closeErr != nil {
		return "", errors.New("materialize private OMP pipeline executable")
	}
	privatePath, privateIdentity, err := canonicalPipelineOMPExecutable(destination)
	if err != nil || privateIdentity.size != expected.size || privateIdentity.digest != expected.digest {
		return "", errors.New("private OMP pipeline executable verification failed")
	}
	if err := verifyPipelineOMPExecutable(source, expected); err != nil {
		return "", err
	}
	return privatePath, nil
}

func normalizePipelineOMPEnvironment(input []string) ([]string, error) {
	if len(input) == 0 {
		input = os.Environ()
	}
	result := make([]string, 0, len(input)+1)
	seen := make(map[string]bool, len(input)+1)
	for _, entry := range input {
		key, value, found := strings.Cut(entry, "=")
		if !found || key == "" || strings.ContainsAny(key, "\x00\r\n") || strings.ContainsRune(value, 0) || seen[key] {
			return nil, errors.New("OMP pipeline environment is malformed or duplicated")
		}
		seen[key] = true
		if key == "AUTOPUS_OMP_MANAGED_INNER" || dangerousPipelineOMPEnvironmentKey(key) {
			continue
		}
		result = append(result, entry)
	}
	return append(result, "AUTOPUS_OMP_MANAGED_INNER=1"), nil
}

func dangerousPipelineOMPEnvironmentKey(key string) bool {
	switch key {
	case "NODE_OPTIONS", "NODE_PATH", "BUN_OPTIONS", "BUN_RUNTIME_TRANSPILER_CACHE_PATH":
		return true
	default:
		return strings.HasPrefix(key, "DYLD_") || strings.HasPrefix(key, "LD_")
	}
}
