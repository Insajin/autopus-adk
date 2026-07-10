package design

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
)

type packageManifest struct {
	Path            string
	Dependencies    map[string]string
	DevDependencies map[string]string
	Scripts         map[string]string
}

type rawPackageManifest struct {
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
	Scripts         map[string]string `json:"scripts"`
}

func collectPackageManifests(root string, maxRefs int) ([]packageManifest, error) {
	var manifests []packageManifest
	err := walkDesignCandidateFiles(root, 0, func(rel, abs string, info os.FileInfo) error {
		if info.IsDir() || filepath.Base(rel) != "package.json" {
			return nil
		}
		manifest, ok, err := tryReadPackageManifest(rel, abs)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		manifests = append(manifests, manifest)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(manifests, func(i, j int) bool {
		return manifests[i].Path < manifests[j].Path
	})
	if len(manifests) > maxRefs {
		manifests = manifests[:maxRefs]
	}
	return manifests, nil
}

func tryReadPackageManifest(rel, abs string) (packageManifest, bool, error) {
	data, err := os.ReadFile(abs)
	if err != nil {
		return packageManifest{}, false, err
	}
	var raw rawPackageManifest
	if err := json.Unmarshal(data, &raw); err != nil {
		return packageManifest{}, false, nil
	}
	return packageManifest{
		Path:            filepath.ToSlash(rel),
		Dependencies:    raw.Dependencies,
		DevDependencies: raw.DevDependencies,
		Scripts:         raw.Scripts,
	}, true, nil
}

func collectLocalDesignRefs(root string, maxRefs int) ([]SourceRef, error) {
	var refs []SourceRef
	err := walkDesignCandidateFiles(root, maxRefs*20, func(rel, _ string, _ os.FileInfo) error {
		switch {
		case isTokenRef(rel):
			refs = appendUniqueLimited(refs, SourceRef{Path: rel, Kind: "token_or_theme"}, maxRefs)
		case isComponentRef(rel):
			refs = appendUniqueLimited(refs, SourceRef{Path: rel, Kind: "component"}, maxRefs)
		}
		return nil
	})
	return refs, err
}

func sortedPackageNames(manifest packageManifest) []string {
	seen := map[string]bool{}
	var names []string
	for name := range manifest.Dependencies {
		if !seen[name] {
			names = append(names, name)
			seen[name] = true
		}
	}
	for name := range manifest.DevDependencies {
		if !seen[name] {
			names = append(names, name)
			seen[name] = true
		}
	}
	sort.Strings(names)
	return names
}

func appendUniqueString(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}
