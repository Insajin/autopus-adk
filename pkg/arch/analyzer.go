package arch

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

// detectProjectType는 프로젝트 유형을 감지한다.
func detectProjectType(dir string) string {
	if fileExists(filepath.Join(dir, "go.mod")) {
		return "go"
	}
	if fileExists(filepath.Join(dir, "package.json")) {
		return "ts"
	}
	if fileExists(filepath.Join(dir, "setup.py")) ||
		fileExists(filepath.Join(dir, "pyproject.toml")) ||
		fileExists(filepath.Join(dir, "requirements.txt")) {
		return "python"
	}
	return "unknown"
}

// analyzeGo는 Go 프로젝트 구조를 분석한다.
func analyzeGo(dir string) ([]Domain, []Layer, []Dependency) {
	// 레이어 정의 (Go 관례)
	layers := []Layer{
		{Name: "cmd", Level: 3, AllowedDeps: []string{"pkg", "internal"}},
		{Name: "pkg", Level: 2, AllowedDeps: []string{"pkg"}},
		{Name: "internal", Level: 1, AllowedDeps: []string{"pkg"}},
	}

	var domains []Domain
	var dependencies []Dependency

	// 디렉터리 구조 탐색
	knownDirs := []string{"cmd", "pkg", "internal"}
	for _, layerDir := range knownDirs {
		layerPath := filepath.Join(dir, layerDir)
		if _, err := os.Stat(layerPath); err != nil {
			continue
		}
		// 서브 디렉터리를 도메인으로 처리
		entries, err := os.ReadDir(layerPath)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			subPath := filepath.Join(layerPath, e.Name())
			pkgs := collectGoPackages(subPath)
			domains = append(domains, Domain{
				Name:        e.Name(),
				Path:        filepath.Join(layerDir, e.Name()),
				Description: layerDir + " 레이어의 " + e.Name() + " 도메인",
				Packages:    pkgs,
			})
		}
	}

	// go.mod에서 모듈 경로 추출
	modulePath := extractGoModule(dir)

	// import 분석으로 의존성 추출
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		pkg := relativePackage(dir, path)
		imports := parseGoImports(path)
		for _, imp := range imports {
			// 외부 패키지 건너뜀 (표준 라이브러리는 "/" 없음, 모듈 내부 패키지만 추적)
			if modulePath != "" && strings.HasPrefix(imp, modulePath+"/") {
				// 모듈 내부 패키지 — 상대 경로로 변환
				rel := strings.TrimPrefix(imp, modulePath+"/")
				dependencies = append(dependencies, Dependency{
					From: pkg,
					To:   rel,
					Type: "import",
				})
			} else if !strings.Contains(imp, ".") && !strings.HasPrefix(imp, "/") {
				// 표준 라이브러리 패키지 (예: fmt, os, strings) — 건너뜀
				_ = imp
			}
		}
		return nil
	})

	return domains, layers, dependencies
}

// analyzeTS는 TypeScript/JavaScript 프로젝트를 분석한다.
func analyzeTS(dir string) ([]Domain, []Layer, []Dependency) {
	layers := []Layer{
		{Name: "src", Level: 2, AllowedDeps: []string{"src"}},
	}

	var domains []Domain
	var dependencies []Dependency

	srcPath := filepath.Join(dir, "src")
	if entries, err := os.ReadDir(srcPath); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			domains = append(domains, Domain{
				Name:        e.Name(),
				Path:        filepath.Join("src", e.Name()),
				Description: "src/" + e.Name() + " 모듈",
				Packages:    []string{e.Name()},
			})
		}
	}

	// import 분석
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".ts") && !strings.HasSuffix(path, ".tsx") &&
			!strings.HasSuffix(path, ".js") && !strings.HasSuffix(path, ".jsx") {
			return nil
		}

		from := relativePackage(dir, path)
		imports := parseTSImports(path)
		for _, imp := range imports {
			dependencies = append(dependencies, Dependency{
				From: from,
				To:   imp,
				Type: "import",
			})
		}
		return nil
	})

	return domains, layers, dependencies
}

