package arch

import (
	"os"
	"path/filepath"
	"strings"
)

// analyzeGo는 Go 프로젝트 구조를 분석한다.
func analyzeGo(dir string) ([]Domain, []Layer, []Dependency) {
	layers := []Layer{
		{Name: "cmd", Level: 3, AllowedDeps: []string{"pkg", "internal"}},
		{Name: "pkg", Level: 2, AllowedDeps: []string{"pkg"}},
		{Name: "internal", Level: 1, AllowedDeps: []string{"pkg"}},
	}

	domains := analyzeGoDomains(dir)
	dependencies := analyzeGoDependencies(dir)
	return domains, layers, dependencies
}

func analyzeGoDomains(dir string) []Domain {
	var domains []Domain

	for _, layerDir := range []string{"cmd", "pkg", "internal"} {
		layerPath := filepath.Join(dir, layerDir)
		if _, err := os.Stat(layerPath); err != nil {
			continue
		}
		entries, err := os.ReadDir(layerPath)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			subPath := filepath.Join(layerPath, entry.Name())
			domains = append(domains, Domain{
				Name:        entry.Name(),
				Path:        filepath.Join(layerDir, entry.Name()),
				Description: layerDir + " 레이어의 " + entry.Name() + " 도메인",
				Packages:    collectGoPackages(subPath),
			})
		}
	}

	return domains
}

func analyzeGoDependencies(dir string) []Dependency {
	var dependencies []Dependency
	modulePath := extractGoModule(dir)

	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		pkg := relativePackage(dir, path)
		for _, imp := range parseGoImports(path) {
			if modulePath != "" && strings.HasPrefix(imp, modulePath+"/") {
				dependencies = append(dependencies, Dependency{
					From: pkg,
					To:   strings.TrimPrefix(imp, modulePath+"/"),
					Type: "import",
				})
			}
		}
		return nil
	})

	return dependencies
}

func collectGoPackages(dir string) []string {
	var pkgs []string
	entries, err := os.ReadDir(dir)
	if err != nil {
		return pkgs
	}
	for _, entry := range entries {
		if entry.IsDir() {
			pkgs = append(pkgs, entry.Name())
		}
	}
	return pkgs
}
