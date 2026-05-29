package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/setup"
)

type workspaceUpdateTarget struct {
	Name    string
	Path    string
	AbsPath string
}

type workspaceUpdateSkip struct {
	Path   string
	Reason string
}

func runWorkspaceUpdate(cmd *cobra.Command, dir, only string, yes, preview bool, statusLine string) error {
	if !preview && !yes {
		return fmt.Errorf("workspace 업데이트는 여러 리포에 쓰기 때문에 --yes 또는 --auto가 필요합니다 (--plan으로 먼저 확인 가능, 현재 repo만이면 --local)")
	}
	if err := validateStatusLineMode(statusLine); err != nil {
		return err
	}

	targets, skips, err := collectWorkspaceUpdateTargets(dir, only)
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	mode := "update"
	if preview {
		mode = "update plan"
	}
	fmt.Fprintf(out, "Workspace %s: %d target(s)", mode, len(targets))
	if len(skips) > 0 {
		fmt.Fprintf(out, ", %d skipped", len(skips))
	}
	fmt.Fprintln(out)
	for _, skip := range skips {
		fmt.Fprintf(out, "  - %s skipped: %s\n", skip.Path, skip.Reason)
	}

	processed := 0
	for _, target := range targets {
		fmt.Fprintf(out, "\n[%s] %s\n", target.Path, target.AbsPath)
		if err := runNestedUpdateCommand(cmd, target, yes, preview, statusLine); err != nil {
			return fmt.Errorf("%s 업데이트 실패: %w", target.Path, err)
		}
		processed++
	}

	if preview {
		fmt.Fprintf(out, "\nWorkspace update plan complete: %d repo(s) planned\n", processed)
		return nil
	}
	fmt.Fprintf(out, "\nWorkspace update complete: %d repo(s) updated\n", processed)
	return nil
}

func collectWorkspaceUpdateTargets(dir, only string) ([]workspaceUpdateTarget, []workspaceUpdateSkip, error) {
	components, err := detectWorkspaceComponents(dir)
	if err != nil {
		return nil, nil, err
	}
	filters := parseWorkspaceOnly(only)
	matched := make(map[string]bool, len(filters))

	var targets []workspaceUpdateTarget
	var skips []workspaceUpdateSkip
	for _, component := range components {
		if len(filters) > 0 {
			ok, matchedFilter := workspaceComponentMatches(component, filters)
			if !ok {
				continue
			}
			matched[matchedFilter] = true
		}

		target := workspaceUpdateTarget{
			Name:    component.Name,
			Path:    component.Path,
			AbsPath: component.AbsPath,
		}
		if !hasAutopusConfig(target.AbsPath) {
			skips = append(skips, workspaceUpdateSkip{
				Path:   target.Path,
				Reason: "missing autopus.yaml",
			})
			continue
		}
		targets = append(targets, target)
	}

	for _, filter := range filters {
		if !matched[filter] {
			return nil, nil, fmt.Errorf("--only 대상 %q을 찾을 수 없습니다", filter)
		}
	}
	return targets, skips, nil
}

func detectWorkspaceComponents(dir string) ([]workspaceUpdateTarget, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}

	if info := setup.DetectMultiRepo(absDir); info != nil && len(info.Components) > 0 {
		targets := make([]workspaceUpdateTarget, 0, len(info.Components))
		for _, component := range info.Components {
			targets = append(targets, workspaceUpdateTarget{
				Name:    component.Name,
				Path:    component.Path,
				AbsPath: component.AbsPath,
			})
		}
		return targets, nil
	}

	return []workspaceUpdateTarget{{
		Name:    filepath.Base(absDir),
		Path:    ".",
		AbsPath: absDir,
	}}, nil
}

func shouldAutoWorkspaceUpdate(dir string) bool {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return false
	}
	if !hasWorkspacePolicy(absDir) {
		return false
	}
	if info := setup.DetectMultiRepo(absDir); info == nil || len(info.Components) < 2 {
		return false
	}
	return true
}

func hasWorkspacePolicy(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".autopus", "project", "workspace.md"))
	return err == nil
}

func parseWorkspaceOnly(only string) []string {
	var filters []string
	for _, raw := range strings.Split(only, ",") {
		filter := filepath.ToSlash(strings.TrimSpace(raw))
		if filter == "" {
			continue
		}
		filters = append(filters, filter)
	}
	return filters
}

func workspaceComponentMatches(component workspaceUpdateTarget, filters []string) (bool, string) {
	for _, filter := range filters {
		switch filter {
		case component.Path, component.Name:
			return true, filter
		}
		if strings.TrimPrefix(filter, "./") == component.Path {
			return true, filter
		}
	}
	return false, ""
}

func hasAutopusConfig(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "autopus.yaml"))
	return err == nil
}

func runNestedUpdateCommand(parent *cobra.Command, target workspaceUpdateTarget, yes, preview bool, statusLine string) error {
	args := []string{"update", "--dir", target.AbsPath, "--local"}
	if preview {
		args = append(args, "--plan")
	}
	if yes {
		args = append(args, "--yes")
	}
	if statusLine != "" {
		args = append(args, "--statusline-mode", statusLine)
	}

	nested := NewRootCmd()
	nested.SetArgs(args)
	nested.SetOut(parent.OutOrStdout())
	nested.SetErr(parent.ErrOrStderr())
	if in := parent.InOrStdin(); in != nil {
		nested.SetIn(in)
	}
	if parent.Context() != nil {
		nested.SetContext(parent.Context())
	}
	return nested.Execute()
}
