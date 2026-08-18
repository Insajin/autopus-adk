package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/insajin/autopus-adk/pkg/config"
)

// requireHarnessProject refuses a directory that holds no autopus.yaml.
//
// config.Load synthesizes DefaultFullConfig(filepath.Base(dir)) for a missing
// file, so every command that acts on an *existing* harness has to gate on
// config.Exists first. Without the gate, running from a directory that merely
// happens to be the shell's cwd — an Orca workspace root sitting above the git
// worktree, say — makes doctor diagnose a project that does not exist and makes
// update plan a full scaffold there.
//
// auto init is deliberately not a caller: creating the config is its job.
func requireHarnessProject(dir string) error {
	if config.Exists(dir) {
		return nil
	}
	return fmt.Errorf(
		"이 디렉터리에는 autopus 프로젝트가 없습니다(autopus.yaml 부재): %s\n"+
			"`auto init`으로 초기화하거나 `--dir`로 프로젝트 루트를 지정하세요",
		harnessProjectDisplayDir(dir),
	)
}

// resolveProjectDir resolves a --dir flag the way resolveDir does and then
// requires the result to be a harness project.
func resolveProjectDir(dir string) (string, error) {
	resolved, err := resolveDir(dir)
	if err != nil {
		return "", err
	}
	if err := requireHarnessProject(resolved); err != nil {
		return "", err
	}
	return resolved, nil
}

// harnessProjectLabel is the banner label for a command operating on dir.
// Callers used to pass the literal "autopus-adk", which printed this
// repository's own name no matter which project was being operated on.
func harnessProjectLabel(dir string, cfg *config.HarnessConfig) string {
	if cfg != nil {
		if name := strings.TrimSpace(cfg.ProjectName); name != "" {
			return name
		}
	}
	return filepath.Base(harnessProjectDisplayDir(dir))
}

// harnessProjectDisplayDir absolutizes dir so that a relative "." reads as a
// real path in messages and yields a usable base name.
func harnessProjectDisplayDir(dir string) string {
	if abs, err := filepath.Abs(dir); err == nil {
		return abs
	}
	return dir
}
