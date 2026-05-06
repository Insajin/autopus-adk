package skillevolve

import (
	"encoding/json"
	"errors"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type qameshManifest struct {
	Status              string `json:"status"`
	ReproductionCommand string `json:"reproduction_command"`
	Runner              struct {
		Command string `json:"command"`
	} `json:"runner"`
}

func replayCommandEvidence(runIndexPath string, runIndex qameshRunIndex, projectDir, expectedCommand string, failures []string) ([]ReplayCommand, []string) {
	dir := filepath.Dir(runIndexPath)
	if expectedCommand == "" {
		return nil, failures
	}
	if len(runIndex.ManifestPaths) == 0 {
		return nil, appendReason(failures, "replay_manifest_missing")
	}
	commands := make([]ReplayCommand, 0, len(runIndex.ManifestPaths))
	for _, manifestRel := range runIndex.ManifestPaths {
		manifestPath, ok := resolveReplayManifestPath(dir, manifestRel, projectDir)
		if !ok {
			failures = appendReason(failures, "replay_manifest_path_invalid")
			continue
		}
		manifest, err := readManifest(manifestPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				failures = appendReason(failures, "replay_manifest_missing")
			} else {
				failures = appendReason(failures, "replay_manifest_invalid")
			}
			continue
		}
		if manifest.Status != "passed" {
			failures = appendReason(failures, "replay_manifest_not_passed")
			continue
		}
		command := manifest.ReproductionCommand
		if command == "" {
			command = manifest.Runner.Command
		}
		if command == "" {
			failures = appendReason(failures, "replay_manifest_command_missing")
			continue
		}
		if expectedCommand != "" && command != expectedCommand {
			failures = appendReason(failures, "replay_command_mismatch")
			continue
		}
		commands = append(commands, ReplayCommand{Command: command})
	}
	return commands, failures
}

func readManifest(path string) (qameshManifest, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return qameshManifest{}, err
	}
	var manifest qameshManifest
	if err := json.Unmarshal(body, &manifest); err != nil {
		return qameshManifest{}, err
	}
	return manifest, nil
}

func resolveReplayManifestPath(runIndexDir, manifestRel, projectDir string) (string, bool) {
	rel := strings.TrimSpace(strings.ReplaceAll(manifestRel, "\\", "/"))
	if rel == "" || path.IsAbs(rel) {
		return "", false
	}
	clean := path.Clean(rel)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", false
	}
	manifestPath := filepath.Join(runIndexDir, filepath.FromSlash(clean))
	if !pathWithinDirForRead(runIndexDir, manifestPath) {
		return "", false
	}
	if !pathWithinProjectForRead(projectDir, manifestPath) {
		return "", false
	}
	return manifestPath, true
}

func pathWithinDirForRead(root, target string) bool {
	if root == "" || target == "" {
		return false
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	if resolvedRoot, err := filepath.EvalSymlinks(rootAbs); err == nil {
		rootAbs = resolvedRoot
	}
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return false
	}
	if resolvedTarget, err := filepath.EvalSymlinks(targetAbs); err == nil {
		targetAbs = resolvedTarget
	}
	rel, err := filepath.Rel(rootAbs, targetAbs)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
