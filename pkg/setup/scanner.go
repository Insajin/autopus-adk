package setup

import "path/filepath"

const maxDepth = 3

// Scan analyzes a project directory and returns ProjectInfo.
func Scan(projectDir string) (*ProjectInfo, error) {
	absDir, err := filepath.Abs(projectDir)
	if err != nil {
		return nil, err
	}

	info := &ProjectInfo{
		Name:    filepath.Base(absDir),
		RootDir: absDir,
	}

	info.MultiRepo = DetectMultiRepo(absDir)
	if info.MultiRepo != nil {
		populateMultiRepoSignals(info, absDir)
		return info, nil
	}

	populateSingleRepoSignals(info, absDir)

	return info, nil
}

func populateSingleRepoSignals(info *ProjectInfo, dir string) {
	info.Languages = detectLanguages(dir)
	info.Frameworks = detectFrameworks(dir)
	info.BuildFiles = detectBuildFiles(dir)
	info.EntryPoints = detectEntryPoints(dir, info.Languages)
	info.TestConfig = detectTestConfig(dir, info.Languages, info.BuildFiles)
	info.Structure = scanDirectoryTree(dir, 0)
	info.Conventions = AnalyzeConventions(dir, info.Languages)
	info.Workspaces = DetectWorkspaces(dir)
}

func populateMultiRepoSignals(info *ProjectInfo, dir string) {
	info.Structure = scanDirectoryTree(dir, 0)
	info.Workspaces = DetectWorkspaces(dir)
	info.Conventions = make(map[string]ConventionSample)

	for _, component := range info.MultiRepo.Components {
		componentInfo := scanRepoSignals(component.AbsPath)
		info.Languages = mergeLanguages(info.Languages, componentInfo.Languages)
		info.Frameworks = mergeFrameworks(info.Frameworks, componentInfo.Frameworks)
		info.BuildFiles = append(info.BuildFiles, prefixBuildFiles(component.Path, componentInfo.BuildFiles)...)
		info.EntryPoints = append(info.EntryPoints, prefixEntryPoints(component.Path, componentInfo.EntryPoints)...)
		info.Workspaces = append(info.Workspaces, prefixWorkspaces(component.Path, componentInfo.Workspaces)...)
		info.Conventions = mergeConventions(info.Conventions, component.Path, componentInfo.Conventions)
		info.TestConfig = mergeTestConfig(info.TestConfig, component.Path, componentInfo.TestConfig)
	}
}

func scanRepoSignals(dir string) *ProjectInfo {
	languages := detectLanguages(dir)
	buildFiles := detectBuildFiles(dir)
	return &ProjectInfo{
		Languages:   languages,
		Frameworks:  detectFrameworks(dir),
		BuildFiles:  buildFiles,
		EntryPoints: detectEntryPoints(dir, languages),
		TestConfig:  detectTestConfig(dir, languages, buildFiles),
		Conventions: AnalyzeConventions(dir, languages),
		Workspaces:  DetectWorkspaces(dir),
	}
}

func mergeLanguages(dst, src []Language) []Language {
	seen := make(map[string]int, len(dst))
	for i, language := range dst {
		seen[language.Name] = i
	}
	for _, language := range src {
		if index, ok := seen[language.Name]; ok {
			if dst[index].Version == "" {
				dst[index].Version = language.Version
			}
			continue
		}
		seen[language.Name] = len(dst)
		dst = append(dst, language)
	}
	return dst
}

func mergeFrameworks(dst, src []Framework) []Framework {
	seen := make(map[string]bool, len(dst))
	for _, framework := range dst {
		seen[framework.Name] = true
	}
	for _, framework := range src {
		if seen[framework.Name] {
			continue
		}
		seen[framework.Name] = true
		dst = append(dst, framework)
	}
	return dst
}

func prefixBuildFiles(prefix string, buildFiles []BuildFile) []BuildFile {
	if prefix == "." {
		return buildFiles
	}
	result := make([]BuildFile, 0, len(buildFiles))
	for _, buildFile := range buildFiles {
		buildFile.Path = joinRepoPath(prefix, buildFile.Path)
		result = append(result, buildFile)
	}
	return result
}

func prefixEntryPoints(prefix string, entryPoints []EntryPoint) []EntryPoint {
	if prefix == "." {
		return entryPoints
	}
	result := make([]EntryPoint, 0, len(entryPoints))
	for _, entryPoint := range entryPoints {
		entryPoint.Path = joinRepoPath(prefix, entryPoint.Path)
		result = append(result, entryPoint)
	}
	return result
}

func prefixWorkspaces(prefix string, workspaces []Workspace) []Workspace {
	if prefix == "." {
		return workspaces
	}
	result := make([]Workspace, 0, len(workspaces))
	for _, workspace := range workspaces {
		workspace.Path = joinRepoPath(prefix, workspace.Path)
		result = append(result, workspace)
	}
	return result
}

func mergeConventions(dst map[string]ConventionSample, prefix string, src map[string]ConventionSample) map[string]ConventionSample {
	for name, sample := range src {
		if prefix != "." {
			for i, file := range sample.ExampleFiles {
				sample.ExampleFiles[i] = joinRepoPath(prefix, file)
			}
		}
		if existing, ok := dst[name]; ok && len(existing.ExampleFiles) > 0 {
			continue
		}
		dst[name] = sample
	}
	return dst
}

func mergeTestConfig(dst TestConfiguration, prefix string, src TestConfiguration) TestConfiguration {
	if src.Framework == "" {
		return dst
	}
	if dst.Framework == "" {
		dst.Framework = src.Framework
		dst.Command = src.Command
		dst.CoverageOn = src.CoverageOn
	}
	for _, dir := range src.Dirs {
		if prefix != "." {
			dir = joinRepoPath(prefix, dir)
		}
		dst.Dirs = append(dst.Dirs, dir)
	}
	dst.CoverageOn = dst.CoverageOn || src.CoverageOn
	return dst
}

func joinRepoPath(prefix, rel string) string {
	if prefix == "" || prefix == "." {
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(filepath.Join(prefix, rel))
}
