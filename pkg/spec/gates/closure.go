package gates

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// MissingInputError reports explicitly named inputs absent from the tree.
type MissingInputError struct {
	Paths []string
}

func (e *MissingInputError) Error() string {
	return "missing input: " + strings.Join(e.Paths, ", ")
}

// ResolveClosure hashes every regular file under root matched by inputGlobs
// and every explicitly named dynamic dependency. Directories and symlinks are
// skipped; results are sorted by path and deduplicated. A dynamic dependency
// that does not exist yields a *MissingInputError alongside the entries that
// could be resolved.
func ResolveClosure(root string, inputGlobs, dynamicDeps []string) (inputs, deps []InputEntry, err error) {
	seen := map[string]bool{}
	for _, glob := range inputGlobs {
		matched, globErr := expandGlob(root, glob)
		if globErr != nil {
			return nil, nil, globErr
		}
		for _, rel := range matched {
			if seen[rel] {
				continue
			}
			seen[rel] = true
			entry, hashErr := hashEntry(root, rel)
			if hashErr != nil {
				return nil, nil, hashErr
			}
			inputs = append(inputs, entry)
		}
	}
	var missing []string
	for _, dep := range normalizePaths(dynamicDeps) {
		entry, hashErr := hashEntry(root, dep)
		switch {
		case errors.Is(hashErr, fs.ErrNotExist):
			missing = append(missing, dep)
			continue
		case hashErr != nil:
			return nil, nil, hashErr
		}
		deps = append(deps, entry)
	}
	sortEntries(inputs)
	sortEntries(deps)
	if len(missing) > 0 {
		return inputs, deps, &MissingInputError{Paths: missing}
	}
	return inputs, deps, nil
}

// ClosureSHA256 hashes the union of the given entry groups: entries are
// deduplicated by path, sorted, and fed as canonical "path\x00sha256\n" lines.
func ClosureSHA256(groups ...[]InputEntry) string {
	byPath := map[string]string{}
	for _, group := range groups {
		for _, entry := range group {
			byPath[entry.Path] = entry.SHA256
		}
	}
	paths := make([]string, 0, len(byPath))
	for p := range byPath {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	h := sha256.New()
	for _, p := range paths {
		h.Write([]byte(p))
		h.Write([]byte{0})
		h.Write([]byte(byPath[p]))
		h.Write([]byte{'\n'})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// expandGlob returns the slash-relative regular files under root matching
// pattern. Patterns without meta characters resolve to a single literal path
// (absent files are simply not matched); the walk starts at the literal
// directory prefix of the pattern and never descends into .git.
func expandGlob(root, pattern string) ([]string, error) {
	pattern = normalizeRel(pattern)
	if pattern == "" {
		return nil, fmt.Errorf("empty input glob")
	}
	if strings.HasPrefix(pattern, "../") || filepath.IsAbs(pattern) {
		return nil, fmt.Errorf("input glob %q escapes the project root", pattern)
	}
	if !hasMeta(pattern) {
		if isRegular(filepath.Join(root, filepath.FromSlash(pattern))) {
			return []string{pattern}, nil
		}
		return nil, nil
	}
	prefix, _ := literalPrefix(pattern)
	start := filepath.Join(root, filepath.FromSlash(prefix))
	if info, err := os.Lstat(start); err != nil || !info.IsDir() {
		return nil, nil
	}
	var matched []string
	walkErr := filepath.WalkDir(start, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" && p != start {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		rel, relErr := filepath.Rel(root, p)
		if relErr != nil {
			return relErr
		}
		if MatchPattern(pattern, filepath.ToSlash(rel)) {
			matched = append(matched, filepath.ToSlash(rel))
		}
		return nil
	})
	if walkErr != nil {
		return nil, fmt.Errorf("expand %q: %w", pattern, walkErr)
	}
	sort.Strings(matched)
	return matched, nil
}

func hashEntry(root, rel string) (InputEntry, error) {
	full := filepath.Join(root, filepath.FromSlash(rel))
	info, err := os.Lstat(full)
	if err != nil {
		return InputEntry{}, err
	}
	if !info.Mode().IsRegular() {
		return InputEntry{}, fmt.Errorf("input %q is not a regular file", rel)
	}
	f, err := os.Open(full)
	if err != nil {
		return InputEntry{}, err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return InputEntry{}, fmt.Errorf("hash %q: %w", rel, err)
	}
	return InputEntry{Path: rel, SHA256: hex.EncodeToString(h.Sum(nil))}, nil
}

func isRegular(full string) bool {
	info, err := os.Lstat(full)
	return err == nil && info.Mode().IsRegular()
}

func sortEntries(entries []InputEntry) {
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
}
