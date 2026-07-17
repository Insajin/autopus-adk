package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var specIDValidPattern = regexp.MustCompile(`^SPEC-[A-Z0-9-]+$`)
var specIDInPath = regexp.MustCompile(`^\.autopus/specs/(SPEC-[A-Z0-9-]+)/`)

type specHost struct {
	repoPath string
	absDir   string
}

func validateSpecID(id string) error {
	if !specIDValidPattern.MatchString(id) {
		return fmt.Errorf("invalid --spec: must match ^SPEC-[A-Z0-9-]+$")
	}
	return nil
}

func specIDFromPath(rel string) (string, bool) {
	match := specIDInPath.FindStringSubmatch(rel)
	if match == nil {
		return "", false
	}
	return match[1], true
}

func locateSpecHost(repos []repoDirty, specID string) (specHost, error) {
	var hosts []specHost
	for _, repo := range repos {
		absDir, exists, err := inspectSpecDirectory(repo.AbsPath, specID)
		if err != nil {
			return specHost{}, err
		}
		if exists {
			hosts = append(hosts, specHost{repoPath: repo.Path, absDir: absDir})
		}
	}
	switch len(hosts) {
	case 0:
		return specHost{}, fmt.Errorf("sync verify: SPEC %s not found under any .autopus/specs tree", specID)
	case 1:
		return hosts[0], nil
	default:
		labels := make([]string, 0, len(hosts))
		for _, host := range hosts {
			labels = append(labels, diagnosticRepoLabel(host.repoPath))
		}
		sort.Strings(labels)
		return specHost{}, fmt.Errorf("sync verify: SPEC %s has multiple hosts: %s", specID, strings.Join(labels, ", "))
	}
}

func inspectSpecDirectory(repoRoot, specID string) (string, bool, error) {
	current := repoRoot
	for _, component := range []string{".autopus", "specs", specID} {
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			return "", false, nil
		}
		if err != nil {
			return "", false, fmt.Errorf("sync verify: cannot inspect SPEC %s containment", specID)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", false, fmt.Errorf("sync verify: SPEC %s path contains a symlink", specID)
		}
		if !info.IsDir() {
			return "", false, fmt.Errorf("sync verify: SPEC %s path component is not a directory", specID)
		}
	}
	rel, err := filepath.Rel(repoRoot, current)
	if err != nil || rel != filepath.Join(".autopus", "specs", specID) {
		return "", false, fmt.Errorf("sync verify: SPEC %s escaped its repository boundary", specID)
	}
	return current, true, nil
}

func readSpecReferences(host specHost, specID string) (string, error) {
	var text strings.Builder
	for _, name := range []string{"spec.md", "plan.md"} {
		data, err := readRegularSpecFile(filepath.Join(host.absDir, name), name, specID)
		if err != nil {
			return "", err
		}
		text.Write(data)
		text.WriteByte('\n')
	}
	return text.String(), nil
}

func readRegularSpecFile(file, name, specID string) ([]byte, error) {
	before, err := os.Lstat(file)
	if err != nil {
		return nil, fmt.Errorf("sync verify: cannot read %s for SPEC %s", name, specID)
	}
	if before.Mode()&os.ModeSymlink != 0 || !before.Mode().IsRegular() {
		return nil, fmt.Errorf("sync verify: %s for SPEC %s must be a regular non-symlink file", name, specID)
	}
	handle, err := os.Open(file)
	if err != nil {
		return nil, fmt.Errorf("sync verify: cannot read %s for SPEC %s", name, specID)
	}
	defer handle.Close()
	after, err := handle.Stat()
	if err != nil || !os.SameFile(before, after) {
		return nil, fmt.Errorf("sync verify: %s for SPEC %s changed during validation", name, specID)
	}
	data, err := io.ReadAll(handle)
	if err != nil {
		return nil, fmt.Errorf("sync verify: cannot read %s for SPEC %s", name, specID)
	}
	return data, nil
}
