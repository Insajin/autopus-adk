package setup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DetectMultiRepo scans the workspace root and its immediate child directories
// for Git repositories. Deeper recursive discovery remains out of scope here.
func DetectMultiRepo(dir string) *MultiRepoInfo {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil
	}

	components := collectRepoComponents(absDir)
	if len(components) < 2 {
		return nil
	}

	info := &MultiRepoInfo{
		IsMultiRepo:   true,
		WorkspaceRoot: absDir,
		Components:    components,
	}
	info.Dependencies = MapCrossRepoDeps(info.Components)
	return info
}

// ScanRepoComponent inspects a single repository.
func ScanRepoComponent(dir string) (*RepoComponent, error) {
	return scanRepoComponent("", dir)
}

func scanRepoComponent(rootDir, dir string) (*RepoComponent, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}

	path := filepath.Base(absDir)
	if rootDir != "" {
		if rel, relErr := filepath.Rel(rootDir, absDir); relErr == nil {
			path = rel
		}
	}
	path = filepath.ToSlash(path)
	if path == "" || path == "." {
		path = "."
	}

	component := &RepoComponent{
		Name:            filepath.Base(absDir),
		Path:            path,
		AbsPath:         absDir,
		RemoteURL:       readGitRemote(absDir),
		ModulePath:      detectComponentModulePath(absDir),
		PackageName:     detectPackageName(absDir),
		PrimaryLanguage: detectPrimaryLanguage(absDir),
	}
	component.Role = inferRepoRole(*component)
	return component, nil
}

func collectRepoComponents(rootDir string) []RepoComponent {
	var components []RepoComponent
	if isGitRepo(rootDir) {
		if rootComponent, err := scanRepoComponent(rootDir, rootDir); err == nil {
			components = append(components, *rootComponent)
		}
	}

	entries, err := os.ReadDir(rootDir)
	if err != nil {
		return components
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, ".") || isIgnoredDir(name) {
			continue
		}
		componentDir := filepath.Join(rootDir, name)
		if !isGitRepo(componentDir) {
			continue
		}
		component, scanErr := scanRepoComponent(rootDir, componentDir)
		if scanErr != nil {
			continue
		}
		components = append(components, *component)
	}

	sort.Slice(components, func(i, j int) bool {
		return components[i].Path < components[j].Path
	})
	return components
}

func isGitRepo(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil && (info.IsDir() || !info.IsDir())
}

func readGitRemote(dir string) string {
	gitPath := filepath.Join(dir, ".git")
	stat, err := os.Stat(gitPath)
	if err != nil {
		return ""
	}

	configPath := filepath.Join(gitPath, "config")
	if !stat.IsDir() {
		data, readErr := os.ReadFile(gitPath)
		if readErr != nil {
			return ""
		}
		gitDir := strings.TrimSpace(strings.TrimPrefix(string(data), "gitdir:"))
		if gitDir == "" {
			return ""
		}
		if !filepath.IsAbs(gitDir) {
			gitDir = filepath.Join(dir, gitDir)
		}
		configPath = filepath.Join(filepath.Clean(gitDir), "config")
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return ""
	}

	inOrigin := false
	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		switch {
		case strings.HasPrefix(line, "["):
			inOrigin = line == `[remote "origin"]`
		case inOrigin && strings.HasPrefix(line, "url = "):
			return strings.TrimSpace(strings.TrimPrefix(line, "url = "))
		}
	}
	return ""
}

func detectComponentModulePath(dir string) string {
	if data, err := os.ReadFile(filepath.Join(dir, "go.mod")); err == nil {
		for _, raw := range strings.Split(string(data), "\n") {
			line := strings.TrimSpace(raw)
			if strings.HasPrefix(line, "module ") {
				return strings.TrimSpace(strings.TrimPrefix(line, "module "))
			}
		}
	}
	if name := detectPackageName(dir); name != "" {
		return name
	}
	if data, err := os.ReadFile(filepath.Join(dir, "Cargo.toml")); err == nil {
		for _, raw := range strings.Split(string(data), "\n") {
			line := strings.TrimSpace(raw)
			if strings.HasPrefix(line, "name = ") {
				return strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, "name = ")), `"'`)
			}
		}
	}
	return ""
}

func detectPackageName(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return ""
	}
	var pkg struct {
		Name string `json:"name"`
	}
	if jsonErr := json.Unmarshal(data, &pkg); jsonErr != nil {
		return ""
	}
	return pkg.Name
}

func detectPrimaryLanguage(dir string) string {
	languages := detectLanguages(dir)
	if len(languages) > 0 {
		return languages[0].Name
	}
	matches, _ := filepath.Glob(filepath.Join(dir, "*.md"))
	if len(matches) > 0 {
		return "Markdown"
	}
	return "Unknown"
}

func inferRepoRole(component RepoComponent) string {
	lower := strings.ToLower(component.Name + " " + component.Path + " " + component.ModulePath)
	switch {
	case component.Path == ".":
		return "meta workspace"
	case strings.Contains(lower, "desktop"):
		return "desktop shell"
	case strings.Contains(lower, "adk") || strings.Contains(lower, "cli"):
		return "CLI and harness source"
	case strings.Contains(lower, "protocol"):
		return "shared protocol"
	case strings.Contains(lower, "docs"):
		return "documentation"
	case strings.Contains(lower, "web") || strings.Contains(lower, "frontend"):
		return "web application"
	case strings.Contains(lower, "backend") || strings.Contains(lower, "api") || strings.Contains(lower, "server"):
		return "backend service"
	case strings.Contains(lower, "tap"):
		return "distribution"
	case component.PrimaryLanguage != "" && component.PrimaryLanguage != "Unknown":
		return component.PrimaryLanguage + " repository"
	default:
		return "repository"
	}
}
