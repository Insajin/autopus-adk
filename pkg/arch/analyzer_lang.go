package arch

import (
	"os"
	"path/filepath"
	"strings"
)

// analyzeTS는 TypeScript/JavaScript 프로젝트를 분석한다.
func analyzeTS(dir string) ([]Domain, []Layer, []Dependency) {
	layers := []Layer{{Name: "src", Level: 2, AllowedDeps: []string{"src"}}}

	var domains []Domain
	var dependencies []Dependency

	srcPath := filepath.Join(dir, "src")
	if entries, err := os.ReadDir(srcPath); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			domains = append(domains, Domain{
				Name:        entry.Name(),
				Path:        filepath.Join("src", entry.Name()),
				Description: "src/" + entry.Name() + " 모듈",
				Packages:    []string{entry.Name()},
			})
		}
	}

	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".ts") && !strings.HasSuffix(path, ".tsx") &&
			!strings.HasSuffix(path, ".js") && !strings.HasSuffix(path, ".jsx") {
			return nil
		}

		from := relativePackage(dir, path)
		for _, imp := range parseTSImports(path) {
			dependencies = append(dependencies, Dependency{From: from, To: imp, Type: "import"})
		}
		return nil
	})

	return domains, layers, dependencies
}

// analyzePython는 Python 프로젝트를 분석한다.
func analyzePython(dir string) ([]Domain, []Layer, []Dependency) {
	layers := []Layer{{Name: "app", Level: 2, AllowedDeps: []string{"app"}}}

	var domains []Domain
	var dependencies []Dependency

	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || !info.IsDir() {
			return nil
		}
		if !fileExists(filepath.Join(path, "__init__.py")) {
			return nil
		}
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
		return nil
	})

	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".py") {
			return nil
		}
		from := relativePackage(dir, path)
		for _, imp := range parsePythonImports(path) {
			dependencies = append(dependencies, Dependency{From: from, To: imp, Type: "import"})
		}
		return nil
	})

	return domains, layers, dependencies
}
