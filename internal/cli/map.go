package cli

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/internal/cli/tui"
	"github.com/insajin/autopus-adk/pkg/setup"
)

type mapRepoPayload struct {
	Name           string   `json:"name"`
	Path           string   `json:"path"`
	Role           string   `json:"role"`
	PrimaryLang    string   `json:"primary_language"`
	ModulePath     string   `json:"module_path,omitempty"`
	RemoteURL      string   `json:"remote_url,omitempty"`
	Branch         string   `json:"branch,omitempty"`
	Dirty          bool     `json:"dirty"`
	StatusLines    []string `json:"status_lines,omitempty"`
	TrackedIgnored []string `json:"tracked_ignored,omitempty"`
}

type mapPayload struct {
	ProjectDir          string           `json:"project_dir"`
	ProjectName         string           `json:"project_name"`
	MultiRepo           bool             `json:"multi_repo"`
	Repositories        []mapRepoPayload `json:"repositories"`
	Languages           []string         `json:"languages"`
	Frameworks          []string         `json:"frameworks"`
	BuildFiles          []string         `json:"build_files"`
	EntryPoints         []string         `json:"entry_points"`
	ProjectContextDir   string           `json:"project_context_dir,omitempty"`
	ProjectContextFiles []string         `json:"project_context_files,omitempty"`
}

func newMapCmd() *cobra.Command {
	var format string
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "map [dir]",
		Short: "Map project structure, repositories, and agent-relevant workspace state",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := resolveDirFromArgs(args)
			if err != nil {
				return err
			}
			jsonMode, err := resolveJSONMode(jsonOutput, format)
			if err != nil {
				return err
			}

			payload, warnings, err := buildMapPayload(dir)
			if err != nil {
				if jsonMode {
					return writeJSONResultAndExit(cmd, jsonStatusError, err, "map_failed", map[string]any{"project_dir": dir}, nil, nil)
				}
				return err
			}
			if jsonMode {
				status := jsonStatusOK
				if len(warnings) > 0 {
					status = jsonStatusWarn
				}
				return writeJSONResult(cmd, status, payload, warnings, nil)
			}
			writeMapText(cmd, payload, warnings)
			return nil
		},
	}
	addJSONFlags(cmd, &jsonOutput, &format)
	return cmd
}

func buildMapPayload(dir string) (mapPayload, []jsonMessage, error) {
	info, err := setup.Scan(dir)
	if err != nil {
		return mapPayload{}, nil, err
	}

	payload := mapPayload{
		ProjectDir:  info.RootDir,
		ProjectName: info.Name,
		MultiRepo:   info.MultiRepo != nil && info.MultiRepo.IsMultiRepo,
		Languages:   mapLanguages(info.Languages),
		Frameworks:  mapFrameworks(info.Frameworks),
		BuildFiles:  mapBuildFiles(info.BuildFiles),
		EntryPoints: mapEntryPoints(info.EntryPoints),
	}
	if context := setup.DetectProjectContext(info.RootDir); context.Exists {
		payload.ProjectContextDir = context.Dir
		payload.ProjectContextFiles = context.Files
	}

	components := []setup.RepoComponent{}
	if info.MultiRepo != nil {
		components = append(components, info.MultiRepo.Components...)
	} else if component, scanErr := setup.ScanRepoComponent(info.RootDir); scanErr == nil {
		components = append(components, *component)
	}

	warnings := []jsonMessage{}
	for _, component := range components {
		repo := mapRepoPayload{
			Name:        component.Name,
			Path:        component.Path,
			Role:        component.Role,
			PrimaryLang: component.PrimaryLanguage,
			ModulePath:  component.ModulePath,
			RemoteURL:   component.RemoteURL,
		}
		state := readRepoMapState(component.AbsPath)
		repo.Branch = state.branch
		repo.Dirty = state.dirty
		repo.StatusLines = state.statusLines
		repo.TrackedIgnored = state.trackedIgnored
		payload.Repositories = append(payload.Repositories, repo)

		if repo.Dirty {
			warnings = append(warnings, jsonMessage{
				Code:    "repo_dirty",
				Message: fmt.Sprintf("%s has uncommitted changes", displayRepoPath(repo.Path)),
			})
		}
		if len(repo.TrackedIgnored) > 0 {
			warnings = append(warnings, jsonMessage{
				Code:    "tracked_ignored",
				Message: fmt.Sprintf("%s has tracked files currently matched by ignore rules", displayRepoPath(repo.Path)),
			})
		}
	}
	sort.Slice(payload.Repositories, func(i, j int) bool {
		return payload.Repositories[i].Path < payload.Repositories[j].Path
	})
	return payload, warnings, nil
}

