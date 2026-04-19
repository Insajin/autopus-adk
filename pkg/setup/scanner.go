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

	info.Languages = detectLanguages(absDir)
	info.Frameworks = detectFrameworks(absDir)
	info.BuildFiles = detectBuildFiles(absDir)
	info.EntryPoints = detectEntryPoints(absDir, info.Languages)
	info.TestConfig = detectTestConfig(absDir, info.Languages, info.BuildFiles)
	info.Structure = scanDirectoryTree(absDir, 0)
	info.Conventions = AnalyzeConventions(absDir, info.Languages)
	info.Workspaces = DetectWorkspaces(absDir)

	return info, nil
}
