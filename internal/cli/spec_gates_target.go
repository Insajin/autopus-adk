package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/insajin/autopus-adk/pkg/spec"
)

// gatesTarget is a resolved SPEC together with the project root that
// changed paths and evidence inputs are relative to.
type gatesTarget struct {
	SpecID  string
	SpecDir string
	Root    string
}

// resolveGatesTarget accepts either a SPEC directory or a SPEC ID. A SPEC
// directory laid out as {root}/.autopus/specs/{id} yields that root; an ID is
// resolved through the shared resolver and its module becomes the root.
func resolveGatesTarget(arg string) (gatesTarget, error) {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return gatesTarget{}, fmt.Errorf("SPEC ID or SPEC directory is required")
	}
	if info, err := os.Stat(filepath.Join(arg, "spec.md")); err == nil && !info.IsDir() {
		specDir := filepath.Clean(arg)
		root := "."
		specsDir := filepath.Dir(specDir)
		if filepath.Base(specsDir) == "specs" && filepath.Base(filepath.Dir(specsDir)) == ".autopus" {
			root = filepath.Dir(filepath.Dir(specsDir))
		}
		return gatesTarget{SpecID: filepath.Base(specDir), SpecDir: specDir, Root: root}, nil
	}
	resolved, err := spec.ResolveSpecDir(".", arg)
	if err != nil {
		return gatesTarget{}, fmt.Errorf("SPEC 로드 실패: %w", err)
	}
	return gatesTarget{SpecID: arg, SpecDir: resolved.SpecDir, Root: resolved.TargetModule}, nil
}

// resolveGatesChangeSet returns the change set for the applicability
// decision: --changed wins, otherwise git reports paths relative to root
// that differ from base (HEAD by default) plus untracked files.
func resolveGatesChangeSet(root, changed, base string) ([]string, error) {
	if strings.TrimSpace(changed) != "" {
		return relativizePaths(root, splitCommaList(changed)), nil
	}
	ref := strings.TrimSpace(base)
	if ref == "" {
		ref = "HEAD"
	}
	commands := [][]string{
		{"diff", "--name-only", "--relative", "--diff-filter=ACMRD", ref},
		{"ls-files", "--others", "--exclude-standard"},
	}
	seen := map[string]bool{}
	var paths []string
	for _, args := range commands {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		out, err := cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("git %s failed in %s (pass --changed to supply the change set): %w", strings.Join(args, " "), root, err)
		}
		for _, line := range strings.Split(string(out), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || seen[line] {
				continue
			}
			seen[line] = true
			paths = append(paths, line)
		}
	}
	sort.Strings(paths)
	return paths, nil
}

// relativizePaths rewrites absolute paths under root as root-relative so the
// classifier sees module roots rather than filesystem prefixes.
func relativizePaths(root string, paths []string) []string {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return paths
	}
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		if filepath.IsAbs(p) {
			if rel, relErr := filepath.Rel(absRoot, p); relErr == nil && !strings.HasPrefix(rel, "..") {
				p = rel
			}
		}
		out = append(out, filepath.ToSlash(p))
	}
	return out
}

func splitCommaList(value string) []string {
	var out []string
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}