// analyzePython는 Python 프로젝트를 분석한다.
func analyzePython(dir string) ([]Domain, []Layer, []Dependency) {
	layers := []Layer{
		{Name: "app", Level: 2, AllowedDeps: []string{"app"}},
	}

	var domains []Domain
	var dependencies []Dependency

	// __init__.py 가 있는 디렉터리를 패키지로 인식
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || !info.IsDir() {
			return nil
		}
		if fileExists(filepath.Join(path, "__init__.py")) {
			rel, _ := filepath.Rel(dir, path)
			if rel == "." {
				return nil
			}
			domains = append(domains, Domain{
				Name:        filepath.Base(path),
				Path:        rel,
				Description: rel + " 패키지",
				Packages:    []string{rel},
			})
		}
		return nil
	})

	// import 분석
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".py") {
			return nil
		}

		from := relativePackage(dir, path)
		imports := parsePythonImports(path)
		for _, imp := range imports {
			dependencies = append(dependencies, Dependency{
				From: from,
				To:   imp,
				Type: "import",
			})
		}
		return nil
	})

	return domains, layers, dependencies
}

// parseGoImports extracts import paths from a Go file.
func parseGoImports(path string) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer func() { _ = f.Close() }()

	var imports []string
	inImport := false
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "import (" {
			inImport = true
			continue
		}
		if inImport && line == ")" {
			inImport = false
			continue
		}
		if inImport {
			// 따옴표 제거
			imp := strings.Trim(line, `"`)
			imp = strings.TrimSpace(imp)
			if imp != "" && !strings.HasPrefix(imp, "//") {
				imports = append(imports, imp)
			}
		}
		// 단일 import
		if strings.HasPrefix(line, `import "`) {
			imp := strings.TrimPrefix(line, `import "`)
			imp = strings.TrimSuffix(imp, `"`)
			imports = append(imports, imp)
		}
	}
	return imports
}

// parseTSImports extracts import paths from a TypeScript file.
func parseTSImports(path string) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer func() { _ = f.Close() }()

	var imports []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// import { X } from 'y' 또는 import X from 'y'
		if strings.HasPrefix(line, "import ") && strings.Contains(line, "from ") {
			parts := strings.Split(line, "from ")
			if len(parts) >= 2 {
				imp := strings.Trim(strings.TrimSpace(parts[len(parts)-1]), `'";`)
				if imp != "" {
					imports = append(imports, imp)
				}
			}
		}
		// require('x')
		if strings.Contains(line, "require(") {
			start := strings.Index(line, "require(")
			if start >= 0 {
				rest := line[start+len("require("):]
				rest = strings.Trim(rest, `'"`)
				end := strings.IndexAny(rest, `'"`)
				if end > 0 {
					imports = append(imports, rest[:end])
				}
			}
		}
	}
	return imports
}

// parsePythonImports extracts import paths from a Python file.
func parsePythonImports(path string) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer func() { _ = f.Close() }()

	var imports []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "import ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				imports = append(imports, parts[1])
			}
		}
		if strings.HasPrefix(line, "from ") && strings.Contains(line, " import ") {
			parts := strings.SplitN(line, " import ", 2)
			pkg := strings.TrimPrefix(parts[0], "from ")
			pkg = strings.TrimSpace(pkg)
			if pkg != "" {
				imports = append(imports, pkg)
			}
		}
	}
	return imports
}

// collectGoPackages는 디렉터리에서 Go 패키지 목록을 수집한다.
func collectGoPackages(dir string) []string {
	var pkgs []string
	entries, err := os.ReadDir(dir)
	if err != nil {
		return pkgs
	}
	for _, e := range entries {
		if e.IsDir() {
			pkgs = append(pkgs, e.Name())
		}
	}
	return pkgs
}

// relativePackage는 파일의 상대 패키지 경로를 반환한다.
func relativePackage(base, path string) string {
	rel, err := filepath.Rel(base, filepath.Dir(path))
	if err != nil {
		return path
	}
	return rel
}

// fileExists는 파일 존재 여부를 확인한다.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// extractGoModule는 go.mod에서 모듈 경로를 추출한다.
func extractGoModule(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module "))
		}
	}
	return ""
}
