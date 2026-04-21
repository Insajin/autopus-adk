package setup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type goModuleRef struct {
	module  string
	version string
}

type goReplaceRef struct {
	module  string
	version string
	target  string
}

// MapCrossRepoDeps derives repository edges from Go and package manifests.
func MapCrossRepoDeps(components []RepoComponent) []RepoDependency {
	byModule := make(map[string]RepoComponent, len(components))
	byPackage := make(map[string]RepoComponent, len(components))
	for _, component := range components {
		if component.ModulePath != "" {
			byModule[component.ModulePath] = component
		}
		if component.PackageName != "" {
			byPackage[component.PackageName] = component
		}
	}

	seen := make(map[string]bool)
	var deps []RepoDependency
	for _, component := range components {
		for _, dep := range collectGoDependencies(component, components, byModule) {
			key := dep.Source + "|" + dep.Target + "|" + dep.Type
			if !seen[key] {
				seen[key] = true
				deps = append(deps, dep)
			}
		}
		for _, dep := range collectPackageDependencies(component, components, byPackage) {
			key := dep.Source + "|" + dep.Target + "|" + dep.Type
			if !seen[key] {
				seen[key] = true
				deps = append(deps, dep)
			}
		}
	}

	sort.Slice(deps, func(i, j int) bool {
		left := deps[i].Source + deps[i].Target + deps[i].Type + deps[i].Version
		right := deps[j].Source + deps[j].Target + deps[j].Type + deps[j].Version
		return left < right
	})
	return deps
}

func collectGoDependencies(component RepoComponent, components []RepoComponent, byModule map[string]RepoComponent) []RepoDependency {
	goModPath := filepath.Join(component.AbsPath, "go.mod")
	requires, replaces := parseGoModDeps(goModPath)
	var deps []RepoDependency

	for _, req := range requires {
		target, ok := byModule[req.module]
		if ok && target.Name != component.Name {
			deps = append(deps, RepoDependency{Source: component.Name, Target: target.Name, Type: "require", Version: req.version})
		}
	}
	for _, replace := range replaces {
		if target := findComponentByRef(component.AbsPath, replace.target, components); target != "" && target != component.Name {
			deps = append(deps, RepoDependency{Source: component.Name, Target: target, Type: "replace", Version: replace.version})
			continue
		}
		target, ok := byModule[replace.module]
		if ok && target.Name != component.Name {
			deps = append(deps, RepoDependency{Source: component.Name, Target: target.Name, Type: "replace", Version: replace.version})
		}
	}
	return deps
}

func collectPackageDependencies(component RepoComponent, components []RepoComponent, byPackage map[string]RepoComponent) []RepoDependency {
	data, err := os.ReadFile(filepath.Join(component.AbsPath, "package.json"))
	if err != nil {
		return nil
	}
	var pkg struct {
		Dependencies     map[string]string `json:"dependencies"`
		DevDependencies  map[string]string `json:"devDependencies"`
		PeerDependencies map[string]string `json:"peerDependencies"`
	}
	if jsonErr := json.Unmarshal(data, &pkg); jsonErr != nil {
		return nil
	}

	var deps []RepoDependency
	for name, version := range mergeMaps(mergeMaps(pkg.Dependencies, pkg.DevDependencies), pkg.PeerDependencies) {
		if target, ok := byPackage[name]; ok && target.Name != component.Name {
			deps = append(deps, RepoDependency{Source: component.Name, Target: target.Name, Type: "package", Version: version})
		}
		if strings.HasPrefix(version, "file:") {
			ref := strings.TrimSpace(strings.TrimPrefix(version, "file:"))
			if target := findComponentByRef(component.AbsPath, ref, components); target != "" && target != component.Name {
				deps = append(deps, RepoDependency{Source: component.Name, Target: target, Type: "file", Version: version})
			}
		}
	}
	return deps
}

func parseGoModDeps(path string) ([]goModuleRef, []goReplaceRef) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil
	}

	var requires []goModuleRef
	var replaces []goReplaceRef
	inRequireBlock := false
	inReplaceBlock := false
	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(strings.SplitN(raw, "//", 2)[0])
		if line == "" {
			continue
		}
		switch {
		case strings.HasPrefix(line, "require ("):
			inRequireBlock = true
		case strings.HasPrefix(line, "replace ("):
			inReplaceBlock = true
		case line == ")":
			inRequireBlock = false
			inReplaceBlock = false
		case strings.HasPrefix(line, "require "):
			requires = append(requires, parseGoModuleRef(strings.TrimSpace(strings.TrimPrefix(line, "require "))))
		case strings.HasPrefix(line, "replace "):
			if replace, ok := parseGoReplaceRef(strings.TrimSpace(strings.TrimPrefix(line, "replace "))); ok {
				replaces = append(replaces, replace)
			}
		case inRequireBlock:
			requires = append(requires, parseGoModuleRef(line))
		case inReplaceBlock:
			if replace, ok := parseGoReplaceRef(line); ok {
				replaces = append(replaces, replace)
			}
		}
	}
	return requires, replaces
}

func parseGoModuleRef(line string) goModuleRef {
	fields := strings.Fields(line)
	ref := goModuleRef{}
	if len(fields) > 0 {
		ref.module = fields[0]
	}
	if len(fields) > 1 {
		ref.version = fields[1]
	}
	return ref
}

func parseGoReplaceRef(line string) (goReplaceRef, bool) {
	parts := strings.Split(line, "=>")
	if len(parts) != 2 {
		return goReplaceRef{}, false
	}
	left := strings.Fields(strings.TrimSpace(parts[0]))
	right := strings.Fields(strings.TrimSpace(parts[1]))
	if len(left) == 0 || len(right) == 0 {
		return goReplaceRef{}, false
	}

	ref := goReplaceRef{module: left[0], target: right[0]}
	if len(left) > 1 {
		ref.version = left[1]
	}
	return ref, true
}

func findComponentByRef(sourceDir, ref string, components []RepoComponent) string {
	if ref == "" {
		return ""
	}
	candidate := ref
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(sourceDir, candidate)
	}
	candidate = filepath.Clean(candidate)

	bestName := ""
	bestLen := -1
	for _, component := range components {
		rel, err := filepath.Rel(component.AbsPath, candidate)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		if len(component.AbsPath) > bestLen {
			bestName = component.Name
			bestLen = len(component.AbsPath)
		}
	}
	return bestName
}