type repoMapState struct {
	branch         string
	dirty          bool
	statusLines    []string
	trackedIgnored []string
}

func readRepoMapState(dir string) repoMapState {
	state := repoMapState{}
	statusLines := runGitLines(dir, "status", "--short", "--branch")
	if len(statusLines) > 0 {
		state.branch = strings.TrimPrefix(statusLines[0], "## ")
		state.statusLines = append(state.statusLines, statusLines[1:]...)
		state.dirty = len(statusLines) > 1
	}
	state.trackedIgnored = runGitLines(dir, "ls-files", "-c", "-i", "--exclude-standard")
	sort.Strings(state.trackedIgnored)
	return state
}

func runGitLines(dir string, args ...string) []string {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil
	}
	text := strings.TrimSpace(out.String())
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}

func writeMapText(cmd *cobra.Command, payload mapPayload, warnings []jsonMessage) {
	out := cmd.OutOrStdout()
	tui.SectionHeader(out, "Workspace Map")
	fmt.Fprintf(out, "Project:     %s\n", payload.ProjectName)
	fmt.Fprintf(out, "Root:        %s\n", payload.ProjectDir)
	fmt.Fprintf(out, "Multi-repo:  %t\n\n", payload.MultiRepo)

	if payload.ProjectContextDir != "" {
		fmt.Fprintf(out, "Project context: %s (%d file(s))\n\n", payload.ProjectContextDir, len(payload.ProjectContextFiles))
	}

	tui.SectionHeader(out, "Repositories")
	if len(payload.Repositories) == 0 {
		tui.Info(out, "No git repositories detected.")
	} else {
		for _, repo := range payload.Repositories {
			status := "clean"
			if repo.Dirty {
				status = "dirty"
			}
			fmt.Fprintf(out, "- %s — %s, %s, %s", displayRepoPath(repo.Path), repo.Role, defaultMapText(repo.PrimaryLang, "Unknown"), status)
			if repo.Branch != "" {
				fmt.Fprintf(out, " [%s]", repo.Branch)
			}
			fmt.Fprintln(out)
			if len(repo.TrackedIgnored) > 0 {
				fmt.Fprintf(out, "  tracked ignored: %s\n", strings.Join(repo.TrackedIgnored, ", "))
			}
		}
	}

	if len(payload.Languages) > 0 || len(payload.Frameworks) > 0 {
		fmt.Fprintln(out)
		tui.SectionHeader(out, "Signals")
		if len(payload.Languages) > 0 {
			fmt.Fprintf(out, "Languages:  %s\n", strings.Join(payload.Languages, ", "))
		}
		if len(payload.Frameworks) > 0 {
			fmt.Fprintf(out, "Frameworks: %s\n", strings.Join(payload.Frameworks, ", "))
		}
		if len(payload.BuildFiles) > 0 {
			fmt.Fprintf(out, "Build files: %s\n", strings.Join(payload.BuildFiles, ", "))
		}
	}

	if len(warnings) > 0 {
		fmt.Fprintln(out)
		tui.SectionHeader(out, "Warnings")
		for _, warning := range warnings {
			tui.Warn(out, warning.Message)
		}
	}
}

func mapLanguages(languages []setup.Language) []string {
	values := make([]string, 0, len(languages))
	for _, language := range languages {
		if language.Version != "" {
			values = append(values, language.Name+" "+language.Version)
		} else {
			values = append(values, language.Name)
		}
	}
	sort.Strings(values)
	return values
}

func mapFrameworks(frameworks []setup.Framework) []string {
	values := make([]string, 0, len(frameworks))
	for _, framework := range frameworks {
		if framework.Version != "" {
			values = append(values, framework.Name+" "+framework.Version)
		} else {
			values = append(values, framework.Name)
		}
	}
	sort.Strings(values)
	return values
}

func mapBuildFiles(buildFiles []setup.BuildFile) []string {
	values := make([]string, 0, len(buildFiles))
	for _, buildFile := range buildFiles {
		values = append(values, filepath.ToSlash(buildFile.Path))
	}
	sort.Strings(values)
	return values
}

func mapEntryPoints(entryPoints []setup.EntryPoint) []string {
	values := make([]string, 0, len(entryPoints))
	for _, entryPoint := range entryPoints {
		values = append(values, filepath.ToSlash(entryPoint.Path))
	}
	sort.Strings(values)
	return values
}

func displayRepoPath(path string) string {
	if strings.TrimSpace(path) == "" || path == "." {
		return "."
	}
	return filepath.ToSlash(path)
}

func defaultMapText(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
