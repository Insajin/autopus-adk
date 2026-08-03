package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// @AX:NOTE [AUTO] @AX:SPEC: SPEC-OMP-004: product skill surfaces are capped at 4 MiB and ambient OMP settings/config must be absent.
const workflowContextManagedProductSurfaceMaxBytes = 4 << 20

// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-OMP-004: project command/skill authority must equal the isolated generated surface.
// @AX:REASON [AUTO]: constructor admission and pre-run identity revalidation both depend on exact files plus an absent-only ambient denylist.
func validateWorkflowContextManagedProductSurface(options WorkflowContextManagedRPCOptions) error {
	if err := compareWorkflowContextManagedProductFile(
		filepath.Join(options.ProjectDir, ".omp", "config.yml"),
		filepath.Join(options.Workspace, ".omp", "config.yml"),
	); err != nil {
		return fmt.Errorf("managed OMP project config is not authoritative: %w", err)
	}
	names, err := workflowContextManagedProductCommandNames(options.Prompts[0])
	if err != nil {
		return err
	}
	for _, name := range names {
		project := filepath.Join(options.ProjectDir, ".agents", "skills", name)
		expected := filepath.Join(options.Workspace, ".agents", "skills", name)
		if err := compareWorkflowContextManagedProductSurface(project, expected); err != nil {
			return fmt.Errorf("managed OMP product skill %s is not authoritative: %w", name, err)
		}
		project = filepath.Join(options.ProjectDir, ".agents", "commands", name+".md")
		expected = filepath.Join(options.Workspace, ".agents", "commands", name+".md")
		if err := compareWorkflowContextManagedProductFile(project, expected); err != nil {
			return fmt.Errorf("managed OMP product command %s is not authoritative: %w", name, err)
		}
	}
	for _, relative := range []string{
		".agent", ".agents/SYSTEM.md", ".agents/prompts",
		".claude/SYSTEM.md", ".claude/commands", ".claude/extensions", ".claude/hooks",
		".claude/plugins", ".claude/tools", ".claude-plugin",
		".codex/AGENTS.md", ".codex/commands", ".codex/extensions", ".codex/hooks",
		".codex/prompts", ".codex/tools",
		".gemini/extensions", ".gemini/system.md", ".github/prompts",
		".github/instructions", ".github/copilot-instructions.md",
		".omp/SYSTEM.md", ".omp/agent", ".omp/commands", ".omp/hooks", ".omp/instructions",
		".omp/plugins", ".omp/prompts", ".omp/tools", ".omp/settings.json",
		".omp/config.json", ".omp/config.yaml",
		".opencode/commands", ".opencode/plugins", ".pi",
	} {
		if err := rejectWorkflowContextManagedAmbientPath(options.ProjectDir, relative); err != nil {
			return errors.New("managed OMP ambient project command or prompt source is not allowed")
		}
	}
	return nil
}

func rejectWorkflowContextManagedAmbientPath(root, relative string) error {
	current := root
	parts := strings.Split(filepath.FromSlash(relative), string(filepath.Separator))
	for index, part := range parts {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("ambient path identity is invalid")
		}
		if index == len(parts)-1 {
			return errors.New("ambient path exists")
		}
		if !info.IsDir() {
			return errors.New("ambient parent is not a directory")
		}
	}
	return nil
}

func compareWorkflowContextManagedProductFile(actualPath, expectedPath string) error {
	actual, err := captureWorkflowContextManagedSourceIdentity(actualPath)
	if err != nil {
		return err
	}
	expected, err := captureWorkflowContextManagedSourceIdentity(expectedPath)
	if err != nil {
		return err
	}
	if actual.size != expected.size || actual.sha256 != expected.sha256 {
		return errors.New("content differs")
	}
	return nil
}

func compareWorkflowContextManagedProductSurface(actualRoot, expectedRoot string) error {
	actual, err := readWorkflowContextManagedProductSurface(actualRoot)
	if err != nil {
		return err
	}
	expected, err := readWorkflowContextManagedProductSurface(expectedRoot)
	if err != nil {
		return err
	}
	if len(actual) != len(expected) {
		return errors.New("file set differs")
	}
	for path, body := range expected {
		if actualBody, ok := actual[path]; !ok || string(actualBody) != string(body) {
			return fmt.Errorf("content differs: %s", path)
		}
	}
	return nil
}

// @AX:WARN [AUTO]: product surface traversal has cyclomatic complexity 16 and nine fail-closed if branches.
// @AX:REASON [AUTO]: root identity, symlink refusal, regular-file limits, total size, and complete file-set readback converge here.
func readWorkflowContextManagedProductSurface(root string) (map[string][]byte, error) {
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("root identity is invalid")
	}
	result, total := map[string][]byte{}, 0
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := entry.Info()
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("path identity is invalid")
		}
		if entry.IsDir() {
			return nil
		}
		if !info.Mode().IsRegular() || info.Size() < 0 || info.Size() > workflowContextManagedProductSurfaceMaxBytes {
			return errors.New("file identity or size is invalid")
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		total += len(body)
		if total > workflowContextManagedProductSurfaceMaxBytes {
			return errors.New("surface exceeds size bound")
		}
		result[filepath.ToSlash(relative)] = body
		return nil
	})
	if err != nil || len(result) == 0 {
		return nil, errors.New("surface is unavailable")
	}
	return result, nil
}

func workflowContextManagedProductSkillNames(options WorkflowContextManagedRPCOptions) string {
	names, _ := workflowContextManagedProductCommandNames(options.Prompts[0])
	sort.Strings(names[:])
	return names[0] + "," + names[1]
}
