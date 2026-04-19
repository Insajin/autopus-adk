package arch

import (
	"fmt"
	"os"
)

// Analyze는 프로젝트 디렉터리를 분석하여 ArchitectureMap을 반환한다.
func Analyze(projectDir string) (*ArchitectureMap, error) {
	if _, err := os.Stat(projectDir); err != nil {
		return nil, fmt.Errorf("프로젝트 디렉터리 접근 실패: %w", err)
	}

	projectType := detectProjectType(projectDir)

	var (
		domains      []Domain
		layers       []Layer
		dependencies []Dependency
	)

	switch projectType {
	case "go":
		domains, layers, dependencies = analyzeGo(projectDir)
	case "ts", "js":
		domains, layers, dependencies = analyzeTS(projectDir)
	case "python":
		domains, layers, dependencies = analyzePython(projectDir)
	}

	return &ArchitectureMap{
		Domains:      domains,
		Layers:       layers,
		Dependencies: dependencies,
		Violations:   nil,
	}, nil
}
